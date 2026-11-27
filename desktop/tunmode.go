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

// TUN mode (docs/10 §3a, D40): the engine's config runs in the system service (package tun); this file picks
// the system TUN settings and keeps the app attached to the service.

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
		// The adapter's DNS server, answered by Xray over DoH. WFP lets nothing but Xray leave outside the
		// tun, except the local network (also while the service blocks everything: proxy stopped).
		s.DNS = []string{"172.19.0.2"}
		s.WFP = []string{"allexceptlan", "misconfigtun"}
	case "linux":
		// systemd-resolved is pointed at the tun when it manages DNS; otherwise DNS stays with the system.
		s.DNSToGateway = resolvedActive()
		s.Mark = tun.Mark // the kill switch lets only Xray's marked sockets out
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

// tunWatch remembers the service's instance to notice when it goes away.
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
			return nil, tunError(err)
		}
		d.tunw.mu.Lock()
		d.tunw.remote, _ = inst.(*tun.Remote)
		d.tunw.mu.Unlock()
		return inst, nil
	}
}

func tunError(err error) error {
	code := "tun_failed"
	if errors.Is(err, tun.ErrDeclined) {
		code = "tun_declined"
	}
	return &engine.Error{Code: code, Detail: err.Error()}
}

// startTUN starts (or restarts, or attaches to) the proxy in TUN mode.
func (d *desk) startTUN(ctx context.Context) error {
	return d.e.StartTUN(ctx, systemTUN(), d.tunLauncher())
}

// tunActive: TUN mode is chosen and the service runs a config (e.g. since boot): the app attaches on launch.
func (d *desk) tunActive() bool {
	return d.e.GetSettings().Options.Mode == engine.ModeVPN && tun.Query().Active
}

// detachTUN leaves the TUN running in the service (the app quits); false when TUN isn't running.
func (d *desk) detachTUN() bool {
	d.tunw.mu.Lock()
	r := d.tunw.remote
	d.tunw.mu.Unlock()
	if r == nil || !d.e.Status().VPN {
		return false
	}
	r.Detach()
	return true
}

// checkTUN notices a lost service connection (the service restarted after a crash: it resumes the last
// config on its own) and attaches again.
func (d *desk) checkTUN(st engine.Status) {
	if !st.Running || !st.VPN {
		return
	}
	d.tunw.mu.Lock()
	r := d.tunw.remote
	d.tunw.mu.Unlock()
	if r == nil || r.Alive() {
		return
	}
	d.e.Stop() // the service blocks or is gone; either way nothing leaks
	if err := d.startTUN(context.Background()); err != nil {
		d.notify(engine.Classify(err))
	}
	d.changed()
}
