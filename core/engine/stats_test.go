package engine

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

type countingInst struct{ up, down, dUp, dDown int64 }

func (c *countingInst) Close() error                                   { return nil }
func (c *countingInst) Traffic() (int64, int64)                        { return c.up, c.down }
func (c *countingInst) DirectTraffic() (int64, int64)                  { return c.dUp, c.dDown }
func (c *countingInst) HTTPClient(time.Duration) *http.Client          { return http.DefaultClient }
func (c *countingInst) ExitIP(context.Context, string) (string, error) { return "", nil }

// Traffic is recorded per day: through the selected chain/server (inbound minus direct) and directly; it
// survives restarts of the engine, and a deleted server keeps the name it had.
func TestStats(t *testing.T) {
	dir := t.TempDir()
	e, _ := Open(dir, nil, "linux")
	id, _ := e.AddOwnServer("trojan://p@a.example.com:443#vps", "")
	e.SetActiveServer(id)
	inst := &countingInst{}
	launch := func([]byte, string) (Instance, error) { return inst, nil }
	sys := xrayconf.SystemTUN{Gateway: []string{"172.19.0.1/30"}, Routes: []string{"0.0.0.0/0"}}
	if err := e.StartTUN(context.Background(), sys, launch); err != nil {
		t.Fatal(err)
	}
	inst.up, inst.down, inst.dUp, inst.dDown = 1000, 5000, 100, 400
	e.Status() // samples
	inst.up, inst.down = 1500, 9000
	e.Stop() // samples and saves

	e2, _ := Open(dir, nil, "linux")
	days := e2.Stats(7)
	if len(days) != 7 || days[6].Date != time.Now().Format(time.DateOnly) {
		t.Fatalf("days: %+v", days)
	}
	got := map[string]StatsEntry{}
	for _, en := range days[6].Entries {
		got[en.Key] = en
	}
	srv, direct := got["server:"+id], got["direct"]
	if srv.Up != 1400 || srv.Down != 8600 || srv.Name != "vps" || srv.Kind != "server" {
		t.Errorf("server: %+v", srv)
	}
	if direct.Up != 100 || direct.Down != 400 || direct.Kind != "direct" {
		t.Errorf("direct: %+v", direct)
	}
	e2.mu.Lock()
	e2.st.ActiveServer = ""
	e2.mu.Unlock()
	e2.RemoveOwnServer(id)
	if en := e2.Stats(1)[0].Entries[0]; !en.Deleted || en.Name != "vps" {
		t.Errorf("deleted server: %+v", en)
	}
	if err := e2.ResetStats(); err != nil || len(e2.Stats(1)[0].Entries) != 0 {
		t.Fatal("not reset", err)
	}
}
