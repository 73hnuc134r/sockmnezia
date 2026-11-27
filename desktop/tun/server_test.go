package tun

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

type fakeRun struct{ closed bool }

func (f *fakeRun) Close() error            { f.closed = true; return nil }
func (f *fakeRun) Traffic() (int64, int64) { return 10, 20 }

func tunConfig(t *testing.T) []byte {
	s, err := links.Parse("trojan://p@a.example.com:443#a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := xrayconf.Build(xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(s.Outbound)},
		Inbound: xrayconf.Inbound{SocksPort: 23456, User: "u", Pass: "p"},
		VPN: &xrayconf.VPN{DoH: []string{"https://1.1.1.1/dns-query"}, System: &xrayconf.SystemTUN{
			Gateway: []string{"172.19.0.1/30"}, Routes: []string{"0.0.0.0/0"}, DNS: []string{"172.19.0.2"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The helper runs only audited configs with the tun inbound, and stops the TUN when the app disconnects.
func TestServer(t *testing.T) {
	l, err := net.Listen("unix", filepath.Join(t.TempDir(), "s"))
	if err != nil {
		t.Fatal(err)
	}
	run := &fakeRun{}
	s := &Server{Version: "1.0", start: func([]byte) (running, error) { return run, nil }}
	done := make(chan error)
	go func() { done <- s.Serve(l) }()

	nc, err := net.Dial("unix", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c := newConn(nc)
	if r, err := c.call(request{Op: "hello", Version: "1.0"}, time.Second); err != nil || r.Version != "1.0" {
		t.Fatal(r, err)
	}
	good := tunConfig(t)
	var m map[string]any
	json.Unmarshal(good, &m)
	m["log"] = map[string]any{"loglevel": "warning", "access": "/etc/shadow-copy"}
	withFile, _ := json.Marshal(m)
	for name, cfg := range map[string][]byte{"log file": withFile, "no tun": []byte(`{"inbounds":[]}`), "junk": []byte("x")} {
		if _, err := c.call(request{Op: "start", Config: string(cfg)}, time.Second); err == nil || !strings.Contains(err.Error(), "refused") && name != "junk" {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := c.call(request{Op: "start", Config: string(good), GeoDir: "relative/dir"}, time.Second); err == nil {
		t.Error("relative geo dir accepted")
	}
	if _, err := c.call(request{Op: "start", Config: string(good), GeoDir: t.TempDir()}, time.Second); err != nil {
		t.Fatal(err)
	}
	if r, err := c.call(request{Op: "traffic"}, time.Second); err != nil || r.Up != 10 || r.Down != 20 {
		t.Fatal(r, err)
	}
	nc.Close() // the app went away
	select {
	case <-done: // not persistent: the helper exits
	case <-time.After(3 * time.Second):
		t.Fatal("helper still serving")
	}
	if !run.closed {
		t.Fatal("TUN left running after the app disconnected")
	}
}
