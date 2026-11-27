// Package mobile is the gomobile API used by the Android app. Complex values travel as JSON strings; every
// error's message is the JSON of an engine.Error ({"code", "field", "detail", "captcha"}).
package mobile

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/runner"
)

// Box encrypts the state at rest (implemented in Kotlin with an Android Keystore key).
type Box interface {
	Seal(plain []byte) ([]byte, error)
	Open(sealed []byte) ([]byte, error)
}

// Core wraps the engine.
type Core struct {
	e *engine.Engine
}

type jsonError struct{ e *engine.Error }

func (j jsonError) Error() string {
	b, _ := json.Marshal(j.e)
	return string(b)
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	return jsonError{engine.Classify(err)}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Open loads the state from dir (the app's private files directory).
func Open(dir string, box Box) (*Core, error) {
	e, err := engine.Open(dir, box, "android")
	if err != nil {
		return nil, wrap(err)
	}
	c := &Core{e: e}
	runner.Redactor = e.Redact
	return c, nil
}

func ctx(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// --- Amnezia --------------------------------------------------------------------------------------------

// SetKey stores the subscription key ("" removes it).
func (c *Core) SetKey(key string) error { return wrap(c.e.SetKey(key)) }

// ImportGatewayKeysFromFile scans an official APK (copied to a readable path) for gateway keys.
func (c *Core) ImportGatewayKeysFromFile(path string) (int, error) {
	n, err := c.e.ImportGatewayKeysFromFile(path)
	return n, wrap(err)
}

// ImportGatewayKeysFromOfficialRelease downloads the latest official APK from GitHub and extracts the keys.
// Returns {"count","release"}.
func (c *Core) ImportGatewayKeysFromOfficialRelease() (string, error) {
	cx, cancel := ctx(15 * time.Minute)
	defer cancel()
	n, tag, err := c.e.ImportGatewayKeysFromOfficialRelease(cx, "")
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(map[string]any{"count": n, "release": tag}), nil
}

// SetGatewayKeysPEM stores pasted key text.
func (c *Core) SetGatewayKeysPEM(text string) (int, error) {
	n, err := c.e.SetGatewayKeysPEM(text)
	return n, wrap(err)
}

// Fetch gets the Amnezia config; requestJSON is {"country","captcha_id","captcha_solution"} (all optional).
func (c *Core) Fetch(requestJSON string) error {
	var r engine.FetchRequest
	if requestJSON != "" {
		if err := json.Unmarshal([]byte(requestJSON), &r); err != nil {
			return wrap(err)
		}
	}
	cx, cancel := ctx(90 * time.Second)
	defer cancel()
	return wrap(c.e.Fetch(cx, r))
}

// CountriesJSON lists locations: [{"server_country_code","server_country_name"}].
func (c *Core) CountriesJSON() string { return toJSON(c.e.Countries()) }

// --- proxy ----------------------------------------------------------------------------------------------

// Start starts the proxy (fetching the Amnezia config first when needed).
func (c *Core) Start() error {
	cx, cancel := ctx(90 * time.Second)
	defer cancel()
	return wrap(c.e.Start(cx))
}

// StartVPN runs the proxy with the VpnService's tun file descriptor (VPN mode, D31). The app keeps the
// descriptor open until after Stop.
func (c *Core) StartVPN(fd int) error {
	cx, cancel := ctx(90 * time.Second)
	defer cancel()
	return wrap(c.e.StartVPN(cx, fd))
}

// VPNInterfaceJSON returns the parameters for VpnService.Builder (engine.VPNInterface).
func (c *Core) VPNInterfaceJSON() string { return toJSON(c.e.VPNInterface()) }

// Stop stops the proxy.
func (c *Core) Stop() { c.e.Stop() }

// StatusJSON returns engine.Status.
func (c *Core) StatusJSON() string { return toJSON(c.e.Status()) }

// SettingsJSON returns engine.Settings (includes the proxy credentials).
func (c *Core) SettingsJSON() string { return toJSON(c.e.GetSettings()) }

// SetSettingsJSON validates and saves engine.Settings; a running proxy restarts.
func (c *Core) SetSettingsJSON(s string) error {
	var set engine.Settings
	if err := json.Unmarshal([]byte(s), &set); err != nil {
		return wrap(err)
	}
	return wrap(c.e.SetSettings(set))
}

// RandomPort returns a free random port (never a well-known proxy port).
func (c *Core) RandomPort(avoid int) int { return engine.RandomPort("127.0.0.1", avoid) }

// RandomUser returns a random username.
func RandomUser() string { return engine.RandomUser() }

// RandomPassword returns a random password.
func RandomPassword() string { return engine.RandomPassword() }

// ExitIP returns the public IP seen through the running proxy.
func (c *Core) ExitIP() (string, error) {
	cx, cancel := ctx(20 * time.Second)
	defer cancel()
	ip, err := c.e.ExitIP(cx)
	return ip, wrap(err)
}

// TestChainJSON runs a one-off request through the chain: {"ip","ms","country"}.
func (c *Core) TestChainJSON() (string, error) {
	cx, cancel := ctx(30 * time.Second)
	defer cancel()
	ip, ms, country, err := c.e.TestChain(cx)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(map[string]any{"ip": ip, "ms": ms, "country": country}), nil
}

// ClearTests forgets every remembered Test result.
func (c *Core) ClearTests() error { return wrap(c.e.ClearTests()) }

// TestSavedJSON measures a saved chain (chainID) or single server (serverID) without selecting it:
// {"ip","ms","country"}. The running proxy is not touched.
func (c *Core) TestSavedJSON(chainID, serverID string) (string, error) {
	cx, cancel := ctx(30 * time.Second)
	defer cancel()
	ip, ms, country, err := c.e.TestSaved(cx, chainID, serverID)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(map[string]any{"ip": ip, "ms": ms, "country": country}), nil
}

// PingCountryJSON fetches a one-off config for country and measures a request through it, without changing
// the saved config: {"ms"}.
func (c *Core) PingCountryJSON(country string) (string, error) {
	cx, cancel := ctx(30 * time.Second)
	defer cancel()
	ms, err := c.e.PingCountry(cx, country)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(map[string]any{"ms": ms}), nil
}

// --- own servers ----------------------------------------------------------------------------------------

// OwnServersJSON lists saved servers (without credentials): engine.ServerView.
func (c *Core) OwnServersJSON() string { return toJSON(c.e.ServerViews()) }

// AddOwnServer imports a link or Xray outbound JSON; returns the new ID.
func (c *Core) AddOwnServer(link, name string) (string, error) {
	id, err := c.e.AddOwnServer(link, name)
	return id, wrap(err)
}

// ServerEditJSON returns {"link": share link or outbound JSON, "endpoint": links.Endpoint or null (JSON only)}.
func (c *Core) ServerEditJSON(id string) (string, error) {
	link, ep, err := c.e.ServerForEdit(id)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(map[string]any{"link": link, "endpoint": ep}), nil
}

// EndpointToLink turns editor fields (links.Endpoint JSON) into a link, validated like a pasted one.
func EndpointToLink(endpointJSON string) (string, error) {
	var ep links.Endpoint
	if err := json.Unmarshal([]byte(endpointJSON), &ep); err != nil {
		return "", wrap(&engine.Error{Code: engine.CodeInvalidLink, Detail: err.Error()})
	}
	link, err := engine.EndpointToLink(ep)
	return link, wrap(err)
}

// LinkToEndpointJSON parses a link or outbound JSON into editor fields (links.Endpoint JSON); error code
// json_only when only the JSON can express it.
func LinkToEndpointJSON(link string) (string, error) {
	ep, err := engine.LinkToEndpoint(link)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(ep), nil
}

// UpdateOwnServer replaces a saved server's settings, keeping its ID (chains follow).
func (c *Core) UpdateOwnServer(id, link, name string) error {
	return wrap(c.e.UpdateOwnServer(id, link, name))
}

// RemoveOwnServer deletes a server.
func (c *Core) RemoveOwnServer(id string) error { return wrap(c.e.RemoveOwnServer(id)) }

// RenameOwnServer renames a server.
func (c *Core) RenameOwnServer(id, name string) error { return wrap(c.e.RenameOwnServer(id, name)) }

// --- chains ---------------------------------------------------------------------------------------------

// ChainsJSON lists the saved chains ([]engine.ChainInfo).
func (c *Core) ChainsJSON() string { return toJSON(c.e.Chains()) }

// SaveChainJSON adds (no "id") or replaces an engine.Chain; returns its ID.
func (c *Core) SaveChainJSON(s string) (string, error) {
	var ch engine.Chain
	if err := json.Unmarshal([]byte(s), &ch); err != nil {
		return "", wrap(&engine.Error{Code: engine.CodeInvalidSettings, Detail: err.Error()})
	}
	id, err := c.e.SaveChain(ch)
	return id, wrap(err)
}

// ValidateChainJSON returns the engine.Error JSON explaining why the chain cannot run, or "" when it can.
func (c *Core) ValidateChainJSON(s string) string {
	var ch engine.Chain
	if err := json.Unmarshal([]byte(s), &ch); err != nil {
		return toJSON(&engine.Error{Code: engine.CodeInvalidSettings, Detail: err.Error()})
	}
	if err := c.e.ValidateChain(ch); err != nil {
		return toJSON(err)
	}
	return ""
}

// DeleteChain removes a chain.
func (c *Core) DeleteChain(id string) error { return wrap(c.e.DeleteChain(id)) }

// ShareServer returns a saved server's share link (contains its credentials).
func (c *Core) ShareServer(id string) (string, error) {
	l, err := c.e.ShareServer(id)
	return l, wrap(err)
}

// ShareChain returns a chain as text ("links") or Xray outbounds JSON ("xray").
func (c *Core) ShareChain(id, format string) (string, error) {
	t, err := c.e.ShareChain(id, format)
	return t, wrap(err)
}

// ImportJSON imports shared text (a chain, links or an outbound JSON) and returns the engine.ImportResult
// JSON; with dryRun nothing is saved.
func (c *Core) ImportJSON(text string, dryRun bool) (string, error) {
	r, err := c.e.Import(text, dryRun)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(r), nil
}

// SetActiveServer selects a single own server (no chain).
func (c *Core) SetActiveServer(id string) error { return wrap(c.e.SetActiveServer(id)) }

// SetActiveChain selects the chain the proxy runs.
func (c *Core) SetActiveChain(id string) error { return wrap(c.e.SetActiveChain(id)) }

// --- routing and geo ------------------------------------------------------------------------------------

// PresetsJSON lists preset IDs with their rules.
func PresetsJSON() string {
	out := []map[string]any{}
	for _, p := range routing.Presets {
		r, _ := routing.PresetByID(p.ID)
		out = append(out, map[string]any{"id": p.ID, "rules": r})
	}
	return toJSON(out)
}

// LockedRulesJSON returns the fixed security rules (shown read-only at the top of the list).
func LockedRulesJSON() string { return toJSON(routing.Locked()) }

// ParseRoutingJSON validates an imported rule list (v2rayNG array or our object) and returns it normalised.
func ParseRoutingJSON(s string) (string, error) {
	r, err := routing.ParseJSON([]byte(s))
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(r), nil
}

// ValidateRuleJSON checks one rule; returns "" or the error JSON message.
func ValidateRuleJSON(s string) error {
	var r routing.Rule
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return wrap(err)
	}
	set := routing.Default()
	set.Rules = []routing.Rule{r}
	return wrap(set.Validate())
}

// TestRouteJSON answers where host:port goes with the saved rules.
func (c *Core) TestRouteJSON(host string, port int, network string) (string, error) {
	v, err := c.e.TestRoute(host, port, network)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(v), nil
}

// GeoStatusJSON: {"has_geoip","has_geosite","updated"}.
func (c *Core) GeoStatusJSON() string {
	s := c.e.GeoStatus()
	m := map[string]any{"has_geoip": s.HasGeoIP, "has_geosite": s.HasGeoSite}
	if !s.Updated.IsZero() {
		m["updated"] = s.Updated.Unix()
	}
	return toJSON(m)
}

// GeoSourcesJSON lists the built-in geo sources.
func GeoSourcesJSON() string { return toJSON(geo.Sources) }

// GeoTagsJSON lists tags of geosite ("site") or geoip ("ip").
func (c *Core) GeoTagsJSON(kind string) (string, error) {
	t, err := c.e.GeoTags(kind)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(t), nil
}

// GeoProgressJSON returns the running geo update's geo.Progress JSON, or "" when none runs.
func (c *Core) GeoProgressJSON() string {
	if p := c.e.GeoProgress(); p != nil {
		return toJSON(p)
	}
	return ""
}

// UpdateGeo downloads geo files (through the proxy when it runs); see GeoProgressJSON.
func (c *Core) UpdateGeo() error {
	cx, cancel := ctx(10 * time.Minute)
	defer cancel()
	return wrap(c.e.UpdateGeo(cx))
}

// --- logs and data --------------------------------------------------------------------------------------

// LogsJSON returns [{"t": unix ms, "level", "msg"}].
func LogsJSON() string {
	out := []map[string]any{}
	for _, l := range runner.Logs() {
		out = append(out, map[string]any{"t": l.Time.UnixMilli(), "level": l.Level, "msg": l.Message})
	}
	return toJSON(out)
}

// ClearLogs empties the log.
func ClearLogs() { runner.ClearLogs() }

// Log adds an app-side line.
func Log(level, msg string) { runner.Record(level, msg) }

// StatsJSON returns the last days of traffic statistics: [engine.StatsDay], oldest first.
func (c *Core) StatsJSON(days int) string { return toJSON(c.e.Stats(days)) }

// ResetStats deletes the traffic statistics.
func (c *Core) ResetStats() error { return wrap(c.e.ResetStats()) }

// Forget deletes all data.
func (c *Core) Forget() error { return wrap(c.e.Forget()) }

// Version is the Xray-core version linked in.
func XrayVersion() string { return xrayVersion() }

// DownloadUpdate downloads the newer release's APK for the device (abis: comma-separated, preferred first)
// into dir, verified against SHA256SUMS; returns its path. See UpdateProgressJSON.
func (c *Core) DownloadUpdate(abis, dir string) (string, error) {
	cx, cancel := ctx(15 * time.Minute)
	defer cancel()
	p, err := c.e.DownloadUpdate(cx, strings.Split(abis, ","), dir)
	return p, wrap(err)
}

// UpdateProgressJSON returns the running update download's progress (engine.DownloadProgress), or "".
func (c *Core) UpdateProgressJSON() string {
	if p := c.e.UpdateProgress(); p != nil {
		return toJSON(p)
	}
	return ""
}

// CheckUpdateJSON returns {"newer","version","url","notes"} for the newest release.
func (c *Core) CheckUpdateJSON(current string, beta bool) (string, error) {
	cx, cancel := ctx(40 * time.Second)
	defer cancel()
	u, err := c.e.CheckUpdate(cx, "", engine.TagAndroid, current, beta)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(u), nil
}
