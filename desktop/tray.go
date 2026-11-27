package main

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	//go:embed icons/tray-stopped.png
	iconStopped []byte
	//go:embed icons/tray-running.png
	iconRunning []byte
	//go:embed icons/tray-error.png
	iconError []byte
)

// tray is the system tray icon and menu (docs/10 §4.3): state, start/stop, switch chain/server (recent
// first), system proxy mode, copy the SOCKS address, open, quit. The menu is rebuilt when what it shows
// changes.
type tray struct {
	d   *desk
	t   *application.SystemTray
	mu  sync.Mutex
	sig string
}

const trayRecent = 8

func newTray(d *desk) *tray {
	tr := &tray{d: d, t: d.app.SystemTray.New()}
	tr.t.SetIcon(iconStopped)
	tr.t.SetTooltip("sockmnezia")
	tr.t.OnClick(d.show)
	tr.Refresh(d.e.Status())
	return tr
}

func (tr *tray) Refresh(st engine.Status) {
	servers, chains := tr.d.e.ServerViews(), tr.d.e.Chains()
	recent := tr.d.prefs.Get().Recent
	name := currentName(st, servers)
	failed := !st.Running && st.LastError != nil
	sig := fmt.Sprint(st.Running, failed, st.ChainID, st.ServerID, st.SystemProxy, st.Mode, name, len(servers), len(chains), recent, tr.d.lang)
	for _, c := range chains {
		sig += c.ID + c.Name
	}
	for _, s := range servers {
		sig += s.ID + s.Name
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if sig == tr.sig {
		return
	}
	tr.sig = sig

	T := tr.d.tr
	switch {
	case st.Running:
		tr.t.SetIcon(iconRunning)
		tr.t.SetTooltip("sockmnezia · " + name)
	case failed:
		tr.t.SetIcon(iconError)
		tr.t.SetTooltip("sockmnezia · " + T("conn_stopped"))
	default:
		tr.t.SetIcon(iconStopped)
		tr.t.SetTooltip("sockmnezia · " + T("conn_stopped"))
	}

	m := tr.d.app.NewMenu()
	head := T("conn_stopped")
	if st.Running {
		head = T("d_running") + " · " + name
	}
	m.Add(head).SetEnabled(false)
	m.AddSeparator()
	if st.Running {
		m.Add(T("action_stop")).OnClick(func(*application.Context) { tr.d.stop() })
	} else {
		m.Add(T("action_start")).OnClick(func(*application.Context) { go tr.d.start() })
	}

	sub := m.AddSubmenu(T("selector_title"))
	type item struct{ key, label string }
	var cs, ss []item
	for _, c := range chains {
		if c.Error == nil {
			cs = append(cs, item{"c:" + c.ID, c.Name})
		}
	}
	for _, s := range servers {
		ss = append(ss, item{"s:" + s.ID, s.Name})
	}
	order := func(l []item) []item { // recent first, then by name; at most trayRecent
		slices.SortStableFunc(l, func(a, b item) int {
			ia, ib := slices.Index(recent, a.key), slices.Index(recent, b.key)
			if ia < 0 {
				ia = 1 << 20
			}
			if ib < 0 {
				ib = 1 << 20
			}
			if ia != ib {
				return ia - ib
			}
			return strings.Compare(strings.ToLower(a.label), strings.ToLower(b.label))
		})
		return l[:min(len(l), trayRecent)]
	}
	active := "c:" + st.ChainID
	if st.ServerID != "" {
		active = "s:" + st.ServerID
	}
	add := func(title string, l []item) {
		if len(l) == 0 {
			return
		}
		sub.Add(title).SetEnabled(false)
		for _, it := range order(l) {
			key := it.key
			sub.AddRadio(it.label, key == active).OnClick(func(*application.Context) { tr.d.selectKey(key) })
		}
	}
	add(T("selector_chains"), cs)
	if len(cs) > 0 && len(ss) > 0 {
		sub.AddSeparator()
	}
	add(T("selector_servers"), ss)
	if len(cs) > trayRecent || len(ss) > trayRecent {
		sub.AddSeparator()
		sub.Add(T("d_more")).OnClick(func(*application.Context) { tr.d.show(); tr.d.app.Event.Emit(EventNavigate, "servers") })
	}

	mode := m.AddSubmenu(T("d_mode"))
	cur := st.SystemProxy
	if st.Mode == engine.ModeVPN {
		cur = "tun"
	}
	for _, o := range []struct{ v, label string }{{"", "d_mode_off"}, {engine.SystemProxyOn, "d_mode_system"}, {engine.SystemProxyPAC, "d_mode_pac"}, {"tun", "d_mode_tun"}} {
		v := o.v
		mode.AddRadio(T(o.label), cur == v).OnClick(func(*application.Context) { go tr.d.setSystemProxy(v) })
	}
	m.Add(T("d_copy_socks")).OnClick(func(*application.Context) { tr.d.copySocks() })
	m.AddSeparator()
	m.Add(T("d_open")).OnClick(func(*application.Context) { tr.d.show() })
	m.Add(T("d_quit")).OnClick(func(*application.Context) { tr.d.quit() })
	tr.t.SetMenu(m)
}

// currentName is the active chain's or single server's name.
func currentName(st engine.Status, servers []engine.ServerView) string {
	if st.ServerID != "" {
		for _, s := range servers {
			if s.ID == st.ServerID {
				return s.Name
			}
		}
	}
	return st.ChainName
}
