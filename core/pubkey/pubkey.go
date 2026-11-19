// Package pubkey finds the gateway's RSA public key(s) in an official Amnezia binary (docs/05).
//
// The key is embedded as a PEM block or as base64 of a PEM ("LS0tLS1CRUdJTiBQVUJMSUMgS0VZ…"). Binaries carry
// several keys (production, sandbox, …), so every candidate is returned and the caller tries them in order.
// For an APK (a zip) the native libraries inside are scanned. Nothing is executed.
package pubkey

import (
	"archive/zip"
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"
)

var (
	pemRe = regexp.MustCompile(`(?s)-----BEGIN PUBLIC KEY-----[A-Za-z0-9+/=\s\\n]{100,1000}?-----END PUBLIC KEY-----`)
	b64Re = regexp.MustCompile(`LS0tLS1CRUdJTiBQVUJMSUMgS0VZ[A-Za-z0-9+/=]{100,1000}[A-Za-z0-9+/=]{0,1000}[A-Za-z0-9+/=]{0,1000}`)
)

const (
	chunk   = 1 << 20
	overlap = 4096 // longer than any match
	// maxEntry caps how much of one zip entry is read (native libs are ~100 MB).
	maxEntry = 512 << 20
)

// ErrNotFound means no RSA public key was found.
var ErrNotFound = errors.New("no RSA public key found in that file")

// Key is one candidate.
type Key struct {
	PEM []byte
	Pub *rsa.PublicKey
}

// FromFile scans a file: an APK/zip (native libraries inside are scanned) or any other binary.
func FromFile(r io.ReaderAt, size int64) ([]Key, error) {
	var head [4]byte
	if _, err := r.ReadAt(head[:], 0); err == nil && bytes.Equal(head[:], []byte("PK\x03\x04")) {
		if keys, err := fromZip(r, size); err == nil {
			return keys, nil
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	var c collector
	if err := c.scan(io.NewSectionReader(r, 0, size)); err != nil {
		return nil, err
	}
	return c.result()
}

func fromZip(r io.ReaderAt, size int64) ([]Key, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	var files []*zip.File
	for _, f := range zr.File {
		n := strings.ToLower(f.Name)
		if strings.HasSuffix(n, ".so") || strings.HasSuffix(n, ".dll") || strings.HasSuffix(n, ".exe") {
			files = append(files, f)
		}
	}
	// Amnezia's own library first, arm64 before other ABIs (same key in each ABI).
	score := func(f *zip.File) int {
		n := strings.ToLower(f.Name)
		s := 0
		if strings.Contains(n, "amnezia") {
			s += 2
		}
		if strings.Contains(n, "arm64") {
			s++
		}
		return s
	}
	sort.SliceStable(files, func(i, j int) bool { return score(files[i]) > score(files[j]) })
	var c collector
	for _, f := range files {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		err = c.scan(io.LimitReader(rc, maxEntry))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if len(c.keys) > 0 && score(f) >= 2 {
			break // found in the main library; other ABIs hold the same keys
		}
	}
	return c.result()
}

// Parse accepts pasted text: one or more PEM blocks, or base64 of a PEM.
func Parse(text []byte) ([]Key, error) {
	var c collector
	c.feed(text)
	if len(c.keys) == 0 {
		if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(text))); err == nil {
			c.feed(dec)
		}
	}
	return c.result()
}

type collector struct {
	keys []Key
}

func (c *collector) scan(r io.Reader) error {
	buf := make([]byte, 0, chunk+overlap)
	tmp := make([]byte, chunk)
	for {
		n, err := io.ReadFull(r, tmp)
		buf = append(buf, tmp[:n]...)
		c.feed(buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(buf) > overlap {
			buf = append(buf[:0], buf[len(buf)-overlap:]...)
		}
	}
}

func (c *collector) feed(b []byte) {
	for _, m := range pemRe.FindAll(b, -1) {
		c.add(m)
	}
	for _, m := range b64Re.FindAll(b, -1) {
		// Trim to a multiple of 4 and decode as much as possible.
		m = m[:len(m)/4*4]
		dec := make([]byte, base64.StdEncoding.DecodedLen(len(m)))
		n, _ := base64.StdEncoding.Decode(dec, m)
		if i := bytes.Index(dec[:n], []byte("-----END PUBLIC KEY-----")); i >= 0 {
			c.add(dec[:i+len("-----END PUBLIC KEY-----")])
		}
	}
}

func (c *collector) add(raw []byte) {
	// Binaries may store the PEM with literal "\n" sequences.
	raw = bytes.ReplaceAll(raw, []byte(`\n`), []byte("\n"))
	blk, _ := pem.Decode(raw)
	if blk == nil {
		return
	}
	pub, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		return
	}
	rk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return
	}
	for _, k := range c.keys {
		if k.Pub.Equal(rk) {
			return
		}
	}
	der, _ := x509.MarshalPKIXPublicKey(rk)
	c.keys = append(c.keys, Key{PEM: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), Pub: rk})
}

func (c *collector) result() ([]Key, error) {
	if len(c.keys) == 0 {
		return nil, ErrNotFound
	}
	return c.keys, nil
}

// Publics returns the RSA keys.
func Publics(keys []Key) []*rsa.PublicKey {
	out := make([]*rsa.PublicKey, len(keys))
	for i, k := range keys {
		out[i] = k.Pub
	}
	return out
}
