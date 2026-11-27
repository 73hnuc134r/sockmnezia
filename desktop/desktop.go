package main

import (
	"encoding/base64"
	"errors"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"rsc.io/qr"
)

// Desktop is the bound service for what only the desktop app has: build info, prefs, autostart, the OS
// proxy state, QR codes.
type Desktop struct {
	d *desk
}

// Info describes the app build.
type Info struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Xray    string `json:"xray"`
	DataDir string `json:"data_dir"`
	// SecretStore is how the saved data is protected: "dpapi", "keyring" or "file" (plain, 0600).
	SecretStore string `json:"secret_store"`
}

func (s *Desktop) Info() Info {
	return Info{Version: Version, OS: runtime.GOOS, Xray: xrayVersion(), DataDir: dataDir(), SecretStore: secretStore}
}

// SetLanguage tells the Go side the UI language ("en", "ru") for the tray and native dialogs.
func (s *Desktop) SetLanguage(lang string) {
	s.d.mu.Lock()
	s.d.lang = lang
	s.d.mu.Unlock()
	go s.d.tray.Refresh(s.d.e.Status())
}

func (s *Desktop) Prefs() Prefs { return s.d.prefs.Get() }

// SetPrefs saves the editable prefs (the recent list and hints are kept).
func (s *Desktop) SetPrefs(p Prefs) error {
	return s.d.prefs.Update(func(cur *Prefs) {
		cur.CloseToTray, cur.ConnectOnLaunch, cur.CheckUpdates, cur.BetaUpdates = p.CloseToTray, p.ConnectOnLaunch, p.CheckUpdates, p.BetaUpdates
	})
}

// Autostart reports whether the app starts on login.
func (s *Desktop) Autostart() (bool, error) {
	st, err := s.d.app.Autostart.Status()
	return st.Enabled, err
}

// SetAutostart registers or removes the login item; on login the app opens to the tray when it closes to
// the tray, otherwise with its window.
func (s *Desktop) SetAutostart(on bool) error {
	if !on {
		return s.d.app.Autostart.Disable()
	}
	return s.d.app.Autostart.EnableWithOptions(application.AutostartOptions{Arguments: []string{"--hidden"}})
}

func (s *Desktop) State() DesktopState { return s.d.state() }

// Quit stops the proxy, restores the OS proxy settings and exits.
func (s *Desktop) Quit() { s.d.quit() }

// errTooLong: the text does not fit in a QR code.
var errTooLong = errors.New("too long for a QR code")

// QR returns text as a QR code PNG (data: URL). Generated locally: shared links hold credentials.
func (s *Desktop) QR(text string) (string, error) {
	if len(text) > 2000 {
		return "", errTooLong
	}
	c, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	c.Scale = 6
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(c.PNG()), nil
}

// LANAddresses lists this computer's private IPv4 addresses (for sharing the proxy on the local network).
func (s *Desktop) LANAddresses() []string {
	out := []string{}
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
				out = append(out, n.IP.String())
			}
		}
	}
	return out
}

// PickFile asks for a file to open (e.g. the official APK) and returns its path, "" when cancelled.
func (s *Desktop) PickFile(title string) (string, error) {
	return s.d.app.Dialog.OpenFile().SetTitle(title).CanChooseFiles(true).PromptForSingleSelection()
}

// SaveText asks where to save text (an export) and writes it there; false when cancelled.
func (s *Desktop) SaveText(title, name, text string) (bool, error) {
	p, err := s.d.app.Dialog.SaveFile().SetFilename(name).PromptForSingleSelection()
	if err != nil || p == "" {
		return false, err
	}
	return true, os.WriteFile(p, []byte(text), 0o600)
}

// OpenURL opens a web page in the default browser.
func (s *Desktop) OpenURL(url string) error {
	if !strings.HasPrefix(url, "https://") {
		return errors.New("only https links are opened")
	}
	return s.d.app.Browser.OpenURL(url)
}
