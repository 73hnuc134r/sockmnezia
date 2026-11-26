package engine

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// releaseDownloads is the only place update files are taken from (this repository's release assets; GitHub
// then redirects to its file host). A variable so tests can point it at a local server.
var releaseDownloads = "https://github.com/73hnuc134r/sockmnezia/releases/download/"

// maxUpdateAPK caps a downloaded update.
const maxUpdateAPK = 300 << 20

// Asset is a file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// DownloadProgress reports a running update download: bytes Done of Total (-1 unknown), then Verifying.
type DownloadProgress struct {
	Done      int64 `json:"done"`
	Total     int64 `json:"total"`
	Verifying bool  `json:"verifying"`
}

// UpdateProgress is the progress of the running update download, nil when none runs.
func (e *Engine) UpdateProgress() *DownloadProgress {
	if p := e.updProg.Load(); p != nil {
		c := *p
		return &c
	}
	return nil
}

// pickUpdateAPK chooses the APK for the device: the first of abis (the device's, preferred first) that the release
// has, otherwise the universal one.
func pickUpdateAPK(version string, assets []Asset, abis []string) (Asset, bool) {
	byName := map[string]Asset{}
	for _, a := range assets {
		byName[a.Name] = a
	}
	for _, abi := range abis {
		if a, ok := byName["sockmnezia-"+version+"-"+abi+".apk"]; ok {
			return a, true
		}
	}
	a, ok := byName["sockmnezia-"+version+".apk"]
	return a, ok
}

// DownloadUpdate downloads the APK of the newer release found by the last CheckUpdate for the device's ABIs
// into dir, checks it against the release's SHA256SUMS and returns its path. Through the proxy while it runs.
// Installing is the app's job; Android refuses an APK signed with another key.
func (e *Engine) DownloadUpdate(ctx context.Context, abis []string, dir string) (string, error) {
	e.mu.Lock()
	u := e.lastUpdate
	inst := e.inst
	e.mu.Unlock()
	if u == nil || !u.Newer {
		return "", newErr(CodeUpdateInvalid, errors.New("no newer release; check for updates first"))
	}
	apk, ok := pickUpdateAPK(u.Version, u.Assets, abis)
	var sums Asset
	for _, a := range u.Assets {
		if a.Name == "SHA256SUMS" {
			sums = a
		}
	}
	if !ok || sums.URL == "" {
		return "", newErr(CodeUpdateInvalid, fmt.Errorf("release %s has no APK for this device or no SHA256SUMS", u.Version))
	}
	for _, a := range []Asset{apk, sums} {
		if !strings.HasPrefix(a.URL, releaseDownloads) {
			return "", newErr(CodeUpdateInvalid, fmt.Errorf("unexpected download location for %s", a.Name))
		}
	}
	if !e.updBusy.TryLock() {
		return "", newErr(CodeUpdateInvalid, errors.New("an update is already downloading"))
	}
	defer e.updBusy.Unlock()
	e.updProg.Store(&DownloadProgress{Total: apk.Size})
	defer e.updProg.Store(nil)

	client := &http.Client{Timeout: 10 * time.Minute}
	if inst != nil {
		client = inst.HTTPClient(10 * time.Minute)
	}
	get := func(url string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return resp, nil
	}

	resp, err := get(sums.URL)
	if err != nil {
		return "", newErr(CodeNetwork, err)
	}
	want := ""
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 64<<10))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == apk.Name && len(f[0]) == 64 {
			want = strings.ToLower(f[0])
		}
	}
	resp.Body.Close()
	if want == "" {
		return "", newErr(CodeUpdateInvalid, fmt.Errorf("SHA256SUMS has no entry for %s", apk.Name))
	}

	resp, err = get(apk.URL)
	if err != nil {
		return "", newErr(CodeNetwork, err)
	}
	defer resp.Body.Close()
	total := resp.ContentLength
	if total > maxUpdateAPK {
		return "", newErr(CodeUpdateInvalid, errors.New("update file too large"))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", newErr(CodeInternal, err)
	}
	f, err := os.CreateTemp(dir, ".update-*")
	if err != nil {
		return "", newErr(CodeInternal, err)
	}
	defer os.Remove(f.Name()) // no-op after the rename
	h := sha256.New()
	cw := &progressWriter{on: func(n int64) { e.updProg.Store(&DownloadProgress{Done: n, Total: total}) }}
	n, err := io.Copy(io.MultiWriter(f, h, cw), io.LimitReader(resp.Body, maxUpdateAPK+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", newErr(CodeNetwork, err)
	}
	e.updProg.Store(&DownloadProgress{Done: n, Total: total, Verifying: true})
	if n > maxUpdateAPK {
		return "", newErr(CodeUpdateInvalid, errors.New("update file too large"))
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return "", newErr(CodeUpdateInvalid, errors.New("the download does not match SHA256SUMS"))
	}
	path := filepath.Join(dir, "update.apk")
	if err := os.Rename(f.Name(), path); err != nil {
		return "", newErr(CodeInternal, err)
	}
	return path, nil
}

type progressWriter struct {
	n  int64
	on func(int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	w.on(w.n)
	return len(p), nil
}
