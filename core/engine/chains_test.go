package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	if got := e.Chains(); len(got) != 2 || got[1].Name != "a → b" || got[1].Active || got[1].Hops[1].Name != "b" {
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
	for mode, want := range map[xrayconf.Mode]int{xrayconf.ModeAmnezia: 0, xrayconf.ModeAmneziaOwn: 1, xrayconf.ModeOwn: -1} {
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
		delete(m, "active_server")
		m["version"], m["mode"], m["active_own"] = 1, string(mode), id
		m["options"].(map[string]any)["fetch_via_own"] = true
		b, _ = json.Marshal(m)
		os.WriteFile(filepath.Join(dir, stateFile), b, 0o600)

		e, err := Open(dir, nil, "linux")
		if err != nil {
			t.Fatal(err)
		}
		s := e.Snapshot()
		// "Own server only" selects the server itself; the other modes select a chain.
		ok := s.ActiveServer == id && s.ActiveChain == ""
		if want >= 0 {
			ok = s.ActiveServer == "" && s.ActiveChain == s.Chains[want].ID
		}
		if s.Version != 2 || len(s.Chains) != 2 || !ok || s.LegacyMode != "" || s.LegacyActiveOwn != "" {
			t.Fatalf("%s: %+v", mode, s)
		}
		if s.Chains[1].Name != "Amnezia → vps" || s.Chains[1].Hops[1].Server != id {
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
	e.st.Chains, e.st.ActiveChain, e.st.ActiveServer = append(e.st.Chains, c), "x", ""
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

func TestSingleServerSelection(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	e.SetKey("") // no Amnezia: the first server is selected on its own
	a, _ := e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	b, _ := e.AddOwnServer("trojan://p@b.example.com:443#b", "")
	if st := e.Status(); st.ServerID != a || st.ChainID != "" || st.ChainName != "a" || len(st.Hops) != 1 {
		t.Fatalf("%+v", st)
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	// Selecting a chain clears the server and the other way round; chains list it as active only then.
	amz := e.Chains()[0].ID
	if err := e.SetActiveServer(b); err != nil || e.Status().ServerID != b || e.Chains()[0].Active {
		t.Fatal(err)
	}
	e.SetActiveChain(amz) // fails to restart (no Amnezia config) but is selected
	if st := e.Status(); st.ServerID != "" || st.ChainID != amz || !e.Chains()[0].Active {
		t.Fatalf("%+v", st)
	}
	if err := e.SetActiveServer("gone"); code(err) != CodeNoOwnServer {
		t.Fatal(err)
	}
	// Deleting the selected server falls back to the first chain.
	e.SetActiveServer(b)
	if err := e.RemoveOwnServer(b); err != nil {
		t.Fatal(err)
	}
	if s := e.Snapshot(); s.ActiveServer != "" || s.ActiveChain != amz {
		t.Fatalf("%+v", s)
	}
}

func TestTestSavedLeavesTheSelection(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	a, _ := e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	b, _ := e.AddOwnServer("trojan://p@b.example.com:443#b", "")
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	before := e.Status()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// The servers do not exist, so the request fails; what matters is that it ran and changed nothing.
	if _, _, _, err := e.TestSaved(ctx, "", b); code(err) != CodeNetwork {
		t.Fatal(err)
	}
	if _, _, _, err := e.TestSaved(ctx, "", "gone"); code(err) != CodeNoOwnServer {
		t.Fatal(err)
	}
	if _, _, _, err := e.TestSaved(ctx, e.Chains()[0].ID, ""); code(err) != CodeNoConfig {
		t.Fatal("an Amnezia chain without a config was tested", err)
	}
	after := e.Status()
	if !after.Running || after.ServerID != a || after.StartedAt != before.StartedAt {
		t.Fatalf("testing changed the running proxy: %+v", after)
	}
}

func TestUpdateOwnServer(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	a, _ := e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	b, _ := e.AddOwnServer("trojan://p@b.example.com:443#b", "")
	cid, _ := e.SaveChain(Chain{Name: "pair", Hops: []ChainHop{{Kind: HopServer, Server: a}, {Kind: HopServer, Server: b}}})
	if err := e.UpdateOwnServer(b, "vless://00000000-0000-4000-8000-000000000001@b2.example.com:8443?type=ws&path=%2Fw#b2", ""); err != nil {
		t.Fatal(err)
	}
	link, ep, err := e.ServerForEdit(b)
	if err != nil || ep == nil || ep.Address != "b2.example.com" || ep.Port != 8443 || ep.Name != "b2" || ep.Path != "/w" {
		t.Fatalf("%s %+v %v", link, ep, err)
	}
	// Same ID: the chain follows the edit.
	for _, c := range e.Chains() {
		if c.ID == cid && c.Hops[1].Name != "b2" {
			t.Fatalf("%+v", c)
		}
	}
	// An invalid edit is refused and leaves the server as it was.
	if err := e.UpdateOwnServer(b, "vless://00000000-0000-4000-8000-000000000001@b.example.com:443?type=kcp", ""); code(err) != CodeUnsupportedLink {
		t.Fatal(err)
	}
	if _, ep, _ := e.ServerForEdit(b); ep.Address != "b2.example.com" {
		t.Fatal("refused edit changed the server")
	}
	if err := e.UpdateOwnServer("gone", "trojan://p@x.example.com:443", ""); code(err) != CodeNoOwnServer {
		t.Fatal(err)
	}
}

func TestLastTestKept(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	a, _ := e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if e.LastTest("", a) != nil {
		t.Fatal("result before any test")
	}
	e.TestSaved(ctx, "", a) // fails: the server doesn't exist
	r := e.LastTest("", a)
	if r == nil || r.Error != CodeNetwork || r.At.IsZero() {
		t.Fatalf("%+v", r)
	}
	// Kept across a restart of the app, dropped when the server changes or goes away.
	dir := e.dir
	e2, _ := Open(dir, nil, "linux")
	if e2.LastTest("", a) == nil {
		t.Fatal("result not saved")
	}
	e2.UpdateOwnServer(a, "trojan://p@b.example.com:443#a", "")
	if e2.LastTest("", a) != nil {
		t.Fatal("result kept after the server changed")
	}
	cid, _ := e2.SaveChain(Chain{Hops: []ChainHop{{Kind: HopServer, Server: a}}})
	e2.TestSaved(ctx, cid, "")
	if c := e2.Chains(); c[len(c)-1].Last == nil {
		t.Fatal("chain result not listed")
	}
	if err := e2.ClearTests(); err != nil || len(e2.Snapshot().Tests) != 0 || e2.Chains()[len(e2.Chains())-1].Last != nil {
		t.Fatal("results kept after ClearTests", err)
	}
	e2.TestSaved(ctx, cid, "")
	e2.DeleteChain(cid)
	if e2.LastTest(cid, "") != nil {
		t.Fatal("result kept for a deleted chain")
	}
}

func TestAutoChainName(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	a, _ := e.AddOwnServer("trojan://p@a.example.com:443#a", "")
	b, _ := e.AddOwnServer("trojan://p@b.example.com:443#b", "")
	id, _ := e.SaveChain(Chain{Hops: []ChainHop{{Kind: HopServer, Server: a}, {Kind: HopServer, Server: b}}})
	name := func() (string, bool) {
		for _, c := range e.Chains() {
			if c.ID == id {
				return c.Name, c.AutoName
			}
		}
		return "", false
	}
	if n, auto := name(); n != "a → b" || !auto {
		t.Fatal(n, auto)
	}
	// Follows a rename and a changed hop list.
	e.RenameOwnServer(b, "b2")
	if n, _ := name(); n != "a → b2" {
		t.Fatal(n)
	}
	e.SaveChain(Chain{ID: id, Hops: []ChainHop{{Kind: HopServer, Server: b}, {Kind: HopServer, Server: a}}})
	if n, _ := name(); n != "b2 → a" {
		t.Fatal(n)
	}
	// A name the user typed stays as it is.
	e.SaveChain(Chain{ID: id, Name: "work", Hops: []ChainHop{{Kind: HopServer, Server: b}, {Kind: HopServer, Server: a}}})
	e.RenameOwnServer(a, "a2")
	if n, auto := name(); n != "work" || auto {
		t.Fatal(n, auto)
	}
}
