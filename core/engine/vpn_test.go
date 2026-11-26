package engine

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

func routesContain(t *testing.T, routes []string, ip string) bool {
	t.Helper()
	a := netip.MustParseAddr(ip)
	for _, r := range routes {
		if netip.MustParsePrefix(r).Contains(a) {
			return true
		}
	}
	return false
}

func TestVPNInterface(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	vi := e.VPNInterface()
	if vi.MTU != 1500 || len(vi.Addresses) != 2 || vi.Apps != VPNAppsAll || len(vi.DNS) != 4 {
		t.Fatalf("%+v", vi)
	}
	// LAN bypass (default): public addresses go into the VPN, the locked rules' private ranges don't.
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "203.0.113.9", "2606:4700:4700::1111", "198.18.0.5"} {
		if !routesContain(t, vi.Routes, ip) {
			t.Errorf("%s not routed into the VPN", ip)
		}
	}
	for _, ip := range []string{"192.168.1.10", "10.0.0.1", "172.16.5.5", "169.254.1.1", "100.64.0.1", "224.0.0.251", "fe80::1", "fd00::5", "127.0.0.1"} {
		if routesContain(t, vi.Routes, ip) {
			t.Errorf("%s (local) routed into the VPN", ip)
		}
	}

	set := e.GetSettings()
	set.Options.VPN = VPNOptions{DNS: VPNDNSCustom, DNSServers: []string{" 9.9.9.9 ", "192.168.1.1"}, LAN: VPNLANBypass, Apps: VPNAppsExcept, AppList: []string{"com.example.bank"}}
	if err := e.SetSettings(set); err != nil {
		t.Fatal(err)
	}
	vi = e.VPNInterface()
	if len(vi.DNS) != 2 || vi.DNS[0] != "9.9.9.9" || vi.Apps != VPNAppsExcept || vi.AppList[0] != "com.example.bank" {
		t.Fatalf("%+v", vi)
	}
	// Everything through the VPN: one default route per family; a local DNS server would then be blocked.
	set.Options.VPN.LAN = VPNLANTunnel
	if err := e.SetSettings(set); code(err) != CodeInvalidSettings {
		t.Fatal("local DNS server accepted with the LAN through the VPN", err)
	}
	set.Options.VPN.DNSServers = []string{"9.9.9.9"}
	if err := e.SetSettings(set); err != nil {
		t.Fatal(err)
	}
	if vi = e.VPNInterface(); len(vi.Routes) != 2 || vi.Routes[0] != "0.0.0.0/0" {
		t.Fatal(vi.Routes)
	}
	for _, bad := range []VPNOptions{
		{DNS: "magic", LAN: VPNLANBypass, Apps: VPNAppsAll},
		{DNS: VPNDNSCustom, LAN: VPNLANBypass, Apps: VPNAppsAll},
		{DNS: VPNDNSCustom, DNSServers: []string{"dns.example.com"}, LAN: VPNLANBypass, Apps: VPNAppsAll},
		{DNS: VPNDNSPublic, LAN: "sometimes", Apps: VPNAppsAll},
		{DNS: VPNDNSPublic, LAN: VPNLANBypass, Apps: VPNAppsOnly},
	} {
		set.Options.VPN = bad
		if err := e.SetSettings(set); code(err) != CodeInvalidSettings {
			t.Errorf("%+v accepted: %v", bad, err)
		}
	}
	set.Options.Mode = "teleport"
	set.Options.VPN = defaultVPNOptions()
	if err := e.SetSettings(set); code(err) != CodeInvalidSettings {
		t.Fatal(err)
	}
}

func TestVPNSpec(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	if err := e.StartVPN(context.Background(), 1); code(err) != CodeInvalidSettings {
		t.Fatal("stdout accepted as the tun", err)
	}
	e.mu.Lock()
	if e.vpnSpecLocked() != nil {
		t.Fatal("VPN spec without a tun")
	}
	e.vpnFD = 99 // as if StartVPN had a tun; build only, not started
	cfg, err := e.buildLocked()
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(cfg, &m)
	if len(m["inbounds"].([]any)) != 2 || m["dns"] != nil {
		t.Fatal(string(cfg))
	}
	set := e.GetSettings()
	set.Options.VPN.DNS = VPNDNSDoH
	if err := e.SetSettings(set); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	cfg, _ = e.buildLocked()
	e.vpnFD = -1
	e.mu.Unlock()
	json.Unmarshal(cfg, &m)
	if m["dns"] == nil || xrayconf.Audit(cfg) != nil {
		t.Fatal("DoH mode without the dns module", string(cfg))
	}
}
