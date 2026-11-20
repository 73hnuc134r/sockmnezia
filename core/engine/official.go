package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OfficialReleaseAPI is where the official Android app is published (docs/05 option 2).
const OfficialReleaseAPI = "https://api.github.com/repos/amnezia-vpn/amnezia-client/releases/latest"

const maxAPK = 600 << 20

// pickAPK chooses the arm64 APK, or any APK.
func pickAPK(assets []releaseAsset) *releaseAsset {
	var any *releaseAsset
	for i := range assets {
		n := strings.ToLower(assets[i].Name)
		if !strings.HasSuffix(n, ".apk") {
			continue
		}
		if strings.Contains(n, "arm64") {
			return &assets[i]
		}
		if any == nil {
			any = &assets[i]
		}
	}
	return any
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// ImportGatewayKeysFromOfficialRelease downloads the latest official Amnezia APK from its GitHub releases,
// extracts the gateway key candidates and deletes the APK. Nothing from the APK is executed. While the proxy
// runs the download goes through it.
func (e *Engine) ImportGatewayKeysFromOfficialRelease(ctx context.Context, apiURL string) (int, string, error) {
	if apiURL == "" {
		apiURL = OfficialReleaseAPI
	}
	e.mu.Lock()
	inst := e.inst
	e.mu.Unlock()
	client := &http.Client{Timeout: 10 * time.Minute}
	if inst != nil {
		client = inst.HTTPClient(10 * time.Minute)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", newErr(CodeNetwork, err)
	}
	var rel struct {
		TagName string         `json:"tag_name"`
		Assets  []releaseAsset `json:"assets"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel)
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		return 0, "", newErr(CodeNetwork, fmt.Errorf("release list: HTTP %d %v", resp.StatusCode, err))
	}
	a := pickAPK(rel.Assets)
	if a == nil {
		return 0, "", newErr(CodeNoGatewayKey, errors.New("the latest official release has no APK"))
	}
	if !strings.HasPrefix(a.URL, "https://") {
		return 0, "", newErr(CodeNetwork, errors.New("asset URL is not https"))
	}
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	resp, err = client.Do(req)
	if err != nil {
		return 0, "", newErr(CodeNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, "", newErr(CodeNetwork, fmt.Errorf("download: HTTP %d", resp.StatusCode))
	}
	f, err := os.CreateTemp(e.dir, ".apk-*")
	if err != nil {
		return 0, "", newErr(CodeInternal, err)
	}
	defer os.Remove(f.Name())
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxAPK+1))
	f.Close()
	if err != nil {
		return 0, "", newErr(CodeNetwork, err)
	}
	if n > maxAPK {
		return 0, "", newErr(CodeNetwork, errors.New("APK too large"))
	}
	count, err := e.ImportGatewayKeysFromFile(f.Name())
	return count, rel.TagName, err
}
