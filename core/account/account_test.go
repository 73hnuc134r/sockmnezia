package account_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nuc134r/sockmnezia/core/account"
	"github.com/nuc134r/sockmnezia/core/gateway"
	"github.com/nuc134r/sockmnezia/core/internal/mockgw"
	"github.com/nuc134r/sockmnezia/core/vpnkey"
)

func TestFetchAgainstMock(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	sub, err := vpnkey.ParseSubscription(mockgw.SubscriptionKey())
	if err != nil {
		t.Fatal(err)
	}
	id := account.NewIdentity()
	if !id.Valid() {
		t.Fatalf("bad identity %+v", id)
	}
	decoy, _ := rsa.GenerateKey(rand.Reader, 2048)
	res, err := account.Fetch(context.Background(), &gateway.Client{BaseURL: g.URL},
		[]*rsa.PublicKey{&decoy.PublicKey, &g.Key.PublicKey},
		account.Request{Key: sub, Identity: id, Country: "nl"})
	if err != nil {
		t.Fatal(err)
	}
	if res.KeyIndex != 1 || res.Country != "nl" || len(res.Countries) != 2 || res.SubscriptionEnd == "" {
		t.Fatalf("unexpected result %+v", res)
	}
	if !strings.Contains(string(res.XrayConfig), id.ClientUUID) {
		t.Fatal("client UUID not in the returned config")
	}
	var x struct{ Outbounds []map[string]any }
	if err := json.Unmarshal(res.XrayConfig, &x); err != nil || len(x.Outbounds) != 3 {
		t.Fatalf("xray config: %v", err)
	}

	reqs := g.Requests()
	p := reqs[len(reqs)-1].Payload
	want := map[string]any{
		"os_version": "android", "app_version": account.AppVersion, "cli_name": account.CLIName, "app_language": "en",
		"installation_uuid": id.InstallationUUID, "user_country_code": "xx", "server_country_code": "nl",
		"service_type": "amnezia-premium", "service_protocol": "vless", "public_key": id.ClientUUID,
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload %s = %v, want %v", k, p[k], v)
		}
	}
	if ad, _ := p["auth_data"].(map[string]any); ad["api_key"] != "synthetic-api-key-for-tests" {
		t.Error("auth_data.api_key missing")
	}
	for _, k := range []string{"captcha_id", "captcha_solution"} {
		if _, ok := p[k]; ok {
			t.Errorf("empty %s must be omitted", k)
		}
	}
}

func TestCaptchaFieldsAndErrors(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	sub, _ := vpnkey.ParseSubscription(mockgw.SubscriptionKey())
	g.SetReply(func(r mockgw.Request) map[string]any {
		if r.Payload["captcha_solution"] == "1234" {
			return mockgw.DefaultReply(r)
		}
		return map[string]any{"http_status": 402, "captcha_id": "c-1", "captcha_image": "aGk="}
	})
	c := &gateway.Client{BaseURL: g.URL}
	keys := []*rsa.PublicKey{&g.Key.PublicKey}
	req := account.Request{Key: sub, Identity: account.NewIdentity()}
	_, err := account.Fetch(context.Background(), c, keys, req)
	var ae *gateway.APIError
	if !errors.As(err, &ae) || ae.Kind != gateway.KindCaptchaRequired || ae.Captcha.ID != "c-1" {
		t.Fatalf("want captcha, got %v", err)
	}
	req.CaptchaID, req.CaptchaSolution = ae.Captcha.ID, "1234"
	if _, err := account.Fetch(context.Background(), c, keys, req); err != nil {
		t.Fatal(err)
	}

	g.SetReply(func(mockgw.Request) map[string]any { return map[string]any{"http_status": 201, "config": "vpn://x"} })
	if _, err := account.Fetch(context.Background(), c, keys, req); err == nil {
		t.Fatal("garbage config accepted")
	}
	g.SetReply(func(mockgw.Request) map[string]any {
		cfg, _ := vpnkey.Encode(map[string]any{"containers": []any{map[string]any{"container": "amnezia-awg"}}}, false)
		return map[string]any{"http_status": 201, "config": cfg}
	})
	if _, err := account.Fetch(context.Background(), c, keys, req); !errors.Is(err, account.ErrNoXray) {
		t.Fatalf("want ErrNoXray, got %v", err)
	}
	if _, err := account.Fetch(context.Background(), c, keys, account.Request{Key: sub}); err == nil {
		t.Fatal("empty identity accepted")
	}
}
