package main

import (
	"path/filepath"
	"testing"

	"github.com/nuc134r/sockmnezia/core/engine"
)

type fakeOS struct {
	cur   map[string]string
	calls []string
}

func (f *fakeOS) Save() (map[string]string, error) {
	m := map[string]string{}
	for k, v := range f.cur {
		m[k] = v
	}
	return m, nil
}
func (f *fakeOS) SetHTTP(hp string, _ []string) error {
	f.cur = map[string]string{"mode": "http", "addr": hp}
	f.calls = append(f.calls, "http")
	return nil
}
func (f *fakeOS) SetPAC(url string) error {
	f.cur = map[string]string{"mode": "pac"}
	f.calls = append(f.calls, "pac")
	return nil
}
func (f *fakeOS) Restore(m map[string]string) error {
	f.cur = m
	f.calls = append(f.calls, "restore")
	return nil
}

// D38: the OS settings stay pointed at the proxy whether it runs or not, across quitting and crashes; only
// choosing Off puts the user's own settings back.
func TestSysProxyFailClosed(t *testing.T) {
	dir := t.TempDir()
	osp := &fakeOS{cur: map[string]string{"mode": "none"}}
	s := &sysProxy{os: osp, file: filepath.Join(dir, "sysproxy-restore.json")}
	on := engine.Status{SystemProxy: engine.SystemProxyOn, SystemProxyPort: 18080}

	s.Sync(on) // chosen while the proxy is stopped: set right away
	if osp.cur["mode"] != "http" {
		t.Fatalf("not applied: %v", osp.cur)
	}
	on.Running = true
	s.Sync(on)
	on.Running = false
	s.Sync(on) // stopped: still set
	if osp.cur["mode"] != "http" {
		t.Fatal("removed when the proxy stopped")
	}

	// Quit or crash, then a new run: nothing is restored on its own, and the original settings are kept.
	s2 := &sysProxy{os: osp, file: s.file}
	s2.adopt()
	s2.Sync(on)
	if osp.cur["mode"] != "http" {
		t.Fatal("removed after a restart")
	}
	for _, c := range osp.calls {
		if c == "restore" {
			t.Fatal("restored without the user choosing Off")
		}
	}

	// Off restores what was there before the first change.
	s2.Sync(engine.Status{})
	if osp.cur["mode"] != "none" {
		t.Fatalf("not restored: %v", osp.cur)
	}

	// Mode turned Off while sockmnezia was not running (state reset): the next run restores.
	s3 := &sysProxy{os: osp, file: s.file}
	s3.Sync(on)
	s4 := &sysProxy{os: osp, file: s.file}
	s4.adopt()
	s4.Sync(engine.Status{})
	if osp.cur["mode"] != "none" {
		t.Fatalf("left applied with the mode off: %v", osp.cur)
	}
}
