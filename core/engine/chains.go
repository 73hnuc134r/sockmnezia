package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// HopInfo describes one hop of a chain for the UI (no credentials).
type HopInfo struct {
	Kind     string `json:"kind"`             // HopAmnezia or HopServer
	Server   string `json:"server,omitempty"` // own server ID
	Name     string `json:"name"`             // server name, or the Amnezia location name
	Country  string `json:"country,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Network  string `json:"network,omitempty"`
	Security string `json:"security,omitempty"`
	Port     int    `json:"port,omitempty"`
	Missing  bool   `json:"missing,omitempty"` // the server was deleted (cannot happen through the API)
}

// ChainInfo is a saved chain as the UI shows it.
type ChainInfo struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Hops   []HopInfo   `json:"hops"`
	Active bool        `json:"active"`
	Error  *Error      `json:"error,omitempty"` // why the chain cannot run, nil when it can
	Last   *TestResult `json:"last,omitempty"`  // the last Test of this chain
}

// TestResult is the outcome of the last Test of a saved chain or server.
type TestResult struct {
	At      time.Time `json:"at"`
	Ms      int64     `json:"ms,omitempty"`
	IP      string    `json:"ip,omitempty"`
	Country string    `json:"country,omitempty"`
	Error   string    `json:"error,omitempty"` // error code when it failed
}

func testKey(chainID, serverID string) string {
	if serverID != "" {
		return "s:" + serverID
	}
	return "c:" + chainID
}

// ClearTests forgets every remembered Test result ("Test all" starts from a clean list).
func (e *Engine) ClearTests() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.st.Tests) == 0 {
		return nil
	}
	e.st.Tests = nil
	return e.persist()
}

// LastTest returns the last Test result of a saved chain or server, nil if never tested.
func (e *Engine) LastTest(chainID, serverID string) *TestResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.st.Tests[testKey(chainID, serverID)]; ok {
		return &r
	}
	return nil
}

// stubAmnezia stands in for the Amnezia config when none is fetched yet, so a chain can be validated
// (Amnezia's own transports are TCP, like the gateway's real answer).
var stubAmnezia = json.RawMessage(`{"outbounds":[{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"amnezia.invalid","port":443,"users":[{"id":"00000000-0000-4000-8000-000000000000","encryption":"none"}]}]}}]}`)

func (e *Engine) serverLocked(id string) *OwnServer {
	for i := range e.st.Own {
		if e.st.Own[i].ID == id {
			return &e.st.Own[i]
		}
	}
	return nil
}

func (e *Engine) chainLocked(id string) *Chain {
	for i := range e.st.Chains {
		if e.st.Chains[i].ID == id {
			return &e.st.Chains[i]
		}
	}
	return nil
}

// activeChainLocked returns what the proxy runs: the active saved chain, or a one-hop chain (no ID) for the
// active single server; nil when nothing is selected.
func (e *Engine) activeChainLocked() *Chain {
	if id := e.st.ActiveServer; id != "" {
		if s := e.serverLocked(id); s != nil {
			return &Chain{Name: s.Name, Hops: []ChainHop{{Kind: HopServer, Server: id}}}
		}
		return nil
	}
	return e.chainLocked(e.st.ActiveChain)
}

// selectFallbackLocked selects the first saved chain (or nothing) after the selection was deleted.
func (e *Engine) selectFallbackLocked() {
	e.st.ActiveChain, e.st.ActiveServer = "", ""
	if len(e.st.Chains) > 0 {
		e.st.ActiveChain = e.st.Chains[0].ID
	}
}

// restartOrStopLocked applies a changed selection: restarts a running proxy, or stops it when nothing is
// selected any more.
func (e *Engine) restartOrStopLocked() error {
	if e.activeChainLocked() == nil {
		e.stopLocked()
		return nil
	}
	return e.restartIfRunningLocked()
}

// SetActiveServer selects a single own server (no chain) and restarts a running proxy.
func (e *Engine) SetActiveServer(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.serverLocked(id) == nil {
		return newErr(CodeNoOwnServer, errors.New("unknown server"))
	}
	if e.st.ActiveServer == id {
		return nil
	}
	e.st.ActiveChain, e.st.ActiveServer = "", id
	if err := e.persist(); err != nil {
		return err
	}
	return e.restartIfRunningLocked()
}

func amneziaIndex(c *Chain) int {
	if c == nil {
		return -1
	}
	for i, h := range c.Hops {
		if h.Kind == HopAmnezia {
			return i
		}
	}
	return -1
}

// hopsLocked resolves a saved chain into xrayconf hops. amz is the Amnezia config to use for the Amnezia
// hop (nil: the chain must not need one, CodeNoConfig otherwise).
func (e *Engine) hopsLocked(c *Chain, amz json.RawMessage) ([]xrayconf.Hop, error) {
	if c == nil {
		return nil, newErr(CodeNoChain, errors.New("no chain selected"))
	}
	if err := checkShape(c); err != nil {
		return nil, err
	}
	hops := make([]xrayconf.Hop, 0, len(c.Hops))
	for i, h := range c.Hops {
		switch h.Kind {
		case HopAmnezia:
			if amz == nil {
				return nil, newErr(CodeNoConfig, errors.New("no Amnezia config fetched yet"))
			}
			hops = append(hops, xrayconf.AmneziaHop(amz))
		case HopServer:
			s := e.serverLocked(h.Server)
			if s == nil || s.Server == nil {
				return nil, &Error{Code: CodeNoOwnServer, Field: fmt.Sprintf("hop%d", i+1), Detail: "unknown server"}
			}
			hops = append(hops, xrayconf.OwnHop(s.Server.Outbound))
		default:
			return nil, &Error{Code: CodeInvalidSettings, Field: fmt.Sprintf("hop%d", i+1), Detail: "unknown hop kind"}
		}
	}
	return hops, nil
}

// checkShape checks what does not need the servers: length and Amnezia at most once.
func checkShape(c *Chain) error {
	var hops []xrayconf.Hop
	for _, h := range c.Hops {
		if h.Kind == HopAmnezia {
			hops = append(hops, xrayconf.AmneziaHop(stubAmnezia))
		} else {
			hops = append(hops, xrayconf.OwnHop(links.Outbound{"protocol": "socks", "settings": map[string]any{"address": "hop.invalid", "port": 1}}))
		}
	}
	if _, _, err := xrayconf.Outbounds(hops); err != nil {
		return Classify(err)
	}
	return nil
}

func (e *Engine) amneziaConfigLocked() json.RawMessage {
	if e.st.Amnezia.Last == nil {
		return nil
	}
	return e.st.Amnezia.Last.XrayConfig
}

// validateLocked reports why c cannot run, using a stand-in for an Amnezia config not fetched yet (that
// is reported when starting, not as a problem of the chain).
func (e *Engine) validateLocked(c *Chain) *Error {
	amz := e.amneziaConfigLocked()
	if amz == nil {
		amz = stubAmnezia
	}
	hops, err := e.hopsLocked(c, amz)
	if err == nil {
		_, _, err = xrayconf.Outbounds(hops)
	}
	return Classify(err)
}

func (e *Engine) hopInfoLocked(h ChainHop) HopInfo {
	v := HopInfo{Kind: h.Kind, Server: h.Server}
	switch h.Kind {
	case HopAmnezia:
		v.Name = "Amnezia"
		if l := e.st.Amnezia.Last; l != nil {
			v.Country = l.Country
			if l.CountryName != "" {
				v.Name = l.CountryName
			}
		}
	case HopServer:
		s := e.serverLocked(h.Server)
		if s == nil {
			v.Missing = true
			break
		}
		v.Name = s.Name
		if s.Server != nil {
			v.Protocol, v.Network, v.Security, v.Port = s.Server.Protocol, s.Server.Network, s.Server.Security, s.Server.Port
		}
	}
	return v
}

func (e *Engine) chainInfoLocked(c *Chain) ChainInfo {
	ci := ChainInfo{ID: c.ID, Name: c.Name, Active: e.st.ActiveServer == "" && c.ID == e.st.ActiveChain, Error: e.validateLocked(c)}
	if r, ok := e.st.Tests[testKey(c.ID, "")]; ok {
		ci.Last = &r
	}
	for _, h := range c.Hops {
		ci.Hops = append(ci.Hops, e.hopInfoLocked(h))
	}
	return ci
}

// Chains lists the saved chains.
func (e *Engine) Chains() []ChainInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := []ChainInfo{}
	for i := range e.st.Chains {
		out = append(out, e.chainInfoLocked(&e.st.Chains[i]))
	}
	return out
}

// ValidateChain reports why a chain (being edited, not necessarily saved) cannot run; nil when it can.
func (e *Engine) ValidateChain(c Chain) *Error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.validateLocked(&c)
}

func (e *Engine) defaultChainNameLocked(c *Chain) string {
	var names []string
	for _, h := range c.Hops {
		if h.Kind == HopAmnezia {
			names = append(names, "Amnezia")
		} else if s := e.serverLocked(h.Server); s != nil {
			names = append(names, s.Name)
		}
	}
	return strings.Join(names, " → ")
}

// SaveChain adds (empty ID) or replaces a chain after validating it; returns its ID. The first chain saved
// when none is active becomes active. Saving the active chain restarts a running proxy.
func (e *Engine) SaveChain(c Chain) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c.Name = strings.TrimSpace(c.Name)
	if err := e.validateLocked(&c); err != nil {
		return "", err
	}
	if c.Name == "" {
		c.Name = e.defaultChainNameLocked(&c)
	}
	if c.ID == "" {
		c.ID = newID()
		e.st.Chains = append(e.st.Chains, c)
	} else if old := e.chainLocked(c.ID); old != nil {
		*old = c
	} else {
		return "", newErr(CodeNoChain, errors.New("unknown chain"))
	}
	if e.activeChainLocked() == nil {
		e.st.ActiveChain = c.ID
	}
	if err := e.persist(); err != nil {
		return "", newErr(CodeInternal, err)
	}
	if c.ID == e.st.ActiveChain && e.st.ActiveServer == "" {
		return c.ID, e.restartIfRunningLocked()
	}
	return c.ID, nil
}

// DeleteChain removes a chain. When it was active the first remaining one becomes active (a running proxy
// switches to it, or stops when no chain is left).
func (e *Engine) DeleteChain(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := e.st.Chains[:0]
	for _, c := range e.st.Chains {
		if c.ID != id {
			out = append(out, c)
		}
	}
	e.st.Chains = out
	delete(e.st.Tests, testKey(id, ""))
	if e.st.ActiveChain != id {
		return e.persist()
	}
	e.selectFallbackLocked()
	if err := e.persist(); err != nil {
		return err
	}
	return e.restartOrStopLocked()
}

// SetActiveChain selects the chain the proxy runs (restarts a running proxy).
func (e *Engine) SetActiveChain(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.chainLocked(id) == nil {
		return newErr(CodeNoChain, errors.New("unknown chain"))
	}
	if e.st.ActiveChain == id && e.st.ActiveServer == "" {
		return nil
	}
	e.st.ActiveChain, e.st.ActiveServer = id, ""
	if err := e.persist(); err != nil {
		return err
	}
	return e.restartIfRunningLocked()
}

// chainsUsingLocked returns the names of the chains that use server id.
func (e *Engine) chainsUsingLocked(id string) []string {
	var names []string
	for _, c := range e.st.Chains {
		for _, h := range c.Hops {
			if h.Kind == HopServer && h.Server == id {
				names = append(names, c.Name)
				break
			}
		}
	}
	return names
}

// ChainsUsing returns the names of the saved chains that use own server id.
func (e *Engine) ChainsUsing(id string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.chainsUsingLocked(id)
}

// fetchHopsLocked returns the hops the gateway request goes through: the active chain's hops before
// Amnezia when there are any (fetching around them would skip what the user put in front of Amnezia),
// otherwise the FetchVia server, otherwise none (direct).
func (e *Engine) fetchHopsLocked() ([]xrayconf.Hop, error) {
	c := e.activeChainLocked()
	if i := amneziaIndex(c); i > 0 {
		prefix := Chain{Hops: c.Hops[:i]}
		return e.hopsLocked(&prefix, nil)
	}
	if id := e.st.Options.FetchVia; id != "" {
		s := e.serverLocked(id)
		if s == nil || s.Server == nil {
			return nil, &Error{Code: CodeNoOwnServer, Field: "fetch_via", Detail: "the server to fetch through is gone"}
		}
		return []xrayconf.Hop{xrayconf.OwnHop(s.Server.Outbound)}, nil
	}
	return nil, nil
}
