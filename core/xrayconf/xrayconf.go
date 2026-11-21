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

	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/routing"
)

// Mode selects the outbound chain.
type Mode string

const (
	ModeAmnezia    Mode = "amnezia"     // local proxy -> Amnezia chain
	ModeAmneziaOwn Mode = "amnezia_own" // local proxy -> own server, dialed through the Amnezia chain
	ModeOwn        Mode = "own"         // local proxy -> own server (no Amnezia key needed)
)

// Outbound tags in the generated config.
const (
	TagAmneziaPrefix = "amz-"
	TagOwn           = "own"
	TagDirect        = "direct"
	TagBlock         = "block"
	TagSocksIn       = "socks-in"
	TagHTTPIn        = "http-in"
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

// Spec is everything Build needs.
type Spec struct {
	Mode     Mode
	Amnezia  json.RawMessage // the gateway's Xray client config (ModeAmnezia, ModeAmneziaOwn)
	Own      links.Outbound  // the own server (ModeAmneziaOwn, ModeOwn)
	Inbound  Inbound
	Routing  routing.Settings
	LogLevel string // debug, info, warning (default), error, none
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

func ownOutbound(o links.Outbound, via string) (map[string]any, error) {
	if o == nil {
		return nil, ferr("own", "no own server selected")
	}
	if err := links.CheckChainable(o); err != nil && via != "" {
		return nil, &Error{Field: "own", Err: err}
	}
	out := clone(map[string]any(o)).(map[string]any)
	delete(out, "proxySettings")
	delete(out, "remarks")
	out["tag"] = TagOwn
	ss := obj(out, "streamSettings")
	so := obj(ss, "sockopt")
	delete(so, "dialerProxy")
	if via != "" {
		so["dialerProxy"] = via
	}
	if len(so) == 0 {
		delete(ss, "sockopt")
	}
	if len(ss) == 0 {
		delete(out, "streamSettings")
	}
	return out, nil
}

// Outbounds returns the outbound list for a mode, head (default outbound) first, plus the head tag.
func Outbounds(mode Mode, amnezia json.RawMessage, own links.Outbound) ([]map[string]any, string, error) {
	var outs []map[string]any
	head := ""
	switch mode {
	case ModeAmnezia, ModeAmneziaOwn:
		amz, h, err := amneziaOutbounds(amnezia)
		if err != nil {
			return nil, "", err
		}
		if mode == ModeAmneziaOwn {
			o, err := ownOutbound(own, h)
			if err != nil {
				return nil, "", err
			}
			outs, head = append([]map[string]any{o}, amz...), TagOwn
		} else {
			outs, head = amz, h
		}
	case ModeOwn:
		o, err := ownOutbound(own, "")
		if err != nil {
			return nil, "", err
		}
		outs, head = []map[string]any{o}, TagOwn
	default:
		return nil, "", ferr("mode", "unknown mode %q", mode)
	}
	outs = append(outs,
		map[string]any{"tag": TagDirect, "protocol": "freedom"},
		map[string]any{"tag": TagBlock, "protocol": "blackhole"},
	)
	return outs, head, nil
}

// Hops lists the server addresses a connection passes, from the phone outwards (the last one is the exit).
func Hops(mode Mode, amnezia json.RawMessage, own links.Outbound) ([]string, error) {
	outs, head, err := Outbounds(mode, amnezia, own)
	if err != nil {
		return nil, err
	}
	byTag := map[string]map[string]any{}
	for _, o := range outs {
		byTag[o["tag"].(string)] = o
	}
	var hops []string
	seen := map[string]bool{}
	for t := head; t != "" && !seen[t]; {
		seen[t] = true
		o := byTag[t]
		if o == nil {
			break
		}
		if a := outboundAddress(o); a != "" {
			hops = append([]string{a}, hops...)
		}
		ss, _ := o["streamSettings"].(map[string]any)
		so, _ := ss["sockopt"].(map[string]any)
		t, _ = so["dialerProxy"].(string)
	}
	return hops, nil
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
	outs, head, err := Outbounds(s.Mode, s.Amnezia, s.Own)
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
	if in.HTTPPort != 0 {
		inbounds = append(inbounds, map[string]any{
			"tag": TagHTTPIn, "listen": in.Listen, "port": in.HTTPPort, "protocol": "http",
			"settings": map[string]any{"accounts": account, "allowTransparent": false},
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
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := Audit(b); err != nil {
		return nil, fmt.Errorf("internal: generated config failed the audit: %w", err)
	}
	return b, nil
}

// BuildDialer returns a config with no inbounds whose only route is the given outbound chain; used to send
// the gateway request through the own server in-process (no listening port).
func BuildDialer(mode Mode, amnezia json.RawMessage, own links.Outbound) ([]byte, error) {
	outs, head, err := Outbounds(mode, amnezia, own)
	if err != nil {
		return nil, err
	}
	rules, _ := routing.Default().Compile(head)
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
	for k := range c {
		switch k {
		case "log", "inbounds", "outbounds", "routing", "policy", "stats":
		default:
			return fmt.Errorf("top-level %q is not allowed", k)
		}
	}
	ins, _ := c["inbounds"].([]any)
	if len(ins) == 0 || len(ins) > 2 {
		return errors.New("expected 1-2 inbounds")
	}
	for _, x := range ins {
		in, _ := x.(map[string]any)
		p, _ := in["protocol"].(string)
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
		default:
			return fmt.Errorf("outbound protocol %v is not allowed", o["protocol"])
		}
		if o["protocol"] == "freedom" && o["tag"] != TagDirect {
			return errors.New("unexpected freedom outbound")
		}
	}
	return nil
}
