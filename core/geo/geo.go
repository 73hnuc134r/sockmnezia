// Package geo manages geoip.dat / geosite.dat for routing rules (plan §5.3): sources, download with checksum
// verification, atomic replacement and tag listing.
package geo

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
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

// File names Xray looks for.
const (
	GeoIP   = "geoip.dat"
	GeoSite = "geosite.dat"
)

// Source is where geo files come from.
type Source struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"` // files are BaseURL + "/geoip.dat" etc.; checksums at + ".sha256sum"
}

// Sources the user can pick (same as v2rayNG); a custom BaseURL is also allowed.
var Sources = []Source{
	{ID: "runetfreedom", Name: "runetfreedom/russia-v2ray-rules-dat", BaseURL: "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download"},
	{ID: "loyalsoldier", Name: "Loyalsoldier/v2ray-rules-dat", BaseURL: "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download"},
	{ID: "chocolate4u", Name: "Chocolate4U/Iran-v2ray-rules", BaseURL: "https://github.com/Chocolate4U/Iran-v2ray-rules/releases/latest/download"},
}

// SourceByID returns a known source.
func SourceByID(id string) (Source, bool) {
	for _, s := range Sources {
		if s.ID == id {
			return s, true
		}
	}
	return Source{}, false
}

// SetAssetDir tells Xray where to find geo files. Process-wide.
func SetAssetDir(dir string) {
	os.Setenv("xray.location.asset", dir)
	os.Setenv("XRAY_LOCATION_ASSET", dir)
}

// Status of the files in a directory.
type Status struct {
	HasGeoIP, HasGeoSite bool
	Updated              time.Time // oldest modification time of the present files
}

// Stat reports which files exist.
func Stat(dir string) Status {
	var st Status
	for _, f := range []struct {
		name string
		has  *bool
	}{{GeoIP, &st.HasGeoIP}, {GeoSite, &st.HasGeoSite}} {
		if fi, err := os.Stat(filepath.Join(dir, f.name)); err == nil && fi.Size() > 0 {
			*f.has = true
			if st.Updated.IsZero() || fi.ModTime().Before(st.Updated) {
				st.Updated = fi.ModTime()
			}
		}
	}
	return st
}

// MaxSize caps a downloaded file.
const MaxSize = 200 << 20

// ErrChecksum means the file did not match its published SHA-256.
var ErrChecksum = errors.New("checksum mismatch")

// Download fetches both files from src into dir. Each file is verified against "<file>.sha256sum" when the
// source publishes one (requireChecksum makes a missing checksum an error), written to a temp file and renamed
// into place only after both succeeded. client decides the network path (e.g. through the running proxy).
func Download(ctx context.Context, client *http.Client, baseURL, dir string, requireChecksum bool) error {
	if !strings.HasPrefix(baseURL, "https://") {
		return errors.New("geo source must be an https:// URL")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var tmps []string
	defer func() {
		for _, t := range tmps {
			os.Remove(t)
		}
	}()
	for _, name := range []string{GeoIP, GeoSite} {
		tmp, err := fetch(ctx, client, strings.TrimRight(baseURL, "/")+"/"+name, dir, requireChecksum)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		tmps = append(tmps, tmp)
	}
	for i, name := range []string{GeoIP, GeoSite} {
		if err := os.Rename(tmps[i], filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	tmps = nil
	return nil
}

func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
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

func fetch(ctx context.Context, client *http.Client, url, dir string, requireChecksum bool) (string, error) {
	want := ""
	if resp, err := get(ctx, client, url+".sha256sum"); err == nil {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if f := strings.Fields(string(b)); len(f) > 0 && len(f[0]) == 64 {
			want = strings.ToLower(f[0])
		}
	}
	if want == "" && requireChecksum {
		return "", errors.New("source publishes no .sha256sum")
	}
	resp, err := get(ctx, client, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	f, err := os.CreateTemp(dir, ".geo-*")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, MaxSize+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > MaxSize {
		err = errors.New("file too large")
	}
	if err == nil && n == 0 {
		err = errors.New("empty file")
	}
	if err == nil && want != "" && hex.EncodeToString(h.Sum(nil)) != want {
		err = ErrChecksum
	}
	if err == nil {
		_, err = Tags(f.Name()) // must parse as a geo list
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// Tags lists the country codes / categories in a geoip.dat or geosite.dat (lowercase, sorted). It streams the
// file and reads only the code of each entry.
func Tags(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<16)
	var tags []string
	for {
		key, err := readVarint(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		num, typ := protowire.DecodeTag(key)
		if num != 1 || typ != protowire.BytesType {
			return nil, errors.New("not a geo list")
		}
		n, err := readVarint(r)
		if err != nil || n > MaxSize {
			return nil, errors.New("not a geo list")
		}
		entry := make([]byte, n)
		if _, err := io.ReadFull(r, entry); err != nil {
			return nil, err
		}
		if code := entryCode(entry); code != "" {
			tags = append(tags, strings.ToLower(code))
		}
	}
	if len(tags) == 0 {
		return nil, errors.New("no entries")
	}
	sort.Strings(tags)
	return tags, nil
}

func entryCode(b []byte) string {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return ""
		}
		b = b[n:]
		if num == 1 && typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return ""
			}
			return string(v)
		}
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return ""
		}
		b = b[n:]
	}
	return ""
}

func readVarint(r io.ByteReader) (uint64, error) {
	var x uint64
	for shift := uint(0); shift < 64; shift += 7 {
		c, err := r.ReadByte()
		if err != nil {
			if shift > 0 && err == io.EOF {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
		x |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return x, nil
		}
	}
	return 0, errors.New("varint overflow")
}
