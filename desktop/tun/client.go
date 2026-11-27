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
// (exit address, update checks) while the instance runs in the helper.
type Socks struct {
	Addr, User, Pass string
}

// Remote is the instance running in the helper (engine.Instance). Close stops the TUN.
type Remote struct {
	c     *conn
	socks func() Socks // read when used: the engine launches with its lock held

	mu      sync.Mutex
	up, dn  int64
	updated time.Time
}

var _ engine.Instance = (*Remote)(nil)

// Start sends the config to a connected helper.
func start(c *conn, cfg []byte, geoDir string, socks func() Socks) (*Remote, error) {
	if _, err := c.call(request{Op: "start", Config: string(cfg), GeoDir: geoDir}, 60*time.Second); err != nil {
		c.c.Close()
		return nil, err
	}
	return &Remote{c: c, socks: socks}, nil
}

func (r *Remote) Close() error {
	r.c.call(request{Op: "stop"}, 10*time.Second)
	return r.c.c.Close() // the helper stops and, unless persistent, exits
}

// Traffic asks the helper at most once a second.
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

// Alive reports whether the helper still answers (it is gone when the TUN broke down).
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

// ErrDeclined: the user did not allow the helper (UAC or password prompt cancelled).
var ErrDeclined = errors.New("administrator rights were not granted")
