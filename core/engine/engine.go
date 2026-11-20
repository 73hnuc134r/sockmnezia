// Package engine is the app's core state machine: settings, persistence, fetching the Amnezia config,
// starting and stopping the proxy. The Android binding (package mobile) and the CLI are thin layers on top.
package engine

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nuc134r/sockmnezia/core/account"
	"github.com/nuc134r/sockmnezia/core/gateway"
	"github.com/nuc134r/sockmnezia/core/geo"
	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/pubkey"
	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/nuc134r/sockmnezia/core/vpnkey"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// Engine owns the state. Safe for concurrent use.
type Engine struct {
	dir string
	box SecretBox
	// OS is reported to the gateway (android, windows, linux).
	OS string

	mu        sync.Mutex
	st        *State
	secrets   atomic.Pointer[[]string] // for log redaction; readable without mu
	inst      *runner.Instance
	startedAt time.Time
	lastErr   *Error
}

// Open loads (or creates) the state in dir.
func Open(dir string, box SecretBox, osName string) (*Engine, error) {
	if box == nil {
		box = PlainBox{}
	}
	st, err := load(dir, box)
	if err != nil {
		return nil, err
	}
	e := &Engine{dir: dir, box: box, OS: osName, st: st}
	e.updateSecrets()
	geo.SetAssetDir(e.GeoDir())
	runner.SetLevel(st.Options.LogLevel)
	if err := save(dir, box, st); err != nil { // persist the generated identity/credentials right away
		return nil, err
	}
	return e, nil
}

// GeoDir is where geo files live.
func (e *Engine) GeoDir() string { return filepath.Join(e.dir, "geo") }

func (e *Engine) persist() error {
	e.updateSecrets()
	return save(e.dir, e.box, e.st)
}

func (e *Engine) updateSecrets() {
	sec := []string{e.st.Amnezia.Key, e.st.Inbound.Pass, e.st.Inbound.User, e.st.Amnezia.Identity.ClientUUID}
	for _, o := range e.st.Own {
		sec = append(sec, o.Link)
		if o.Server != nil {
			sec = append(sec, o.Server.Address)
		}
	}
	e.secrets.Store(&sec)
}

// Secrets returns strings that must never appear in logs (does not take the engine lock).
func (e *Engine) Secrets() []string {
	if p := e.secrets.Load(); p != nil {
		return *p
	}
	return nil
}

// Snapshot returns a copy of the state (secrets included; callers decide what to show).
func (e *Engine) Snapshot() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	b, _ := json.Marshal(e.st)
	var c State
	json.Unmarshal(b, &c)
	return c
}

// --- Amnezia subscription -------------------------------------------------------------------------------

// SetKey stores a subscription key after validating it. An empty key removes it.
func (e *Engine) SetKey(key string) error {
	key = strings.TrimSpace(key)
	if key != "" {
		if _, err := vpnkey.ParseSubscription(key); err != nil {
			return Classify(err)
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if key != e.st.Amnezia.Key {
		e.st.Amnezia.Key = key
		e.st.Amnezia.Last = nil
		e.st.Amnezia.FetchedAt = time.Time{}
	}
	return e.persist()
}

// ImportGatewayKeysFromFile scans an official APK/binary for gateway keys and stores the candidates.
func (e *Engine) ImportGatewayKeysFromFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, newErr(CodeInternal, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, newErr(CodeInternal, err)
	}
	keys, err := pubkey.FromFile(f, fi.Size())
	if err != nil {
		return 0, Classify(err)
	}
	return len(keys), e.setGatewayKeys(keys)
}

// SetGatewayKeysPEM stores pasted PEM text (or base64 of a PEM).
func (e *Engine) SetGatewayKeysPEM(text string) (int, error) {
	keys, err := pubkey.Parse([]byte(text))
	if err != nil {
		return 0, Classify(err)
	}
	return len(keys), e.setGatewayKeys(keys)
}

func (e *Engine) setGatewayKeys(keys []pubkey.Key) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.st.Amnezia.GatewayPEMs = nil
	for _, k := range keys {
		e.st.Amnezia.GatewayPEMs = append(e.st.Amnezia.GatewayPEMs, string(k.PEM))
	}
	return e.persist()
}

// FetchRequest asks for a (new) Amnezia config.
type FetchRequest struct {
	Country         string `json:"country"`
	CaptchaID       string `json:"captcha_id"`
	CaptchaSolution string `json:"captcha_solution"`
}

