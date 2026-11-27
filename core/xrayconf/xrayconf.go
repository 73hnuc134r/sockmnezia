// Package xrayconf builds the Xray config the app runs (plan §4.4, §5.1, §5.3). Every security rule lives
// here, not in the UI: whatever the caller passes, the result has authenticated inbounds on the chosen
// address only, no API/stats/extra inbounds, the locked private-address block first, and a fail-closed
// outbound chain.
package xrayconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/routing"
)

// Mode is the legacy outbound selection (state version 1); the engine migrates it to chains.
type Mode string

const (
	ModeAmnezia    Mode = "amnezia"     // Amnezia only
	ModeAmneziaOwn Mode = "amnezia_own" // Amnezia -> own server
	ModeOwn        Mode = "own"         // own server only
)

// Hop is one element of an outbound chain: exactly one of Amnezia (the gateway's Xray client config, which
// may itself be a relay chain) or Own (a saved own server) is set. Hops are listed from the phone outwards:
// the first one is dialed directly, every later one through the hop before it, the last one is the exit.
type Hop struct {
	Amnezia json.RawMessage
	Own     links.Outbound
}

// AmneziaHop and OwnHop build a Hop.
func AmneziaHop(cfg json.RawMessage) Hop { return Hop{Amnezia: cfg} }
func OwnHop(o links.Outbound) Hop        { return Hop{Own: o} }

// MaxHops limits the chain length: every hop adds latency and a point of failure.
const MaxHops = 5

// Chain errors (wrapped in *Error with Field "chain" or "hop<N>", N counted from 1).
var (
	ErrChainEmpty    = errors.New("the chain has no hops")
	ErrChainTooLong  = fmt.Errorf("a chain has at most %d hops", MaxHops)
	ErrAmneziaTwice  = errors.New("Amnezia can be used only once in a chain")
	ErrNotChainable  = errors.New("this hop cannot be reached through another hop")
	ErrHopIncomplete = errors.New("hop has no server")
)

// Outbound tags in the generated config.
const (
	TagAmneziaPrefix = "amz-"
	TagHopPrefix     = "hop" // own servers: hop1, hop2, … by position (never a name or address)
	TagDirect        = "direct"
	TagBlock         = "block"
	TagSocksIn       = "socks-in"
	TagHTTPIn        = "http-in"
	TagTunIn         = "tun-in" // VPN mode (D31)
	// TunName is the tun interface (Linux) / Wintun adapter (Windows) name.
	TunName = "sockmnezia"
	// TagSysProxyIn is the desktop system-proxy port (D30): HTTP on 127.0.0.1 without authentication, because
	// OS proxy settings cannot carry credentials. Only while the system-proxy mode is on; never on a LAN address.
	TagSysProxyIn = "sysproxy-in"
	TagDNSOut     = "dns-out" // VPN mode with DNS-over-HTTPS: answers DNS queries arriving on the tun
)

// Inbound is the local proxy.
type Inbound struct {
	Listen    string `json:"listen"`     // default 127.0.0.1; a LAN address only when the user shares on LAN
	SocksPort int    `json:"socks_port"` // 1024-65535
	HTTPPort  int    `json:"http_port"`  // 0 = no HTTP inbound
	User      string `json:"user"`
	Pass      string `json:"pass"`
	UDP       bool   `json:"udp"`
}

// VPN is the system-wide mode (Android VpnService, desktop TUN; D31): Xray's own tun inbound in the same
// instance, next to the SOCKS/HTTP inbounds. The tun file descriptor is handed to Xray by the runner.
type VPN struct {
	MTU int
	// DoH, when set, answers DNS queries arriving on the tun (UDP port 53, any address) with Xray's DNS
	// module over these DNS-over-HTTPS URLs (IP literals, so no bootstrap lookup), sent through the chain.
	// Empty: DNS packets are ordinary traffic and go through the chain like everything else.
	DoH []string
	// System, on the desktop (TUN mode, D37), has Xray create the interface, its address and the system
	// routes itself; nil on Android, where the VpnService does that and hands over a file descriptor.
	System *SystemTUN
}

