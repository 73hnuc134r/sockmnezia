package tun

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// running is what the service runs; runner.Instance in the real service, a fake in tests.
type running interface {
	Close() error
	Traffic() (up, down int64)
}

// saved is the service's state file: the last config and whether it runs or blocks.
type saved struct {
	Config string `json:"config"`
	GeoDir string `json:"geo"`
	Active bool   `json:"active"`
}

// Service is the system service side.
type Service struct {
	Version  string
	StateDir string // administrators/root only
	// start runs a config; runner.Start by default.
	start func(cfg []byte) (running, error)

	mu    sync.Mutex
	inst  running
	state saved
}

func (s *Service) run(cfg []byte, geoDir string) (running, error) {
	if s.start != nil {
		return s.start(cfg)
	}
	if geoDir != "" {
		geo.SetAssetDir(geoDir)
	}
	return runner.Start(cfg)
}

func (s *Service) statePath() string { return filepath.Join(s.StateDir, "tun.json") }

// Load resumes the last state (at boot, after a restart): the config, or the same config blocking everything.
func (s *Service) Load() error {
	b, err := os.ReadFile(s.statePath())
	if err != nil {
		return nil // nothing sent yet
	}
	var st saved
	if err := json.Unmarshal(b, &st); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
	return s.applyLocked()
}

// applyLocked runs the saved state: its config when active, otherwise the config with everything blocked.
func (s *Service) applyLocked() error {
	if s.inst != nil {
		s.inst.Close()
		s.inst = nil
	}
	cfg := []byte(s.state.Config)
	if !s.state.Active {
		var err error
		if cfg, err = xrayconf.BlockAll(cfg); err != nil {
			return err
		}
	}
	if err := checkConfig(cfg); err != nil {
		return err
	}
	inst, err := s.run(cfg, s.state.GeoDir)
	if err != nil {
		return err
	}
	s.inst = inst
	return nil
}

func (s *Service) saveLocked() error {
	b, _ := json.Marshal(s.state)
	tmp := s.statePath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath())
}

// checkConfig: only audited configs with the tun inbound.
func checkConfig(cfg []byte) error {
	if err := xrayconf.Audit(cfg); err != nil {
		return fmt.Errorf("config refused: %w", err)
	}
	if !xrayconf.HasTun(cfg) {
		return errors.New("config refused: no tun inbound")
	}
	return nil
}

// Serve answers the app until the listener closes; connections come and go, the TUN stays.
func (s *Service) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer c.Close()
			s.handle(newConn(c))
		}()
	}
}

func (s *Service) handle(c *conn) {
	for {
		var req request
		if err := c.read(&req); err != nil {
			return
		}
		var resp response
		switch req.Op {
		case "hello":
			resp.Version = s.Version
			s.mu.Lock()
			resp.Active = s.state.Active && s.inst != nil
			s.mu.Unlock()
		case "start":
			if err := s.startConfig([]byte(req.Config), req.GeoDir); err != nil {
				resp.Error = err.Error()
			}
		case "stop":
			if err := s.block(); err != nil {
				resp.Error = err.Error()
			}
		case "traffic":
			s.mu.Lock()
			if s.inst != nil && s.state.Active {
				resp.Up, resp.Down = s.inst.Traffic()
			}
			s.mu.Unlock()
		default:
			resp.Error = "unknown request"
		}
		if c.write(resp) != nil {
			return
		}
	}
}

// startConfig runs cfg (unchanged when it already runs: the app attaching after a restart).
func (s *Service) startConfig(cfg []byte, geoDir string) error {
	if err := checkConfig(cfg); err != nil {
		return err
	}
	if geoDir != "" && !filepath.IsAbs(geoDir) {
		return errors.New("geo directory must be absolute")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Active && s.inst != nil && bytes.Equal([]byte(s.state.Config), cfg) && s.state.GeoDir == geoDir {
		return nil
	}
	prev := s.state
	s.state = saved{Config: string(cfg), GeoDir: geoDir, Active: true}
	if err := s.applyLocked(); err != nil {
		s.state = prev
		s.applyLocked() // back to what ran (or blocked) before
		return err
	}
	return s.saveLocked()
}

// block keeps the tun with everything blocked (the user stopped the proxy in TUN mode).
func (s *Service) block() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Config == "" {
		return nil
	}
	s.state.Active = false
	if err := s.saveLocked(); err != nil {
		return err
	}
	return s.applyLocked()
}

// Close stops Xray (the service is stopping: uninstall or shutdown); the routes go with it.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inst != nil {
		s.inst.Close()
		s.inst = nil
	}
}
