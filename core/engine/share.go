package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// Sharing (docs/09-multihop-redesign.md §3). A chain is shared as text: a header line, then one standard
// share link per hop, phone first. Other apps import the links as separate servers; sockmnezia rebuilds
// the chain. Amnezia is never exported: its config holds this subscription's per-device credentials, so it
// is a placeholder line that the receiver's own subscription fills.
const (
	ChainHeader        = "sockmnezia://chain"
	AmneziaPlaceholder = "sockmnezia://amnezia"
)

// Share formats.
const (
	ShareLinks = "links" // header + one link per hop (default)
	ShareXray  = "xray"  // Xray outbounds JSON with dialerProxy (chains of own servers only)
)

// shareServerLocked returns a server's share link (named as saved), or its outbound JSON when no link can
// describe it exactly.
func shareServerLocked(s *OwnServer) string {
	if link, err := links.Format(s.Server.Outbound, s.Name); err == nil {
		return link
	}
	b, _ := json.Marshal(s.Server.Outbound)
	return string(b)
}

// ShareServer returns the share link of a saved server (contains its credentials).
func (e *Engine) ShareServer(id string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.serverLocked(id)
	if s == nil || s.Server == nil {
		return "", newErr(CodeNoOwnServer, errors.New("unknown server"))
	}
	return shareServerLocked(s), nil
}

// ShareChain returns a saved chain as text (ShareLinks) or as Xray outbounds JSON (ShareXray; refused for
// a chain with Amnezia, whose config must not leave the device).
func (e *Engine) ShareChain(id, format string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.chainLocked(id)
	if c == nil {
		return "", newErr(CodeNoChain, errors.New("unknown chain"))
	}
	switch format {
	case ShareXray:
		if amneziaIndex(c) >= 0 {
			return "", &Error{Code: CodeInvalidSettings, Field: "format", Detail: "a chain with Amnezia cannot be exported as Xray JSON"}
		}
		hops, err := e.hopsLocked(c, nil)
		if err != nil {
			return "", err
		}
		outs, _, err := xrayconf.Outbounds(hops)
		if err != nil {
			return "", Classify(err)
		}
		b, _ := json.MarshalIndent(map[string]any{"outbounds": outs}, "", "  ")
		return string(b), nil
	case ShareLinks, "":
		lines := []string{ChainHeader + "?v=1&name=" + url.QueryEscape(e.chainNameLocked(c))}
		for _, h := range c.Hops {
			if h.Kind == HopAmnezia {
				lines = append(lines, AmneziaPlaceholder)
				continue
			}
			s := e.serverLocked(h.Server)
			if s == nil || s.Server == nil {
				return "", newErr(CodeNoOwnServer, errors.New("unknown server"))
			}
			lines = append(lines, shareServerLocked(s))
		}
		return strings.Join(lines, "\n") + "\n", nil
	}
	return "", &Error{Code: CodeInvalidSettings, Field: "format", Detail: "unknown format"}
}

// ImportResult describes what Import added (or would add, in a dry run).
type ImportResult struct {
	Chain        string   `json:"chain,omitempty"`    // name of the imported chain ("" when the text is not a chain)
	ChainID      string   `json:"chain_id,omitempty"` // set after a real import
	Hops         int      `json:"hops,omitempty"`
	NewServers   []string `json:"new_servers"`   // names of servers that will be / were added
	KnownServers []string `json:"known_servers"` // names of saved servers the text refers to (reused, not duplicated)
	NeedsAmnezia bool     `json:"needs_amnezia"` // the chain has an Amnezia hop but no subscription is set up
}

