package tun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/nuc134r/sockmnezia/core/engine"
)

// Socks is the local SOCKS5 proxy of the running config, through which the app makes its own requests
// (exit address, update checks) while the instance runs in the service.
type Socks struct {
	Addr, User, Pass string
}

// Remote is the instance running in the service (engine.Instance).
type Remote struct {
	c     *conn
	socks func() Socks // read when used: the engine launches with its lock held

	mu      sync.Mutex
	up, dn  int64
	updated time.Time
}

var _ engine.Instance = (*Remote)(nil)

// Close is the user stopping the proxy: the service blocks everything until it starts again (fail-closed).
func (r *Remote) Close() error {
	_, err := r.c.call(request{Op: "stop"}, 30*time.Second)
	r.c.c.Close()
	return err
}

// Detach leaves the TUN running in the service (the app quits).
func (r *Remote) Detach() { r.c.c.Close() }

// Traffic asks the service at most once a second.
func (r *Remote) Traffic() (int64, int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.updated) > 900*time.Millisecond {
		if resp, err := r.c.call(request{Op: "traffic"}, 3*time.Second); err == nil {
			r.up, r.dn = resp.Up, resp.Down
		}
		r.updated = time.Now()
	}
	return r.up, r.dn
}

// Alive reports whether the service still answers.
func (r *Remote) Alive() bool {
	_, err := r.c.call(request{Op: "traffic"}, 3*time.Second)
	return err == nil
}

func (r *Remote) HTTPClient(timeout time.Duration) *http.Client {
	so := r.socks()
	d, _ := proxy.SOCKS5("tcp", so.Addr, &proxy.Auth{User: so.User, Password: so.Pass}, &net.Dialer{Timeout: 10 * time.Second})
	cd, _ := d.(proxy.ContextDialer)
	return &http.Client{Timeout: timeout, Transport: &http.Transport{DialContext: cd.DialContext, DisableKeepAlives: true}}
}

func (r *Remote) ExitIP(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.HTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(b))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("unexpected answer from %s", url)
	}
	return ip, nil
}

// Status is what the app shows about the service.
type Status struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"` // answers on its pipe
	Active    bool   `json:"active"`  // running a config (not blocking)
	Version   string `json:"version"`
}

// Query asks the service for its state without starting anything.
func Query() Status {
	st := Status{Installed: Installed()}
	nc, err := dialService()
	if err != nil {
		return st
	}
	defer nc.Close()
	if resp, err := newConn(nc).call(request{Op: "hello"}, 3*time.Second); err == nil {
		st.Running, st.Active, st.Version = true, resp.Active, resp.Version
	}
	return st
}

// ErrOtherVersion: the installed service is another version; Install replaces it.
var ErrOtherVersion = errors.New("the TUN service is another version")

// connect reaches the service and checks its version.
func connect(version string) (*conn, error) {
	nc, err := dialService()
	if err != nil {
		return nil, err
	}
	c := newConn(nc)
	resp, err := c.call(request{Op: "hello", Version: version}, 5*time.Second)
	if err != nil {
		nc.Close()
		return nil, err
	}
	if resp.Version != version {
		nc.Close()
		return nil, ErrOtherVersion
	}
	return c, nil
}

// Launcher returns the engine's launcher for TUN mode: it sends the config to the service, installing or
// updating the service first when needed (that asks for administrator rights). socks is called later,
// when the app makes a request through the proxy (the engine holds its lock while launching).
func Launcher(version string, socks func() Socks) engine.Launcher {
	return func(cfg []byte, geoDir string) (engine.Instance, error) {
		c, err := connect(version)
		if err != nil {
			if !Installed() || errors.Is(err, ErrOtherVersion) {
				if err := Install(); err != nil {
					return nil, err
				}
				c, err = connect(version)
			}
			if err != nil {
				return nil, fmt.Errorf("the TUN service does not answer: %w", err)
			}
		}
		if _, err := c.call(request{Op: "start", Config: string(cfg), GeoDir: geoDir}, 60*time.Second); err != nil {
			c.c.Close()
			return nil, err
		}
		return &Remote{c: c, socks: socks}, nil
	}
}

// Install installs (or updates) and starts the service; one administrator prompt. It returns when the
// service answers.
func Install() error {
	if err := runPrivileged("--tun-install"); err != nil {
		return err
	}
	return waitFor(func() bool { _, err := dialService(); return err == nil }, "the TUN service did not start")
}

// Uninstall stops and removes the service, restoring the routes; one administrator prompt.
func Uninstall() error {
	if !Installed() {
		return nil
	}
	if err := runPrivileged("--tun-uninstall"); err != nil {
		return err
	}
	return waitFor(func() bool { return !Installed() }, "the TUN service was not removed")
}

func waitFor(ok func() bool, msg string) error {
	for deadline := time.Now().Add(120 * time.Second); time.Now().Before(deadline); time.Sleep(300 * time.Millisecond) {
		if ok() {
			return nil
		}
	}
	return errors.New(msg)
}
