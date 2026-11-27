package engine

import (
	"strings"
	"testing"

	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// The desktop system-proxy mode adds the loopback HTTP port without authentication (D30) only while on.
func TestSystemProxyOption(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	build := func() string {
		e.mu.Lock()
		defer e.mu.Unlock()
		cfg, err := e.buildLocked()
		if err != nil {
			t.Fatal(err)
		}
		if err := xrayconf.Audit(cfg); err != nil {
			t.Fatal(err)
		}
		return string(cfg)
	}
	if strings.Contains(build(), xrayconf.TagSysProxyIn) {
		t.Fatal("system-proxy port while off")
	}
	set := e.GetSettings()
	set.Options.SystemProxy = SystemProxyPAC
	if err := e.SetSettings(set); err != nil {
		t.Fatal(err)
	}
	st := e.Status()
	in := e.GetSettings().Inbound
	if p := st.SystemProxyPort; p == 0 || p == in.SocksPort || p == in.HTTPPort || st.SystemProxy != SystemProxyPAC {
		t.Fatalf("status %+v", st)
	}
	if !strings.Contains(build(), xrayconf.TagSysProxyIn) {
		t.Fatal("no system-proxy port while on")
	}
	set = e.GetSettings()
	set.Options.SystemProxy = "teleport"
	if err := e.SetSettings(set); code(err) != CodeInvalidSettings {
		t.Fatal(err)
	}
	set.Options.SystemProxy = ""
	if err := e.SetSettings(set); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(build(), xrayconf.TagSysProxyIn) {
		t.Fatal("system-proxy port after turning it off")
	}
}
