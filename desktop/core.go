package main

import (
	"context"
	"time"

	"github.com/nuc134r/sockmnezia/core/account"
	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/runner"
)

// Core is the engine as the UI sees it (bound to the web UI by Wails): the same operations as the Android
// binding (core/mobile), with typed values instead of JSON strings. Every error is an *engine.Error, which
// reaches the UI as the call error's cause ({code, field, detail, captcha}).
type Core struct {
	e *engine.Engine
	// changed is called after anything that changes saved state, so the tray and other windows refresh.
	changed func()
	// selected records the chain ("c:<id>") or server ("s:<id>") just selected (recent list in the tray).
	selected func(key string)
}

func wrap(err error) error {
	if err == nil {
		return nil // never a typed nil *engine.Error in an error interface
	}
	return engine.Classify(err)
}

func (c *Core) touch(err error) error {
	if err == nil && c.changed != nil {
		c.changed()
	}
	return wrap(err)
}

func timeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

// --- status and run -------------------------------------------------------------------------------------

func (c *Core) Status() engine.Status { return c.e.Status() }

func (c *Core) Start(ctx context.Context) error {
	cx, cancel := timeout(ctx, 90*time.Second)
	defer cancel()
	return c.touch(c.e.Start(cx))
}

func (c *Core) Stop() {
	c.e.Stop()
	c.touch(nil)
}

// TestChain measures the running chain: exit address, time and country.
func (c *Core) TestChain(ctx context.Context) (TestResult, error) {
	cx, cancel := timeout(ctx, 30*time.Second)
	defer cancel()
	ip, ms, country, err := c.e.TestChain(cx)
	return TestResult{IP: ip, Ms: ms, Country: country}, wrap(err)
}

// TestSaved measures a saved chain or single server without selecting it (the running proxy is untouched).
func (c *Core) TestSaved(ctx context.Context, chainID, serverID string) (TestResult, error) {
	cx, cancel := timeout(ctx, 30*time.Second)
	defer cancel()
	ip, ms, country, err := c.e.TestSaved(cx, chainID, serverID)
	return TestResult{IP: ip, Ms: ms, Country: country}, wrap(err)
}

// TestResult is one measurement.
type TestResult struct {
	IP      string `json:"ip"`
	Ms      int64  `json:"ms"`
	Country string `json:"country"`
}

func (c *Core) ClearTests() error { return wrap(c.e.ClearTests()) }

// --- settings -------------------------------------------------------------------------------------------

func (c *Core) Settings() engine.Settings { return c.e.GetSettings() }

func (c *Core) SetSettings(s engine.Settings) error { return c.touch(c.e.SetSettings(s)) }

func (c *Core) RandomPort(avoid int) int { return engine.RandomPort("127.0.0.1", avoid) }
func (c *Core) RandomUser() string       { return engine.RandomUser() }
func (c *Core) RandomPassword() string   { return engine.RandomPassword() }

// --- servers and chains ---------------------------------------------------------------------------------

func (c *Core) Servers() []engine.ServerView { return c.e.ServerViews() }
func (c *Core) Chains() []engine.ChainInfo   { return c.e.Chains() }

func (c *Core) AddServer(link, name string) (string, error) {
	id, err := c.e.AddOwnServer(link, name)
	return id, c.touch(err)
}

// ServerEdit is a saved server for the editor: its link (or outbound JSON) and fields (nil: JSON only).
type ServerEdit struct {
	Link     string          `json:"link"`
	Endpoint *links.Endpoint `json:"endpoint"`
}

func (c *Core) ServerEdit(id string) (ServerEdit, error) {
	link, ep, err := c.e.ServerForEdit(id)
	return ServerEdit{Link: link, Endpoint: ep}, wrap(err)
}

func (c *Core) UpdateServer(id, link, name string) error {
	return c.touch(c.e.UpdateOwnServer(id, link, name))
}
func (c *Core) RemoveServer(id string) error       { return c.touch(c.e.RemoveOwnServer(id)) }
func (c *Core) RenameServer(id, name string) error { return c.touch(c.e.RenameOwnServer(id, name)) }

func (c *Core) EndpointToLink(ep links.Endpoint) (string, error) {
	l, err := engine.EndpointToLink(ep)
	return l, wrap(err)
}

func (c *Core) LinkToEndpoint(link string) (links.Endpoint, error) {
	ep, err := engine.LinkToEndpoint(link)
	return ep, wrap(err)
}

func (c *Core) SaveChain(ch engine.Chain) (string, error) {
	id, err := c.e.SaveChain(ch)
	return id, c.touch(err)
}

// ValidateChain returns why the chain cannot run, or nil.
func (c *Core) ValidateChain(ch engine.Chain) *engine.Error { return c.e.ValidateChain(ch) }

func (c *Core) DeleteChain(id string) error     { return c.touch(c.e.DeleteChain(id)) }
func (c *Core) SetActiveChain(id string) error {
	err := c.e.SetActiveChain(id)
	if err == nil && c.selected != nil {
		c.selected("c:" + id)
	}
	return c.touch(err)
}

func (c *Core) SetActiveServer(id string) error {
	err := c.e.SetActiveServer(id)
	if err == nil && c.selected != nil {
		c.selected("s:" + id)
	}
	return c.touch(err)
}

