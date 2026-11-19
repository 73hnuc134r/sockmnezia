// Package vpnkey decodes and encodes Amnezia's vpn:// blobs (docs/02-subscription-key-format.md).
//
// Format: "vpn://" + base64url(prefix[4] + zlib stream), padding optional. The prefix is either Qt's
// big-endian uncompressed length (qCompress) or, for subscription keys, the signature 00 00 00 ff.
package vpnkey

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Scheme is the prefix of every Amnezia key.
const Scheme = "vpn://"

// SubscriptionSignature is the 4-byte prefix used by Premium subscription keys.
var SubscriptionSignature = [4]byte{0, 0, 0, 0xff}

// maxDecoded caps the decompressed size so a hostile blob cannot exhaust memory.
const maxDecoded = 8 << 20

// ErrNotAKey is returned for input that is not a vpn:// blob.
var ErrNotAKey = errors.New("not a vpn:// key")

// DecodeRaw returns the JSON document inside a vpn:// blob. The "vpn://" scheme is optional and
// surrounding whitespace is ignored.
func DecodeRaw(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, Scheme)
	s = strings.TrimRight(s, "=")
	if s == "" {
		return nil, ErrNotAKey
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		// Some producers use the standard alphabet.
		if raw, err = base64.RawStdEncoding.DecodeString(s); err != nil {
			return nil, fmt.Errorf("%w: bad base64", ErrNotAKey)
		}
	}
	var data []byte
	if len(raw) > 4 {
		data, err = inflate(raw[4:])
	}
	if data == nil || err != nil {
		// Fallback: no 4-byte prefix.
		if data, err = inflate(raw); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNotAKey, err)
		}
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w: payload is not JSON", ErrNotAKey)
	}
	return data, nil
}

func inflate(b []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, maxDecoded+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDecoded {
		return nil, errors.New("decoded payload too large")
	}
	return data, nil
}

// Decode unmarshals the JSON inside a vpn:// blob into v.
func Decode(s string, v any) error {
	data, err := DecodeRaw(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// EncodeRaw wraps a JSON document as vpn://. With subscription=true the prefix is the subscription
// signature, otherwise Qt's length header.
func EncodeRaw(doc []byte, subscription bool) string {
	var buf bytes.Buffer
	if subscription {
		buf.Write(SubscriptionSignature[:])
	} else {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(doc)))
		buf.Write(n[:])
	}
	w := zlib.NewWriter(&buf)
	w.Write(doc)
	w.Close()
	return Scheme + base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

// Encode marshals v and wraps it as vpn://.
func Encode(v any, subscription bool) (string, error) {
	doc, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return EncodeRaw(doc, subscription), nil
}

// APIConfig is the api_config object of a subscription key.
type APIConfig struct {
	ServiceType     string `json:"service_type,omitempty"`
	ServiceProtocol string `json:"service_protocol,omitempty"`
	UserCountryCode string `json:"user_country_code,omitempty"`
}

// Subscription is a decoded Premium subscription key. APIKey is a secret.
type Subscription struct {
	Name          string    `json:"name,omitempty"`
	Description   string    `json:"description,omitempty"`
	ConfigVersion int       `json:"config_version,omitempty"`
	APIConfig     APIConfig `json:"api_config"`
	AuthData      struct {
		APIKey string `json:"api_key"`
	} `json:"auth_data"`
}

// ErrNoAPIKey means the key decodes but is not a gateway-based subscription key.
var ErrNoAPIKey = errors.New("this key has no auth_data.api_key (not a Premium subscription key)")

// ParseSubscription decodes a Premium subscription key and checks it carries an API key.
func ParseSubscription(s string) (*Subscription, error) {
	var sub Subscription
	if err := Decode(s, &sub); err != nil {
		return nil, err
	}
	if sub.AuthData.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	return &sub, nil
}

// String never prints the API key.
func (s *Subscription) String() string {
	return fmt.Sprintf("Subscription{%s, %s/%s}", s.Name, s.APIConfig.ServiceType, s.APIConfig.ServiceProtocol)
}
