package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"

	"github.com/nuc134r/sockmnezia/core/engine"
)

// The state file holds the subscription key and server credentials; on the desktop it is encrypted with a
// key the OS keeps for this user (docs/10 §5): DPAPI on Windows, the Secret Service keyring on Linux. Where
// neither is available the file stays plain JSON, readable only by the user (0600), and the UI says so.
//
// File formats: plain JSON (starts with "{"), "SMZK1" + AES-GCM nonce + sealed (keyring key), "SMZD1" + DPAPI
// blob. Open reads plain JSON too, so a file written before encryption was available is upgraded on save.

var (
	magicKeyring = []byte("SMZK1")
	magicDPAPI   = []byte("SMZD1")
)

// errLocked: the file is encrypted but the key is not available (keyring missing, locked or reset).
var errLocked = errors.New("the saved data is encrypted with a key from the system keyring, which is not available")

// SecretStore names the protection in use, for the UI: "dpapi", "keyring" or "file".
var secretStore = "file"

// plainOnly opens only plain files (no OS key available).
type plainOnly struct{}

func (plainOnly) Seal(p []byte) ([]byte, error) { return p, nil }
func (plainOnly) Open(b []byte) ([]byte, error) {
	if bytes.HasPrefix(b, magicKeyring) || bytes.HasPrefix(b, magicDPAPI) {
		return nil, errLocked
	}
	return b, nil
}

// gcmBox seals with AES-256-GCM under a 32-byte key.
type gcmBox struct{ key []byte }

func (g gcmBox) aead() (cipher.AEAD, error) {
	blk, err := aes.NewCipher(g.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

func (g gcmBox) Seal(p []byte) ([]byte, error) {
	a, err := g.aead()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append(append([]byte{}, magicKeyring...), nonce...)
	return a.Seal(out, nonce, p, magicKeyring), nil
}

func (g gcmBox) Open(b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, magicKeyring) {
		return plainOnly{}.Open(b)
	}
	a, err := g.aead()
	if err != nil {
		return nil, err
	}
	b = b[len(magicKeyring):]
	if len(b) < a.NonceSize() {
		return nil, errors.New("encrypted state is truncated")
	}
	return a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], magicKeyring)
}

var _ engine.SecretBox = gcmBox{}