// SetSystemProxy sets the OS proxy mode: "" (off), "system" or "pac" (applied while the proxy runs).
func (c *Core) SetSystemProxy(mode string) error {
	s := c.e.GetSettings()
	s.Options.SystemProxy = mode
	return c.touch(c.e.SetSettings(s))
}

func (c *Core) ShareServer(id string) (string, error) {
	s, err := c.e.ShareServer(id)
	return s, wrap(err)
}

func (c *Core) ShareChain(id, format string) (string, error) {
	s, err := c.e.ShareChain(id, format)
	return s, wrap(err)
}

// Import reads shared text (a chain, links or an outbound JSON); dryRun only describes it.
func (c *Core) Import(text string, dryRun bool) (engine.ImportResult, error) {
	r, err := c.e.Import(text, dryRun)
	if dryRun {
		return r, wrap(err)
	}
	return r, c.touch(err)
}

// --- Amnezia --------------------------------------------------------------------------------------------

func (c *Core) SetKey(key string) error { return c.touch(c.e.SetKey(key)) }

func (c *Core) SetGatewayKeysPEM(text string) (int, error) {
	n, err := c.e.SetGatewayKeysPEM(text)
	return n, c.touch(err)
}

func (c *Core) ImportGatewayKeysFromFile(path string) (int, error) {
	n, err := c.e.ImportGatewayKeysFromFile(path)
	return n, c.touch(err)
}

// OfficialImport is the result of reading the gateway keys from the official release.
type OfficialImport struct {
	Count   int    `json:"count"`
	Release string `json:"release"`
}

func (c *Core) ImportGatewayKeysFromOfficialRelease(ctx context.Context) (OfficialImport, error) {
	cx, cancel := timeout(ctx, 15*time.Minute)
	defer cancel()
	n, tag, err := c.e.ImportGatewayKeysFromOfficialRelease(cx, "")
	return OfficialImport{Count: n, Release: tag}, c.touch(err)
}

func (c *Core) Fetch(ctx context.Context, req engine.FetchRequest) error {
	cx, cancel := timeout(ctx, 90*time.Second)
	defer cancel()
	return c.touch(c.e.Fetch(cx, req))
}

func (c *Core) Countries() []account.Country { return c.e.Countries() }

func (c *Core) PingCountry(ctx context.Context, country string) (int64, error) {
	cx, cancel := timeout(ctx, 30*time.Second)
	defer cancel()
	ms, err := c.e.PingCountry(cx, country)
	return ms, wrap(err)
}

// --- routing and geo ------------------------------------------------------------------------------------

// Preset is a built-in rule set.
type Preset struct {
	ID    string         `json:"id"`
	Rules []routing.Rule `json:"rules"`
}

func (c *Core) Presets() []Preset {
	out := []Preset{}
	for _, p := range routing.Presets {
		r, _ := routing.PresetByID(p.ID)
		out = append(out, Preset{ID: p.ID, Rules: r})
	}
	return out
}

func (c *Core) LockedRules() []routing.Rule { return routing.Locked() }

// ParseRouting validates an imported rule list (v2rayNG array or our object) and returns it normalised.
func (c *Core) ParseRouting(text string) (routing.Settings, error) {
	r, err := routing.ParseJSON([]byte(text))
	if err != nil {
		return routing.Settings{}, wrap(err)
	}
	return r, nil
}

func (c *Core) ValidateRule(r routing.Rule) error {
	set := routing.Default()
	set.Rules = []routing.Rule{r}
	return wrap(set.Validate())
}

func (c *Core) TestRoute(host string, port int, network string) (routing.Verdict, error) {
	v, err := c.e.TestRoute(host, port, network)
	return v, wrap(err)
}

// GeoStatus says which geo files exist and when they were updated (unix seconds, 0 = never).
type GeoStatus struct {
	HasGeoIP   bool  `json:"has_geoip"`
	HasGeoSite bool  `json:"has_geosite"`
	Updated    int64 `json:"updated"`
}

func (c *Core) GeoStatus() GeoStatus {
	s := c.e.GeoStatus()
	g := GeoStatus{HasGeoIP: s.HasGeoIP, HasGeoSite: s.HasGeoSite}
	if !s.Updated.IsZero() {
		g.Updated = s.Updated.Unix()
	}
	return g
}

func (c *Core) GeoSources() []geo.Source { return geo.Sources }

func (c *Core) GeoTags(kind string) ([]string, error) {
	t, err := c.e.GeoTags(kind)
	return t, wrap(err)
}

// GeoProgress returns the running geo update's progress, or nil.
func (c *Core) GeoProgress() *geo.Progress { return c.e.GeoProgress() }

func (c *Core) UpdateGeo(ctx context.Context) error {
	cx, cancel := timeout(ctx, 10*time.Minute)
	defer cancel()
	return c.touch(c.e.UpdateGeo(cx))
}

// --- logs and data --------------------------------------------------------------------------------------

// LogLine is one log entry (t: unix milliseconds).
type LogLine struct {
	T     int64  `json:"t"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

func (c *Core) Logs() []LogLine {
	out := []LogLine{}
	for _, l := range runner.Logs() {
		out = append(out, LogLine{T: l.Time.UnixMilli(), Level: l.Level, Msg: l.Message})
	}
	return out
}

func (c *Core) ClearLogs() { runner.ClearLogs() }

func (c *Core) Forget() error { return c.touch(c.e.Forget()) }
