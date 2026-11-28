package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
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

// ShareAll returns every saved server and chain as one text (contains credentials): first the servers that
// are in no chain, one link per line, then each chain as in ShareChain. Import reads it back; other apps import
// the links. Amnezia stays a placeholder.
func (e *Engine) ShareAll() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	inChain := map[string]bool{}
	for _, c := range e.st.Chains {
		for _, h := range c.Hops {
			inChain[h.Server] = true
		}
	}
	var lines []string
	for i := range e.st.Own {
		if s := &e.st.Own[i]; s.Server != nil && !inChain[s.ID] {
			lines = append(lines, shareServerLocked(s))
		}
	}
	for i := range e.st.Chains {
		c := &e.st.Chains[i]
		lines = append(lines, ChainHeader+"?v=1&name="+url.QueryEscape(e.chainNameLocked(c)))
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
	}
	if len(lines) == 0 {
		return "", newErr(CodeNoOwnServer, errors.New("nothing saved"))
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// ImportResult describes what Import added (or would add, in a dry run).
type ImportResult struct {
	Chain        string   `json:"chain,omitempty"`    // name of the imported chain, when the text has exactly one
	ChainID      string   `json:"chain_id,omitempty"` // set after a real import of exactly one chain
	Chains       []string `json:"chains"`             // names of the chains that will be / were added
	KnownChains  []string `json:"known_chains"`       // chains already saved with the same hops (not duplicated)
	Hops         int      `json:"hops,omitempty"`     // hops of the one chain
	NewServers   []string `json:"new_servers"`        // names of servers that will be / were added
	KnownServers []string `json:"known_servers"`      // names of saved servers the text refers to (reused, not duplicated)
	NeedsAmnezia bool     `json:"needs_amnezia"`      // a chain has an Amnezia hop but no subscription is set up
}

// Import reads shared text: one or more server links / one Xray outbound JSON, a chain (header line, then its
// hops), or everything at once (ShareAll: links, then chains). Servers already saved (same outbound) are reused
// instead of duplicated. With dryRun nothing is saved. Every chain is validated before anything is written,
// so a failed import leaves nothing behind.
func (e *Engine) Import(text string, dryRun bool) (ImportResult, error) {
	res := ImportResult{Chains: []string{}, KnownChains: []string{}, NewServers: []string{}, KnownServers: []string{}}
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

	// Blocks: the lines before the first chain header are loose servers; each header starts a chain.
	type block struct {
		chain bool
		name  string
		lines []string
	}
	blocks := []block{{}}
	for _, l := range lines {
		if strings.HasPrefix(l, ChainHeader) {
			u, err := url.Parse(l)
			if err != nil {
				return res, newErr(CodeInvalidLink, fmt.Errorf("bad chain header: %v", err))
			}
			blocks = append(blocks, block{chain: true, name: strings.TrimSpace(u.Query().Get("name"))})
			continue
		}
		blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, l)
	}

	type item struct {
		amnezia bool
		srv     *links.Server
		line    string
		id      string // existing server, or the ID it gets
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	var added []item // new servers, each once
	resolve := func(l string, hop int) (item, error) {
		srv, err := links.Parse(l)
		if err != nil {
			code := CodeInvalidLink
			if errors.Is(err, links.ErrUnsupported) {
				code = CodeUnsupportedLink
			}
			return item{}, &Error{Code: code, Field: fmt.Sprintf("hop%d", hop), Detail: err.Error()}
		}
		it := item{srv: srv, line: l}
		for _, o := range e.st.Own {
			if o.Server != nil && links.SameOutbound(o.Server.Outbound, srv.Outbound) {
				it.id = o.ID
				if !slices.Contains(res.KnownServers, o.Name) {
					res.KnownServers = append(res.KnownServers, o.Name)
				}
				return it, nil
			}
		}
		for _, prev := range added { // the same new server twice in one text is added once
			if links.SameOutbound(prev.srv.Outbound, srv.Outbound) {
				it.id = prev.id
				return it, nil
			}
		}
		it.id = newID()
		added = append(added, it)
		res.NewServers = append(res.NewServers, srv.Name)
		return it, nil
	}

	var chains []Chain
	for _, b := range blocks {
		if !b.chain {
			for i, l := range b.lines {
				if l == AmneziaPlaceholder {
					return res, newErr(CodeInvalidLink, errors.New("an Amnezia hop outside a chain"))
				}
				if _, err := resolve(l, i+1); err != nil {
					return res, err
				}
			}
			continue
		}
		if len(b.lines) == 0 {
			return res, newErr(CodeChainEmpty, errors.New("the chain has no hops"))
		}
		var chain Chain
		var hops []xrayconf.Hop
		for i, l := range b.lines {
			if l == AmneziaPlaceholder {
				chain.Hops = append(chain.Hops, ChainHop{Kind: HopAmnezia})
				hops = append(hops, xrayconf.AmneziaHop(stubAmnezia))
				if e.st.Amnezia.Key == "" {
					res.NeedsAmnezia = true
				}
				continue
			}
			it, err := resolve(l, i+1)
			if err != nil {
				return res, err
			}
			chain.Hops = append(chain.Hops, ChainHop{Kind: HopServer, Server: it.id})
			hops = append(hops, xrayconf.OwnHop(it.srv.Outbound))
		}
		if _, _, err := xrayconf.Outbounds(hops); err != nil {
			return res, Classify(err)
		}
		chain.Name = b.name
		if known := e.sameChainLocked(chain.Hops); known != nil {
			res.KnownChains = append(res.KnownChains, e.chainNameLocked(known))
			continue
		}
		chains = append(chains, chain)
		res.Chains = append(res.Chains, b.name)
	}
	if len(chains) == 1 {
		res.Chain, res.Hops = chains[0].Name, len(chains[0].Hops)
	}
	if dryRun {
		return res, nil
	}

	for _, it := range added {
		e.st.Own = append(e.st.Own, OwnServer{ID: it.id, Name: it.srv.Name, Link: it.line, Server: it.srv})
	}
	for i := range chains {
		chain := chains[i]
		chain.ID = newID()
		if chain.Name == "" {
			chain.Name, chain.AutoName = e.defaultChainNameLocked(&chain), true
		}
		e.st.Chains = append(e.st.Chains, chain)
		if e.activeChainLocked() == nil {
			e.st.ActiveChain = chain.ID
		}
		res.Chains[i] = chain.Name
		if len(chains) == 1 {
			res.ChainID, res.Chain = chain.ID, chain.Name
		}
	}
	if err := e.persist(); err != nil {
		return res, newErr(CodeInternal, err)
	}
	return res, nil
}

// sameChainLocked finds a saved chain with exactly these hops.
func (e *Engine) sameChainLocked(hops []ChainHop) *Chain {
	for i := range e.st.Chains {
		if slices.Equal(e.st.Chains[i].Hops, hops) {
			return &e.st.Chains[i]
		}
	}
	return nil
}
