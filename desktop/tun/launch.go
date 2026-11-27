package tun

import (
	"errors"
	"net"
	"time"

	"github.com/nuc134r/sockmnezia/core/engine"
)

// Launcher returns the engine's launcher for TUN mode: it connects to the helper, starting it with
// administrator rights when none runs (a prompt, unless no-prompt mode is set up), and sends the config.
// socks is called only later, when the app makes a request through the proxy (the engine holds its lock
// while launching).
func Launcher(version string, socks func() Socks) engine.Launcher {
	return func(cfg []byte, geoDir string) (engine.Instance, error) {
		c, err := connect(version)
		if err != nil {
			return nil, err
		}
		return start(c, cfg, geoDir, socks)
	}
}

// connect reaches a helper of this version, starting one if needed.
func connect(version string) (*conn, error) {
	if c, err := hello(version); err == nil {
		return c, nil
	}
	done, err := launchHelper()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(120 * time.Second) // time for the prompt
	for time.Now().Before(deadline) {
		select {
		case err := <-done: // the launcher (pkexec, the task) ended without a helper answering
			if c, herr := hello(version); herr == nil {
				return c, nil
			}
			if err == nil {
				err = errors.New("the TUN helper did not start")
			}
			return nil, err
		case <-time.After(300 * time.Millisecond):
		}
		if c, err := hello(version); err == nil {
			return c, nil
		}
	}
	return nil, errors.New("the TUN helper did not start in time")
}

// hello connects to a running helper and checks its version; an older helper (persistent, from before an
// update) is asked to exit.
func hello(version string) (*conn, error) {
	nc, err := dialHelper()
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
		c.call(request{Op: "exit"}, 5*time.Second)
		nc.Close()
		return nil, errors.New("TUN helper of another version")
	}
	return c, nil
}

// RunHelper is the helper process: it serves the app until it disconnects (persistent: until logout).
func RunHelper(version string, args []string) error {
	l, persistent, err := listenHelper(args)
	if err != nil {
		return err
	}
	defer l.Close()
	s := &Server{Version: version, Persistent: persistent}
	err = s.Serve(l)
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
