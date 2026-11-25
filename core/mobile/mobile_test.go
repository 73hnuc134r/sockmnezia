package mobile

import (
	"encoding/json"
	"strings"
	"testing"
)

type nopBox struct{}

func (nopBox) Seal(p []byte) ([]byte, error) { return p, nil }
func (nopBox) Open(p []byte) ([]byte, error) { return p, nil }

func TestErrorsAreJSON(t *testing.T) {
	c, err := Open(t.TempDir(), nopBox{})
	if err != nil {
		t.Fatal(err)
	}
	err = c.SetKey("vpn://nonsense")
	var e struct{ Code string }
	if err == nil || json.Unmarshal([]byte(err.Error()), &e) != nil || e.Code != "invalid_key" {
		t.Fatalf("%v", err)
	}
	if _, err := c.AddOwnServer("trojan://p@vps.example.com:443#v", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.OwnServersJSON(), `"used_in":[],"active":true`) || strings.Contains(c.OwnServersJSON(), "trojan://") {
		t.Fatal(c.OwnServersJSON())
	}
	// An older app sending "mode" is ignored.
	var set map[string]any
	json.Unmarshal([]byte(c.SettingsJSON()), &set)
	set["mode"] = "own"
	b, _ := json.Marshal(set)
	if err := c.SetSettingsJSON(string(b)); err != nil {
		t.Fatal(err)
	}
	if v := c.ValidateChainJSON(`{"hops":[{"kind":"amnezia"},{"kind":"amnezia"}]}`); !strings.Contains(v, `"chain_amnezia_twice"`) {
		t.Fatal(v)
	}
	if v := c.ValidateChainJSON(`{"hops":[{"kind":"amnezia"}]}`); v != "" {
		t.Fatal(v)
	}
	// The server added above is selected on its own, so no chain is active.
	if !strings.Contains(c.ChainsJSON(), `"name":"Amnezia"`) || strings.Contains(c.ChainsJSON(), `"active":true`) {
		t.Fatal(c.ChainsJSON())
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	if !strings.Contains(c.StatusJSON(), `"running":true`) {
		t.Fatal(c.StatusJSON())
	}
	if XrayVersion() == "" {
		t.Fatal("no version")
	}
}

func TestRedact(t *testing.T) {
	c, _ := Open(t.TempDir(), nopBox{})
	c.AddOwnServer("trojan://p@vps.example.com:443#v", "")
	pass := c.e.Snapshot().Inbound.Pass
	in := "dial vps.example.com 203.0.113.9:443 user 00000000-0000-4000-8000-000000000001 pw " + pass + " lo 127.0.0.1 v6 2001:db8::1"
	out := c.redact(in)
	for _, leak := range []string{"vps.example.com", "203.0.113.9", "00000000-0000", pass, "2001:db8"} {
		if strings.Contains(out, leak) {
			t.Errorf("leaked %q: %s", leak, out)
		}
	}
	if !strings.Contains(out, "127.0.0.1") {
		t.Error(out)
	}
}
