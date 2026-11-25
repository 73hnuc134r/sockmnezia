package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

func TestChainsSaveSwitchDelete(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	a, _ := e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	b, _ := e.AddOwnServer("trojan://p@b.example.com:443#b", "")
	// UDP transports cannot be imported (links.Parse refuses them); the chain rule is a second line of defence.
	kcp := "kcp-server"
	e.st.Own = append(e.st.Own, OwnServer{ID: kcp, Name: "k", Server: &links.Server{Outbound: links.Outbound{
		"protocol": "vless", "settings": map[string]any{}, "streamSettings": map[string]any{"network": "kcp"}}}})
	srv := func(id string) ChainHop { return ChainHop{Kind: HopServer, Server: id} }
	amz := ChainHop{Kind: HopAmnezia}

	cases := []struct {
		hops []ChainHop
		code string
		fld  string
	}{
		{nil, CodeChainEmpty, "chain"},
		{[]ChainHop{amz, srv(a), amz}, CodeChainAmneziaTwice, "hop3"},
		{[]ChainHop{srv(a), srv(kcp)}, CodeHopNotChainable, "hop2"},
		{[]ChainHop{srv(a), srv("gone")}, CodeNoOwnServer, "hop2"},
		{[]ChainHop{srv(a), srv(a), srv(a), srv(a), srv(a), srv(a)}, CodeChainTooLong, "chain"},
	}
	for _, c := range cases {
		err := e.ValidateChain(Chain{Hops: c.hops})
		if err == nil || err.Code != c.code || err.Field != c.fld {
			t.Errorf("%v: %+v", c.hops, err)
		}
		if _, err := e.SaveChain(Chain{Hops: c.hops}); code(err) != c.code {
			t.Errorf("saved an invalid chain %v: %v", c.hops, err)
		}
	}
	// Valid without a fetched Amnezia config: Amnezia in the middle, kcp as the entry.
	if err := e.ValidateChain(Chain{Hops: []ChainHop{srv(kcp), amz, srv(b)}}); err != nil {
		t.Fatal(err)
	}
	id, err := e.SaveChain(Chain{Hops: []ChainHop{srv(a), srv(b)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Chains(); len(got) != 3 || got[2].Name != "a → b" || got[2].Active || got[2].Hops[1].Name != "b" {
		t.Fatalf("%+v", got)
	}
	if err := e.SetActiveChain(id); err != nil {
		t.Fatal(err)
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	st := e.Status()
	if !st.Running || st.ChainID != id || len(st.Chain) != 2 || st.Chain[0] != "a.example.com" || st.OwnServer != "b" || st.AmneziaHop != 0 {
		t.Fatalf("%+v", st)
	}
	// Renaming the active chain keeps it running; an Amnezia chain without a config cannot start.
	if _, err := e.SaveChain(Chain{ID: id, Name: "work", Hops: []ChainHop{srv(a), srv(b)}}); err != nil || !e.Status().Running {
		t.Fatal(err)
	}
	amzID, _ := e.SaveChain(Chain{Hops: []ChainHop{srv(a), amz}})
	if err := e.SetActiveChain(amzID); code(err) != CodeNoConfig {
		t.Fatal(err)
	}
	if s := e.Status(); s.AmneziaHop != 2 || s.Running {
		t.Fatalf("%+v", s)
	}
	if err := e.RemoveOwnServer(b); code(err) != CodeServerInUse {
		t.Fatal(err)
	}
	// Deleting the active chain falls back to the first one.
	e.DeleteChain(amzID)
	if s := e.Snapshot(); s.ActiveChain != s.Chains[0].ID {
		t.Fatal("active not moved")
	}
}

func TestMigrateV1(t *testing.T) {
	for mode, want := range map[xrayconf.Mode]int{xrayconf.ModeAmnezia: 0, xrayconf.ModeAmneziaOwn: 1, xrayconf.ModeOwn: 2} {
		dir := t.TempDir()
		e, _ := Open(dir, nil, "linux")
		id, _ := e.AddOwnServer("trojan://p@vps.example.com:443#vps", "")
		e.AddOwnServer("trojan://p@other.example.com:443#other", "")
		// Rewrite the file as a v1 state.
		var m map[string]any
		b, _ := os.ReadFile(filepath.Join(dir, stateFile))
		json.Unmarshal(b, &m)
		delete(m, "chains")
		delete(m, "active_chain")
		m["version"], m["mode"], m["active_own"] = 1, string(mode), id
		m["options"].(map[string]any)["fetch_via_own"] = true
		b, _ = json.Marshal(m)
		os.WriteFile(filepath.Join(dir, stateFile), b, 0o600)

		e, err := Open(dir, nil, "linux")
		if err != nil {
			t.Fatal(err)
		}
		s := e.Snapshot()
		if s.Version != 2 || len(s.Chains) != 3 || s.ActiveChain != s.Chains[want].ID || s.LegacyMode != "" || s.LegacyActiveOwn != "" {
			t.Fatalf("%s: %+v", mode, s)
		}
		if s.Chains[1].Name != "Amnezia → vps" || s.Chains[1].Hops[1].Server != id || s.Chains[2].Hops[0].Server != id {
			t.Fatalf("%+v", s.Chains)
		}
		if s.Options.FetchVia != id || s.Options.LegacyFetchViaOwn {
			t.Fatalf("%+v", s.Options)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, stateFile))
		var after map[string]any
		json.Unmarshal(raw, &after)
		if _, ok := after["mode"]; ok {
			t.Fatal("legacy mode still saved")
		}
	}
}

func TestFetchHops(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	a, _ := e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	e.mu.Lock()
	defer e.mu.Unlock()
	if h, err := e.fetchHopsLocked(); err != nil || len(h) != 0 {
		t.Fatal("direct by default", h, err)
	}
	e.st.Options.FetchVia = a
	if h, _ := e.fetchHopsLocked(); len(h) != 1 {
		t.Fatal("fetch_via ignored")
	}
	c := Chain{ID: "x", Hops: []ChainHop{{Kind: HopServer, Server: a}, {Kind: HopAmnezia}}}
	e.st.Chains, e.st.ActiveChain = append(e.st.Chains, c), "x"
	e.st.Options.FetchVia = ""
	if h, _ := e.fetchHopsLocked(); len(h) != 1 {
		t.Fatal("hops before Amnezia not used for the fetch")
	}
}

func TestGeoUpdateOneAtATime(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	if e.GeoProgress() != nil {
		t.Fatal("progress without an update")
	}
	e.geoBusy.Lock() // an update is running
	if err := e.UpdateGeo(context.Background()); code(err) != CodeGeoBusy {
		t.Fatal(err)
	}
	e.geoBusy.Unlock()
}
