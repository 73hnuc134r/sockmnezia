// Package tun runs the desktop TUN mode (docs/10 §3a, D40). Creating the interface and the system routes
// needs administrator (Windows) or root (Linux) rights, so the Xray instance with the tun inbound runs in a
// system service that is installed when the user chooses TUN mode (one prompt) and removed when they choose
// another mode (one prompt): a Windows service (LocalSystem) or a systemd unit (root). The app, its window
// and the web view never run elevated.
//
// The service keeps running when the app is closed and starts at boot, with the last config. The app
// builds every config (the engine) and sends it over a local pipe that only the installing user can open; the
// service runs only configs that pass the core's security audit (xrayconf.Audit) and have the tun inbound.
// It is fail-closed: while TUN mode is chosen but the proxy is stopped, it runs the same config with every
// connection blocked (xrayconf.BlockAll), so the routes stay and nothing leaves around them; on Linux its
// firewall rules also drop what isn't Xray's own traffic while it restarts after a crash.
//
// The service binary is a copy in an administrators-only directory, never the user-writable one the app runs
// from (that would let any program of the user run code as SYSTEM/root); after an app update the copy is
// replaced (one prompt).
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

// ServiceName is the Windows service / systemd unit name.
const ServiceName = "sockmnezia-tun"

// request and response are the protocol: one JSON object per line.
type request struct {
	Op      string `json:"op"`                // hello, start, stop, traffic
	Version string `json:"version,omitempty"` // hello
	Config  string `json:"config,omitempty"`  // start: the Xray config (JSON)
	GeoDir  string `json:"geo,omitempty"`     // start: where the geo files are
}

type response struct {
	Error   string `json:"error,omitempty"`
	Version string `json:"version,omitempty"`
	Active  bool   `json:"active,omitempty"` // hello: running a config (not blocking)
	Up      int64  `json:"up,omitempty"`
	Down    int64  `json:"down,omitempty"`
}

const maxMessage = 4 << 20

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
	line, err := c.r.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		var buf []byte
		buf = append(buf, line...)
		for errors.Is(err, bufio.ErrBufferFull) && len(buf) <= maxMessage {
			line, err = c.r.ReadSlice('\n')
			buf = append(buf, line...)
		}
		line = buf
	}
	if err != nil {
		return err
	}
	if len(line) > maxMessage {
		return errors.New("message too long")
	}
	return json.Unmarshal(line, v)
}

// call sends a request and waits for its response (app side).
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
			return resp, errors.New("the TUN service closed the connection")
		}
		return resp, err
	}
	if resp.Error != "" {
		return resp, fmt.Errorf("TUN service: %s", resp.Error)
	}
	return resp, nil
}

// ErrDeclined: the user did not allow the install (UAC or password prompt cancelled).
var ErrDeclined = errors.New("administrator rights were not granted")

// Mark is put on every socket Xray opens in TUN mode (Linux SO_MARK); the kill switch lets exactly those
// leave outside the tun.
const Mark = 0x534d58
