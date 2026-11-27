package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Runs against a real GSettings backend when there is one (e.g. under dbus-run-session); skipped otherwise.
func TestGnomeProxyRoundTrip(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus")
	}
	p, ok := platformProxy().(linuxProxy)
	if !ok || !p.gnome {
		t.Skip("no GSettings proxy schema")
	}
	get := func(schema, key string) string {
		out, _ := exec.Command("gsettings", "get", schema, key).Output()
		return strings.TrimSpace(string(out))
	}
	saved, err := p.Save()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SetHTTP("127.0.0.1:18080", bypass); err != nil {
		t.Fatal(err)
	}
	if get("org.gnome.system.proxy", "mode") != "'manual'" || get("org.gnome.system.proxy.http", "port") != "18080" ||
		!strings.Contains(get("org.gnome.system.proxy", "ignore-hosts"), "'192.168.0.0/16'") {
		t.Fatal("HTTP proxy not set")
	}
	if err := p.SetPAC("http://127.0.0.1:1/proxy.pac"); err != nil {
		t.Fatal(err)
	}
	if get("org.gnome.system.proxy", "mode") != "'auto'" {
		t.Fatal("PAC not set")
	}
	if err := p.Restore(saved); err != nil {
		t.Fatal(err)
	}
	after, _ := p.Save()
	for k, v := range saved {
		if after[k] != v {
			t.Errorf("%s: %s, want %s", k, after[k], v)
		}
	}
}