// Fetch posts v1/config and stores the answer. While the proxy runs it is restarted with the new config.
func (e *Engine) Fetch(ctx context.Context, req FetchRequest) error {
	e.mu.Lock()
	st := e.st
	key, pems, id, url := st.Amnezia.Key, st.Amnezia.GatewayPEMs, st.Amnezia.Identity, st.Amnezia.GatewayURL
	country := req.Country
	if country == "" {
		country = st.Amnezia.Country
	}
	viaOwn := st.Options.FetchViaOwn
	own := e.activeOwnLocked()
	e.mu.Unlock()

	if key == "" {
		return newErr(CodeInvalidKey, errors.New("no subscription key"))
	}
	sub, err := vpnkey.ParseSubscription(key)
	if err != nil {
		return Classify(err)
	}
	if len(pems) == 0 {
		return newErr(CodeNoGatewayKey, errors.New("no gateway public key"))
	}
	var pubs []*rsa.PublicKey
	for _, p := range pems {
		k, err := pubkey.Parse([]byte(p))
		if err != nil {
			return Classify(err)
		}
		pubs = append(pubs, k[0].Pub)
	}
	client := &gateway.Client{BaseURL: url}
	if viaOwn {
		if own == nil {
			return newErr(CodeNoOwnServer, errors.New("fetching through the own server is on, but none is selected"))
		}
		cfg, err := xrayconf.BuildDialer(xrayconf.ModeOwn, nil, own.Server.Outbound)
		if err != nil {
			return Classify(err)
		}
		d, err := runner.Start(cfg)
		if err != nil {
			return Classify(err)
		}
		defer d.Close()
		client.Dial = d.Dial
	}
	res, err := account.Fetch(ctx, client, pubs, account.Request{
		Key: sub, Identity: id, Country: country, OS: e.OS,
		CaptchaID: req.CaptchaID, CaptchaSolution: req.CaptchaSolution,
	})
	if err != nil {
		runner.Record("warning", "Amnezia fetch failed: "+err.Error())
		return Classify(err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.st.Amnezia.Key != key { // key changed meanwhile
		return newErr(CodeInvalidKey, errors.New("subscription key changed during fetch"))
	}
	if res.KeyIndex > 0 { // remember the working key first
		p := e.st.Amnezia.GatewayPEMs
		p[0], p[res.KeyIndex] = p[res.KeyIndex], p[0]
	}
	e.st.Amnezia.Last = res
	e.st.Amnezia.FetchedAt = time.Now()
	if req.Country != "" {
		e.st.Amnezia.Country = req.Country
	}
	runner.Record("info", "Amnezia config updated ("+res.Country+")")
	if err := e.persist(); err != nil {
		return newErr(CodeInternal, err)
	}
	if e.inst != nil {
		return e.restartLocked()
	}
	return nil
}

// --- own servers ----------------------------------------------------------------------------------------

// AddOwnServer parses a link/JSON and saves it; the first server becomes active. Returns its ID.
func (e *Engine) AddOwnServer(link, name string) (string, error) {
	s, err := links.Parse(link)
	if err != nil {
		if errors.Is(err, links.ErrUnsupported) {
			return "", newErr(CodeUnsupportedLink, err)
		}
		return "", newErr(CodeInvalidLink, err)
	}
	if strings.TrimSpace(name) != "" {
		s.Name = strings.TrimSpace(name)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	id := gateway.NewUUID()
	e.st.Own = append(e.st.Own, OwnServer{ID: id, Name: s.Name, Link: strings.TrimSpace(link), Server: s})
	if e.st.ActiveOwn == "" {
		e.st.ActiveOwn = id
	}
	return id, e.persist()
}

// RemoveOwnServer deletes a saved server.
func (e *Engine) RemoveOwnServer(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := e.st.Own[:0]
	for _, s := range e.st.Own {
		if s.ID != id {
			out = append(out, s)
		}
	}
	e.st.Own = out
	if e.st.ActiveOwn == id {
		e.st.ActiveOwn = ""
		if len(out) > 0 {
			e.st.ActiveOwn = out[0].ID
		}
	}
	return e.persist()
}

// RenameOwnServer changes the display name.
func (e *Engine) RenameOwnServer(id, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.st.Own {
		if e.st.Own[i].ID == id {
			e.st.Own[i].Name = strings.TrimSpace(name)
		}
	}
	return e.persist()
}

// SetActiveOwnServer selects the server used in the own modes (restarts a running proxy).
func (e *Engine) SetActiveOwnServer(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	found := false
	for _, s := range e.st.Own {
		found = found || s.ID == id
	}
	if !found {
		return newErr(CodeNoOwnServer, errors.New("unknown server"))
	}
	e.st.ActiveOwn = id
	if err := e.persist(); err != nil {
		return err
	}
	return e.restartIfRunningLocked()
}

func (e *Engine) activeOwnLocked() *OwnServer {
	for i := range e.st.Own {
		if e.st.Own[i].ID == e.st.ActiveOwn {
			s := e.st.Own[i]
			return &s
		}
	}
	return nil
}

// --- settings -------------------------------------------------------------------------------------------

// Settings is the editable part of the state.
type Settings struct {
	Mode    xrayconf.Mode    `json:"mode"`
	Inbound xrayconf.Inbound `json:"inbound"`
	Routing routing.Settings `json:"routing"`
	Options Options          `json:"options"`
}

// GetSettings returns the current settings.
func (e *Engine) GetSettings() Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Settings{Mode: e.st.Mode, Inbound: e.st.Inbound, Routing: e.st.Routing, Options: e.st.Options}
}

// SetSettings validates and stores settings; a running proxy is restarted.
func (e *Engine) SetSettings(s Settings) error {
	if s.Inbound.Listen == "" {
		s.Inbound.Listen = "127.0.0.1"
	}
	if err := xrayconf.ValidateInbound(s.Inbound); err != nil {
		return Classify(err)
	}
	switch s.Mode {
	case xrayconf.ModeAmnezia, xrayconf.ModeAmneziaOwn, xrayconf.ModeOwn:
	default:
		return &Error{Code: CodeInvalidSettings, Field: "mode", Detail: "unknown mode"}
	}
	if s.Routing.DomainStrategy == "" {
		s.Routing.DomainStrategy = routing.AsIs
	}
	if err := s.Routing.Validate(); err != nil {
		return Classify(err)
	}
	if s.Options.GeoSource == "custom" && !strings.HasPrefix(s.Options.GeoCustomURL, "https://") {
		return &Error{Code: CodeInvalidSettings, Field: "geo_custom_url", Detail: "use an https:// URL"}
	}
	if _, ok := geo.SourceByID(s.Options.GeoSource); !ok && s.Options.GeoSource != "custom" {
		return &Error{Code: CodeInvalidSettings, Field: "geo_source", Detail: "unknown source"}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.st.Mode, e.st.Inbound, e.st.Routing, e.st.Options = s.Mode, s.Inbound, s.Routing, s.Options
	runner.SetLevel(s.Options.LogLevel)
	if err := e.persist(); err != nil {
		return err
	}
	return e.restartIfRunningLocked()
}

// --- proxy ----------------------------------------------------------------------------------------------

func (e *Engine) buildLocked() ([]byte, error) {
	spec := xrayconf.Spec{Mode: e.st.Mode, Inbound: e.st.Inbound, Routing: e.st.Routing, LogLevel: e.st.Options.LogLevel}
	if e.st.Mode != xrayconf.ModeOwn {
		if e.st.Amnezia.Last == nil {
			return nil, newErr(CodeNoConfig, errors.New("no Amnezia config fetched yet"))
		}
		spec.Amnezia = e.st.Amnezia.Last.XrayConfig
	}
	if e.st.Mode != xrayconf.ModeAmnezia {
		own := e.activeOwnLocked()
		if own == nil {
			return nil, newErr(CodeNoOwnServer, errors.New("no own server selected"))
		}
		spec.Own = own.Server.Outbound
	}
	if site, ip := e.st.Routing.UsesGeo(); site || ip {
		gs := geo.Stat(e.GeoDir())
		if (site && !gs.HasGeoSite) || (ip && !gs.HasGeoIP) {
			return nil, newErr(CodeGeoMissing, errors.New("routing rules use geosite/geoip but the files are not downloaded"))
		}
	}
	return xrayconf.Build(spec)
}

// NeedsRefresh reports whether the Amnezia config should be re-fetched before starting.
func (e *Engine) NeedsRefresh() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.st.Mode == xrayconf.ModeOwn || e.st.Amnezia.Key == "" {
		return false
	}
	if e.st.Amnezia.Last == nil {
		return true
	}
	h := e.st.Options.RefreshAfterHrs
	return h > 0 && time.Since(e.st.Amnezia.FetchedAt) > time.Duration(h)*time.Hour
}

// Start runs the proxy. When the Amnezia config is missing or stale it is fetched first; a failed refresh
// of a stale config falls back to the cached one.
func (e *Engine) Start(ctx context.Context) error {
	if e.NeedsRefresh() {
		if err := e.Fetch(ctx, FetchRequest{}); err != nil {
			e.mu.Lock()
			have := e.st.Amnezia.Last != nil
			e.mu.Unlock()
			if !have {
				e.setLastErr(Classify(err))
				return err
			}
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inst != nil {
		return nil
	}
	err := e.startLocked()
	e.lastErr = Classify(err)
	return e.lastErrOrNil()
}

func (e *Engine) lastErrOrNil() error {
	if e.lastErr == nil {
		return nil
	}
	return e.lastErr
}

func (e *Engine) setLastErr(err *Error) {
	e.mu.Lock()
	e.lastErr = err
	e.mu.Unlock()
}

func (e *Engine) startLocked() error {
	cfg, err := e.buildLocked()
	if err != nil {
		return err
	}
	inst, err := runner.Start(cfg)
	if err != nil {
		return err
	}
	e.inst = inst
	e.startedAt = time.Now()
	runner.Record("info", fmt.Sprintf("proxy started on %s:%d (%s)", e.st.Inbound.Listen, e.st.Inbound.SocksPort, e.st.Mode))
	return nil
}

func (e *Engine) restartLocked() error {
	if e.inst != nil {
		e.inst.Close()
		e.inst = nil
	}
	err := e.startLocked()
	e.lastErr = Classify(err)
	return e.lastErrOrNil()
}

func (e *Engine) restartIfRunningLocked() error {
	if e.inst == nil {
		return nil
	}
	return e.restartLocked()
}

// Stop stops the proxy.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inst != nil {
		e.inst.Close()
		e.inst = nil
		runner.Record("info", "proxy stopped")
	}
}

// Status is shown on the main screen and in the notification.
type Status struct {
	Running        bool      `json:"running"`
	Mode           string    `json:"mode"`
	Listen         string    `json:"listen"`
	SocksPort      int       `json:"socks_port"`
	HTTPPort       int       `json:"http_port"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	Up             int64     `json:"up"`
	Down           int64     `json:"down"`
	AuthFailures   int64     `json:"auth_failures"`
	HasKey         bool      `json:"has_key"`
	HasGatewayKey  bool      `json:"has_gateway_key"`
	HasConfig      bool      `json:"has_config"`
	Country        string    `json:"country"`
	CountryName    string    `json:"country_name"`
	FetchedAt      time.Time `json:"fetched_at,omitempty"`
	SubscriptionTo string    `json:"subscription_to,omitempty"`
	OwnServer      string    `json:"own_server,omitempty"`
	LastError      *Error    `json:"last_error,omitempty"`
}

// Status returns the current status.
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := Status{
		Running: e.inst != nil, Mode: string(e.st.Mode), Listen: e.st.Inbound.Listen,
		SocksPort: e.st.Inbound.SocksPort, HTTPPort: e.st.Inbound.HTTPPort,
		AuthFailures: runner.AuthFailures(), HasKey: e.st.Amnezia.Key != "",
		HasGatewayKey: len(e.st.Amnezia.GatewayPEMs) > 0, HasConfig: e.st.Amnezia.Last != nil,
		FetchedAt: e.st.Amnezia.FetchedAt, LastError: e.lastErr,
	}
	if e.inst != nil {
		s.StartedAt = e.startedAt
		s.Up, s.Down = e.inst.Traffic()
	}
	if l := e.st.Amnezia.Last; l != nil {
		s.Country, s.CountryName, s.SubscriptionTo = l.Country, l.CountryName, l.SubscriptionEnd
	}
	if o := e.activeOwnLocked(); o != nil {
		s.OwnServer = o.Name
	}
	return s
}

// Countries from the last answer.
func (e *Engine) Countries() []account.Country {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.st.Amnezia.Last == nil {
		return nil
	}
	return append([]account.Country(nil), e.st.Amnezia.Last.Countries...)
}

// ExitIP checks the public address through the running proxy (on demand only).
func (e *Engine) ExitIP(ctx context.Context) (string, error) {
	e.mu.Lock()
	inst, url := e.inst, e.st.Options.ExitIPURL
	e.mu.Unlock()
	if inst == nil {
		return "", newErr(CodeNotRunning, nil)
	}
	ip, err := inst.ExitIP(ctx, url)
	if err != nil {
		return "", newErr(CodeNetwork, err)
	}
	return ip, nil
}

// TestChain measures a request through the chain without the local proxy running: it starts a temporary
// in-process instance (no listening port) for the current mode and fetches the exit IP.
func (e *Engine) TestChain(ctx context.Context) (ip string, ms int64, err error) {
	e.mu.Lock()
	var amz json.RawMessage
	if e.st.Amnezia.Last != nil {
		amz = e.st.Amnezia.Last.XrayConfig
	}
	mode, url := e.st.Mode, e.st.Options.ExitIPURL
	var own links.Outbound
	if o := e.activeOwnLocked(); o != nil {
		own = o.Server.Outbound
	}
	e.mu.Unlock()
	if mode != xrayconf.ModeOwn && amz == nil {
		return "", 0, newErr(CodeNoConfig, nil)
	}
	cfg, err := xrayconf.BuildDialer(mode, amz, own)
	if err != nil {
		return "", 0, Classify(err)
	}
	inst, err := runner.Start(cfg)
	if err != nil {
		return "", 0, Classify(err)
	}
	defer inst.Close()
	t0 := time.Now()
	ip, err = inst.ExitIP(ctx, url)
	if err != nil {
		return "", 0, newErr(CodeNetwork, err)
	}
	return ip, time.Since(t0).Milliseconds(), nil
}

// --- routing and geo ------------------------------------------------------------------------------------

// TestRoute answers "where does host:port go" with the saved rules.
func (e *Engine) TestRoute(host string, port int, network string) (routing.Verdict, error) {
	e.mu.Lock()
	rt := e.st.Routing
	e.mu.Unlock()
	t, err := routing.NewTester(rt)
	if err != nil {
		return routing.Verdict{}, Classify(err)
	}
	return t.Match(host, port, network), nil
}

// GeoStatus reports the geo files.
func (e *Engine) GeoStatus() geo.Status { return geo.Stat(e.GeoDir()) }

// GeoTags lists tags in geosite.dat ("site") or geoip.dat ("ip").
func (e *Engine) GeoTags(kind string) ([]string, error) {
	name := geo.GeoSite
	if kind == "ip" {
		name = geo.GeoIP
	}
	tags, err := geo.Tags(filepath.Join(e.GeoDir(), name))
	if err != nil {
		return nil, newErr(CodeGeoMissing, err)
	}
	return tags, nil
}

// UpdateGeo downloads geo files from the selected source. While the proxy runs the download goes through
// it (not visible on the local network) and the proxy is restarted to load the new files.
func (e *Engine) UpdateGeo(ctx context.Context) error {
	e.mu.Lock()
	src := e.st.Options.GeoSource
	base := e.st.Options.GeoCustomURL
	inst := e.inst
	e.mu.Unlock()
	if s, ok := geo.SourceByID(src); ok {
		base = s.BaseURL
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	if inst != nil {
		client = inst.HTTPClient(5 * time.Minute)
	}
	if err := geo.Download(ctx, client, base, e.GeoDir(), src != "custom"); err != nil {
		return newErr(CodeNetwork, err)
	}
	runner.Record("info", "geo files updated")
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.restartIfRunningLocked()
}

// --- misc -----------------------------------------------------------------------------------------------

// Forget deletes everything (key, identity, servers, settings) and stops the proxy. The Amnezia device slot
// stays used until it is freed in the Personal Dashboard.
func (e *Engine) Forget() error {
	e.Stop()
	e.mu.Lock()
	defer e.mu.Unlock()
	os.RemoveAll(e.GeoDir())
	e.st = defaultState()
	return e.persist()
}

// SetGatewayURL overrides the gateway base URL (empty = production). For tests and mirrors.
func (e *Engine) SetGatewayURL(url string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.st.Amnezia.GatewayURL = strings.TrimSpace(url)
	return e.persist()
}
