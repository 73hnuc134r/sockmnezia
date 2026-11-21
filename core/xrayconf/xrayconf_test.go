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
	m := build(t, Spec{Mode: ModeAmnezia, Amnezia: amz(t), Inbound: goodIn})
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

func TestOwnModes(t *testing.T) {
	link := "vless://" + uid + "@vps.example.com:443?type=tcp&security=reality&pbk=K&sni=www.example.com"
	m := build(t, Spec{Mode: ModeAmneziaOwn, Amnezia: amz(t), Own: own(t, link), Inbound: goodIn})
	o := outs(m)
	if o[0]["tag"] != "own" || dialer(o[0]) != "amz-proxy" || o[1]["tag"] != "amz-proxy" {
		t.Fatalf("%v", o[:2])
	}
	rules := m["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 2 {
		t.Fatal(rules)
	}

	m = build(t, Spec{Mode: ModeOwn, Own: own(t, link), Inbound: goodIn})
	o = outs(m)
	if o[0]["tag"] != "own" || dialer(o[0]) != nil || len(o) != 3 {
		t.Fatalf("%v", o)
	}
	// Socks/ss outbounds have no streamSettings; chaining must add one.
	m = build(t, Spec{Mode: ModeAmneziaOwn, Amnezia: amz(t), Own: own(t, "socks5://a:b@192.0.2.1:1080"), Inbound: goodIn})
	if dialer(outs(m)[0]) != "amz-proxy" {
		t.Fatal("socks own server not chained")
	}
}

func TestOwnUDPTransportRefusedInChain(t *testing.T) {
	o := links.Outbound{"protocol": "vless", "settings": map[string]any{}, "streamSettings": map[string]any{"network": "kcp"}}
	if _, err := Build(Spec{Mode: ModeAmneziaOwn, Amnezia: amz(t), Own: o, Inbound: goodIn}); !errors.Is(err, links.ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestRoutingProxyMapsToHead(t *testing.T) {
	rt := routing.Default()
	rt.Rules = []routing.Rule{{Domain: []string{"example.org"}, OutboundTag: routing.OutProxy}}
	m := build(t, Spec{Mode: ModeAmnezia, Amnezia: amz(t), Inbound: goodIn, Routing: rt})
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
	m := build(t, Spec{Mode: ModeAmnezia, Amnezia: amz(t), Inbound: in})
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
		if _, err := Build(Spec{Mode: ModeAmnezia, Amnezia: amz(t), Inbound: in}); err == nil {
			t.Errorf("case %d accepted: %+v", i, in)
		}
	}
	var e *Error
	_, err := Build(Spec{Mode: ModeAmnezia, Amnezia: amz(t), Inbound: Inbound{SocksPort: 23456, User: "u", Pass: "a b"}})
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
		m := build(t, Spec{Mode: ModeAmnezia, Amnezia: json.RawMessage(cfg), Inbound: goodIn})
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
		if _, err := Build(Spec{Mode: ModeAmnezia, Amnezia: json.RawMessage(cfg), Inbound: goodIn}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestAudit(t *testing.T) {
	b, _ := Build(Spec{Mode: ModeAmnezia, Amnezia: amz(t), Inbound: goodIn})
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
	b, err := BuildDialer(ModeOwn, nil, own(t, "trojan://p@vps.example.com:443"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["inbounds"] != nil || outs(m)[0]["tag"] != "own" {
		t.Fatal(string(b))
	}
}

func TestHops(t *testing.T) {
	link := "vless://" + uid + "@vps.example.com:443?type=tcp&security=reality&pbk=K&sni=www.example.com"
	h, err := Hops(ModeOwn, nil, own(t, link))
	if err != nil || len(h) != 1 || h[0] != "vps.example.com" {
		t.Fatal(h, err)
	}
	a, _ := Hops(ModeAmnezia, amz(t), nil)
	c, err := Hops(ModeAmneziaOwn, amz(t), own(t, link))
	if err != nil || len(c) != len(a)+1 || c[len(c)-1] != "vps.example.com" {
		t.Fatal(a, c, err)
	}
}
