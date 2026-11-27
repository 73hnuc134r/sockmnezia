// Command sockmnezia-desktop is the desktop app (Windows, Linux): the shared Go core with a web UI in the
// OS webview (Wails). Plan: docs/10-desktop-plan.md.
package main

import (
	"embed"
	"html"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/nuc134r/sockmnezia/desktop/tun"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version is set at build time (-ldflags "-X main.Version=…").
var Version = "0.0.0-dev"

// Events sent to the UI.
const (
	EventStatus   = "status"   // engine.Status, every second
	EventDesktop  = "desktop"  // DesktopState, every second
	EventChanged  = "changed"  // saved state changed: reload lists
	EventNavigate = "navigate" // page to show (from the tray)
)

func init() {
	application.RegisterEvent[engine.Status](EventStatus)
	application.RegisterEvent[DesktopState](EventDesktop)
	application.RegisterEvent[application.Void](EventChanged)
	application.RegisterEvent[string](EventNavigate)
}

// dataDir holds the state, geo files and logs: per user, private.
func dataDir() string {
	if d := os.Getenv("SOCKMNEZIA_DESKTOP_DIR"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "sockmnezia", "desktop")
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "sockmnezia")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share", "sockmnezia")
}

func main() {
	// The TUN service and its setup run with administrator rights and never open a window (D40).
	if len(os.Args) > 1 {
		run := map[string]func() error{
			"--tun-service":   func() error { return tun.RunService(Version, os.Args[2:]) },
			"--tun-install":   func() error { return tun.RunInstall(os.Args[2:]) },
			"--tun-uninstall": func() error { return tun.RunUninstall(os.Args[2:]) },
		}[os.Args[1]]
		if run != nil {
			if err := run(); err != nil {
				log.Fatal(err)
			}
			return
		}
	}
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Fatal(err)
	}
	e, err := engine.Open(dir, secretBox(dir), runtime.GOOS)
	if err != nil {
		fatalWindow(err)
		return
	}
	runner.Redactor = e.Redact
	d := &desk{e: e, prefs: loadPrefs(dir), sys: newSysProxy(e, dir), lang: "en"}
	d.core = &Core{e: e, changed: d.changed, selected: d.prefs.Used, start: d.startProxy, setMode: d.setMode}

	d.app = application.New(application.Options{
		Name:        "sockmnezia",
		Description: "Local SOCKS5/HTTP proxy with chains of your own servers and Amnezia",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(d.core),
			application.NewService(&Desktop{d: d}),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "io.nuc134r.sockmnezia.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { d.show() },
		},
		OnShutdown: func() {
			// TUN keeps running in the service when the app quits; everything else stops.
			if !d.detachTUN() {
				e.Stop()
			}
		},
	})

	hidden := slices.Contains(os.Args[1:], "--hidden")
	d.win = d.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "sockmnezia",
		Width:            1100,
		Height:           720,
		MinWidth:         860,
		MinHeight:        560,
		Hidden:           hidden && d.prefs.Get().CloseToTray,
		BackgroundColour: application.NewRGB(18, 19, 24),
		URL:              "/",
	})
	d.win.RegisterHook(events.Common.WindowClosing, d.onClose)
	d.tray = newTray(d)

	if d.prefs.Get().ConnectOnLaunch || d.tunActive() {
		go d.start() // in TUN mode with the service running: attach to it (no restart)
	}
	go d.loop()

	if err := d.app.Run(); err != nil {
		log.Fatal(err)
	}
}

// loop pushes the status to the UI and keeps the system proxy and the tray in step with it.
func (d *desk) loop() {
	for i := 0; ; i++ {
		if i%3600 == 30 { // hourly, starting half a minute after launch
			go d.geoAutoUpdate()
		}
		time.Sleep(time.Second)
		st := d.e.Status()
		if i%5 == 0 {
			d.checkTUN(st)
		}
		d.sys.Sync(st)
		d.app.Event.Emit(EventStatus, st)
		d.app.Event.Emit(EventDesktop, d.state())
		d.tray.Refresh(st)
	}
}

// fatalWindow explains why the saved data could not be opened (e.g. the keyring holding its key is not
// available). The file is left as it is: nothing is overwritten.
func fatalWindow(err error) {
	app := application.New(application.Options{Name: "sockmnezia", Icon: appIcon})
	msg := html.EscapeString(err.Error())
	dir := html.EscapeString(dataDir())
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "sockmnezia", Width: 640, Height: 300,
		HTML: `<body style="font:14px system-ui;padding:24px;line-height:1.5"><h3>sockmnezia cannot open its saved data</h3>` +
			`<p>` + msg + `</p><p>The data in <code>` + dir + `</code> was not changed. If the system keyring is locked or ` +
			`not running, unlock or start it and open sockmnezia again.</p></body>`,
	})
	app.Run()
}
