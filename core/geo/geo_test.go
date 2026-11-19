package geo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/internal/geotest"
)

func serve(t *testing.T, files map[string][]byte) *httptest.Server {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fixtures(t *testing.T) map[string][]byte {
	dir := t.TempDir()
	if err := geotest.Write(dir); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, n := range []string{geo.GeoIP, geo.GeoSite} {
		b, _ := os.ReadFile(filepath.Join(dir, n))
		sum := sha256.Sum256(b)
		files["/dl/"+n] = b
		files["/dl/"+n+".sha256sum"] = []byte(hex.EncodeToString(sum[:]) + "  " + n + "\n")
	}
	return files
}

func TestDownloadAndTags(t *testing.T) {
	srv := serve(t, fixtures(t))
	dir := t.TempDir()
	if err := geo.Download(context.Background(), srv.Client(), srv.URL+"/dl", dir, true); err != nil {
		t.Fatal(err)
	}
	st := geo.Stat(dir)
	if !st.HasGeoIP || !st.HasGeoSite || st.Updated.IsZero() {
		t.Fatalf("%+v", st)
	}
	tags, err := geo.Tags(filepath.Join(dir, geo.GeoSite))
	if err != nil || !reflect.DeepEqual(tags, []string{"category-ads-all", "category-ru"}) {
		t.Fatalf("%v %v", tags, err)
	}
	tags, _ = geo.Tags(filepath.Join(dir, geo.GeoIP))
	if !reflect.DeepEqual(tags, []string{"ru", "test"}) {
		t.Fatal(tags)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 2 {
		t.Fatalf("temp files left: %v", ents)
	}
}

func TestChecksumMismatchKeepsOldFiles(t *testing.T) {
	f := fixtures(t)
	dir := t.TempDir()
	srv := serve(t, f)
	if err := geo.Download(context.Background(), srv.Client(), srv.URL+"/dl", dir, true); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, geo.GeoIP))
	f["/dl/geosite.dat"] = append([]byte{}, f["/dl/geosite.dat"]...)
	f["/dl/geosite.dat"][3] ^= 0xff
	srv2 := serve(t, f)
	err := geo.Download(context.Background(), srv2.Client(), srv2.URL+"/dl", dir, true)
	if !errors.Is(err, geo.ErrChecksum) {
		t.Fatalf("want ErrChecksum, got %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, geo.GeoIP))
	if string(before) != string(after) {
		t.Fatal("geoip.dat replaced although geosite.dat failed")
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 2 {
		t.Fatalf("temp files left: %v", ents)
	}
}

func TestChecksumRequiredAndJunk(t *testing.T) {
	f := fixtures(t)
	delete(f, "/dl/geoip.dat.sha256sum")
	srv := serve(t, f)
	if err := geo.Download(context.Background(), srv.Client(), srv.URL+"/dl", t.TempDir(), true); err == nil {
		t.Fatal("missing checksum accepted")
	}
	if err := geo.Download(context.Background(), srv.Client(), srv.URL+"/dl", t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	f2 := map[string][]byte{"/dl/geoip.dat": []byte("<html>not a geo file</html>"), "/dl/geosite.dat": []byte("x")}
	srv2 := serve(t, f2)
	if err := geo.Download(context.Background(), srv2.Client(), srv2.URL+"/dl", t.TempDir(), false); err == nil {
		t.Fatal("junk accepted")
	}
	if err := geo.Download(context.Background(), srv.Client(), "http://example.com/dl", t.TempDir(), false); err == nil {
		t.Fatal("http source accepted")
	}
}
