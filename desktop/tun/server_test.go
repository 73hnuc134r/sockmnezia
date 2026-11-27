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
			Gateway: []string{"172.19.0.1/30"}, Routes: []string{"0.0.0.0/0"}, DNS: []string{"172.19.0.2"}, Mark: Mark}}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// blocks reports whether cfg ends with the catch-all block rule (xrayconf.BlockAll).
func blocks(cfg []byte) bool {
	var m map[string]any
	json.Unmarshal(cfg, &m)
	rules := m["routing"].(map[string]any)["rules"].([]any)
	last := rules[len(rules)-1].(map[string]any)
	return last["outboundTag"] == xrayconf.TagBlock && last["network"] == "tcp,udp"
}

type runs struct{ cfgs [][]byte }

func (r *runs) start(cfg []byte) (running, error) {
	r.cfgs = append(r.cfgs, cfg)
	return &fakeRun{}, nil
}

func dial(t *testing.T, path string) *conn {
	nc, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	return newConn(nc)
}

// D40: the service runs only audited TUN configs, keeps running when the app goes away, blocks everything
// when the proxy is stopped, and comes back in the same state after a restart (boot, crash).
func TestService(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "s")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	r := &runs{}
	s := &Service{Version: "1.0", StateDir: dir, start: r.start}
	go s.Serve(l)
	defer l.Close()

	c := dial(t, sock)
	if resp, err := c.call(request{Op: "hello"}, time.Second); err != nil || resp.Version != "1.0" || resp.Active {
		t.Fatal(resp, err)
	}
	good := tunConfig(t)
	var m map[string]any
	json.Unmarshal(good, &m)
	m["log"] = map[string]any{"loglevel": "warning", "access": "/etc/cron.d/x"}
	withFile, _ := json.Marshal(m)
	for name, cfg := range map[string][]byte{"log file": withFile, "no tun": []byte(`{"inbounds":[]}`)} {
		if _, err := c.call(request{Op: "start", Config: string(cfg)}, time.Second); err == nil || !strings.Contains(err.Error(), "refused") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := c.call(request{Op: "start", Config: string(good), GeoDir: "relative"}, time.Second); err == nil {
		t.Error("relative geo dir accepted")
	}
	geo := t.TempDir()
	if _, err := c.call(request{Op: "start", Config: string(good), GeoDir: geo}, time.Second); err != nil {
		t.Fatal(err)
	}
	if resp, err := c.call(request{Op: "traffic"}, time.Second); err != nil || resp.Up != 10 {
		t.Fatal(resp, err)
	}
	c.c.Close() // the app quits: the TUN stays

	c = dial(t, sock)
	if resp, _ := c.call(request{Op: "hello"}, time.Second); !resp.Active {
		t.Fatal("not running after the app disconnected")
	}
	n := len(r.cfgs)
	if _, err := c.call(request{Op: "start", Config: string(good), GeoDir: geo}, time.Second); err != nil || len(r.cfgs) != n {
		t.Fatal("the same config was restarted (the app attaching should not interrupt traffic)", err)
	}
	if _, err := c.call(request{Op: "stop"}, time.Second); err != nil {
		t.Fatal(err)
	}
	if !blocks(r.cfgs[len(r.cfgs)-1]) {
		t.Fatal("stopped, but not blocking")
	}
	if resp, _ := c.call(request{Op: "hello"}, time.Second); resp.Active {
		t.Fatal("still active after stop")
	}

	// A restart (crash, boot) resumes blocking, then running once started again.
	r2 := &runs{}
	s2 := &Service{Version: "1.0", StateDir: dir, start: r2.start}
	if err := s2.Load(); err != nil || len(r2.cfgs) != 1 || !blocks(r2.cfgs[0]) {
		t.Fatalf("after a restart: %v %d", err, len(r2.cfgs))
	}
	c.call(request{Op: "start", Config: string(good), GeoDir: geo}, time.Second)
	r3 := &runs{}
	s3 := &Service{Version: "1.0", StateDir: dir, start: r3.start}
	if err := s3.Load(); err != nil || len(r3.cfgs) != 1 || blocks(r3.cfgs[0]) {
		t.Fatal("not resumed running")
	}
}
