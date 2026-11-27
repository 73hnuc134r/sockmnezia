package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/appicon.png
var appIcon []byte

//go:embed frontend/src/lib/strings.json
var stringsJSON []byte

// desk ties the engine to the desktop: window, tray, system proxy, prefs.
type desk struct {
	app   *application.App
	e     *engine.Engine
	core  *Core
	prefs *prefStore
	sys   *sysProxy
	win   *application.WebviewWindow
	tray  *tray

	tunw tunWatch

	mu       sync.Mutex
	lang     string
	quitting bool
}

// DesktopState is pushed to the UI with the status.
type DesktopState struct {
	SysProxy SysProxyState `json:"sys_proxy"`
}

func (d *desk) state() DesktopState { return DesktopState{SysProxy: d.sys.State()} }

var translations = func() map[string]map[string]any {
	var m map[string]map[string]any
	json.Unmarshal(stringsJSON, &m)
	return m
}()

// tr translates a UI string for the tray and native dialogs (the web UI translates its own).
func (d *desk) tr(key string) string {
	d.mu.Lock()
	lang := d.lang
	d.mu.Unlock()
	for _, l := range []string{lang, "en"} {
		if s, ok := translations[l][key].(string); ok {
			return s
		}
	}
	return key
}

func (d *desk) changed() {
	d.app.Event.Emit(EventChanged)
	go d.tray.Refresh(d.e.Status())
}

func (d *desk) show() {
	d.win.Show()
	d.win.UnMinimise()
	d.win.Focus()
}

func (d *desk) start() {
	if err := d.startProxy(context.Background()); err != nil {
		d.notify(engine.Classify(err))
	}
	d.changed()
}

// startProxy starts the proxy in the configured mode: TUN (through the helper) or proxy only. The time limit
// leaves room for the administrator prompt.
func (d *desk) startProxy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	if d.e.GetSettings().Options.Mode == engine.ModeVPN {
		return d.startTUN(ctx)
	}
	return d.e.Start(ctx)
}

// setMode switches between Off / System proxy ("", "system") and TUN ("tun"); a running proxy
// restarts in the new mode.
func (d *desk) setMode(mode string) error {
	s := d.e.GetSettings()
	wasTUN := s.Options.Mode == engine.ModeVPN
	if mode == "tun" {
		s.Options.Mode, s.Options.SystemProxy = engine.ModeVPN, ""
	} else {
		s.Options.Mode, s.Options.SystemProxy = engine.ModeProxy, mode
	}
	if err := d.e.SetSettings(s); err != nil {
		return err
	}
	if d.e.Status().Running && wasTUN != (mode == "tun") {
		d.e.Stop()
		err := d.startProxy(context.Background())
		d.changed()
		return err
	}
	d.changed()
	return nil
}

func (d *desk) stop() {
	d.e.Stop()
	d.changed()
}

// selectKey switches to a chain ("c:<id>") or single server ("s:<id>") from the tray.
func (d *desk) selectKey(key string) {
	var err error
	if id, ok := strings.CutPrefix(key, "c:"); ok {
		err = d.core.SetActiveChain(id)
	} else if id, ok := strings.CutPrefix(key, "s:"); ok {
		err = d.core.SetActiveServer(id)
	}
	if err != nil {
		d.notify(engine.Classify(err))
	}
}

func (d *desk) setSystemProxy(mode string) {
	if err := d.setMode(mode); err != nil {
		d.notify(engine.Classify(err))
	}
}

func (d *desk) copySocks() {
	in := d.e.GetSettings().Inbound
	d.app.Clipboard.SetText(fmt.Sprintf("socks5://%s:%s@%s:%d", in.User, in.Pass, in.Listen, in.SocksPort))
}

// notify shows an error from a tray action: in the window when it is visible, otherwise as a dialog.
func (d *desk) notify(e *engine.Error) {
	if e == nil {
		return
	}
	msg := d.tr("err_" + e.Code)
	if strings.HasPrefix(msg, "err_") {
		msg = d.tr("err_internal")
	}
	if e.Detail != "" {
		msg += "\n" + e.Detail
	}
	d.app.Dialog.Error().SetTitle("sockmnezia").SetMessage(msg).Show()
}

// onClose hides the window to the tray when that is on (the proxy keeps running); otherwise it quits.
func (d *desk) onClose(ev *application.WindowEvent) {
	d.mu.Lock()
	quitting := d.quitting
	d.mu.Unlock()
	p := d.prefs.Get()
	if quitting || !p.CloseToTray {
		d.quit()
		return
	}
	ev.Cancel()
	d.win.Hide()
	if !p.CloseHintShown {
		d.prefs.Update(func(p *Prefs) { p.CloseHintShown = true })
		d.app.Dialog.Info().SetTitle("sockmnezia").SetMessage(d.tr("d_close_hint")).Show()
	}
}

func (d *desk) quit() {
	d.mu.Lock()
	d.quitting = true
	d.mu.Unlock()
	d.app.Quit()
}

// geoAutoUpdate refreshes the geo files weekly when the user turned that on (through the proxy while it runs).
func (d *desk) geoAutoUpdate() {
	if !d.e.GetSettings().Options.GeoAutoUpdate || d.e.GeoProgress() != nil {
		return
	}
	if g := d.e.GeoStatus(); g.HasGeoIP && g.HasGeoSite && time.Since(g.Updated) < 7*24*time.Hour {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := d.e.UpdateGeo(ctx); err == nil {
		d.changed()
	}
}
