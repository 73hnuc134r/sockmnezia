// Command sockmnezia-desktop is the desktop app (Windows, Linux): the shared Go core with a web UI in the
// OS webview (Wails). Plan: docs/10-desktop-plan.md.
package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version is set at build time (-ldflags "-X main.Version=…").
var Version = "0.0.0-dev"

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
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Fatal(err)
	}
	e, err := engine.Open(dir, engine.PlainBox{}, runtime.GOOS)
	if err != nil {
		log.Fatal(err)
	}
	runner.Redactor = e.Redact

	core := &Core{e: e}
	app := application.New(application.Options{
		Name:        "sockmnezia",
		Description: "Local SOCKS5/HTTP proxy with chains of your own servers and Amnezia",
		Services: []application.Service{
			application.NewService(core),
			application.NewService(&Desktop{version: Version}),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		OnShutdown: func() {
			e.Stop()
		},
	})
	core.changed = func() { app.Event.Emit(EventChanged) }

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "sockmnezia",
		Width:            1100,
		Height:           720,
		MinWidth:         860,
		MinHeight:        560,
		BackgroundColour: application.NewRGB(18, 19, 24),
		URL:              "/",
	})

	// The UI follows the status through events instead of polling the core.
	go func() {
		for range time.Tick(time.Second) {
			app.Event.Emit(EventStatus, e.Status())
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// Events sent to the UI.
const (
	EventStatus  = "status"  // engine.Status, every second
	EventChanged = "changed" // saved state changed: reload lists
)

func init() {
	application.RegisterEvent[engine.Status](EventStatus)
	application.RegisterEvent[application.Void](EventChanged)
}