// SystemTUN is the desktop TUN: Xray's tun inbound brings the interface up and routes into it, and binds
// its own outbound sockets to the physical interface so they don't loop back (autoOutboundsInterface).
type SystemTUN struct {
	Gateway []string // interface address(es), e.g. 172.19.0.1/30
	Routes  []string // autoSystemRoutingTable, e.g. 0.0.0.0/0 and ::/0
	// DNS is the resolver the system is pointed at, inside Gateway, answered by the DoH module (Windows: the
	// adapter's DNS; Linux: through systemd-resolved when DNSToGateway).
	DNS          []string
	DNSToGateway bool     // Linux: point systemd-resolved at the tun (autoSystemDnsToGateway)
	WFP          []string // Windows: leak filters (autoSystemWfpBlockLeak), e.g. allexceptlan
	// Mark (Linux) is put on every socket Xray opens (SO_MARK), so the TUN service's firewall rules let
	// exactly Xray's own connections leave outside the tun and drop everything else (D40).
	Mark int
}

// Spec is everything Build needs.
type Spec struct {
	VPN      *VPN  // nil: proxy mode only
	Hops     []Hop // phone first, exit last
	Inbound  Inbound
	Routing  routing.Settings
	LogLevel string // debug, info, warning (default), error, none
	// SystemProxyPort, when set, adds the unauthenticated loopback HTTP inbound for the OS proxy settings
	// (desktop, D30). It always listens on 127.0.0.1, whatever Inbound.Listen is.
	SystemProxyPort int
}

// Error is a config problem the UI can show next to the field.
type Error struct {
	Field string
	Err   error
}

func (e *Error) Error() string { return e.Field + ": " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

func ferr(field, format string, a ...any) error {
	return &Error{Field: field, Err: fmt.Errorf(format, a...)}
}

// ValidateCredential checks a SOCKS5/HTTP username or password: 1-255 bytes (RFC 1929) of printable ASCII
// without spaces, so it survives every client's settings screen and URL form.
func ValidateCredential(s string) error {
	if len(s) == 0 {
		return errors.New("must not be empty")
	}
	if len(s) > 255 {
		return errors.New("at most 255 characters")
	}
	for _, c := range []byte(s) {
		if c < 0x21 || c > 0x7e {
			return errors.New("use printable ASCII characters without spaces")
		}
	}
	return nil
}

// ValidateInbound checks the local proxy settings.
func ValidateInbound(in Inbound) error {
	addr, err := netip.ParseAddr(in.Listen)
	if err != nil {
		return ferr("listen", "not an IP address")
	}
	if addr.IsUnspecified() {
		return ferr("listen", "listening on all interfaces is not allowed; pick one address")
	}
	if !addr.IsLoopback() && !addr.IsPrivate() && !addr.IsLinkLocalUnicast() {
		return ferr("listen", "only loopback or local-network addresses are allowed")
	}
	if in.SocksPort < 1024 || in.SocksPort > 65535 {
		return ferr("socks_port", "use a port between 1024 and 65535")
	}
	if in.HTTPPort != 0 && (in.HTTPPort < 1024 || in.HTTPPort > 65535) {
		return ferr("http_port", "use a port between 1024 and 65535")
	}
	if in.HTTPPort == in.SocksPort {
		return ferr("http_port", "must differ from the SOCKS port")
	}
	if err := ValidateCredential(in.User); err != nil {
		return &Error{Field: "user", Err: err}
	}
	if err := ValidateCredential(in.Pass); err != nil {
		return &Error{Field: "pass", Err: err}
	}
	return nil
}

func clone(v any) any {
	b, _ := json.Marshal(v)
	var out any
	json.Unmarshal(b, &out)
	return out
}

func obj(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	if v == nil {
		v = map[string]any{}
		m[k] = v
	}
	return v
}

// amneziaOutbounds takes the proxy outbounds of the gateway config, retags them with the amz- prefix,
// keeps their dialerProxy chain and returns them with the head tag (the original "proxy").
func amneziaOutbounds(raw json.RawMessage) ([]map[string]any, string, error) {
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, "", ferr("amnezia", "gateway config is not valid JSON")
	}
	var outs []map[string]any
	head := ""
	for _, o := range cfg.Outbounds {
		p, _ := o["protocol"].(string)
		switch p {
		case "vless", "vmess", "trojan", "shadowsocks":
		default:
			continue // dns-out, freedom, … are not needed and must not become reachable
		}
		tag, _ := o["tag"].(string)
		if tag == "" {
			tag = fmt.Sprintf("out%d", len(outs))
		}
		o = clone(o).(map[string]any)
		delete(o, "proxySettings")
		o["tag"] = TagAmneziaPrefix + tag
		if head == "" || tag == "proxy" {
			head = TagAmneziaPrefix + tag
		}
		outs = append(outs, o)
	}
	if len(outs) == 0 {
		return nil, "", ferr("amnezia", "gateway config has no proxy outbound")
	}
	// Rewrite dialerProxy to the new tags; drop references to outbounds that are gone.
	tags := map[string]bool{}
	for _, o := range outs {
		tags[o["tag"].(string)] = true
	}
	next := map[string]string{}
	for _, o := range outs {
		ss, _ := o["streamSettings"].(map[string]any)
		so, _ := ss["sockopt"].(map[string]any)
		if so == nil {
			continue
		}
		if d, _ := so["dialerProxy"].(string); d != "" {
			nt := TagAmneziaPrefix + d
			if tags[nt] {
				so["dialerProxy"] = nt
				next[o["tag"].(string)] = nt
			} else {
				delete(so, "dialerProxy")
			}
		}
	}
	for start := range next { // a cycle would make Xray dial forever
		seen := map[string]bool{}
		for t := start; t != ""; t = next[t] {
			if seen[t] {
				return nil, "", ferr("amnezia", "gateway config has a dialerProxy cycle")
			}
			seen[t] = true
		}
	}
	// Head first (Xray's default outbound), then the rest in their original order.
	sorted := []map[string]any{}
	for _, o := range outs {
		if o["tag"] == head {
			sorted = append(sorted, o)
		}
	}
	for _, o := range outs {
		if o["tag"] != head {
			sorted = append(sorted, o)
		}
	}
	return sorted, head, nil
}

