package engine

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

type fakeInst struct{ closed bool }

func (f *fakeInst) Close() error                                   { f.closed = true; return nil }
func (f *fakeInst) Traffic() (int64, int64)                        { return 3, 4 }
func (f *fakeInst) HTTPClient(time.Duration) *http.Client          { return http.DefaultClient }
func (f *fakeInst) ExitIP(context.Context, string) (string, error) { return "203.0.113.9", nil }

// Desktop TUN (D37): the config goes to the launcher (the elevated helper), audited, with the system TUN.
func TestStartTUN(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	var got []byte
	inst := &fakeInst{}
	launch := func(cfg []byte, geoDir string) (Instance, error) {
		got = cfg
		return inst, nil
	}
	sys := xrayconf.SystemTUN{Gateway: []string{"172.19.0.1/30"}, Routes: []string{"0.0.0.0/0"}, DNS: []string{"172.19.0.2"}}
	if err := e.StartTUN(context.Background(), sys, launch); err != nil {
		t.Fatal(err)
	}
	if err := xrayconf.Audit(got); err != nil || !strings.Contains(string(got), `"autoSystemRoutingTable"`) || !strings.Contains(string(got), xrayconf.TagDNSOut) {
		t.Fatalf("config: %v\n%s", err, got)
	}
	if st := e.Status(); !st.Running || !st.VPN || st.Up != 3 {
		t.Fatalf("status %+v", st)
	}
	e.Stop()
	if !inst.closed || e.Status().Running {
		t.Fatal("not stopped")
	}
	// A failing launcher (prompt refused) leaves the proxy stopped and out of TUN mode.
	fail := func([]byte, string) (Instance, error) { return nil, errors.New("refused") }
	if err := e.StartTUN(context.Background(), sys, fail); err == nil {
		t.Fatal("no error")
	}
	e.mu.Lock()
	tun := e.tun
	e.mu.Unlock()
	if tun != nil || e.Status().Running {
		t.Fatal("left in TUN mode")
	}
}