// Import reads shared text: a chain (header line first), or one or more server links / one Xray outbound
// JSON. Servers already saved (same outbound) are reused instead of duplicated. With dryRun nothing is
// saved. The chain is validated before anything is written, so a failed import leaves nothing behind.
func (e *Engine) Import(text string, dryRun bool) (ImportResult, error) {
	res := ImportResult{NewServers: []string{}, KnownServers: []string{}}
	text = strings.TrimSpace(text)
	var lines []string
	if strings.HasPrefix(text, "{") {
		lines = []string{text} // one (possibly pretty-printed) JSON outbound
	} else {
		for _, l := range strings.Split(text, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, l)
			}
		}
	}
	if len(lines) == 0 {
		return res, newErr(CodeInvalidLink, errors.New("nothing to import"))
	}
	isChain := strings.HasPrefix(lines[0], ChainHeader)
	if isChain {
		u, err := url.Parse(lines[0])
		if err != nil {
			return res, newErr(CodeInvalidLink, fmt.Errorf("bad chain header: %v", err))
		}
		res.Chain = strings.TrimSpace(u.Query().Get("name"))
		lines = lines[1:]
		if len(lines) == 0 {
			return res, newErr(CodeChainEmpty, errors.New("the chain has no hops"))
		}
	}

	type item struct {
		amnezia bool
		srv     *links.Server
		id      string // existing server, or the ID it gets
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	var items []item
	for i, l := range lines {
		if l == AmneziaPlaceholder {
			if !isChain {
				return res, newErr(CodeInvalidLink, errors.New("an Amnezia hop outside a chain"))
			}
			items = append(items, item{amnezia: true})
			continue
		}
		srv, err := links.Parse(l)
		if err != nil {
			code := CodeInvalidLink
			if errors.Is(err, links.ErrUnsupported) {
				code = CodeUnsupportedLink
			}
			return res, &Error{Code: code, Field: fmt.Sprintf("hop%d", i+1), Detail: err.Error()}
		}
		it := item{srv: srv}
		for _, o := range e.st.Own {
			if o.Server != nil && links.SameOutbound(o.Server.Outbound, srv.Outbound) {
				it.id = o.ID
				res.KnownServers = append(res.KnownServers, o.Name)
				break
			}
		}
		if it.id == "" {
			// The same new server twice in one text is added once.
			for _, prev := range items {
				if prev.srv != nil && links.SameOutbound(prev.srv.Outbound, srv.Outbound) {
					it.id = prev.id
				}
			}
			if it.id == "" {
				it.id = newID()
				res.NewServers = append(res.NewServers, srv.Name)
			}
		}
		items = append(items, it)
	}

	var chain Chain
	if isChain {
		var hops []xrayconf.Hop
		for _, it := range items {
			if it.amnezia {
				chain.Hops = append(chain.Hops, ChainHop{Kind: HopAmnezia})
				hops = append(hops, xrayconf.AmneziaHop(stubAmnezia))
				res.NeedsAmnezia = e.st.Amnezia.Key == ""
			} else {
				chain.Hops = append(chain.Hops, ChainHop{Kind: HopServer, Server: it.id})
				hops = append(hops, xrayconf.OwnHop(it.srv.Outbound))
			}
		}
		if _, _, err := xrayconf.Outbounds(hops); err != nil {
			return res, Classify(err)
		}
		res.Hops = len(chain.Hops)
	}
	if dryRun {
		return res, nil
	}

	for i, it := range items {
		if it.srv != nil && e.serverLocked(it.id) == nil {
			e.st.Own = append(e.st.Own, OwnServer{ID: it.id, Name: it.srv.Name, Link: lines[i], Server: it.srv})
		}
	}
	if isChain {
		chain.ID, chain.Name = newID(), res.Chain
		if chain.Name == "" {
			chain.Name, chain.AutoName = e.defaultChainNameLocked(&chain), true
		}
		e.st.Chains = append(e.st.Chains, chain)
		if e.activeChainLocked() == nil {
			e.st.ActiveChain = chain.ID
		}
		res.ChainID, res.Chain = chain.ID, chain.Name
	}
	if err := e.persist(); err != nil {
		return res, newErr(CodeInternal, err)
	}
	return res, nil
}
