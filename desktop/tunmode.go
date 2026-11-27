package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
	"github.com/nuc134r/sockmnezia/desktop/tun"
)

// TUN mode (docs/10 §3a, D37): the engine's config runs in the elevated helper (package tun); this file picks
// the system TUN settings and watches the helper.

// systemTUN is the interface, routes and DNS for this OS. The local network stays reachable: its routes are
// more specific than the default routes, and private addresses are blocked inside the proxy anyway.
func systemTUN() xrayconf.SystemTUN {
	s := xrayconf.SystemTUN{
		Gateway: []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
		Routes:  []string{"0.0.0.0/0", "::/0"},
	}
	// Tests: route only these prefixes (e.g. a documentation range) instead of everything.
	if r := os.Getenv("SOCKMNEZIA_TUN_ROUTES"); r != "" {
		s.Routes = strings.Split(r, ",")
	}
	switch runtime.GOOS {
	case "windows":
		// The adapter's DNS server, answered by Xray over DoH; WFP keeps DNS and IPv6 from leaving elsewhere.
		s.DNS = []string{"172.19.0.2"}
		s.WFP = []string{"dns", "misconfigtun"}
	case "linux":
		// systemd-resolved is pointed at the tun when it manages DNS; otherwise DNS stays with the system.
		s.DNSToGateway = resolvedActive()
		// Without IPv6 the address can't be set (and there is no IPv6 traffic to route).
		if !ipv6Enabled() {
			s.Gateway = s.Gateway[:1]
			s.Routes = slices.DeleteFunc(s.Routes, func(r string) bool { return strings.Contains(r, ":") })
		}
	}
	return s
}

func ipv6Enabled() bool {
	b, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/disable_ipv6")
	return err == nil && strings.TrimSpace(string(b)) == "0"
}

func resolvedActive() bool {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		return false
	}
	return exec.Command("resolvectl", "status").Run() == nil
}

// tunWatch remembers the helper's instance to notice when it goes away.
type tunWatch struct {
	mu     sync.Mutex
	remote *tun.Remote
}

func (d *desk) tunLauncher() engine.Launcher {
	inner := tun.Launcher(Version, func() tun.Socks {
		in := d.e.GetSettings().Inbound
		return tun.Socks{Addr: fmt.Sprintf("127.0.0.1:%d", in.SocksPort), User: in.User, Pass: in.Pass}
	})
	return func(cfg []byte, geoDir string) (engine.Instance, error) {
		inst, err := inner(cfg, geoDir)
		if err != nil {
			code := "tun_failed"
			if errors.Is(err, tun.ErrDeclined) {
				code = "tun_declined"
			}
			return nil, &engine.Error{Code: code, Detail: err.Error()}
		}
		d.tunw.mu.Lock()
		d.tunw.remote, _ = inst.(*tun.Remote)
		d.tunw.mu.Unlock()
		return inst, nil
	}
}

// startTUN starts (or restarts) the proxy in TUN mode.
func (d *desk) startTUN(ctx context.Context) error {
	return d.e.StartTUN(ctx, systemTUN(), d.tunLauncher())
}

// checkTUN stops the proxy when the helper is gone (killed, or its TUN failed), so the app doesn't show a
// connection that no longer exists.
func (d *desk) checkTUN(st engine.Status) {
	if !st.Running || !st.VPN {
		return
	}
	d.tunw.mu.Lock()
	r := d.tunw.remote
	d.tunw.mu.Unlock()
	if r != nil && !r.Alive() {
		d.e.Stop()
		d.notify(&engine.Error{Code: "tun_failed", Detail: "the TUN helper stopped"})
		d.changed()
	}
}
