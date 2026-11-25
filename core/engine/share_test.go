package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nuc134r/sockmnezia/core/links"
)

func TestShareAndImportChain(t *testing.T) {
	a, _ := Open(t.TempDir(), nil, "linux")
	entry, _ := a.AddOwnServer("vless://00000000-0000-4000-8000-000000000001@entry.example.com:443?type=ws&security=tls&path=%2Fws#entry", "")
	exit, _ := a.AddOwnServer("trojan://secret@exit.example.com:443#exit", "")
	a.RenameOwnServer(exit, "My exit")
	id, err := a.SaveChain(Chain{Name: "Work & play", Hops: []ChainHop{{Kind: HopServer, Server: entry}, {Kind: HopAmnezia}, {Kind: HopServer, Server: exit}}})
	if err != nil {
		t.Fatal(err)
	}
	text, err := a.ShareChain(id, ShareLinks)
	if err != nil {
		t.Fatal(err)
	}
	l := strings.Split(strings.TrimSpace(text), "\n")
	if len(l) != 4 || !strings.HasPrefix(l[0], ChainHeader) || l[2] != AmneziaPlaceholder ||
		!strings.HasPrefix(l[1], "vless://") || !strings.HasSuffix(l[3], "#My%20exit") {
		t.Fatal(text)
	}
	if _, err := a.ShareChain(id, ShareXray); code(err) != CodeInvalidSettings {
		t.Fatal("Amnezia chain exported as Xray JSON", err)
	}

	// Another install already has the exit server: it is reused, the entry is added, Amnezia needs setting up.
	b, _ := Open(t.TempDir(), nil, "linux")
	known, _ := b.AddOwnServer("trojan://secret@exit.example.com:443", "Exit at b")
	dry, err := b.Import(text, true)
	if err != nil || dry.Chain != "Work & play" || dry.Hops != 3 || len(dry.NewServers) != 1 || dry.NewServers[0] != "entry" ||
		len(dry.KnownServers) != 1 || dry.KnownServers[0] != "Exit at b" || !dry.NeedsAmnezia || dry.ChainID != "" {
		t.Fatalf("%+v %v", dry, err)
	}
	if n := len(b.Snapshot().Own); n != 1 {
		t.Fatal("dry run saved servers")
	}
	res, err := b.Import(text, false)
	if err != nil || res.ChainID == "" {
		t.Fatal(res, err)
	}
	s := b.Snapshot()
	var got *Chain
	for i := range s.Chains {
		if s.Chains[i].ID == res.ChainID {
			got = &s.Chains[i]
		}
	}
	if len(s.Own) != 2 || got == nil || len(got.Hops) != 3 || got.Hops[1].Kind != HopAmnezia || got.Hops[2].Server != known {
		t.Fatalf("%+v %+v", s.Own, got)
	}
	if s.Own[1].Link != l[1] {
		t.Fatal("imported server lost its own link")
	}
}

func TestShareServerAndXray(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	a, _ := e.AddOwnServer("trojan://secret@a.example.com:443#a", "")
	// A raw JSON outbound with settings no link can carry is shared as JSON.
	j, _ := e.AddOwnServer(`{"protocol":"trojan","settings":{"servers":[{"address":"j.example.com","port":443,"password":"p"}]},"streamSettings":{"network":"tcp","security":"tls"},"mux":{"enabled":true}}`, "json")
	if l, err := e.ShareServer(a); err != nil || l != "trojan://secret@a.example.com:443?security=tls&sni=a.example.com&type=tcp#a" {
		t.Fatal(l, err)
	}
	if l, _ := e.ShareServer(j); !strings.HasPrefix(l, "{") || !strings.Contains(l, `"mux"`) {
		t.Fatal(l)
	}
	if s, err := links.Parse(mustShare(t, e, j)); err != nil || s.Outbound["mux"] == nil {
		t.Fatal("JSON share does not import back", err)
	}
	id, _ := e.SaveChain(Chain{Hops: []ChainHop{{Kind: HopServer, Server: a}, {Kind: HopServer, Server: j}}})
	x, err := e.ShareChain(id, ShareXray)
	var m map[string][]map[string]any
	if err != nil || json.Unmarshal([]byte(x), &m) != nil || len(m["outbounds"]) != 4 || m["outbounds"][0]["tag"] != "hop2" {
		t.Fatal(x, err)
	}
	// Importing several plain links adds servers, no chain; known ones are not duplicated.
	res, err := e.Import("trojan://secret@a.example.com:443#again\ntrojan://p2@c.example.com:443#c\n", false)
	if err != nil || res.Chain != "" || len(res.NewServers) != 1 || len(res.KnownServers) != 1 || len(e.Snapshot().Own) != 3 {
		t.Fatalf("%+v %v", res, err)
	}
	// Invalid chains are refused before anything is saved.
	if _, err := e.Import(ChainHeader+"?v=1&name=x\n"+AmneziaPlaceholder+"\n"+AmneziaPlaceholder+"\ntrojan://p3@d.example.com:443\n", false); code(err) != CodeChainAmneziaTwice {
		t.Fatal(err)
	}
	if len(e.Snapshot().Own) != 3 {
		t.Fatal("a failed import saved a server")
	}
}

func mustShare(t *testing.T, e *Engine, id string) string {
	l, err := e.ShareServer(id)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
