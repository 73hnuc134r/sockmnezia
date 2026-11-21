// Package routing is the user's rule list (plan §5.3). The JSON shape of a rule matches v2rayNG's
// RulesetItem, so rule lists can be copied between the apps.
package routing

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/xtls/xray-core/common/geodata"
)

// Outbound targets a rule can name.
const (
	OutProxy  = "proxy"  // the chain head (own server or Amnezia)
	OutDirect = "direct" // leaves from the phone's own network
	OutBlock  = "block"
)

// Domain strategies (Xray routing.domainStrategy).
const (
	AsIs         = "AsIs"
	IPIfNonMatch = "IPIfNonMatch"
	IPOnDemand   = "IPOnDemand"
)

// Rule is one routing rule.
type Rule struct {
	ID          string   `json:"id,omitempty"`
	Remarks     string   `json:"remarks,omitempty"`
	Domain      []string `json:"domain,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Port        string   `json:"port,omitempty"`
	Network     string   `json:"network,omitempty"`
	Protocol    []string `json:"protocol,omitempty"`
	OutboundTag string   `json:"outboundTag"`
	Enabled     *bool    `json:"enabled,omitempty"` // nil = enabled (v2rayNG default)
	Locked      bool     `json:"locked,omitempty"`
}

// IsEnabled treats a missing flag as enabled.
func (r Rule) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }

// Settings is the whole routing configuration.
type Settings struct {
	DomainStrategy string `json:"domainStrategy"`
	Rules          []Rule `json:"rules"`
}

// Default is "everything through the proxy".
func Default() Settings { return Settings{DomainStrategy: AsIs} }

// PrivateCIDRs are never reachable through the proxy (plan §4.4, locked rule).
var PrivateCIDRs = []string{
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
	"192.168.0.0/16", "224.0.0.0/4", "240.0.0.0/4", "255.255.255.255/32",
	"::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8",
}

// PrivateDomains are local names, blocked for the same reason.
var PrivateDomains = []string{"full:localhost", "domain:localhost", "domain:local", "domain:localdomain", "domain:home.arpa"}

// Locked returns the security rules that always come first and cannot be edited.
func Locked() []Rule {
	return []Rule{
		{ID: "locked-private-ip", Remarks: "Block private and loopback addresses", IP: PrivateCIDRs, OutboundTag: OutBlock, Locked: true},
		{ID: "locked-private-domain", Remarks: "Block local names", Domain: PrivateDomains, OutboundTag: OutBlock, Locked: true},
	}
}

// ParseJSON accepts either a Settings object or a bare rule array (v2rayNG export).
func ParseJSON(b []byte) (Settings, error) {
	s := Default()
	trim := strings.TrimSpace(string(b))
	if strings.HasPrefix(trim, "[") {
		if err := json.Unmarshal(b, &s.Rules); err != nil {
			return s, err
		}
	} else if err := json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	if s.DomainStrategy == "" {
		s.DomainStrategy = AsIs
	}
	// Locked rules from an import are ours to add; drop copies.
	out := s.Rules[:0]
	for _, r := range s.Rules {
		if !r.Locked {
			out = append(out, r)
		}
	}
	s.Rules = out
	return s, s.Validate()
}

// RuleError points at a rule and field.
type RuleError struct {
	Index int
	Field string
	Err   error
}

func (e *RuleError) Error() string {
	return fmt.Sprintf("rule %d, %s: %v", e.Index+1, e.Field, e.Err)
}
func (e *RuleError) Unwrap() error { return e.Err }

// ErrGeoMissing is returned when a rule names geosite:/geoip: tags and the geo files are absent.
var ErrGeoMissing = errors.New("geo files are not downloaded")

// Validate checks syntax (not geo file contents).
func (s Settings) Validate() error {
	switch s.DomainStrategy {
	case AsIs, IPIfNonMatch, IPOnDemand:
	default:
		return fmt.Errorf("unknown domain strategy %q", s.DomainStrategy)
	}
	for i, r := range s.Rules {
		if err := r.validate(); err != nil {
			err.Index = i
			return err
		}
	}
	return nil
}

func (r Rule) validate() *RuleError {
	switch r.OutboundTag {
	case OutProxy, OutDirect, OutBlock:
	default:
		return &RuleError{Field: "outboundTag", Err: fmt.Errorf("must be proxy, direct or block, not %q", r.OutboundTag)}
	}
	if len(r.Domain) == 0 && len(r.IP) == 0 && r.Port == "" && r.Network == "" && len(r.Protocol) == 0 {
		return &RuleError{Field: "rule", Err: errors.New("matches nothing: set domain, ip, port, network or protocol")}
	}
	for _, d := range r.Domain {
		if err := checkDomain(d); err != nil {
			return &RuleError{Field: "domain", Err: fmt.Errorf("%q: %w", d, err)}
		}
	}
	for _, ip := range r.IP {
		if err := checkIP(ip); err != nil {
			return &RuleError{Field: "ip", Err: fmt.Errorf("%q: %w", ip, err)}
		}
	}
	if r.Port != "" {
		if _, err := ParsePorts(r.Port); err != nil {
			return &RuleError{Field: "port", Err: err}
		}
	}
	for _, n := range strings.Split(r.Network, ",") {
		switch strings.TrimSpace(n) {
		case "", "tcp", "udp":
		default:
			return &RuleError{Field: "network", Err: fmt.Errorf("%q: use tcp, udp or tcp,udp", n)}
		}
	}
	for _, p := range r.Protocol {
		switch p {
		case "http", "tls", "quic", "bittorrent":
		default:
			return &RuleError{Field: "protocol", Err: fmt.Errorf("%q: use http, tls, quic or bittorrent", p)}
		}
	}
	return nil
}

func checkDomain(d string) error {
	if strings.TrimSpace(d) == "" {
		return errors.New("empty")
	}
	if strings.HasPrefix(d, "ext:") || strings.HasPrefix(d, "ext-domain:") {
		return errors.New("ext: files are not supported; use geosite")
	}
	if tag, ok := strings.CutPrefix(strings.TrimLeft(d, "!"), "geosite:"); ok {
		if tag == "" {
			return errors.New("empty geosite tag")
		}
		return nil // checked against the file when the rules are compiled
	}
	if re, ok := strings.CutPrefix(d, "regexp:"); ok {
		if _, err := regexp.Compile(re); err != nil {
			return err
		}
	}
	_, err := geodata.ParseDomainRule(d, geodata.Domain_Substr)
	return err
}

func checkIP(ip string) error {
	ip = strings.TrimLeft(ip, "!")
	if strings.HasPrefix(ip, "geoip:") {
		if len(ip) == len("geoip:") {
			return errors.New("empty geoip tag")
		}
		return nil
	}
	if strings.HasPrefix(ip, "ext:") || strings.HasPrefix(ip, "ext-ip:") {
		return errors.New("ext: files are not supported; use geoip")
	}
	if strings.Contains(ip, "/") {
		_, err := netip.ParsePrefix(ip)
		return err
	}
	_, err := netip.ParseAddr(ip)
	return err
}

// PortRange is inclusive.
type PortRange struct{ From, To int }

// ParsePorts parses "80", "80,443", "1000-2000".
func ParsePorts(s string) ([]PortRange, error) {
	var out []PortRange
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		a, b, isRange := strings.Cut(p, "-")
		from, err1 := strconv.Atoi(strings.TrimSpace(a))
		to := from
		var err2 error
		if isRange {
			to, err2 = strconv.Atoi(strings.TrimSpace(b))
		}
		if err1 != nil || err2 != nil || from < 1 || to > 65535 || from > to {
			return nil, fmt.Errorf("bad port %q (use 80, 80,443 or 1000-2000)", p)
		}
		out = append(out, PortRange{from, to})
	}
	if len(out) == 0 {
		return nil, errors.New("empty port list")
	}
	return out, nil
}

// UsesGeo reports whether any enabled rule needs geosite.dat / geoip.dat.
func (s Settings) UsesGeo() (site, ip bool) {
	for _, r := range s.Rules {
		if !r.IsEnabled() {
			continue
		}
		for _, d := range r.Domain {
			if strings.HasPrefix(strings.TrimLeft(d, "!"), "geosite:") {
				site = true
			}
		}
		for _, i := range r.IP {
			if strings.HasPrefix(strings.TrimLeft(i, "!"), "geoip:") {
				ip = true
			}
		}
	}
	return
}

// Compile returns Xray routing.rules for the locked rules followed by the enabled user rules.
// proxyTag is the outbound tag the "proxy" target maps to.
func (s Settings) Compile(proxyTag string) ([]map[string]any, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, r := range append(Locked(), s.Rules...) {
		if !r.IsEnabled() {
			continue
		}
		tag := r.OutboundTag
		if tag == OutProxy {
			tag = proxyTag
		}
		m := map[string]any{"outboundTag": tag}
		if r.ID != "" {
			m["ruleTag"] = r.ID
		}
		if len(r.Domain) > 0 {
			m["domain"] = r.Domain
		}
		if len(r.IP) > 0 {
			m["ip"] = r.IP
		}
		if r.Port != "" {
			m["port"] = r.Port
		}
		if r.Network != "" {
			m["network"] = r.Network
		}
		if len(r.Protocol) > 0 {
			m["protocol"] = r.Protocol
		}
		out = append(out, m)
	}
	return out, nil
}
