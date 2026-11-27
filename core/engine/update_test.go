package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.1.9", 1}, {"1.2.0", "1.2.0", 0}, {"1.2.0-beta.1", "1.2.0", -1},
		{"1.2.0-beta.2", "1.2.0-beta.10", -1}, {"1.2.0-5-gabc1234", "1.2.0", 1}, {"0.1.0-dev", "0.1.0", -1},
		{"v2.0.0", "1.99.99", 1}, {"1.2.0-rc.1", "1.2.0-beta.3", 1},
	} {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("%s vs %s = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{
			map[string]any{"tag_name": "v0.3.0-beta.1", "prerelease": true, "html_url": "u3", "body": "b3"},
			map[string]any{"tag_name": "v0.2.0", "html_url": "u2", "body": "b2"},
			map[string]any{"tag_name": "v0.1.0", "html_url": "u1"},
		})
	}))
	defer srv.Close()
	e, _ := Open(t.TempDir(), nil, "linux")
	u, err := e.CheckUpdate(context.Background(), srv.URL, TagAndroid, "0.1.0", false)
	if err != nil || !u.Newer || u.Version != "0.2.0" || u.Notes != "b2" {
		t.Fatalf("%+v %v", u, err)
	}
	u, _ = e.CheckUpdate(context.Background(), srv.URL, TagAndroid, "0.2.0", true)
	if !u.Newer || u.Version != "0.3.0-beta.1" {
		t.Fatalf("beta: %+v", u)
	}
	u, _ = e.CheckUpdate(context.Background(), srv.URL, TagAndroid, "0.2.0", false)
	if u.Newer {
		t.Fatal("no update expected")
	}
}

// Android and desktop releases share the repository (D33): each app sees only its own tags.
func TestCheckUpdateTagPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{
			map[string]any{"tag_name": "desktop-v0.4.0", "html_url": "d4"},
			map[string]any{"tag_name": "v1.2.0", "html_url": "a12"},
			map[string]any{"tag_name": "desktop-v0.3.0", "html_url": "d3"},
			map[string]any{"tag_name": "vnext", "html_url": "junk"},
		})
	}))
	defer srv.Close()
	e, _ := Open(t.TempDir(), nil, "linux")
	if u, _ := e.CheckUpdate(context.Background(), srv.URL, TagAndroid, "1.1.5", false); !u.Newer || u.Version != "1.2.0" {
		t.Fatalf("android: %+v", u)
	}
	if u, _ := e.CheckUpdate(context.Background(), srv.URL, TagAndroid, "1.2.0", false); u.Newer {
		t.Fatalf("android saw a desktop release: %+v", u)
	}
	if u, _ := e.CheckUpdate(context.Background(), srv.URL, TagDesktop, "0.3.0", false); !u.Newer || u.Version != "0.4.0" || u.URL != "d4" {
		t.Fatalf("desktop: %+v", u)
	}
	if u, _ := e.CheckUpdate(context.Background(), srv.URL, TagDesktop, "0.4.0", false); u.Newer {
		t.Fatalf("desktop saw an Android release: %+v", u)
	}
}
