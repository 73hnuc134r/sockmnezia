package routing

import (
	"fmt"
	"net"
	"strings"

	"github.com/xtls/xray-core/common/geodata"
)

// Verdict is the answer of "where does X go?".
type Verdict struct {
	RuleIndex   int    `json:"rule_index"` // index in Locked()+Rules (enabled or not); -1 = no rule, default outbound
	RuleID      string `json:"rule_id,omitempty"`
	Remarks     string `json:"remarks,omitempty"`
	Locked      bool   `json:"locked"`
	OutboundTag string `json:"outbound"` // proxy, direct or block
}

type compiled struct {
	rule   Rule
	index  int
	domain geodata.DomainMatcher
	ip     geodata.IPMatcher
	ports  []PortRange
	nets   map[string]bool
}

// Tester answers which rule a destination matches, using Xray's own matchers (so geosite/geoip tags behave
// exactly like in the proxy). Geo files are read from the Xray asset directory (geo.SetAssetDir).
type Tester struct {
	rules []compiled
}

// NewTester compiles the locked rules and the enabled user rules.
func NewTester(s Settings) (*Tester, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	t := &Tester{}
	for i, r := range append(Locked(), s.Rules...) {
		if !r.IsEnabled() {
			continue
		}
		c := compiled{rule: r, index: i}
		if len(r.Domain) > 0 {
			dr, err := geodata.ParseDomainRules(r.Domain, geodata.Domain_Substr)
			if err == nil {
				c.domain, err = geodata.DomainReg.BuildDomainMatcher(dr)
			}
			if err != nil {
				return nil, wrapGeo(i, "domain", err)
			}
		}
		if len(r.IP) > 0 {
			ir, err := geodata.ParseIPRules(r.IP)
			if err == nil {
				c.ip, err = geodata.IPReg.BuildIPMatcher(ir)
			}
			if err != nil {
				return nil, wrapGeo(i, "ip", err)
			}
		}
		if r.Port != "" {
			c.ports, _ = ParsePorts(r.Port)
		}
		if r.Network != "" {
			c.nets = map[string]bool{}
			for _, n := range strings.Split(r.Network, ",") {
				c.nets[strings.TrimSpace(n)] = true
			}
		}
		t.rules = append(t.rules, c)
	}
	return t, nil
}

func wrapGeo(i int, field string, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "no such file") || strings.Contains(msg, "not found") || strings.Contains(msg, "failed to open") {
		err = fmt.Errorf("%w (%v)", ErrGeoMissing, err)
	}
	return &RuleError{Index: i, Field: field, Err: err}
}

// Match returns where host:port over network ("tcp" or "udp") goes. host is a domain or an IP literal.
// Protocol (sniffing) conditions never match here: they depend on the traffic itself.
func (t *Tester) Match(host string, port int, network string) Verdict {
	host = strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	ip := net.ParseIP(host)
	for _, c := range t.rules {
		r := c.rule
		if len(r.Protocol) > 0 {
			continue
		}
		if c.domain != nil && (ip != nil || !c.domain.MatchAny(host)) {
			continue
		}
		if c.ip != nil && (ip == nil || !c.ip.Match(ip)) {
			continue
		}
		if c.ports != nil && !inPorts(c.ports, port) {
			continue
		}
		if c.nets != nil && !c.nets[network] {
			continue
		}
		return Verdict{RuleIndex: c.index, RuleID: r.ID, Remarks: r.Remarks, Locked: r.Locked, OutboundTag: r.OutboundTag}
	}
	return Verdict{RuleIndex: -1, OutboundTag: OutProxy}
}

func inPorts(ps []PortRange, p int) bool {
	for _, r := range ps {
		if p >= r.From && p <= r.To {
			return true
		}
	}
	return false
}
