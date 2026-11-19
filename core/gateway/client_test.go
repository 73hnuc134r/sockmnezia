package gateway_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuc134r/sockmnezia/core/gateway"
	"github.com/nuc134r/sockmnezia/core/internal/mockgw"
)

func decoy(t *testing.T) *rsa.PublicKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &k.PublicKey
}

func TestPostSuccess201(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	c := &gateway.Client{BaseURL: g.URL}
	doc, err := c.Post(context.Background(), &g.Key.PublicKey, "v1/config", map[string]any{"public_key": "u-1"})
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		HTTPStatus int    `json:"http_status"`
		Config     string `json:"config"`
	}
	json.Unmarshal(doc, &resp)
	if resp.HTTPStatus != 201 || resp.Config == "" {
		t.Fatalf("unexpected answer %s", doc)
	}
	reqs := g.Requests()
	if len(reqs) != 1 || reqs[0].Path != "/v1/config" || reqs[0].Payload["public_key"] != "u-1" {
		t.Fatalf("mock saw %+v", reqs)
	}
}

func TestWrongKeyThenRightKey(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	c := &gateway.Client{BaseURL: g.URL}
	if _, err := c.Post(context.Background(), decoy(t), "v1/config", map[string]any{}); !errors.Is(err, gateway.ErrWrongKey) {
		t.Fatalf("want ErrWrongKey, got %v", err)
	}
	_, idx, err := c.PostAny(context.Background(), []*rsa.PublicKey{decoy(t), &g.Key.PublicKey}, "v1/config", map[string]any{})
	if err != nil || idx != 1 {
		t.Fatalf("PostAny: idx=%d err=%v", idx, err)
	}
	if _, _, err := c.PostAny(context.Background(), []*rsa.PublicKey{decoy(t)}, "v1/config", map[string]any{}); !errors.Is(err, gateway.ErrWrongKey) {
		t.Fatalf("all wrong: %v", err)
	}
}

func TestErrorMapping(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	c := &gateway.Client{BaseURL: g.URL}
	cases := []struct {
		reply map[string]any
		kind  gateway.Kind
	}{
		{map[string]any{"http_status": 409, "message": "config limit"}, gateway.KindDeviceLimit},
		{map[string]any{"http_status": 409, "message": "Trial subscription already used"}, gateway.KindTrialUsed},
		{map[string]any{"http_status": 404}, gateway.KindNotFound},
		{map[string]any{"http_status": 408}, gateway.KindTimeout},
		{map[string]any{"http_status": 429}, gateway.KindRateLimit},
		{map[string]any{"http_status": 501}, gateway.KindUpdateRequired},
		{map[string]any{"http_status": 422, "message": "Failed to retrieve subscription information. Is it activated?"}, gateway.KindSubscriptionExpired},
		{map[string]any{"http_status": 422, "message": "other"}, gateway.KindUnprocessable},
		{map[string]any{"http_status": 402, "captcha_id": "c1", "captcha_image": "aGk=", "hint": "digits"}, gateway.KindCaptchaRequired},
		{map[string]any{"http_status": 402, "message": "rate_limit_exceeded"}, gateway.KindCaptchaRequired},
		{map[string]any{"http_status": 402, "message": "invalid_captcha"}, gateway.KindCaptchaInvalid},
		{map[string]any{"http_status": 402, "message": "refresh_captcha"}, gateway.KindCaptchaRefresh},
		{map[string]any{"http_status": 402, "message": "payment required"}, gateway.KindSubscriptionNotActive},
		{map[string]any{"http_status": 500, "message": "boom"}, gateway.KindUnknown},
	}
	for _, tc := range cases {
		g.SetReply(func(mockgw.Request) map[string]any { return tc.reply })
		_, err := c.Post(context.Background(), &g.Key.PublicKey, "v1/config", map[string]any{})
		var ae *gateway.APIError
		if !errors.As(err, &ae) || ae.Kind != tc.kind {
			t.Errorf("%v: want %v, got %v", tc.reply, tc.kind, err)
			continue
		}
		if tc.kind == gateway.KindCaptchaRequired && tc.reply["captcha_id"] != nil {
			if ae.Captcha == nil || ae.Captcha.ID != "c1" || ae.Captcha.ImageBase64 != "aGk=" || ae.Captcha.Hint != "digits" {
				t.Errorf("captcha not parsed: %+v", ae.Captcha)
			}
		}
	}
	// 200/201 with http_status < 300 are success.
	g.SetReply(func(mockgw.Request) map[string]any { return map[string]any{"http_status": 200, "x": 1} })
	if _, err := c.Post(context.Background(), &g.Key.PublicKey, "v1/x", map[string]any{}); err != nil {
		t.Fatal(err)
	}
}

func TestNonGatewayAnswers(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"html block page": func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<html>blocked</html>")) },
		"empty":           func(w http.ResponseWriter, _ *http.Request) {},
		"16 random bytes": func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("0123456789abcdef")) },
	} {
		srv := httptest.NewServer(h)
		c := &gateway.Client{BaseURL: srv.URL}
		_, err := c.Post(context.Background(), decoy(t), "v1/config", map[string]any{})
		if !errors.Is(err, gateway.ErrWrongKey) {
			t.Errorf("%s: want ErrWrongKey, got %v", name, err)
		}
		srv.Close()
	}
}

func TestUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	c := &gateway.Client{BaseURL: url}
	if _, err := c.Post(context.Background(), decoy(t), "v1/config", map[string]any{}); !errors.Is(err, gateway.ErrUnreachable) {
		t.Fatalf("want ErrUnreachable, got %v", err)
	}
}

func TestEnvelopeQuirks(t *testing.T) {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	s, _ := gateway.NewSession()
	body, err := s.Seal(&k.PublicKey, []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	s2, pt, err := gateway.OpenRequest(k, body)
	if err != nil || string(pt) != `{"a":1}` {
		t.Fatalf("open: %v %s", err, pt)
	}
	if s2.Key != s.Key || s2.IV != s.IV || s2.Salt != s.Salt {
		t.Fatal("key material differs (IV must travel as 32 bytes)")
	}
}
