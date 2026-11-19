package gateway

import (
	"errors"
	"fmt"
	"strings"
)

// Kind classifies gateway errors (mapping follows the official client's handling of http_status).
type Kind int

const (
	KindUnknown               Kind = iota
	KindDeviceLimit                // 409: all device slots used
	KindTrialUsed                  // 409 with "trial subscription already used"
	KindNotFound                   // 404
	KindTimeout                    // 408
	KindRateLimit                  // 429
	KindUpdateRequired             // 501
	KindSubscriptionExpired        // 422 "Is it activated?"
	KindUnprocessable              // other 422
	KindCaptchaRequired            // 402 with captcha_id/captcha_image or rate_limit_exceeded
	KindCaptchaInvalid             // 402 invalid_captcha
	KindCaptchaRefresh             // 402 refresh_captcha
	KindSubscriptionNotActive      // other 402
)

var kindNames = map[Kind]string{
	KindUnknown: "unknown", KindDeviceLimit: "device_limit", KindTrialUsed: "trial_used",
	KindNotFound: "not_found", KindTimeout: "timeout", KindRateLimit: "rate_limit",
	KindUpdateRequired: "update_required", KindSubscriptionExpired: "subscription_expired",
	KindUnprocessable: "unprocessable", KindCaptchaRequired: "captcha_required",
	KindCaptchaInvalid: "captcha_invalid", KindCaptchaRefresh: "captcha_refresh",
	KindSubscriptionNotActive: "subscription_not_active",
}

func (k Kind) String() string { return kindNames[k] }

// Captcha is the challenge the gateway sends with KindCaptchaRequired.
type Captcha struct {
	ID          string `json:"captcha_id"`
	ImageBase64 string `json:"captcha_image"`
	Hint        string `json:"hint,omitempty"`
}

// APIError is an error answer from the gateway (a decrypted JSON body with http_status >= 300).
type APIError struct {
	Status  int
	Message string
	Kind    Kind
	Captcha *Captcha
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gateway error %d (%s): %s", e.Status, e.Kind, e.Message)
}

// ErrWrongKey means the gateway (or something in between) answered with something that is not
// encrypted under our session key: typically the wrong gateway public key (plain JSON 500).
var ErrWrongKey = errors.New("gateway answer could not be decrypted (wrong gateway key or blocked)")

// ErrUnreachable wraps network errors.
var ErrUnreachable = errors.New("gateway unreachable")

const (
	msgUnprocessableSubscription = "Failed to retrieve subscription information. Is it activated?"
	msgTrialUsed                 = "trial subscription already used"
	msgRefreshCaptcha            = "refresh_captcha"
	msgInvalidCaptcha            = "invalid_captcha"
	msgRateLimitExceeded         = "rate_limit_exceeded"
)

func classify(status int, msg string, hasCaptcha bool) Kind {
	lower := strings.ToLower(msg)
	switch status {
	case 429:
		return KindRateLimit
	case 409:
		if strings.Contains(lower, msgTrialUsed) {
			return KindTrialUsed
		}
		return KindDeviceLimit
	case 404:
		return KindNotFound
	case 408:
		return KindTimeout
	case 501:
		return KindUpdateRequired
	case 422:
		if msg == msgUnprocessableSubscription {
			return KindSubscriptionExpired
		}
		return KindUnprocessable
	case 402:
		switch {
		case strings.Contains(lower, msgRefreshCaptcha):
			return KindCaptchaRefresh
		case strings.Contains(lower, msgInvalidCaptcha):
			return KindCaptchaInvalid
		case hasCaptcha || strings.Contains(lower, msgRateLimitExceeded):
			return KindCaptchaRequired
		}
		return KindSubscriptionNotActive
	}
	return KindUnknown
}
