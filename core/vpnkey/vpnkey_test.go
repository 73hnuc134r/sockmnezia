package vpnkey

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestRoundTripSubscription(t *testing.T) {
	in := map[string]any{
		"name":        "Test",
		"api_config":  map[string]any{"service_type": "amnezia-premium", "service_protocol": "awg", "user_country_code": "xx"},
		"auth_data":   map[string]any{"api_key": "synthetic-api-key"},
		"description": "d",
	}
	s, err := Encode(in, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "vpn://AAAA_3ic") {
		t.Fatalf("subscription keys start with AAAA_3ic, got %.20s", s)
	}
	sub, err := ParseSubscription(s)
	if err != nil {
		t.Fatal(err)
	}
	if sub.AuthData.APIKey != "synthetic-api-key" || sub.APIConfig.ServiceType != "amnezia-premium" || sub.APIConfig.UserCountryCode != "xx" {
		t.Fatalf("unexpected %+v", sub)
	}
	if strings.Contains(sub.String(), "synthetic-api-key") {
		t.Fatal("String() leaks the API key")
	}
}

func TestQtLengthPrefixAndPadding(t *testing.T) {
	s, _ := Encode(map[string]int{"a": 1}, false)
	var v map[string]int
	if err := Decode("  "+s+"==\n", &v); err != nil || v["a"] != 1 {
		t.Fatalf("decode: %v %v", err, v)
	}
	// Without the scheme too.
	if err := Decode(strings.TrimPrefix(s, Scheme), &v); err != nil {
		t.Fatal(err)
	}
}

func TestNoPrefixFallback(t *testing.T) {
	full, _ := Encode(map[string]int{"b": 2}, false)
	raw, _ := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(full, Scheme))
	noPrefix := Scheme + base64.RawURLEncoding.EncodeToString(raw[4:])
	var v map[string]int
	if err := Decode(noPrefix, &v); err != nil || v["b"] != 2 {
		t.Fatalf("decode without prefix: %v %v", err, v)
	}
}

func TestRejects(t *testing.T) {
	for _, s := range []string{"", "vpn://", "vpn://!!!", "vpn://AAAAAAAA", "hello"} {
		if _, err := DecodeRaw(s); !errors.Is(err, ErrNotAKey) {
			t.Errorf("%q: want ErrNotAKey, got %v", s, err)
		}
	}
	s, _ := Encode(map[string]any{"name": "self-hosted", "containers": []any{}}, false)
	if _, err := ParseSubscription(s); !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("want ErrNoAPIKey, got %v", err)
	}
}
