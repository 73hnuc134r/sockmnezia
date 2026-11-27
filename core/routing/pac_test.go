package routing

import (
	"strings"
	"testing"
)

func TestPAC(t *testing.T) {
	off := false
	s := Settings{DomainStrategy: AsIs, Rules: []Rule{
		{Domain: []string{"domain:example.com", "full:www.example.org"}, OutboundTag: OutDirect},
		{Domain: []string{"keyword:ads"}, OutboundTag: OutBlock},
		{IP: []string{"198.51.100.0/24"}, OutboundTag: OutDirect},
		{Domain: []string{"domain:disabled.example"}, OutboundTag: OutDirect, Enabled: &off},
		{Domain: []string{"geosite:category-ru"}, OutboundTag: OutDirect},
		{Domain: []string{"domain:after.example"}, OutboundTag: OutDirect},
	}}
	js := PAC(s, "127.0.0.1:18080")
	for _, want := range []string{
		`dnsDomainIs(host, ".example.com")) || host === "www.example.org") return "DIRECT"`,
		`host.indexOf("ads") >= 0) return "PROXY 127.0.0.1:18080"`, // block: Xray blocks it
		`isInNet(host, "198.51.100.0", "255.255.255.0"))) return "DIRECT"`,
		`isInNet(host, "192.168.0.0", "255.255.0.0")`, // private: direct, the proxy would block it
		`return "PROXY 127.0.0.1:18080";` + "\n}",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("missing %q in\n%s", want, js)
		}
	}
	// Disabled rules are skipped; a geosite rule ends the script (only Xray can evaluate it), so a later
	// direct rule must not go direct: it could be shadowed by the geosite rule's (unknown) match.
	for _, bad := range []string{"disabled.example", "after.example", "dnsResolve"} {
		if strings.Contains(js, bad) {
			t.Errorf("unexpected %q in\n%s", bad, js)
		}
	}

	// Rules with ports or resolving strategies end the script too.
	s2 := Settings{DomainStrategy: IPIfNonMatch, Rules: []Rule{
		{IP: []string{"203.0.113.0/24"}, OutboundTag: OutDirect},
		{Domain: []string{"domain:example.net"}, OutboundTag: OutDirect},
	}}
	if js := PAC(s2, "127.0.0.1:1"); strings.Contains(js, "203.0.113.0") || strings.Contains(js, "example.net") {
		t.Errorf("IP rule with a resolving strategy was expressed:\n%s", js)
	}
	s3 := Settings{Rules: []Rule{{Domain: []string{"domain:example.net"}, Port: "443", OutboundTag: OutDirect}}}
	if js := PAC(s3, "127.0.0.1:1"); strings.Contains(js, "example.net") {
		t.Errorf("rule with ports was expressed:\n%s", js)
	}
}
