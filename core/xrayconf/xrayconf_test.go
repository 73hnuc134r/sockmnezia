package xrayconf

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nuc134r/sockmnezia/core/internal/mockgw"
	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/routing"
)

const uid = "00000000-0000-4000-8000-000000000001"

func amz(t *testing.T) json.RawMessage {
	b, _ := json.Marshal(mockgw.XrayConfig(uid))
	return b
}

func own(t *testing.T, link string) links.Outbound {
	s, err := links.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return s.Outbound
}

var goodIn = Inbound{SocksPort: 23456, User: "user", Pass: "pass"}

func build(t *testing.T, s Spec) map[string]any {
	t.Helper()
	b, err := Build(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

func outs(m map[string]any) []map[string]any {
	var r []map[string]any
	for _, o := range m["outbounds"].([]any) {
		r = append(r, o.(map[string]any))
	}
	return r
}

func dialer(o map[string]any) any {
	ss, _ := o["streamSettings"].(map[string]any)
	so, _ := ss["sockopt"].(map[string]any)
	return so["dialerProxy"]
}

func TestAmneziaMode(t *testing.T) {
	m := build(t, Spec{Hops: []Hop{AmneziaHop(amz(t))}, Inbound: goodIn})
	o := outs(m)
	tags := []string{}
	for _, x := range o {
		tags = append(tags, x["tag"].(string))
	}
	if strings.Join(tags, ",") != "amz-proxy,amz-proxy-relay,direct,block" {
		t.Fatalf("tags %v (dns-out must be dropped, head first)", tags)
	}
	if dialer(o[0]) != "amz-proxy-relay" {
		t.Fatalf("chain lost: %v", dialer(o[0]))
	}
	in := m["inbounds"].([]any)[0].(map[string]any)
	st := in["settings"].(map[string]any)
	if in["listen"] != "127.0.0.1" || st["auth"] != "password" || st["udp"] != false || len(m["inbounds"].([]any)) != 1 {
		t.Fatalf("inbound %v", in)
	}
	if in["sniffing"].(map[string]any)["routeOnly"] != true {
		t.Fatal("sniffing must be routeOnly")
	}
	for k := range m {
		if k == "api" || k == "dns" || k == "reverse" {
			t.Fatalf("unexpected %s", k)
		}
	}
}

const vpsLink = "vless://" + uid + "@vps.example.com:443?type=tcp&security=reality&pbk=K&sni=www.example.com"

func tags(o []map[string]any) string {
	var l []string
	for _, x := range o {
		l = append(l, x["tag"].(string))
	}
	return strings.Join(l, ",")
}

func TestOwnChains(t *testing.T) {
	// Amnezia -> own (the old "Amnezia→own" mode).
	m := build(t, Spec{Hops: []Hop{AmneziaHop(amz(t)), OwnHop(own(t, vpsLink))}, Inbound: goodIn})
	o := outs(m)
	if o[0]["tag"] != "hop2" || dialer(o[0]) != "amz-proxy" || o[1]["tag"] != "amz-proxy" {
		t.Fatalf("%v", o[:2])
	}
	rules := m["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 2 {
		t.Fatal(rules)
	}

	// Own server only.
	m = build(t, Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn})
	o = outs(m)
	if o[0]["tag"] != "hop1" || dialer(o[0]) != nil || len(o) != 3 {
		t.Fatalf("%v", o)
	}
	// Socks/ss outbounds have no streamSettings; chaining must add one.
	m = build(t, Spec{Hops: []Hop{AmneziaHop(amz(t)), OwnHop(own(t, "socks5://a:b@192.0.2.1:1080"))}, Inbound: goodIn})
	if dialer(outs(m)[0]) != "amz-proxy" {
		t.Fatal("socks own server not chained")
	}
}

func TestAmneziaInTheMiddle(t *testing.T) {
	// own -> Amnezia (relay -> proxy) -> own: Amnezia's relay, the outbound it dials first, goes through hop1.
	m := build(t, Spec{Hops: []Hop{
		OwnHop(own(t, "trojan://p@entry.example.com:443")), AmneziaHop(amz(t)), OwnHop(own(t, vpsLink)),
	}, Inbound: goodIn})
	o := outs(m)
	if got := tags(o); got != "hop3,amz-proxy,amz-proxy-relay,hop1,direct,block" {
		t.Fatal(got)
	}
	want := map[string]any{"hop3": "amz-proxy", "amz-proxy": "amz-proxy-relay", "amz-proxy-relay": "hop1", "hop1": nil}
	for _, x := range o[:4] {
		if dialer(x) != want[x["tag"].(string)] {
			t.Errorf("%s dials through %v, want %v", x["tag"], dialer(x), want[x["tag"].(string)])
		}
	}
	// Amnezia as the exit, reached through an own server.
	m = build(t, Spec{Hops: []Hop{OwnHop(own(t, vpsLink)), AmneziaHop(amz(t))}, Inbound: goodIn})
	o = outs(m)
	if o[0]["tag"] != "amz-proxy" || dialer(o[1]) != "hop1" {
		t.Fatalf("%v", o)
	}
}

func TestFiveOwnHops(t *testing.T) {
	var hops []Hop
	for i := 0; i < MaxHops; i++ {
		hops = append(hops, OwnHop(own(t, vpsLink)))
	}
	o := outs(build(t, Spec{Hops: hops, Inbound: goodIn}))
	if got := tags(o); got != "hop5,hop4,hop3,hop2,hop1,direct,block" {
		t.Fatal(got)
	}
	for i := 0; i < 4; i++ {
		if dialer(o[i]) != o[i+1]["tag"] {
			t.Fatalf("%v", o[i])
		}
	}
	if _, err := Build(Spec{Hops: append(hops, OwnHop(own(t, vpsLink))), Inbound: goodIn}); !errors.Is(err, ErrChainTooLong) {
		t.Fatal(err)
	}
}

func TestChainErrors(t *testing.T) {
	kcp := links.Outbound{"protocol": "vless", "settings": map[string]any{}, "streamSettings": map[string]any{"network": "kcp"}}
	cases := []struct {
		name  string
		hops  []Hop
		err   error
		field string
	}{
		{"empty", nil, ErrChainEmpty, "chain"},
		{"amnezia twice", []Hop{AmneziaHop(amz(t)), AmneziaHop(amz(t))}, ErrAmneziaTwice, "hop2"},
		{"udp after amnezia", []Hop{AmneziaHop(amz(t)), OwnHop(kcp)}, ErrNotChainable, "hop2"},
		{"udp in the middle", []Hop{OwnHop(own(t, vpsLink)), OwnHop(kcp), OwnHop(own(t, vpsLink))}, ErrNotChainable, "hop2"},
		{"empty hop", []Hop{OwnHop(own(t, vpsLink)), {}}, ErrHopIncomplete, "hop2"},
	}
	for _, c := range cases {
		_, err := Build(Spec{Hops: c.hops, Inbound: goodIn})
		var xe *Error
		if !errors.Is(err, c.err) || !errors.As(err, &xe) || xe.Field != c.field {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	// A UDP transport is fine as the entry: nothing dials through it.
	if _, err := Build(Spec{Hops: []Hop{OwnHop(kcp), OwnHop(own(t, vpsLink))}, Inbound: goodIn}); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(Spec{Hops: []Hop{AmneziaHop(amz(t)), OwnHop(kcp)}, Inbound: goodIn}); !errors.Is(err, links.ErrUnsupported) {
		t.Fatal("not-chainable must still wrap links.ErrUnsupported", err)
	}
}

func TestRoutingProxyMapsToHead(t *testing.T) {
	rt := routing.Default()
	rt.Rules = []routing.Rule{{Domain: []string{"example.org"}, OutboundTag: routing.OutProxy}}
	m := build(t, Spec{Hops: []Hop{AmneziaHop(amz(t))}, Inbound: goodIn, Routing: rt})
	rules := m["routing"].(map[string]any)["rules"].([]any)
	if rules[2].(map[string]any)["outboundTag"] != "amz-proxy" {
		t.Fatal(rules)
	}
}

func TestHTTPInboundAndLAN(t *testing.T) {
	in := goodIn
	in.HTTPPort = 23457
	in.Listen = "192.168.1.20"
	in.UDP = true
	m := build(t, Spec{Hops: []Hop{AmneziaHop(amz(t))}, Inbound: in})
	ins := m["inbounds"].([]any)
	h := ins[1].(map[string]any)
	if len(ins) != 2 || h["protocol"] != "http" || h["listen"] != "192.168.1.20" ||
		len(h["settings"].(map[string]any)["accounts"].([]any)) != 1 {
		t.Fatal(ins)
	}
	if ins[0].(map[string]any)["settings"].(map[string]any)["ip"] != "192.168.1.20" {
		t.Fatal("UDP response address must be the listen address")
	}
}

func TestInboundValidation(t *testing.T) {
	bad := []Inbound{
		{Listen: "0.0.0.0", SocksPort: 23456, User: "u", Pass: "p"},
		{Listen: "::", SocksPort: 23456, User: "u", Pass: "p"},
		{Listen: "8.8.8.8", SocksPort: 23456, User: "u", Pass: "p"},
		{Listen: "localhost", SocksPort: 23456, User: "u", Pass: "p"},
		{SocksPort: 80, User: "u", Pass: "p"},
		{SocksPort: 70000, User: "u", Pass: "p"},
		{SocksPort: 23456, HTTPPort: 23456, User: "u", Pass: "p"},
		{SocksPort: 23456, User: "", Pass: "p"},
		{SocksPort: 23456, User: "u", Pass: ""},
		{SocksPort: 23456, User: "u", Pass: "with space"},
		{SocksPort: 23456, User: "u", Pass: "пароль"},
		{SocksPort: 23456, User: strings.Repeat("u", 256), Pass: "p"},
	}
	for i, in := range bad {
		if _, err := Build(Spec{Hops: []Hop{AmneziaHop(amz(t))}, Inbound: in}); err == nil {
			t.Errorf("case %d accepted: %+v", i, in)
		}
	}
	var e *Error
	_, err := Build(Spec{Hops: []Hop{AmneziaHop(amz(t))}, Inbound: Inbound{SocksPort: 23456, User: "u", Pass: "a b"}})
	if !errors.As(err, &e) || e.Field != "pass" {
		t.Fatalf("field error: %v", err)
	}
}

func TestAmneziaSanitising(t *testing.T) {
	cases := map[string]string{
		"dangling dialerProxy dropped": `{"outbounds":[{"tag":"proxy","protocol":"vless","streamSettings":{"sockopt":{"dialerProxy":"gone"}}}]}`,
		"no proxy tag: first vless":    `{"outbounds":[{"tag":"dns-out","protocol":"dns"},{"tag":"x","protocol":"vless"}]}`,
		"api and inbounds ignored":     `{"api":{"tag":"api"},"inbounds":[{"protocol":"dokodemo-door","port":1}],"outbounds":[{"tag":"proxy","protocol":"vless"},{"tag":"f","protocol":"freedom"}]}`,
	}
	for name, cfg := range cases {
		m := build(t, Spec{Hops: []Hop{AmneziaHop(json.RawMessage(cfg))}, Inbound: goodIn})
		o := outs(m)
		if o[0]["protocol"] != "vless" || dialer(o[0]) != nil {
			t.Errorf("%s: %v", name, o)
		}
		for _, x := range o {
			if x["protocol"] == "freedom" && x["tag"] != "direct" {
				t.Errorf("%s: foreign freedom kept", name)
			}
		}
	}
	for name, cfg := range map[string]string{
		"no outbounds": `{"outbounds":[{"protocol":"dns"}]}`,
		"not json":     `nope`,
		"cycle":        `{"outbounds":[{"tag":"a","protocol":"vless","streamSettings":{"sockopt":{"dialerProxy":"b"}}},{"tag":"b","protocol":"vless","streamSettings":{"sockopt":{"dialerProxy":"a"}}}]}`,
	} {
		if _, err := Build(Spec{Hops: []Hop{AmneziaHop(json.RawMessage(cfg))}, Inbound: goodIn}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestAudit(t *testing.T) {
	b, _ := Build(Spec{Hops: []Hop{AmneziaHop(amz(t))}, Inbound: goodIn})
	if err := Audit(b); err != nil {
		t.Fatal(err)
	}
	mutate := func(f func(m map[string]any)) []byte {
		var m map[string]any
		json.Unmarshal(b, &m)
		f(m)
		r, _ := json.Marshal(m)
		return r
	}
	bad := map[string][]byte{
		"api": mutate(func(m map[string]any) { m["api"] = map[string]any{} }),
		"noauth": mutate(func(m map[string]any) {
			m["inbounds"].([]any)[0].(map[string]any)["settings"].(map[string]any)["auth"] = "noauth"
		}),
		"any address": mutate(func(m map[string]any) { m["inbounds"].([]any)[0].(map[string]any)["listen"] = "0.0.0.0" }),
		"dokodemo": mutate(func(m map[string]any) {
			m["inbounds"] = append(m["inbounds"].([]any), map[string]any{"protocol": "dokodemo-door"})
		}),
		"locked moved": mutate(func(m map[string]any) { r := m["routing"].(map[string]any); r["rules"] = r["rules"].([]any)[1:] }),
		"direct first": mutate(func(m map[string]any) {
			o := m["outbounds"].([]any)
			m["outbounds"] = append([]any{map[string]any{"tag": "direct", "protocol": "freedom"}}, o...)
		}),
	}
	for name, cfg := range bad {
		if Audit(cfg) == nil {
			t.Errorf("audit passed %s", name)
		}
	}
}

func TestBuildDialer(t *testing.T) {
	b, err := BuildDialer([]Hop{OwnHop(own(t, "trojan://p@vps.example.com:443"))})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	first := outs(m)[0]["tag"].(string)
	if m["inbounds"] != nil || !strings.HasPrefix(first, "t") || !strings.HasSuffix(first, "-hop1") {
		t.Fatal(string(b))
	}
	// Every temporary config gets its own tags, all references included.
	b2, _ := BuildDialer([]Hop{AmneziaHop(amz(t)), OwnHop(own(t, vpsLink))})
	json.Unmarshal(b2, &m)
	ns := strings.TrimSuffix(outs(m)[0]["tag"].(string), "hop2")
	if ns == strings.TrimSuffix(first, "hop1") {
		t.Fatal("namespace reused")
	}
	for _, o := range outs(m) {
		if !strings.HasPrefix(o["tag"].(string), ns) || (dialer(o) != nil && !strings.HasPrefix(dialer(o).(string), ns)) {
			t.Fatalf("%v", o)
		}
	}
	for _, r := range m["routing"].(map[string]any)["rules"].([]any) {
		if !strings.HasPrefix(r.(map[string]any)["outboundTag"].(string), ns) {
			t.Fatalf("%v", r)
		}
	}
}

func TestHops(t *testing.T) {
	h, err := Hops([]Hop{OwnHop(own(t, vpsLink))})
	if err != nil || len(h) != 1 || h[0] != "vps.example.com" {
		t.Fatal(h, err)
	}
	a, _ := Hops([]Hop{AmneziaHop(amz(t))})
	c, err := Hops([]Hop{AmneziaHop(amz(t)), OwnHop(own(t, vpsLink))})
	if err != nil || len(c) != len(a)+1 || c[len(c)-1] != "vps.example.com" {
		t.Fatal(a, c, err)
	}
	d, err := Hops([]Hop{OwnHop(own(t, "trojan://p@entry.example.com:443")), AmneziaHop(amz(t)), OwnHop(own(t, vpsLink))})
	if err != nil || len(d) != len(a)+2 || d[0] != "entry.example.com" || d[len(d)-1] != "vps.example.com" {
		t.Fatal(d, err)
	}
}

func TestVPNMode(t *testing.T) {
	hops := []Hop{OwnHop(own(t, vpsLink))}
	m := build(t, Spec{Hops: hops, Inbound: goodIn, VPN: &VPN{}})
	ins := m["inbounds"].([]any)
	if len(ins) != 2 {
		t.Fatal(ins)
	}
	tun := ins[1].(map[string]any)
	if tun["tag"] != TagTunIn || tun["protocol"] != "tun" || tun["listen"] != nil || tun["settings"].(map[string]any)["mtu"].(float64) != 1500 {
		t.Fatalf("%v", tun)
	}
	if m["dns"] != nil {
		t.Fatal("dns section without DoH")
	}
	// The SOCKS inbound keeps its password beside the tun; locked rules stay first.
	if ins[0].(map[string]any)["settings"].(map[string]any)["auth"] != "password" {
		t.Fatal("socks lost its auth")
	}

	m = build(t, Spec{Hops: hops, Inbound: goodIn, VPN: &VPN{MTU: 1400, DoH: []string{"https://1.1.1.1/dns-query", "https://[2606:4700:4700::1111]/dns-query"}}})
	d := m["dns"].(map[string]any)
	if len(d["servers"].([]any)) != 2 {
		t.Fatal(d)
	}
	rules := m["routing"].(map[string]any)["rules"].([]any)
	r := rules[len(routing.Locked())].(map[string]any)
	if r["outboundTag"] != TagDNSOut || r["port"] != "53" || r["inboundTag"].([]any)[0] != TagTunIn {
		t.Fatalf("dns rule not right after the locked rules: %v", rules)
	}
	if tags(outs(m)) != "hop1,direct,block,dns-out" {
		t.Fatal(tags(outs(m)))
	}

	for _, bad := range []string{"http://1.1.1.1/dns-query", "https://dns.google/dns-query", "8.8.8.8"} {
		if _, err := Build(Spec{Hops: hops, Inbound: goodIn, VPN: &VPN{DoH: []string{bad}}}); err == nil {
			t.Errorf("DoH server %q accepted", bad)
		}
	}
	if _, err := Build(Spec{Hops: hops, Inbound: goodIn, VPN: &VPN{MTU: 500}}); err == nil {
		t.Error("MTU 500 accepted")
	}
}

func TestAuditVPN(t *testing.T) {
	b, _ := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn, VPN: &VPN{DoH: []string{"https://1.1.1.1/dns-query"}}})
	if err := Audit(b); err != nil {
		t.Fatal(err)
	}
	proxyOnly, _ := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn})
	mutate := func(src []byte, f func(map[string]any)) []byte {
		var m map[string]any
		json.Unmarshal(src, &m)
		f(m)
		out, _ := json.Marshal(m)
		return out
	}
	cases := map[string][]byte{
		"dns section in proxy mode": mutate(proxyOnly, func(m map[string]any) {
			m["dns"] = map[string]any{"servers": []any{"https://1.1.1.1/dns-query"}}
		}),
		"dns outbound in proxy mode": mutate(proxyOnly, func(m map[string]any) {
			m["outbounds"] = append(m["outbounds"].([]any), map[string]any{"tag": TagDNSOut, "protocol": "dns"})
		}),
		"local resolver":      mutate(b, func(m map[string]any) { m["dns"] = map[string]any{"servers": []any{"localhost"}} }),
		"dns without servers": mutate(b, func(m map[string]any) { m["dns"] = map[string]any{} }),
		"second tun": mutate(b, func(m map[string]any) {
			m["inbounds"] = append(m["inbounds"].([]any), map[string]any{"tag": TagTunIn, "protocol": "tun"})
		}),
		"tun with a port": mutate(b, func(m map[string]any) {
			m["inbounds"].([]any)[1].(map[string]any)["port"] = 1080
		}),
		"tun only, no proxy inbound": mutate(b, func(m map[string]any) { m["inbounds"] = m["inbounds"].([]any)[1:] }),
	}
	for name, cfg := range cases {
		if Audit(cfg) == nil {
			t.Errorf("audit passed: %s", name)
		}
	}
}

// The system-proxy inbound (D30) is the only one without authentication: HTTP, 127.0.0.1 only, even when
// the proxy itself is shared on a LAN address, and only one of it.
func TestSystemProxyInbound(t *testing.T) {
	lan := goodIn
	lan.Listen = "192.168.1.10"
	b, err := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: lan, SystemProxyPort: 18080})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	var sys map[string]any
	for _, x := range m["inbounds"].([]any) {
		if in := x.(map[string]any); in["tag"] == TagSysProxyIn {
			sys = in
		}
	}
	if sys == nil || sys["listen"] != "127.0.0.1" || sys["protocol"] != "http" || sys["port"] != float64(18080) {
		t.Fatalf("system-proxy inbound: %v", sys)
	}
	for _, p := range []int{80, 70000, goodIn.SocksPort} {
		if _, err := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn, SystemProxyPort: p}); err == nil {
			t.Errorf("port %d accepted", p)
		}
	}
	none, _ := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn})
	if strings.Contains(string(none), TagSysProxyIn) {
		t.Error("system-proxy inbound without SystemProxyPort")
	}

	mutate := func(f func([]any) []any) []byte {
		var m map[string]any
		json.Unmarshal(b, &m)
		m["inbounds"] = f(m["inbounds"].([]any))
		out, _ := json.Marshal(m)
		return out
	}
	sysIdx := func(ins []any) map[string]any {
		for _, x := range ins {
			if in := x.(map[string]any); in["tag"] == TagSysProxyIn {
				return in
			}
		}
		return nil
	}
	cases := map[string][]byte{
		"on a LAN address":   mutate(func(ins []any) []any { sysIdx(ins)["listen"] = "192.0.2.10"; return ins }),
		"on all addresses":   mutate(func(ins []any) []any { sysIdx(ins)["listen"] = "0.0.0.0"; return ins }),
		"as SOCKS":           mutate(func(ins []any) []any { sysIdx(ins)["protocol"] = "socks"; return ins }),
		"twice":              mutate(func(ins []any) []any { return append(ins, sysIdx(ins)) }),
		"only it":            mutate(func(ins []any) []any { return []any{sysIdx(ins)} }),
		"other tag, no auth": mutate(func(ins []any) []any { s := sysIdx(ins); s["tag"] = "x-in"; return ins }),
	}
	for name, cfg := range cases {
		if Audit(cfg) == nil {
			t.Errorf("audit passed: %s", name)
		}
	}
}

