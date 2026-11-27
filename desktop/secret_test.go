package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
)

func TestGCMBox(t *testing.T) {
	k := make([]byte, 32)
	rand.Read(k)
	box := gcmBox{key: k}
	plain := []byte(`{"version":2,"amnezia":{"key":"vpn://secret"}}`)
	sealed, err := box.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("secret")) || !bytes.HasPrefix(sealed, magicKeyring) {
		t.Fatal("not sealed")
	}
	if got, err := box.Open(sealed); err != nil || !bytes.Equal(got, plain) {
		t.Fatal("round trip", err)
	}
	// A plain file (written before a keyring was available) is read, and sealed on the next save.
	if got, err := box.Open(plain); err != nil || !bytes.Equal(got, plain) {
		t.Fatal("plain file", err)
	}
	// Another key, or no key at all, never yields data (and never a silent empty state).
	rand.Read(k)
	if _, err := (gcmBox{key: k}).Open(sealed); err == nil {
		t.Fatal("opened with another key")
	}
	if _, err := (plainOnly{}).Open(sealed); !errors.Is(err, errLocked) {
		t.Fatal("encrypted file opened without the key", err)
	}
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	if _, err := box.Open(tampered); err == nil {
		t.Fatal("tampered file opened")
	}
}
