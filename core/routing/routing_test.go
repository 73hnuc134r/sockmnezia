package routing_test

import (
	"errors"
	"os"
	"testing"

	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/internal/geotest"
	"github.com/nuc134r/sockmnezia/core/routing"
)

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "geo")
	if err := geotest.Write(dir); err != nil {
		panic(err)
	}
	geo.SetAssetDir(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestLockedRulesFirst(t *testing.T) {
	s := routing.Default()
	s.Rules = []routing.Rule{{ID: "x", IP: []string{"127.0.0.1"}, OutboundTag: routing.OutDirect}}
	rules, err := s.Compile("own")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 || rules[0]["outboundTag"] != "block" || rules[1]["outboundTag"] != "block" || rules[2]["outboundTag"] != "direct" {
		t.Fatalf("%v", rules)
	}
	tst, _ := routing.NewTester(s)
	if v := tst.Match("127.0.0.1", 80, "tcp"); v.OutboundTag != "block" || !v.Locked {
		t.Fatalf("user rule reopened loopback: %+v", v)
	}
	for _, h := range []string{"192.168.1.1", "10.1.2.3", "::1", "fe80::1", "localhost", "printer.local", "LOCALHOST."} {
		if v := tst.Match(h, 80, "tcp"); v.OutboundTag != "block" {
			t.Errorf("%s: %+v", h, v)
		}
	}
	if v := tst.Match("example.com", 443, "tcp"); v.OutboundTag != "proxy" || v.RuleIndex != -1 {
		t.Fatalf("%+v", v)
	}
}

func TestProxyTagMapping(t *testing.T) {
	s := routing.Default()
	s.Rules = []routing.Rule{{Domain: []string{"domain:example.org"}, OutboundTag: routing.OutProxy}}
	rules, _ := s.Compile("amz-proxy")
	if rules[2]["outboundTag"] != "amz-proxy" {
		t.Fatal(rules)
	}
}

func TestPresetsWithGeo(t *testing.T) {
	s, ok := routing.Default().Apply("bypass_ru", false)
	if !ok {
		t.Fatal("preset missing")
	}
	s, _ = s.Apply("block_ads", false)
	s, _ = s.Apply("block_quic", false)
	s, _ = s.Apply("bypass_ru", false) // no duplicates
	if len(s.Rules) != 4 {
		t.Fatalf("%d rules", len(s.Rules))
	}
	site, ip := s.UsesGeo()
	if !site || !ip {
		t.Fatal("UsesGeo")
	}
	tst, err := routing.NewTester(s)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		host string
		port int
		net  string
		want string
	}{
		{"www.ru.example", 443, "tcp", "direct"},
		{"exact.ru.example", 443, "tcp", "direct"},
		{"198.51.100.20", 443, "tcp", "direct"},
		{"x.ads.example", 443, "tcp", "block"},
		{"203.0.113.5", 443, "udp", "block"}, // QUIC rule
		{"203.0.113.5", 443, "tcp", "proxy"},
		{"example.com", 80, "tcp", "proxy"},
	}
	for _, c := range cases {
		if v := tst.Match(c.host, c.port, c.net); v.OutboundTag != c.want {
			t.Errorf("%s:%d/%s → %+v, want %s", c.host, c.port, c.net, v, c.want)
		}
	}
	s, _ = s.Apply("global", true)
	if len(s.Rules) != 0 {
		t.Fatal("global must clear rules")
	}
}

func TestDisabledRuleIgnored(t *testing.T) {
	off := false
	s := routing.Default()
	s.Rules = []routing.Rule{{Domain: []string{"example.org"}, OutboundTag: routing.OutBlock, Enabled: &off}}
	tst, _ := routing.NewTester(s)
	if v := tst.Match("example.org", 443, "tcp"); v.OutboundTag != "proxy" {
		t.Fatal(v)
	}
	r, _ := s.Compile("p")
	if len(r) != 2 {
		t.Fatal("disabled rule compiled")
	}
}

func TestMissingGeoTag(t *testing.T) {
	s := routing.Default()
	s.Rules = []routing.Rule{{Domain: []string{"geosite:does-not-exist"}, OutboundTag: routing.OutDirect}}
	if _, err := routing.NewTester(s); err == nil {
		t.Fatal("unknown geosite tag accepted")
	}
}

func TestValidation(t *testing.T) {
	bad := []routing.Rule{
		{OutboundTag: "proxy"},
		{Domain: []string{"x"}, OutboundTag: "elsewhere"},
		{Domain: []string{"regexp:("}, OutboundTag: "block"},
		{Domain: []string{"ext:foo.dat:x"}, OutboundTag: "block"},
		{IP: []string{"300.1.1.1"}, OutboundTag: "block"},
		{IP: []string{"10.0.0.0/33"}, OutboundTag: "block"},
		{Port: "0", OutboundTag: "block"},
		{Port: "2000-1000", OutboundTag: "block"},
		{Network: "sctp", OutboundTag: "block"},
		{Protocol: []string{"ssh"}, OutboundTag: "block"},
	}
	for i, r := range bad {
		s := routing.Default()
		s.Rules = []routing.Rule{r}
		var re *routing.RuleError
		if err := s.Validate(); !errors.As(err, &re) {
			t.Errorf("case %d accepted: %+v", i, r)
		}
	}
	s := routing.Default()
	s.DomainStrategy = "Whatever"
	if s.Validate() == nil {
		t.Fatal("bad strategy accepted")
	}
}

func TestParseV2rayNGExport(t *testing.T) {
	// Shape of v2rayNG's custom_routing_white_russia asset.
	in := `[
	  {"remarks":"Bypass bittorrent","outboundTag":"direct","protocol":["bittorrent"]},
	  {"remarks":"Block udp443","outboundTag":"block","port":"443","network":"udp"},
	  {"remarks":"Direct LAN IP","outboundTag":"direct","ip":["geoip:private"],"locked":true},
	  {"remarks":"Bypass Russia domains","outboundTag":"direct","domain":["geosite:category-ru"],"enabled":false}
	]`
	s, err := routing.ParseJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Rules) != 3 || s.DomainStrategy != routing.AsIs || s.Rules[2].IsEnabled() {
		t.Fatalf("%+v", s)
	}
}