func ownOutbound(o links.Outbound, tag string) (map[string]any, error) {
	if o == nil {
		return nil, ErrHopIncomplete
	}
	out := clone(map[string]any(o)).(map[string]any)
	delete(out, "proxySettings")
	delete(out, "remarks")
	out["tag"] = tag
	ss := obj(out, "streamSettings")
	so := obj(ss, "sockopt")
	delete(so, "dialerProxy")
	if len(so) == 0 {
		delete(ss, "sockopt")
	}
	if len(ss) == 0 {
		delete(out, "streamSettings")
	}
	return out, nil
}

func dialerProxy(o map[string]any) string {
	ss, _ := o["streamSettings"].(map[string]any)
	so, _ := ss["sockopt"].(map[string]any)
	d, _ := so["dialerProxy"].(string)
	return d
}

func setDialerProxy(o map[string]any, via string) {
	so := obj(obj(o, "streamSettings"), "sockopt")
	so["dialerProxy"] = via
}

// tail follows head's dialerProxy chain to the outbound that is dialed first (the one without dialerProxy).
func tail(byTag map[string]map[string]any, head string) map[string]any {
	seen := map[string]bool{}
	o := byTag[head]
	for o != nil && !seen[o["tag"].(string)] {
		seen[o["tag"].(string)] = true
		next := byTag[dialerProxy(o)]
		if next == nil {
			return o
		}
		o = next
	}
	return o
}

func hopErr(i int, err error) error { return &Error{Field: fmt.Sprintf("hop%d", i+1), Err: err} }

