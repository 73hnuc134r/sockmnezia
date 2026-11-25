package runner

import (
	"context"
	"errors"
	"sync"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/transport/internet"
)

// Xray resolves sockopt.dialerProxy through one process-wide outbound manager (and DNS client), set by
// whichever instance was created last. With a temporary instance next to the running proxy (connection
// check, location ping, gateway fetch through earlier hops) the proxy's chain would then look its hops up
// in the temporary instance, and after that instance closes, in a closed one. So every live instance is
// registered here and the global manager searches them all: a tag is found in whichever instance has it.
// Temporary configs carry unique tags (xrayconf.BuildDialer), so a lookup never lands in the wrong one.

var (
	liveMu sync.Mutex
	live   []*Instance
)

func register(i *Instance) {
	liveMu.Lock()
	live = append(live, i)
	liveMu.Unlock()
	reinstall()
}

func unregister(i *Instance) {
	liveMu.Lock()
	for k, x := range live {
		if x == i {
			live = append(live[:k:k], live[k+1:]...)
			break
		}
	}
	liveMu.Unlock()
	// No reinstall: the installed globals look instances up here, so dropping one is enough (and writing
	// the globals while other dials read them is what Xray itself does on every core.New).
}

// reinstall points Xray's globals at the registry (and the oldest live instance's DNS client). Called after
// anything that runs core.New, which overwrites them.
func reinstall() {
	internet.InitSystemDialer(liveDNS{}, liveOutbounds{})
}

func snapshot() []*Instance {
	liveMu.Lock()
	defer liveMu.Unlock()
	return append([]*Instance(nil), live...)
}

// liveOutbounds is an outbound.Manager over every live instance (lookups only).
type liveOutbounds struct{}

func (liveOutbounds) Type() any    { return outbound.ManagerType() }
func (liveOutbounds) Start() error { return nil }
func (liveOutbounds) Close() error { return nil }
func (liveOutbounds) GetHandler(tag string) outbound.Handler {
	for _, i := range snapshot() {
		if om, ok := i.inst.GetFeature(outbound.ManagerType()).(outbound.Manager); ok {
			if h := om.GetHandler(tag); h != nil {
				return h
			}
		}
	}
	return nil
}
func (liveOutbounds) GetDefaultHandler() outbound.Handler { return nil }
func (liveOutbounds) AddHandler(context.Context, outbound.Handler) error {
	return errors.New("not supported")
}
func (liveOutbounds) RemoveHandler(context.Context, string) error { return errors.New("not supported") }
func (liveOutbounds) ListHandlers(ctx context.Context) []outbound.Handler {
	var hs []outbound.Handler
	for _, i := range snapshot() {
		if om, ok := i.inst.GetFeature(outbound.ManagerType()).(outbound.Manager); ok {
			hs = append(hs, om.ListHandlers(ctx)...)
		}
	}
	return hs
}

// liveDNS forwards to the oldest live instance's DNS client (all of ours use the same system resolver).
type liveDNS struct{}

func (liveDNS) Type() any    { return dns.ClientType() }
func (liveDNS) Start() error { return nil }
func (liveDNS) Close() error { return nil }
func (liveDNS) LookupIP(domain string, option dns.IPOption) ([]net.IP, uint32, error) {
	for _, i := range snapshot() {
		if c, ok := i.inst.GetFeature(dns.ClientType()).(dns.Client); ok {
			return c.LookupIP(domain, option)
		}
	}
	return nil, 0, errors.New("no running Xray instance")
}
