package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Prefs are the desktop app's own settings (the proxy's settings live in the engine state).
type Prefs struct {
	// CloseToTray: closing the window keeps the app (and the proxy) running in the tray. Default on Windows;
	// off on Linux, where not every desktop shows tray icons.
	CloseToTray bool `json:"close_to_tray"`
	// ConnectOnLaunch starts the proxy when the app opens (with autostart: on login).
	ConnectOnLaunch bool `json:"connect_on_launch"`
	CheckUpdates    bool `json:"check_updates"`
	BetaUpdates     bool `json:"beta_updates"`
	// Recent chains and servers, most recent first ("c:<id>" / "s:<id>"), for the tray menu.
	Recent []string `json:"recent"`
	// CloseHintShown: the "still running in the tray" notice was shown once.
	CloseHintShown bool `json:"close_hint_shown"`
}

type prefStore struct {
	mu   sync.Mutex
	path string
	p    Prefs
}

func loadPrefs(dir string) *prefStore {
	s := &prefStore{path: filepath.Join(dir, "desktop.json"), p: Prefs{CloseToTray: runtime.GOOS == "windows", CheckUpdates: true}}
	if b, err := os.ReadFile(s.path); err == nil {
		json.Unmarshal(b, &s.p)
	}
	return s
}

func (s *prefStore) Get() Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.p
	p.Recent = append([]string(nil), s.p.Recent...)
	return p
}

// Update changes the prefs and saves them.
func (s *prefStore) Update(f func(*Prefs)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.p)
	b, _ := json.MarshalIndent(s.p, "", "  ")
	return os.WriteFile(s.path, b, 0o600)
}

// Used records a chain or server as the most recent selection.
func (s *prefStore) Used(key string) {
	s.Update(func(p *Prefs) {
		l := []string{key}
		for _, k := range p.Recent {
			if k != key && len(l) < 16 {
				l = append(l, k)
			}
		}
		p.Recent = l
	})
}
