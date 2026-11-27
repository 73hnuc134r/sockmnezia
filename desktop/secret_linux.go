package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/zalando/go-keyring"
)

const keyringService, keyringUser = "sockmnezia", "state-key"

// secretBox uses a random key kept in the Secret Service keyring (GNOME Keyring, KWallet); plain file
// otherwise.
func secretBox(dir string) engine.SecretBox {
	s, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		k := make([]byte, 32)
		rand.Read(k)
		s = base64.StdEncoding.EncodeToString(k)
		err = keyring.Set(keyringService, keyringUser, s)
	}
	if err != nil {
		return plainOnly{}
	}
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(key) != 32 {
		return plainOnly{}
	}
	secretStore = "keyring"
	return gcmBox{key: key}
}