// Desktop TUN (D37): Xray brings the interface and routes up itself and binds its own sockets to the
// physical interface; the config may not write files, since the elevated helper runs it.
func TestSystemTUN(t *testing.T) {
	sys := &SystemTUN{Gateway: []string{"172.19.0.1/30"}, Routes: []string{"0.0.0.0/0", "::/0"}, DNS: []string{"172.19.0.2"},
		WFP: []string{"dns", "misconfigtun"}}
	b, err := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn, VPN: &VPN{DoH: []string{"https://1.1.1.1/dns-query"}, System: sys}})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	var tun map[string]any
	for _, x := range m["inbounds"].([]any) {
		if in := x.(map[string]any); in["protocol"] == "tun" {
			tun = in["settings"].(map[string]any)
		}
	}
	if tun["name"] != TunName || tun["autoOutboundsInterface"] != "auto" || len(tun["autoSystemRoutingTable"].([]any)) != 2 || tun["dns"] == nil {
		t.Fatalf("tun settings: %v", tun)
	}
	// Pointing the system resolver at the tun needs the DoH module to answer it.
	if _, err := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn, VPN: &VPN{System: sys}}); err == nil {
		t.Error("system DNS without DoH accepted")
	}
	if _, err := Build(Spec{Hops: []Hop{OwnHop(own(t, vpsLink))}, Inbound: goodIn, VPN: &VPN{DoH: []string{"https://1.1.1.1/dns-query"}, System: &SystemTUN{}}}); err == nil {
		t.Error("system TUN without routes accepted")
	}

	mutate := func(f func(m map[string]any)) []byte {
		var m map[string]any
		json.Unmarshal(b, &m)
		f(m)
		out, _ := json.Marshal(m)
		return out
	}
	tunOf := func(m map[string]any) map[string]any {
		for _, x := range m["inbounds"].([]any) {
			if in := x.(map[string]any); in["protocol"] == "tun" {
				return in["settings"].(map[string]any)
			}
		}
		return nil
	}
	cases := map[string][]byte{
		"access log file":      mutate(func(m map[string]any) { m["log"].(map[string]any)["access"] = "/etc/cron.d/x" }),
		"error log file":       mutate(func(m map[string]any) { m["log"].(map[string]any)["error"] = `C:\Windows\x.dll` }),
		"unknown log setting":  mutate(func(m map[string]any) { m["log"].(map[string]any)["maskAddress"] = "full" }),
		"other interface name": mutate(func(m map[string]any) { tunOf(m)["name"] = "eth0" }),
		"outbounds bound to a named interface": mutate(func(m map[string]any) {
			tunOf(m)["autoOutboundsInterface"] = "tun0"
		}),
		"unknown tun setting": mutate(func(m map[string]any) { tunOf(m)["userLevel"] = 1 }),
	}
	for name, cfg := range cases {
		if Audit(cfg) == nil {
			t.Errorf("audit passed: %s", name)
		}
	}
}
