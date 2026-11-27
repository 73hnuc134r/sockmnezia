package tun

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"

	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// running is what the helper runs; runner.Instance in the real helper, a fake in tests.
type running interface {
	Close() error
	Traffic() (up, down int64)
}

// Server is the helper side. One client at a time; when it disconnects the TUN stops and, unless the helper
// is persistent (started at login), the helper exits.
type Server struct {
	Version    string
	Persistent bool
	// start runs a config; runner.Start by default.
	start func(cfg []byte) (running, error)

	mu   sync.Mutex
	inst running
}

func (s *Server) run(cfg []byte, geoDir string) (running, error) {
	if s.start != nil {
		return s.start(cfg)
	}
	if geoDir != "" {
		geo.SetAssetDir(geoDir)
	}
	return runner.Start(cfg)
}

// Serve answers clients until the listener closes, or after the first client unless Persistent.
func (s *Server) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		s.handle(newConn(c))
		c.Close()
		s.stop()
		if !s.Persistent {
			return nil
		}
	}
}

// ErrExit asks Serve's caller to exit (an app of another version wants its own helper).
var ErrExit = errors.New("exit requested")

func (s *Server) handle(c *conn) {
	for {
		var req request
		if err := c.read(&req); err != nil {
			return
		}
		var resp response
		switch req.Op {
		case "hello":
			resp.Version = s.Version
		case "start":
			if err := s.startConfig([]byte(req.Config), req.GeoDir); err != nil {
				resp.Error = err.Error()
			}
		case "traffic":
			s.mu.Lock()
			if s.inst != nil {
				resp.Up, resp.Down = s.inst.Traffic()
			}
			s.mu.Unlock()
		case "stop":
			s.stop()
		case "exit":
			s.stop()
			c.write(resp)
			s.Persistent = false
			return
		default:
			resp.Error = "unknown request"
		}
		if c.write(resp) != nil {
			return
		}
	}
}

// startConfig runs cfg if it passes the security audit and has the tun inbound.
func (s *Server) startConfig(cfg []byte, geoDir string) error {
	if err := xrayconf.Audit(cfg); err != nil {
		return fmt.Errorf("config refused: %w", err)
	}
	if !xrayconf.HasTun(cfg) {
		return errors.New("config refused: no tun inbound")
	}
	if geoDir != "" && !filepath.IsAbs(geoDir) {
		return errors.New("geo directory must be absolute")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inst != nil {
		s.inst.Close()
		s.inst = nil
	}
	inst, err := s.run(cfg, geoDir)
	if err != nil {
		return err
	}
	s.inst = inst
	return nil
}

func (s *Server) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inst != nil {
		s.inst.Close()
		s.inst = nil
	}
}
