// Package tun runs the desktop TUN mode (docs/10 §3a, D37). Creating the interface and changing the system
// routes needs administrator (Windows) or root (Linux) rights, so the Xray instance with the tun inbound
// runs in a small helper process started with those rights (the same binary with --tun-helper); the app
// itself, its window and the web view, never run elevated.
//
// The app builds the config (the engine, as for every start) and sends it to the helper over a local pipe
// (Windows: a named pipe only this user can open; Linux: a socket in the user's runtime directory). The
// helper only runs configs that pass the core's security audit (xrayconf.Audit: no files written, the tun
// interface and its options fixed) and stops the TUN as soon as the app disconnects, so a crashed app never
// leaves routes behind.
package tun

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// request and response are the protocol: one JSON object per line.
type request struct {
	Op      string `json:"op"`                // hello, start, traffic, stop, exit
	Version string `json:"version,omitempty"` // hello
	Config  string `json:"config,omitempty"`  // start: the Xray config (JSON)
	GeoDir  string `json:"geo,omitempty"`     // start: where the geo files are
}

type response struct {
	Error   string `json:"error,omitempty"`
	Version string `json:"version,omitempty"`
	Up      int64  `json:"up,omitempty"`
	Down    int64  `json:"down,omitempty"`
}

// conn is one request/response stream.
type conn struct {
	mu sync.Mutex
	c  net.Conn
	r  *bufio.Reader
}

func newConn(c net.Conn) *conn { return &conn{c: c, r: bufio.NewReaderSize(c, 64<<10)} }

func (c *conn) write(v any) error {
	b, _ := json.Marshal(v)
	c.c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.c.Write(append(b, '\n'))
	return err
}

func (c *conn) read(v any) error {
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return err
	}
	if len(line) > maxMessage {
		return errors.New("message too long")
	}
	return json.Unmarshal(line, v)
}

// call sends a request and waits for its response (client side).
func (c *conn) call(req request, timeout time.Duration) (response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var resp response
	if err := c.write(req); err != nil {
		return resp, err
	}
	c.c.SetReadDeadline(time.Now().Add(timeout))
	defer c.c.SetReadDeadline(time.Time{})
	if err := c.read(&resp); err != nil {
		if errors.Is(err, io.EOF) {
			return resp, errors.New("the TUN helper closed the connection")
		}
		return resp, err
	}
	if resp.Error != "" {
		return resp, fmt.Errorf("TUN helper: %s", resp.Error)
	}
	return resp, nil
}

const maxMessage = 4 << 20
