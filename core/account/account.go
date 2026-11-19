// Package account turns a subscription key into an Xray config via the gateway, keeping one stable device
// identity (docs/03, docs/06). It does no storage itself: callers persist Identity and Result.
package account

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nuc134r/sockmnezia/core/gateway"
	"github.com/nuc134r/sockmnezia/core/vpnkey"
)

// Reported to the gateway like the official client does (see docs/DECISIONS.md, "Client identity").
const (
	AppVersion = "5.1.1.10"
	CLIName    = "AmneziaVPN"
)

// Identity is generated once per install and reused for every request, so the account does not lose
// a device slot per fetch. Both values are UUID v4.
type Identity struct {
	InstallationUUID string `json:"installation_uuid"`
	ClientUUID       string `json:"client_uuid"` // sent as public_key for vless; becomes the Xray user id
}

// NewIdentity creates a fresh identity.
func NewIdentity() Identity {
	return Identity{InstallationUUID: gateway.NewUUID(), ClientUUID: gateway.NewUUID()}
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// Valid reports whether both ids are lowercase UUID v4.
func (i Identity) Valid() bool {
	return uuidRe.MatchString(i.InstallationUUID) && uuidRe.MatchString(i.ClientUUID)
}

// Request describes one v1/config call.
type Request struct {
	Key      *vpnkey.Subscription
	Identity Identity
	// Country selects a location (server_country_code); empty = the gateway's default.
	Country string
	// OS is reported as os_version ("android", "windows", "linux").
	OS       string
	Language string // app_language, default "en"
	// CaptchaID/CaptchaSolution answer a previous KindCaptchaRequired error.
	CaptchaID       string
	CaptchaSolution string
}

// Payload builds the JSON body; empty values are omitted like the official client does.
func (r Request) Payload() map[string]any {
	p := map[string]any{
		"os_version":          r.OS,
		"app_version":         AppVersion,
		"cli_name":            CLIName,
		"app_language":        r.Language,
		"installation_uuid":   r.Identity.InstallationUUID,
		"user_country_code":   r.Key.APIConfig.UserCountryCode,
		"server_country_code": r.Country,
		"service_type":        r.Key.APIConfig.ServiceType,
		"service_protocol":    "vless",
		"public_key":          r.Identity.ClientUUID,
		"auth_data":           map[string]any{"api_key": r.Key.AuthData.APIKey},
		"captcha_id":          r.CaptchaID,
		"captcha_solution":    r.CaptchaSolution,
	}
	if r.OS == "" {
		p["os_version"] = "android"
	}
	if r.Language == "" {
		p["app_language"] = "en"
	}
	for k, v := range p {
		if s, ok := v.(string); ok && s == "" {
			delete(p, k)
		}
	}
	return p
}

// Country is a selectable location.
type Country struct {
	Code string `json:"server_country_code"`
	Name string `json:"server_country_name"`
}

// Result is what a successful fetch yields.
type Result struct {
	// XrayConfig is the gateway's Xray client config (last_config), unmodified.
	XrayConfig  json.RawMessage `json:"xray_config"`
	Country     string          `json:"country"`
	CountryName string          `json:"country_name"`
	Countries   []Country       `json:"countries"`
	// SubscriptionEnd is the raw end_date string when present.
	SubscriptionEnd string `json:"subscription_end,omitempty"`
	// KeyIndex is the index of the gateway public key that worked.
	KeyIndex int `json:"-"`
}

// ErrNoXray means the gateway answered but the entry has no amnezia-xray container.
var ErrNoXray = errors.New("gateway answer has no Xray config (amnezia-xray container)")

// Fetch posts v1/config, trying each gateway key in order.
func Fetch(ctx context.Context, c *gateway.Client, keys []*rsa.PublicKey, r Request) (*Result, error) {
	if r.Key == nil || r.Key.AuthData.APIKey == "" {
		return nil, vpnkey.ErrNoAPIKey
	}
	if !r.Identity.Valid() {
		return nil, errors.New("invalid device identity")
	}
	doc, idx, err := c.PostAny(ctx, keys, "v1/config", r.Payload())
	if err != nil {
		return nil, err
	}
	res, err := ParseAnswer(doc)
	if err != nil {
		return nil, err
	}
	res.KeyIndex = idx
	return res, nil
}

// ParseAnswer extracts the result from a decrypted v1/config answer.
func ParseAnswer(doc []byte) (*Result, error) {
	var ans struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(doc, &ans); err != nil || ans.Config == "" {
		return nil, fmt.Errorf("unexpected gateway answer (no config)")
	}
	var entry struct {
		CountryCode string `json:"server_country_code"`
		CountryName string `json:"server_country_name"`
		APIConfig   struct {
			CountryCode  string    `json:"server_country_code"`
			Countries    []Country `json:"available_countries"`
			Subscription struct {
				EndDate string `json:"end_date"`
			} `json:"subscription"`
		} `json:"api_config"`
		Containers []struct {
			Container string `json:"container"`
			Xray      struct {
				LastConfig json.RawMessage `json:"last_config"`
			} `json:"xray"`
		} `json:"containers"`
	}
	if err := vpnkey.Decode(ans.Config, &entry); err != nil {
		return nil, fmt.Errorf("gateway config: %w", err)
	}
	res := &Result{
		Country:         firstNonEmpty(entry.CountryCode, entry.APIConfig.CountryCode),
		CountryName:     entry.CountryName,
		Countries:       entry.APIConfig.Countries,
		SubscriptionEnd: entry.APIConfig.Subscription.EndDate,
	}
	for _, c := range entry.Containers {
		if c.Container != "amnezia-xray" || len(c.Xray.LastConfig) == 0 {
			continue
		}
		lc := c.Xray.LastConfig
		// last_config is a JSON string holding JSON; accept an object too.
		var s string
		if json.Unmarshal(lc, &s) == nil {
			lc = json.RawMessage(s)
		}
		var probe struct {
			Outbounds []json.RawMessage `json:"outbounds"`
		}
		if err := json.Unmarshal(lc, &probe); err != nil || len(probe.Outbounds) == 0 {
			return nil, fmt.Errorf("gateway Xray config is not valid JSON with outbounds")
		}
		res.XrayConfig = lc
		return res, nil
	}
	return nil, ErrNoXray
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
