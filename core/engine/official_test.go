package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuc134r/sockmnezia/core/internal/mockgw"
)

func TestOfficialReleaseDownload(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	var apk bytes.Buffer
	zw := zip.NewWriter(&apk)
	w, _ := zw.Create("lib/arm64-v8a/libAmneziaVPN_arm64-v8a.so")
	w.Write(append(append([]byte("junk"), g.PEM...), []byte("junk")...))
	zw.Close()

	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "9.9.9", "assets": []any{
				map[string]any{"name": "AmneziaVPN_9.9.9_x86_64.apk", "browser_download_url": srv.URL + "/x86.apk"},
				map[string]any{"name": "AmneziaVPN_9.9.9_arm64-v8a.apk", "browser_download_url": srv.URL + "/arm.apk"},
				map[string]any{"name": "AmneziaVPN_9.9.9.exe", "browser_download_url": srv.URL + "/x.exe"},
			}})
		case "/arm.apk":
			w.Write(apk.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	http.DefaultTransport = srv.Client().Transport // trust the test certificate

	e, _ := Open(t.TempDir(), nil, "android")
	n, tag, err := e.ImportGatewayKeysFromOfficialRelease(context.Background(), srv.URL+"/latest")
	if err != nil || n != 1 || tag != "9.9.9" {
		t.Fatalf("%d %s %v", n, tag, err)
	}
	if e.Snapshot().Amnezia.GatewayPEMs[0] != string(g.PEM) {
		t.Fatal("key not stored")
	}
}
