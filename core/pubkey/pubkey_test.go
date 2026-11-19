package pubkey

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
)

func newPEM(t *testing.T) ([]byte, *rsa.PublicKey) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), &k.PublicKey
}

func junk(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// A fake native library: random bytes, a decoy key as base64 PEM, then the real key as literal PEM
// with escaped newlines (as a C string literal would be stored).
func fakeLib(t *testing.T, decoy, real []byte) []byte {
	var b bytes.Buffer
	b.Write(junk(3 << 20))
	b.WriteString(base64.StdEncoding.EncodeToString(decoy))
	b.Write([]byte{0})
	b.Write(junk(chunk - 7)) // make the next key straddle a chunk boundary
	b.WriteString(strings.ReplaceAll(string(real), "\n", `\n`))
	b.Write(junk(1000))
	return b.Bytes()
}

func TestApkTwoKeysDecoyFirst(t *testing.T) {
	decoyPEM, decoy := newPEM(t)
	realPEM, real := newPEM(t)
	var z bytes.Buffer
	zw := zip.NewWriter(&z)
	w, _ := zw.Create("classes.dex")
	w.Write(junk(1000))
	w, _ = zw.Create("lib/x86_64/libother.so")
	w.Write(junk(5000))
	w, _ = zw.Create("lib/arm64-v8a/libAmneziaVPN_arm64-v8a.so")
	w.Write(fakeLib(t, decoyPEM, realPEM))
	zw.Close()

	keys, err := FromFile(bytes.NewReader(z.Bytes()), int64(z.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || !keys[0].Pub.Equal(decoy) || !keys[1].Pub.Equal(real) {
		t.Fatalf("want [decoy, real], got %d keys", len(keys))
	}
	if !bytes.HasPrefix(keys[1].PEM, []byte("-----BEGIN PUBLIC KEY-----\n")) {
		t.Fatal("PEM not normalised")
	}
}

func TestRawBinaryAndDedup(t *testing.T) {
	p, k := newPEM(t)
	blob := append(append(junk(5000), p...), append(junk(10), p...)...)
	keys, err := FromFile(bytes.NewReader(blob), int64(len(blob)))
	if err != nil || len(keys) != 1 || !keys[0].Pub.Equal(k) {
		t.Fatalf("got %d keys, %v", len(keys), err)
	}
}

func TestParsePasted(t *testing.T) {
	p, k := newPEM(t)
	for _, in := range [][]byte{p, []byte(base64.StdEncoding.EncodeToString(p))} {
		keys, err := Parse(in)
		if err != nil || len(keys) != 1 || !keys[0].Pub.Equal(k) {
			t.Fatalf("parse: %v", err)
		}
	}
}

func TestNotFound(t *testing.T) {
	b := junk(10000)
	if _, err := FromFile(bytes.NewReader(b), int64(len(b))); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := Parse([]byte("hello")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