// Outbounds returns the outbound list for a chain, head (the exit hop, Xray's default outbound) first, plus
// the head tag. Hop i+1 is dialed through hop i via sockopt.dialerProxy; nothing ever dials around a hop.
func Outbounds(hops []Hop) ([]map[string]any, string, error) {
	switch {
	case len(hops) == 0:
		return nil, "", &Error{Field: "chain", Err: ErrChainEmpty}
	case len(hops) > MaxHops:
		return nil, "", &Error{Field: "chain", Err: ErrChainTooLong}
	}
	var groups [][]map[string]any // per hop, its head first
	prevHead := ""
	amzSeen := false
	for i, h := range hops {
		var outs []map[string]any
		var head string
		switch {
		case h.Amnezia != nil && h.Own != nil:
			return nil, "", hopErr(i, errors.New("hop is both Amnezia and an own server"))
		case h.Amnezia != nil:
			if amzSeen {
				return nil, "", hopErr(i, ErrAmneziaTwice)
			}
			amzSeen = true
			var err error
			if outs, head, err = amneziaOutbounds(h.Amnezia); err != nil {
				var xe *Error
				if errors.As(err, &xe) {
					xe.Field = fmt.Sprintf("hop%d", i+1)
				}
				return nil, "", err
			}
		case h.Own != nil:
			head = fmt.Sprintf("%s%d", TagHopPrefix, i+1)
			o, err := ownOutbound(h.Own, head)
			if err != nil {
				return nil, "", hopErr(i, err)
			}
			outs = []map[string]any{o}
		default:
			return nil, "", hopErr(i, ErrHopIncomplete)
		}
		if prevHead != "" {
			byTag := map[string]map[string]any{}
			for _, o := range outs {
				byTag[o["tag"].(string)] = o
			}
			first := tail(byTag, head)
			if err := links.CheckChainable(links.Outbound(first)); err != nil {
				return nil, "", hopErr(i, fmt.Errorf("%w: %w", ErrNotChainable, err))
			}
			setDialerProxy(first, prevHead)
		}
		groups = append(groups, outs)
		prevHead = head
	}
	// Exit hop first (Xray's default outbound), then the hops before it from the exit inwards.
	var outs []map[string]any
	for i := len(groups) - 1; i >= 0; i-- {
		outs = append(outs, groups[i]...)
	}
	outs = append(outs,
		map[string]any{"tag": TagDirect, "protocol": "freedom"},
		map[string]any{"tag": TagBlock, "protocol": "blackhole"},
	)
	return outs, prevHead, nil
}

// Hops lists the server addresses a connection passes, from the phone outwards (the last one is the exit).
func Hops(hops []Hop) ([]string, error) {
	outs, head, err := Outbounds(hops)
	if err != nil {
		return nil, err
	}
	byTag := map[string]map[string]any{}
	for _, o := range outs {
		byTag[o["tag"].(string)] = o
	}
	var addrs []string
	seen := map[string]bool{}
	for t := head; t != "" && !seen[t]; t = dialerProxy(byTag[t]) {
		seen[t] = true
		o := byTag[t]
		if o == nil {
			break
		}
		if a := outboundAddress(o); a != "" {
			addrs = append([]string{a}, addrs...)
		}
	}
	return addrs, nil
}

func outboundAddress(o map[string]any) string {
	st, _ := o["settings"].(map[string]any)
	if a, _ := st["address"].(string); a != "" {
		return a
	}
	for _, k := range []string{"vnext", "servers"} {
		if l, _ := st[k].([]any); len(l) > 0 {
			if m, _ := l[0].(map[string]any); m != nil {
				if a, _ := m["address"].(string); a != "" {
					return a
				}
			}
		}
	}
	return ""
}

var logLevels = map[string]bool{"debug": true, "info": true, "warning": true, "error": true, "none": true}

