// Package mobile is the gomobile API used by the Android app. Complex values travel as JSON strings; every
// error's message is the JSON of an engine.Error ({"code", "field", "detail", "captcha"}).
package mobile

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/geo"
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
	runner.Redactor = c.redact
	return c, nil
}

var (
	uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	ipv4Re = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	ipv6Re = regexp.MustCompile(`\[?[0-9a-fA-F]{1,4}(:[0-9a-fA-F]{0,4}){2,7}\]?`)
)

// redact masks secrets and addresses in log lines (logs can be shared by the user).
func (c *Core) redact(s string) string {
	for _, sec := range c.e.Secrets() {
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

// --- own servers ----------------------------------------------------------------------------------------

type ownView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Network  string `json:"network"`
	Security string `json:"security"`
	Active   bool   `json:"active"`
}

// OwnServersJSON lists saved servers (without credentials).
func (c *Core) OwnServersJSON() string {
	st := c.e.Snapshot()
	out := []ownView{}
	for _, o := range st.Own {
		v := ownView{ID: o.ID, Name: o.Name, Active: o.ID == st.ActiveOwn}
		if o.Server != nil {
			v.Protocol, v.Address, v.Port, v.Network, v.Security = o.Server.Protocol, o.Server.Address, o.Server.Port, o.Server.Network, o.Server.Security
		}
		out = append(out, v)
	}
	return toJSON(out)
}

// AddOwnServer imports a link or Xray outbound JSON; returns the new ID.
func (c *Core) AddOwnServer(link, name string) (string, error) {
	id, err := c.e.AddOwnServer(link, name)
	return id, wrap(err)
}

// RemoveOwnServer deletes a server.
func (c *Core) RemoveOwnServer(id string) error { return wrap(c.e.RemoveOwnServer(id)) }

// RenameOwnServer renames a server.
func (c *Core) RenameOwnServer(id, name string) error { return wrap(c.e.RenameOwnServer(id, name)) }

// SetActiveOwnServer selects a server.
func (c *Core) SetActiveOwnServer(id string) error { return wrap(c.e.SetActiveOwnServer(id)) }

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

// UpdateGeo downloads geo files (through the proxy when it runs).
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

// Forget deletes all data.
func (c *Core) Forget() error { return wrap(c.e.Forget()) }

// Version is the Xray-core version linked in.
func XrayVersion() string { return xrayVersion() }

// CheckUpdateJSON returns {"newer","version","url","notes"} for the newest release.
func (c *Core) CheckUpdateJSON(current string, beta bool) (string, error) {
	cx, cancel := ctx(40 * time.Second)
	defer cancel()
	u, err := c.e.CheckUpdate(cx, "", current, beta)
	if err != nil {
		return "", wrap(err)
	}
	return toJSON(u), nil
}
