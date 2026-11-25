// Package runner runs a generated config in embedded Xray-core and offers in-process dialing through it.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/infra/conf/serial"

	xnet "github.com/xtls/xray-core/common/net"
)

// Instance is a running Xray.
type Instance struct {
	inst   *core.Instance
	closed atomic.Bool
}

// Start loads cfg (JSON) and starts it. Listening errors (port in use) are returned here.
func Start(cfg []byte) (*Instance, error) {
	pb, err := serial.LoadJSONConfig(bytes.NewReader(cfg))
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	inst, err := core.New(pb)
	if err != nil {
		reinstall()
		return nil, fmt.Errorf("config: %w", err)
	}
	i := &Instance{inst: inst}
	register(i) // before Start: core.New replaced the dialer globals
	if err := inst.Start(); err != nil {
		unregister(i)
		inst.Close()
		return nil, err
	}
	attachLog()
	return i, nil
}

// Check loads cfg without starting anything (config validation by Xray itself).
func Check(cfg []byte) error {
	pb, err := serial.LoadJSONConfig(bytes.NewReader(cfg))
	if err != nil {
		return err
	}
	inst, err := core.New(pb)
	reinstall() // core.New replaced the dialer globals
	if err != nil {
		return err
	}
	return inst.Close()
}

// Close stops the instance and frees its ports.
func (i *Instance) Close() error {
	if i.closed.Swap(true) {
		return nil
	}
	unregister(i)
	return i.inst.Close()
}

// Dial opens a TCP connection to addr through the instance's routing (as if it came from the local proxy,
// including the locked block rules), without a listening port.
func (i *Instance) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, errors.New("only tcp")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	dest := xnet.TCPDestination(xnet.ParseAddress(host), xnet.Port(p))
	return core.Dial(ctx, i.inst, dest)
}

// HTTPClient returns a client whose connections go through the instance.
func (i *Instance) HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:             nil,
			DialContext:       i.Dial,
			DisableKeepAlives: true,
		},
	}
}

// Traffic returns bytes sent and received through the local proxy inbounds since start.
func (i *Instance) Traffic() (up, down int64) {
	m, _ := i.inst.GetFeature(stats.ManagerType()).(stats.Manager)
	if m == nil {
		return 0, 0
	}
	for _, tag := range []string{"socks-in", "http-in"} {
		if c := m.GetCounter("inbound>>>" + tag + ">>>traffic>>>uplink"); c != nil {
			up += c.Value()
		}
		if c := m.GetCounter("inbound>>>" + tag + ">>>traffic>>>downlink"); c != nil {
			down += c.Value()
		}
	}
	return
}

// ExitIP asks url (a "what is my IP" endpoint returning the address as text) through the instance.
func (i *Instance) ExitIP(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := i.HTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	ip := string(bytes.TrimSpace(b))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("unexpected answer from %s", url)
	}
	return ip, nil
}
