package engine

import (
	"errors"
	"regexp"
	"strings"

	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/links"
)

// Views shared by the front ends (Android binding, desktop app): no credentials in them.

// ServerView is a saved own server as the UI lists it (no credentials).
type ServerView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Network  string   `json:"network"`
	Security string   `json:"security"`
	UsedIn   []string `json:"used_in"` // names of the chains that use this server
	Active   bool     `json:"active"`  // selected on its own (not as part of a chain)
	// HostCountry is the geoip country of Address when it is an IP address ("" for names: never resolved).
	HostCountry string      `json:"host_country,omitempty"`
	Last        *TestResult `json:"last,omitempty"` // the last Test of this server
}

// ServerViews lists saved servers in their saved order.
func (e *Engine) ServerViews() []ServerView {
	st := e.Snapshot()
	out := []ServerView{}
	for _, o := range st.Own {
		v := ServerView{ID: o.ID, Name: o.Name, UsedIn: append([]string{}, e.ChainsUsing(o.ID)...), Active: o.ID == st.ActiveServer,
			Last: e.LastTest("", o.ID)}
		if o.Server != nil {
			v.Protocol, v.Address, v.Port, v.Network, v.Security = o.Server.Protocol, o.Server.Address, o.Server.Port, o.Server.Network, o.Server.Security
			v.HostCountry = geo.LookupCountry(e.GeoDir(), o.Server.Address) // IP literals only, no DNS
		}
		out = append(out, v)
	}
	return out
}

// EndpointToLink turns editor fields into a link, validated like a pasted one.
func EndpointToLink(ep links.Endpoint) (string, error) {
	link, err := ep.Link()
	if err == nil {
		_, err = links.Parse(link)
	}
	if err != nil {
		code := CodeInvalidLink
		if errors.Is(err, links.ErrUnsupported) {
			code = CodeUnsupportedLink
		}
		return "", &Error{Code: code, Detail: err.Error()}
	}
	return link, nil
}

// LinkToEndpoint parses a link or outbound JSON into editor fields; error code json_only when only the JSON
// can express it.
func LinkToEndpoint(link string) (links.Endpoint, error) {
	ep, err := links.EndpointFromLink(link)
	if err != nil {
		code := CodeInvalidLink
		switch {
		case errors.Is(err, links.ErrJSONOnly):
			code = CodeJSONOnly
		case errors.Is(err, links.ErrUnsupported):
			code = CodeUnsupportedLink
		}
		return ep, &Error{Code: code, Detail: err.Error()}
	}
	return ep, nil
}

var (
	uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	ipv4Re = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	ipv6Re = regexp.MustCompile(`\[?[0-9a-fA-F]{1,4}(:[0-9a-fA-F]{0,4}){2,7}\]?`)
)

// Redact masks secrets and addresses in log lines (logs can be shared by the user). Install it as
// runner.Redactor.
func (e *Engine) Redact(s string) string {
	for _, sec := range e.Secrets() {
		if len(sec) >= 4 {
			s = strings.ReplaceAll(s, sec, "***")
		}
	}
	s = uuidRe.ReplaceAllString(s, "<uuid>")
	s = ipv4Re.ReplaceAllStringFunc(s, func(ip string) string {
		if strings.HasPrefix(ip, "127.") {
			return ip
		}
		return "<ip>"
	})
	return ipv6Re.ReplaceAllString(s, "<ip6>")
}
