package engine

import (
	"errors"
	"strings"

	"github.com/nuc134r/sockmnezia/core/gateway"
	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/pubkey"
	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/vpnkey"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// Error codes the UI translates (keep in sync with the app's strings).
const (
	CodeInvalidKey            = "invalid_key"
	CodeNoGatewayKey          = "no_gateway_key"
	CodeGatewayWrongKey       = "gateway_wrong_key"
	CodeGatewayUnreachable    = "gateway_unreachable"
	CodeDeviceLimit           = "device_limit"
	CodeTrialUsed             = "trial_used"
	CodeSubscriptionExpired   = "subscription_expired"
	CodeSubscriptionNotActive = "subscription_not_active"
	CodeCaptchaRequired       = "captcha_required"
	CodeCaptchaInvalid        = "captcha_invalid"
	CodeRateLimit             = "rate_limit"
	CodeUpdateRequired        = "update_required"
	CodeGatewayError          = "gateway_error"
	CodeNoConfig              = "no_config"
	CodeNoOwnServer           = "no_own_server"
	CodePortInUse             = "port_in_use"
	CodeInvalidSettings       = "invalid_settings"
	CodeGeoMissing            = "geo_missing"
	CodeUnsupportedLink       = "unsupported_link"
	CodeInvalidLink           = "invalid_link"
	CodeNotRunning            = "not_running"
	CodeGeoBusy               = "geo_busy"      // a geo update is already running
	CodeNoChain               = "no_chain"      // no chain saved/selected
	CodeServerInUse           = "server_in_use" // Detail: names of the chains using it
	CodeChainEmpty            = "chain_empty"
	CodeChainTooLong          = "chain_too_long"
	CodeChainAmneziaTwice     = "chain_amnezia_twice"
	CodeHopNotChainable       = "hop_not_chainable" // UDP transport anywhere but the first hop; Field hop<N>
	CodeNetwork               = "network"
	CodeInternal              = "internal"
)

// Error is what the UI shows: a code to translate, a field to highlight and the technical detail.
type Error struct {
	Code    string           `json:"code"`
	Field   string           `json:"field,omitempty"`
	Detail  string           `json:"detail,omitempty"`
	Captcha *gateway.Captcha `json:"captcha,omitempty"`
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Code + ": " + e.Detail
	}
	return e.Code
}

func newErr(code string, err error) *Error {
	e := &Error{Code: code}
	if err != nil {
		e.Detail = err.Error()
	}
	return e
}

// Classify maps any error from the core to an *Error.
func Classify(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	var ae *gateway.APIError
	if errors.As(err, &ae) {
		code := map[gateway.Kind]string{
			gateway.KindDeviceLimit: CodeDeviceLimit, gateway.KindTrialUsed: CodeTrialUsed,
			gateway.KindSubscriptionExpired: CodeSubscriptionExpired, gateway.KindSubscriptionNotActive: CodeSubscriptionNotActive,
			gateway.KindCaptchaRequired: CodeCaptchaRequired, gateway.KindCaptchaInvalid: CodeCaptchaInvalid,
			gateway.KindCaptchaRefresh: CodeCaptchaRequired, gateway.KindRateLimit: CodeRateLimit,
			gateway.KindUpdateRequired: CodeUpdateRequired,
		}[ae.Kind]
		if code == "" {
			code = CodeGatewayError
		}
		return &Error{Code: code, Detail: ae.Error(), Captcha: ae.Captcha}
	}
	var xe *xrayconf.Error
	if errors.As(err, &xe) {
		code := CodeInvalidSettings
		switch {
		case errors.Is(err, xrayconf.ErrChainEmpty):
			code = CodeChainEmpty
		case errors.Is(err, xrayconf.ErrChainTooLong):
			code = CodeChainTooLong
		case errors.Is(err, xrayconf.ErrAmneziaTwice):
			code = CodeChainAmneziaTwice
		case errors.Is(err, xrayconf.ErrNotChainable):
			code = CodeHopNotChainable
		case errors.Is(err, links.ErrUnsupported):
			code = CodeUnsupportedLink
		}
		return &Error{Code: code, Field: xe.Field, Detail: xe.Err.Error()}
	}
	switch {
	case errors.Is(err, vpnkey.ErrNotAKey), errors.Is(err, vpnkey.ErrNoAPIKey):
		return newErr(CodeInvalidKey, err)
	case errors.Is(err, pubkey.ErrNotFound):
		return newErr(CodeNoGatewayKey, err)
	case errors.Is(err, gateway.ErrWrongKey):
		return newErr(CodeGatewayWrongKey, err)
	case errors.Is(err, gateway.ErrUnreachable):
		return newErr(CodeGatewayUnreachable, err)
	case errors.Is(err, routing.ErrGeoMissing):
		return newErr(CodeGeoMissing, err)
	case errors.Is(err, links.ErrUnsupported):
		return newErr(CodeUnsupportedLink, err)
	}
	var re *routing.RuleError
	if errors.As(err, &re) {
		return &Error{Code: CodeInvalidSettings, Field: "routing", Detail: err.Error()}
	}
	if s := err.Error(); strings.Contains(s, "address already in use") || strings.Contains(s, "bind:") {
		return newErr(CodePortInUse, err)
	}
	return newErr(CodeInternal, err)
}
