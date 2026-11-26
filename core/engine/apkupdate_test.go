package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDownloadUpdate(t *testing.T) {
	files := map[string][]byte{
		"sockmnezia-0.9.0-arm64-v8a.apk": []byte("arm64 apk bytes"),
		"sockmnezia-0.9.0.apk":           []byte("universal apk bytes"),
	}
	sums := func() string {
		s := ""
		for n, b := range files {
			h := sha256.Sum256(b)
			s += hex.EncodeToString(h[:]) + "  " + n + "\n"
		}
		return s
	}
	withSums := true
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api":
			var assets []any
			for n, b := range files {
				assets = append(assets, map[string]any{"name": n, "browser_download_url": srv.URL + "/download/v0.9.0/" + n, "size": len(b)})
			}
			if withSums {
				assets = append(assets, map[string]any{"name": "SHA256SUMS", "browser_download_url": srv.URL + "/download/v0.9.0/SHA256SUMS"})
			}
			json.NewEncoder(w).Encode([]any{map[string]any{"tag_name": "v0.9.0", "html_url": "u", "assets": assets}})
		case r.URL.Path == "/download/v0.9.0/SHA256SUMS":
			fmt.Fprint(w, sums())
		default:
			for n, b := range files {
				if r.URL.Path == "/download/v0.9.0/"+n {
					w.Write(b)
					return
				}
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := releaseDownloads
	releaseDownloads = srv.URL + "/download/"
	defer func() { releaseDownloads = old }()

	e, _ := Open(t.TempDir(), nil, "linux")
	dir := t.TempDir()
	if _, err := e.DownloadUpdate(context.Background(), []string{"arm64-v8a"}, dir); code(err) != CodeUpdateInvalid {
		t.Fatal("download without a check", err)
	}
	if u, err := e.CheckUpdate(context.Background(), srv.URL+"/api", "0.8.0", false); err != nil || !u.Newer {
		t.Fatal(u, err)
	}
	p, err := e.DownloadUpdate(context.Background(), []string{"arm64-v8a", "armeabi-v7a"}, dir)
	if b, _ := os.ReadFile(p); err != nil || string(b) != "arm64 apk bytes" {
		t.Fatalf("%s %q %v", p, b, err)
	}
	// No APK for the ABI: the universal one.
	p, _ = e.DownloadUpdate(context.Background(), []string{"x86_64"}, dir)
	if b, _ := os.ReadFile(p); string(b) != "universal apk bytes" {
		t.Fatalf("%q", b)
	}
	if e.UpdateProgress() != nil {
		t.Fatal("progress left after the download")
	}
	// A file that doesn't match SHA256SUMS is refused (sums computed before the swap).
	good := sums()
	files["sockmnezia-0.9.0-arm64-v8a.apk"] = []byte("tampered")
	srvSums := sums
	sums = func() string { return good }
	if _, err := e.DownloadUpdate(context.Background(), []string{"arm64-v8a"}, dir); code(err) != CodeUpdateInvalid {
		t.Fatal("tampered APK accepted", err)
	}
	sums = srvSums
	// Without SHA256SUMS nothing is downloaded.
	withSums = false
	e.CheckUpdate(context.Background(), srv.URL+"/api", "0.8.0", false)
	if _, err := e.DownloadUpdate(context.Background(), []string{"arm64-v8a"}, dir); code(err) != CodeUpdateInvalid {
		t.Fatal("download without SHA256SUMS", err)
	}
	// Files from anywhere but this repository's releases are refused.
	withSums = true
	e.CheckUpdate(context.Background(), srv.URL+"/api", "0.8.0", false)
	releaseDownloads = "https://github.com/73hnuc134r/sockmnezia/releases/download/"
	if _, err := e.DownloadUpdate(context.Background(), []string{"arm64-v8a"}, dir); code(err) != CodeUpdateInvalid {
		t.Fatal("foreign download location accepted", err)
	}
}
