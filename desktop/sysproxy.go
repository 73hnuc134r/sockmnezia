package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/routing"
)

// System proxy (docs/10 §3, D30): while the proxy runs with a system-proxy mode, the OS proxy settings point
// at the loopback HTTP port (or at a PAC script served on loopback). The settings found before are saved to
// a file first and put back on stop, on quit, and on the next start after a crash.

// bypass lists what the OS should never send to the proxy: local names and private addresses, which the
// proxy blocks (locked rules), so the local network keeps working.
var bypass = []string{"localhost", "127.0.0.0/8", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "*.local"}

// errSysProxyUnsupported: no OS proxy settings this app knows how to change (the UI shows manual steps).
var errSysProxyUnsupported = errors.New("no supported system proxy settings on this desktop")

// osProxy is the platform part (sysproxy_<os>.go).
type osProxy interface {
	// Save returns the current settings, in a form Restore takes back.
	Save() (map[string]string, error)
	// SetHTTP points the OS at an HTTP proxy (host:port), with the bypass list.
	SetHTTP(hostport string, bypass []string) error
	// SetPAC points the OS at a proxy auto-config URL.
	SetPAC(url string) error
	Restore(map[string]string) error
}

type sysProxy struct {
	e    *engine.Engine
	os   osProxy
	file string // saved settings while applied (survives a crash)

	mu      sync.Mutex
	applied string // "", engine.SystemProxyOn, engine.SystemProxyPAC
	port    int
	err     error
	failed  string // the mode:port that failed (not retried until it changes)
	pacPort int
}

func newSysProxy(e *engine.Engine, dir string) *sysProxy {
	return &sysProxy{e: e, os: platformProxy(), file: filepath.Join(dir, "sysproxy-restore.json")}
}

// RecoverCrash puts back settings saved by a run that did not get to restore them.
func (s *sysProxy) RecoverCrash() {
	b, err := os.ReadFile(s.file)
	if err != nil {
		return
	}
	var saved map[string]string
	if json.Unmarshal(b, &saved) == nil && s.os != nil {
		s.os.Restore(saved)
	}
	os.Remove(s.file)
}

// Sync applies or removes the OS settings to match the status; called on every status tick.
func (s *sysProxy) Sync(st engine.Status) {
	key := ""
	if st.Running && !st.VPN && st.SystemProxy != "" {
		key = fmt.Sprintf("%s:%d", st.SystemProxy, st.SystemProxyPort)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := ""
	if s.applied != "" {
		cur = fmt.Sprintf("%s:%d", s.applied, s.port)
	}
	if key == cur || (s.err != nil && key == s.failed) {
		return // done, or failing: don't retry the platform every second
	}
	want := ""
	if key != "" {
		want = st.SystemProxy
	}
	if s.err = s.applyLocked(want, st.SystemProxyPort); s.err != nil {
		s.failed = key
	} else {
		s.failed = ""
	}
}

func (s *sysProxy) applyLocked(want string, port int) error {
	if s.os == nil {
		if want != "" {
			return errSysProxyUnsupported
		}
		return nil
	}
	if want == "" {
		return s.restoreLocked()
	}
	if s.applied == "" {
		saved, err := s.os.Save()
		if err != nil {
			return err
		}
		b, _ := json.Marshal(saved)
		if err := os.WriteFile(s.file, b, 0o600); err != nil {
			return err
		}
	}
	var err error
	switch want {
	case engine.SystemProxyOn:
		err = s.os.SetHTTP(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), bypass)
	case engine.SystemProxyPAC:
		if err = s.servePACLocked(); err == nil {
			err = s.os.SetPAC(fmt.Sprintf("http://127.0.0.1:%d/proxy.pac", s.pacPort))
		}
	}
	if err != nil {
		s.restoreLocked()
		return err
	}
	s.applied, s.port = want, port
	return nil
}

func (s *sysProxy) restoreLocked() error {
	s.applied, s.port = "", 0
	b, err := os.ReadFile(s.file)
	if err != nil {
		return nil // nothing applied
	}
	var saved map[string]string
	if err := json.Unmarshal(b, &saved); err != nil {
		os.Remove(s.file)
		return err
	}
	if err := s.os.Restore(saved); err != nil {
		return err
	}
	return os.Remove(s.file)
}

// Restore removes the OS settings (on quit).
func (s *sysProxy) Restore() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.os != nil {
		s.restoreLocked()
	}
}

// servePACLocked starts the loopback PAC server once; the script is built from the current rules on every
// request, so rule changes apply without touching the OS settings.
func (s *sysProxy) servePACLocked() error {
	if s.pacPort != 0 {
		return nil
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s.pacPort = l.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy.pac" {
			http.NotFound(w, r)
			return
		}
		st := s.e.Status()
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(routing.PAC(s.e.GetSettings().Routing, net.JoinHostPort("127.0.0.1", strconv.Itoa(st.SystemProxyPort)))))
	})}
	go srv.Serve(l)
	return nil
}

// SysProxyState is what the UI shows about the OS settings.
type SysProxyState struct {
	Applied string `json:"applied"` // the mode currently set in the OS, "" = none
	Error   string `json:"error"`   // why it could not be set
	// Supported is false when the app cannot change this desktop's settings (the UI shows manual steps).
	Supported bool `json:"supported"`
}

func (s *sysProxy) State() SysProxyState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := SysProxyState{Applied: s.applied, Supported: s.os != nil}
	if s.err != nil {
		st.Error = s.err.Error()
	}
	return st
}
