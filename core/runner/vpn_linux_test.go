//go:build linux

package runner_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// openTun creates a TUN interface with addr (e.g. 198.18.0.1/24, so the kernel routes that subnet into it) and
// returns its file descriptor, as Android's VpnService would hand it over. Skips when the test may not create one
// (not root, no /dev/net/tun).
func openTun(t *testing.T, addr netip.Prefix) int {
	t.Helper()
	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("no TUN here: %v", err)
	}
	t.Cleanup(func() { unix.Close(fd) })
	ifr, _ := unix.NewIfreq("sockmnezia") // the name xrayconf gives the tun inbound (Linux checks it)
	ifr.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	if err := unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr); err != nil {
		t.Skipf("cannot create a TUN interface: %v", err)
	}
	name := ifr.Name()
	s, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(s)
	set := func(req uint, f func(*unix.Ifreq) error) {
		r, _ := unix.NewIfreq(name)
		if err := f(r); err != nil {
			t.Fatal(err)
		}
		if err := unix.IoctlIfreq(s, req, r); err != nil {
			t.Skipf("cannot configure %s: %v", name, err)
		}
	}
	set(unix.SIOCSIFADDR, func(r *unix.Ifreq) error { return r.SetInet4Addr(addr.Addr().AsSlice()) })
	mask := net.CIDRMask(addr.Bits(), 32)
	set(unix.SIOCSIFNETMASK, func(r *unix.Ifreq) error { return r.SetInet4Addr(mask) })
	set(unix.SIOCSIFFLAGS, func(r *unix.Ifreq) error { r.SetUint16(unix.IFF_UP | unix.IFF_RUNNING); return nil })
	return fd
}

// redirectAll is a fake VLESS server whose every connection, whatever its destination, goes to target: the
// destinations behind the TUN don't exist.
func redirectAll(t *testing.T, target int) int {
	port := freePort(t)
	allow := []any{map[string]any{"action": "allow"}}
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": port, "protocol": "vless",
			"settings":       map[string]any{"clients": []any{map[string]any{"id": testUUID}}, "decryption": "none"},
			"streamSettings": map[string]any{"network": "ws", "security": "none", "wsSettings": map[string]any{"path": "/ws"}}}},
		"outbounds": []any{map[string]any{"protocol": "freedom", "settings": map[string]any{
			"redirect": fmt.Sprintf("127.0.0.1:%d", target), "finalRules": allow}}},
	}
	b, _ := json.Marshal(cfg)
	inst, err := runner.Start(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inst.Close() })
	return port
}

func TestVPNModeCarriesTrafficThroughTheChain(t *testing.T) {
	// 198.18.0.0/24 (benchmarking range): public as far as the locked rules go, routed into the TUN by the kernel.
	fd := openTun(t, netip.MustParsePrefix("198.18.0.1/24"))
	target := httpTarget(t, "through-the-tun")
	hop := redirectAll(t, target)

	os.Setenv("xray.tun.fd", strconv.Itoa(fd))
	t.Cleanup(func() { os.Unsetenv("xray.tun.fd") })
	in := xrayconf.Inbound{SocksPort: freePort(t), User: "u-vpn", Pass: "correct-horse-battery"}
	cfg, err := xrayconf.Build(xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(ownLink(hop))}, Inbound: in, VPN: &xrayconf.VPN{}})
	if err != nil {
		t.Fatal(err)
	}
	inst, err := runner.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	// A plain connection, no proxy settings: the kernel sends it into the TUN, Xray takes it from there.
	hc := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	var body []byte
	for i := 0; i < 20; i++ { // the interface may take a moment to come up
		resp, err := hc.Get("http://198.18.0.5/")
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if string(body) != "through-the-tun" {
		t.Fatalf("got %q through the TUN", body)
	}
	if up, down := inst.Traffic(); up == 0 || down == 0 {
		t.Errorf("tun traffic not counted: %d/%d", up, down)
	}
}