// Build returns the full Xray JSON config.
func Build(s Spec) ([]byte, error) {
	if s.Inbound.Listen == "" {
		s.Inbound.Listen = "127.0.0.1"
	}
	if err := ValidateInbound(s.Inbound); err != nil {
		return nil, err
	}
	if s.Routing.DomainStrategy == "" {
		s.Routing.DomainStrategy = routing.AsIs
	}
	outs, head, err := Outbounds(s.Hops)
	if err != nil {
		return nil, err
	}
	rules, err := s.Routing.Compile(head)
	if err != nil {
		return nil, &Error{Field: "routing", Err: err}
	}
	lvl := s.LogLevel
	if !logLevels[lvl] {
		lvl = "warning"
	}
	in := s.Inbound
	account := []any{map[string]any{"user": in.User, "pass": in.Pass}}
	sniffing := map[string]any{"enabled": true, "destOverride": []any{"http", "tls", "quic"}, "routeOnly": true}
	inbounds := []any{map[string]any{
		"tag": TagSocksIn, "listen": in.Listen, "port": in.SocksPort, "protocol": "socks",
		"settings": map[string]any{"auth": "password", "accounts": account, "udp": in.UDP, "ip": in.Listen},
		"sniffing": sniffing,
	}}
	var dnsCfg map[string]any
	if v := s.VPN; v != nil {
		mtu := v.MTU
		if mtu == 0 {
			mtu = 1500
		}
		if mtu < 1280 || mtu > 9000 {
			return nil, ferr("vpn", "MTU %d out of range (1280-9000)", mtu)
		}
		tun := map[string]any{"name": TunName, "mtu": mtu}
		if sys := v.System; sys != nil {
			if len(sys.Gateway) == 0 || len(sys.Routes) == 0 {
				return nil, ferr("vpn", "system TUN needs an address and routes")
			}
			if len(v.DoH) == 0 && (sys.DNSToGateway || len(sys.DNS) > 0) {
				return nil, ferr("vpn", "the system resolver can only point at the TUN with DNS-over-HTTPS")
			}
			tun["gateway"] = sys.Gateway
			tun["autoSystemRoutingTable"] = sys.Routes
			tun["autoOutboundsInterface"] = "auto"
			if len(sys.DNS) > 0 {
				tun["dns"] = sys.DNS
			}
			if sys.DNSToGateway {
				tun["autoSystemDnsToGateway"] = true
			}
			if len(sys.WFP) > 0 {
				tun["autoSystemWfpBlockLeak"] = sys.WFP
			}
			if sys.Mark != 0 {
				for _, o := range outs {
					obj(obj(o, "streamSettings"), "sockopt")["mark"] = sys.Mark
				}
			}
		}
		inbounds = append(inbounds, map[string]any{
			"tag": TagTunIn, "protocol": "tun",
			"settings": tun,
			"sniffing": sniffing,
		})
		if len(v.DoH) > 0 {
			var servers []any
			for _, u := range v.DoH {
				pu, err := url.Parse(u)
				if err != nil || pu.Scheme != "https" {
					return nil, ferr("vpn", "DNS-over-HTTPS server %q must be an https:// URL", u)
				}
				if _, err := netip.ParseAddr(pu.Hostname()); err != nil {
					return nil, ferr("vpn", "DNS-over-HTTPS server %q must use an IP address", u)
				}
				servers = append(servers, u)
			}
			dnsCfg = map[string]any{"servers": servers, "queryStrategy": "UseIP"}
			outs = append(outs, map[string]any{"tag": TagDNSOut, "protocol": "dns"})
			// Right after the locked rules: DNS from the tun goes to Xray's DNS module, which resolves over DoH
			// through the chain (the module's own requests use the default outbound, the chain head).
			n := len(routing.Locked())
			dnsRule := map[string]any{"type": "field", "inboundTag": []any{TagTunIn}, "network": "udp", "port": "53", "outboundTag": TagDNSOut}
			rules = append(rules[:n], append([]map[string]any{dnsRule}, rules[n:]...)...)
		}
	}
	if in.HTTPPort != 0 {
		inbounds = append(inbounds, map[string]any{
			"tag": TagHTTPIn, "listen": in.Listen, "port": in.HTTPPort, "protocol": "http",
			"settings": map[string]any{"accounts": account, "allowTransparent": false},
			"sniffing": sniffing,
		})
	}
	if p := s.SystemProxyPort; p != 0 {
		if p < 1024 || p > 65535 {
			return nil, ferr("system_proxy_port", "port %d out of range (1024-65535)", p)
		}
		if p == in.SocksPort || p == in.HTTPPort {
			return nil, ferr("system_proxy_port", "port %d is already used by the proxy", p)
		}
		inbounds = append(inbounds, map[string]any{
			"tag": TagSysProxyIn, "listen": "127.0.0.1", "port": p, "protocol": "http",
			"settings": map[string]any{"allowTransparent": false},
			"sniffing": sniffing,
		})
	}
	cfg := map[string]any{
		"log":      map[string]any{"loglevel": lvl, "access": "none", "dnsLog": false},
		"inbounds": inbounds,
		"outbounds": func() []any {
			l := make([]any, len(outs))
			for i, o := range outs {
				l[i] = o
			}
			return l
		}(),
		"routing": map[string]any{"domainStrategy": s.Routing.DomainStrategy, "rules": rules},
		"policy": map[string]any{
			"levels": map[string]any{"0": map[string]any{"handshake": 4, "connIdle": 300, "uplinkOnly": 1, "downlinkOnly": 1,
				"statsUserUplink": false, "statsUserDownlink": false}},
			"system": map[string]any{"statsInboundUplink": true, "statsInboundDownlink": true},
		},
		// In-process counters only; there is no API inbound to read them from outside.
		"stats": map[string]any{},
	}
	if dnsCfg != nil {
		cfg["dns"] = dnsCfg
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := Audit(b); err != nil {
		return nil, fmt.Errorf("internal: generated config failed the audit: %w", err)
	}
	return b, nil
}

var dialerSeq atomic.Uint64

// BuildDialer returns a config with no inbounds whose only route is the given chain; used for in-process
// requests (connection check, location ping, gateway fetch through earlier hops) with no listening port.
// Its tags get a unique prefix ("t<N>-"): Xray resolves dialerProxy through one process-wide registry of
// live instances (see runner), so a temporary instance must not reuse the running proxy's tags.
func BuildDialer(hops []Hop) ([]byte, error) {
	outs, head, err := Outbounds(hops)
	if err != nil {
		return nil, err
	}
	ns := fmt.Sprintf("t%d-", dialerSeq.Add(1))
	for _, o := range outs {
		o["tag"] = ns + o["tag"].(string)
		if d := dialerProxy(o); d != "" {
			setDialerProxy(o, ns+d)
		}
	}
	head = ns + head
	rules, _ := routing.Default().Compile(head)
	for _, r := range rules {
		if t, _ := r["outboundTag"].(string); t != "" && !strings.HasPrefix(t, ns) {
			r["outboundTag"] = ns + t
		}
	}
	l := make([]any, len(outs))
	for i, o := range outs {
		l[i] = o
	}
	return json.Marshal(map[string]any{
		"log":       map[string]any{"loglevel": "warning", "access": "none"},
		"outbounds": l,
		"routing":   map[string]any{"domainStrategy": routing.AsIs, "rules": rules},
	})
}

// Audit re-checks a generated config against the security policy (defence in depth, also used in tests).
func Audit(cfg []byte) error {
	var c map[string]any
	if err := json.Unmarshal(cfg, &c); err != nil {
		return err
	}
	ins, _ := c["inbounds"].([]any)
	tuns := 0
	for _, x := range ins {
		if in, _ := x.(map[string]any); in["protocol"] == "tun" {
			tuns++
			if in["tag"] != TagTunIn || in["listen"] != nil || in["port"] != nil {
				return errors.New("unexpected tun inbound")
			}
			st, _ := in["settings"].(map[string]any)
			if st["name"] != TunName {
				return errors.New("unexpected tun interface name")
			}
			for k := range st {
				switch k {
				case "name", "mtu", "gateway", "dns", "autoSystemRoutingTable", "autoOutboundsInterface",
					"autoSystemDnsToGateway", "autoSystemWfpBlockLeak":
				default:
					return fmt.Errorf("tun setting %q is not allowed", k)
				}
			}
			if o, ok := st["autoOutboundsInterface"]; ok && o != "auto" {
				return errors.New("tun outbounds must be bound to the default interface")
			}
		}
	}
	vpn := tuns == 1
	if tuns > 1 {
		return errors.New("more than one tun inbound")
	}
	for k, v := range c {
		switch k {
		case "log":
			// Never a file: the desktop TUN helper runs configs with administrator rights (D37).
			l, _ := v.(map[string]any)
			for lk, lv := range l {
				switch lk {
				case "loglevel", "dnsLog":
				case "access", "error":
					if lv != "none" && lv != "" {
						return fmt.Errorf("log %s file is not allowed", lk)
					}
				default:
					return fmt.Errorf("log setting %q is not allowed", lk)
				}
			}
		case "inbounds", "outbounds", "routing", "policy", "stats":
		case "dns": // only in VPN mode, and only DNS-over-HTTPS to IP addresses (never the local resolver)
			if !vpn {
				return errors.New(`top-level "dns" outside VPN mode`)
			}
			d, _ := v.(map[string]any)
			srv, _ := d["servers"].([]any)
			if len(srv) == 0 {
				return errors.New("dns without servers would use the local resolver")
			}
			for _, x := range srv {
				u, _ := x.(string)
				pu, err := url.Parse(u)
				if err != nil || pu.Scheme != "https" {
					return fmt.Errorf("dns server %v is not DNS-over-HTTPS", x)
				}
				if _, err := netip.ParseAddr(pu.Hostname()); err != nil {
					return fmt.Errorf("dns server %v needs a bootstrap lookup", x)
				}
			}
		default:
			return fmt.Errorf("top-level %q is not allowed", k)
		}
	}
	sysProxies := 0
	for _, x := range ins {
		in, _ := x.(map[string]any)
		if in["tag"] != TagSysProxyIn {
			continue
		}
		// The one inbound allowed without authentication: HTTP, IPv4 loopback only (D30).
		sysProxies++
		if in["protocol"] != "http" || in["listen"] != "127.0.0.1" {
			return errors.New("system-proxy inbound must be HTTP on 127.0.0.1")
		}
	}
	if sysProxies > 1 {
		return errors.New("more than one system-proxy inbound")
	}
	if n := len(ins) - tuns - sysProxies; n < 1 || n > 2 {
		return errors.New("expected 1-2 proxy inbounds")
	}
	for _, x := range ins {
		in, _ := x.(map[string]any)
		p, _ := in["protocol"].(string)
		if p == "tun" || in["tag"] == TagSysProxyIn {
			continue
		}
		if p != "socks" && p != "http" {
			return fmt.Errorf("inbound protocol %q is not allowed", p)
		}
		l, _ := in["listen"].(string)
		a, err := netip.ParseAddr(l)
		if err != nil || a.IsUnspecified() {
			return fmt.Errorf("inbound listen %q is not allowed", l)
		}
		st, _ := in["settings"].(map[string]any)
		acc, _ := st["accounts"].([]any)
		if len(acc) == 0 || (p == "socks" && st["auth"] != "password") {
			return errors.New("inbound without authentication")
		}
	}
	rt, _ := c["routing"].(map[string]any)
	rules, _ := rt["rules"].([]any)
	if len(rules) < 2 {
		return errors.New("locked rules missing")
	}
	for i, want := range routing.Locked() {
		r, _ := rules[i].(map[string]any)
		if r["outboundTag"] != TagBlock || r["ruleTag"] != want.ID {
			return errors.New("locked rules are not first")
		}
	}
	outs, _ := c["outbounds"].([]any)
	if len(outs) == 0 {
		return errors.New("no outbounds")
	}
	first, _ := outs[0].(map[string]any)
	if p := first["protocol"]; p == "freedom" || p == "blackhole" {
		return errors.New("default outbound must be the proxy chain (fail-closed)")
	}
	for _, x := range outs {
		o, _ := x.(map[string]any)
		switch o["protocol"] {
		case "vless", "vmess", "trojan", "shadowsocks", "socks", "http", "freedom", "blackhole":
		case "dns":
			if !vpn || o["tag"] != TagDNSOut || c["dns"] == nil {
				return errors.New("dns outbound outside VPN mode with DNS-over-HTTPS")
			}
		default:
			return fmt.Errorf("outbound protocol %v is not allowed", o["protocol"])
		}
		if o["protocol"] == "freedom" && o["tag"] != TagDirect {
			return errors.New("unexpected freedom outbound")
		}
	}
	return nil
}

// HasTun reports whether cfg has the tun inbound (the desktop TUN helper runs nothing else, D37).
func HasTun(cfg []byte) bool {
	var c struct {
		Inbounds []struct {
			Protocol string `json:"protocol"`
		} `json:"inbounds"`
	}
	if json.Unmarshal(cfg, &c) != nil {
		return false
	}
	for _, in := range c.Inbounds {
		if in.Protocol == "tun" {
			return true
		}
	}
	return false
}

// BlockAll returns cfg with every connection blocked (after the locked rules, which block too): what the
// desktop TUN service runs while TUN mode is chosen but the proxy is stopped, so the tun and its routes stay
// and nothing leaves around them (fail-closed, D40). Inbounds and outbounds stay, so it passes the audit.
func BlockAll(cfg []byte) ([]byte, error) {
	var c map[string]any
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	rt, _ := c["routing"].(map[string]any)
	if rt == nil {
		return nil, errors.New("no routing")
	}
	rules, _ := rt["rules"].([]any)
	n := len(routing.Locked())
	if len(rules) < n {
		return nil, errors.New("locked rules missing")
	}
	rt["rules"] = append(append([]any{}, rules[:n]...), map[string]any{"type": "field", "network": "tcp,udp", "outboundTag": TagBlock})
	delete(c, "dns")
	var outs []any
	for _, x := range c["outbounds"].([]any) {
		if o, _ := x.(map[string]any); o["tag"] != TagDNSOut {
			outs = append(outs, x)
		}
	}
	c["outbounds"] = outs
	return json.Marshal(c)
}
