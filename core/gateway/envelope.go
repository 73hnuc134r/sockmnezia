// Package gateway is a clean-room client for the Amnezia API gateway (docs/03-gateway-protocol.md).
//
// Request: {"key_payload": b64(RSA-PKCS1v15(keys_json)), "api_payload": b64(AES-256-CBC(payload))} where
// keys_json carries a random 32-byte AES key, a 32-byte IV (only the first 16 bytes are used by CBC) and an
// unused 8-byte salt. The response body is raw AES-256-CBC ciphertext with the same key and IV.
package gateway

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
)

// Session holds the symmetric key material of one request.
type Session struct {
	Key  [32]byte
	IV   [32]byte // sent as 32 bytes; CBC uses IV[:16]
	Salt [8]byte  // sent, unused by the server
}

// NewSession creates random key material.
func NewSession() (*Session, error) {
	s := &Session{}
	for _, b := range [][]byte{s.Key[:], s.IV[:], s.Salt[:]} {
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type keysJSON struct {
	AESKey  string `json:"aes_key"`
	AESIV   string `json:"aes_iv"`
	AESSalt string `json:"aes_salt"`
}

type envelope struct {
	KeyPayload string `json:"key_payload"`
	APIPayload string `json:"api_payload"`
}

// Seal builds the request body for payload, encrypted to pub.
func (s *Session) Seal(pub *rsa.PublicKey, payload []byte) ([]byte, error) {
	kj, err := json.Marshal(keysJSON{
		AESKey:  base64.StdEncoding.EncodeToString(s.Key[:]),
		AESIV:   base64.StdEncoding.EncodeToString(s.IV[:]),
		AESSalt: base64.StdEncoding.EncodeToString(s.Salt[:]),
	})
	if err != nil {
		return nil, err
	}
	encKeys, err := rsa.EncryptPKCS1v15(rand.Reader, pub, kj) //nolint:staticcheck // the gateway requires PKCS#1 v1.5
	if err != nil {
		return nil, err
	}
	ct, err := s.encrypt(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{
		KeyPayload: base64.StdEncoding.EncodeToString(encKeys),
		APIPayload: base64.StdEncoding.EncodeToString(ct),
	})
}

func (s *Session) encrypt(pt []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.Key[:])
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(pt, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, s.IV[:aes.BlockSize]).CryptBlocks(out, padded)
	return out, nil
}

// errOpen is returned when a response is not ciphertext under this session's key.
var errOpen = errors.New("response is not encrypted with the session key")

// Open decrypts a response body.
func (s *Session) Open(body []byte) ([]byte, error) {
	if len(body) == 0 || len(body)%aes.BlockSize != 0 {
		return nil, errOpen
	}
	block, err := aes.NewCipher(s.Key[:])
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(body))
	cipher.NewCBCDecrypter(block, s.IV[:aes.BlockSize]).CryptBlocks(out, body)
	out, err = pkcs7Unpad(out, aes.BlockSize)
	if err != nil {
		return nil, errOpen
	}
	return out, nil
}

// Encrypt is the server side of Open; exported for the mock gateway in tests.
func (s *Session) Encrypt(pt []byte) ([]byte, error) { return s.encrypt(pt) }

func pkcs7Pad(b []byte, bs int) []byte {
	n := bs - len(b)%bs
	return append(append([]byte{}, b...), bytes.Repeat([]byte{byte(n)}, n)...)
}

func pkcs7Unpad(b []byte, bs int) ([]byte, error) {
	if len(b) == 0 || len(b)%bs != 0 {
		return nil, errOpen
	}
	n := int(b[len(b)-1])
	if n == 0 || n > bs || n > len(b) {
		return nil, errOpen
	}
	for _, c := range b[len(b)-n:] {
		if int(c) != n {
			return nil, errOpen
		}
	}
	return b[:len(b)-n], nil
}

// OpenRequest is the server side of Seal (used by the mock gateway): it decrypts the key payload with
// priv and returns the session and the plaintext payload.
func OpenRequest(priv *rsa.PrivateKey, body []byte) (*Session, []byte, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, nil, err
	}
	ek, err := base64.StdEncoding.DecodeString(env.KeyPayload)
	if err != nil {
		return nil, nil, err
	}
	kjRaw, err := rsa.DecryptPKCS1v15(nil, priv, ek) //nolint:staticcheck // protocol requirement
	if err != nil {
		return nil, nil, err
	}
	var kj keysJSON
	if err := json.Unmarshal(kjRaw, &kj); err != nil {
		return nil, nil, err
	}
	s := &Session{}
	for _, f := range []struct {
		dst []byte
		src string
	}{{s.Key[:], kj.AESKey}, {s.IV[:], kj.AESIV}, {s.Salt[:], kj.AESSalt}} {
		b, err := base64.StdEncoding.DecodeString(f.src)
		if err != nil || len(b) != len(f.dst) {
			return nil, nil, errors.New("bad key material")
		}
		copy(f.dst, b)
	}
	ct, err := base64.StdEncoding.DecodeString(env.APIPayload)
	if err != nil {
		return nil, nil, err
	}
	pt, err := s.Open(ct)
	if err != nil {
		return nil, nil, err
	}
	return s, pt, nil
}
