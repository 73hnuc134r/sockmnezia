package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ReleasesAPI lists this app's releases.
const ReleasesAPI = "https://api.github.com/repos/73hnuc134r/sockmnezia/releases?per_page=20"

// Update describes the newest release.
type Update struct {
	Newer   bool   `json:"newer"`
	Version string `json:"version"`
	URL     string `json:"url"`   // release page
	Notes   string `json:"notes"` // release body (CHANGELOG section)
}

// CheckUpdate compares current with the newest release (pre-releases only when beta). Runs through the proxy
// when it is running; called only when the user enabled update checks.
func (e *Engine) CheckUpdate(ctx context.Context, apiURL, current string, beta bool) (*Update, error) {
	if apiURL == "" {
		apiURL = ReleasesAPI
	}
	e.mu.Lock()
	inst := e.inst
	e.mu.Unlock()
	client := &http.Client{Timeout: 30 * time.Second}
	if inst != nil {
		client = inst.HTTPClient(30 * time.Second)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, newErr(CodeNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, newErr(CodeNetwork, fmt.Errorf("HTTP %d", resp.StatusCode))
	}
	var rels []struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rels); err != nil {
		return nil, newErr(CodeNetwork, err)
	}
	best := &Update{}
	for _, r := range rels {
		if r.Draft || (r.Prerelease && !beta) {
			continue
		}
		v := strings.TrimPrefix(r.Tag, "v")
		if best.Version == "" || CompareVersions(v, best.Version) > 0 {
			best = &Update{Version: v, URL: r.URL, Notes: r.Body}
		}
	}
	if best.Version != "" {
		best.Newer = CompareVersions(best.Version, current) > 0
	}
	return best, nil
}

// CompareVersions compares semver-like versions: 1.2.0 > 1.2.0-beta.1 > 1.1.9; "1.2.0-5-gabc" (a build after
// the 1.2.0 tag) > 1.2.0.
func CompareVersions(a, b string) int {
	ac, ap := splitVersion(a)
	bc, bp := splitVersion(b)
	for i := 0; i < 3; i++ {
		if ac[i] != bc[i] {
			if ac[i] < bc[i] {
				return -1
			}
			return 1
		}
	}
	rank := func(p string) int { // pre-release < release < post-tag build
		switch {
		case p == "":
			return 1
		case len(p) > 0 && p[0] >= '0' && p[0] <= '9':
			return 2
		}
		return 0
	}
	ra, rb := rank(ap), rank(bp)
	if ra != rb {
		if ra < rb {
			return -1
		}
		return 1
	}
	return comparePre(ap, bp)
}

func splitVersion(v string) ([3]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	core, pre, _ := strings.Cut(v, "-")
	var n [3]int
	for i, p := range strings.SplitN(core, ".", 3) {
		n[i], _ = strconv.Atoi(p)
	}
	return n, pre
}

func comparePre(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		ai, aerr := strconv.Atoi(as[i])
		bi, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if ai != bi {
				if ai < bi {
					return -1
				}
				return 1
			}
		case as[i] != bs[i]:
			if as[i] < bs[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}
