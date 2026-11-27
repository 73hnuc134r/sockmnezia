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
	vpnFD     int // the VpnService tun while VPN mode runs, -1 otherwise
	startedAt time.Time
	lastErr   *Error

	geoBusy    sync.Mutex                       // one geo update at a time (button and weekly job)
	updBusy    sync.Mutex                       // one app update download at a time
	updProg    atomic.Pointer[DownloadProgress] // progress of the running app update download
	lastUpdate *Update                          // the newest release from the last CheckUpdate
	geoProg    atomic.Pointer[geo.Progress]     // progress of the running geo update, nil when none runs
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
	e := &Engine{dir: dir, box: box, OS: osName, st: st, vpnFD: -1}
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
	via, viaErr := e.fetchHopsLocked()
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
	if viaErr != nil {
		return Classify(viaErr)
	}
	client := &gateway.Client{BaseURL: url}
	if len(via) > 0 {
		cfg, err := xrayconf.BuildDialer(via)
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
	if e.inst != nil && amneziaIndex(e.activeChainLocked()) >= 0 {
		return e.restartLocked()
	}
	return nil
}

// --- own servers ----------------------------------------------------------------------------------------

// AddOwnServer parses a link/JSON and saves it. Returns its ID. The very first server is selected when
// Amnezia is not set up (so an own-server-only user is ready to go).
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
	if len(e.st.Own) == 1 && e.st.Amnezia.Key == "" && e.inst == nil {
		e.st.ActiveChain, e.st.ActiveServer = "", id
	}
	return id, e.persist()
}

// RemoveOwnServer deletes a saved server. A server used by a chain is not deleted (CodeServerInUse, the
// chain names in Detail): the user removes it from those chains first. Deleting the selected server selects
// the first chain (a running proxy switches to it, or stops when there is none).
func (e *Engine) RemoveOwnServer(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if used := e.chainsUsingLocked(id); len(used) > 0 {
		return &Error{Code: CodeServerInUse, Detail: strings.Join(used, ", ")}
	}
	out := e.st.Own[:0]
	for _, s := range e.st.Own {
		if s.ID != id {
			out = append(out, s)
		}
	}
	e.st.Own = out
	delete(e.st.Tests, testKey("", id))
	if e.st.Options.FetchVia == id {
		e.st.Options.FetchVia = ""
	}
	if e.st.ActiveServer != id {
		return e.persist()
	}
	e.selectFallbackLocked()
	if err := e.persist(); err != nil {
		return err
	}
	return e.restartOrStopLocked()
}

// UpdateOwnServer replaces a saved server's settings (link or outbound JSON, as from the endpoint editor),
// keeping its ID so the chains that use it follow. name, when not empty, overrides the link's name. Refused
// when a chain using the server would no longer be valid (e.g. a UDP transport after the first hop). A
// running proxy that uses the server restarts.
func (e *Engine) UpdateOwnServer(id, link, name string) error {
	s, err := links.Parse(link)
	if err != nil {
		if errors.Is(err, links.ErrUnsupported) {
			return newErr(CodeUnsupportedLink, err)
		}
		return newErr(CodeInvalidLink, err)
	}
	if strings.TrimSpace(name) != "" {
		s.Name = strings.TrimSpace(name)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	srv := e.serverLocked(id)
	if srv == nil {
		return newErr(CodeNoOwnServer, errors.New("unknown server"))
	}
	old := *srv
	srv.Name, srv.Link, srv.Server = s.Name, strings.TrimSpace(link), s
	for i := range e.st.Chains {
		c := &e.st.Chains[i]
		for _, h := range c.Hops {
			if h.Kind == HopServer && h.Server == id {
				if verr := e.validateLocked(c); verr != nil {
					*srv = old
					verr.Detail = c.Name + ": " + verr.Detail
					return verr
				}
				break
			}
		}
	}
	delete(e.st.Tests, testKey("", id)) // measured with the old settings
	if err := e.persist(); err != nil {
		return err
	}
	if c := e.activeChainLocked(); c != nil {
		for _, h := range c.Hops {
			if h.Server == id {
				return e.restartIfRunningLocked()
			}
		}
	}
	return nil
}

// ServerForEdit returns a saved server's share link (or outbound JSON) and, when a link can describe it,
// its editable fields (nil: JSON only).
func (e *Engine) ServerForEdit(id string) (string, *links.Endpoint, error) {
	e.mu.Lock()
	s := e.serverLocked(id)
	if s == nil || s.Server == nil {
		e.mu.Unlock()
		return "", nil, newErr(CodeNoOwnServer, errors.New("unknown server"))
	}
	link := shareServerLocked(s)
	e.mu.Unlock()
	if ep, err := links.EndpointFromLink(link); err == nil {
		return link, &ep, nil
	}
	return link, nil, nil
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

// --- settings -------------------------------------------------------------------------------------------

// Settings is the editable part of the state.
type Settings struct {
	Inbound xrayconf.Inbound `json:"inbound"`
	Routing routing.Settings `json:"routing"`
	Options Options          `json:"options"`
}

// GetSettings returns the current settings.
func (e *Engine) GetSettings() Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Settings{Inbound: e.st.Inbound, Routing: e.st.Routing, Options: e.st.Options}
}

// SetSettings validates and stores settings; a running proxy is restarted.
func (e *Engine) SetSettings(s Settings) error {
	if s.Inbound.Listen == "" {
		s.Inbound.Listen = "127.0.0.1"
	}
	if err := xrayconf.ValidateInbound(s.Inbound); err != nil {
		return Classify(err)
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
	switch s.Options.Mode {
	case "":
		s.Options.Mode = ModeProxy
	case ModeProxy, ModeVPN:
	default:
		return &Error{Code: CodeInvalidSettings, Field: "mode", Detail: "unknown mode"}
	}
	if err := validateVPN(&s.Options.VPN); err != nil {
		return err
	}
	switch s.Options.SystemProxy {
	case "":
	case SystemProxyOn, SystemProxyPAC:
		if p := s.Options.SystemProxyPort; p == 0 || p == s.Inbound.SocksPort || p == s.Inbound.HTTPPort {
			s.Options.SystemProxyPort = RandomPort("127.0.0.1", s.Inbound.SocksPort, s.Inbound.HTTPPort)
		}
	default:
		return &Error{Code: CodeInvalidSettings, Field: "system_proxy", Detail: "unknown mode"}
	}
	s.Options.LegacyFetchViaOwn = false
	e.mu.Lock()
	defer e.mu.Unlock()
	if id := s.Options.FetchVia; id != "" && e.serverLocked(id) == nil {
		return &Error{Code: CodeNoOwnServer, Field: "fetch_via", Detail: "unknown server"}
	}
	e.st.Inbound, e.st.Routing, e.st.Options = s.Inbound, s.Routing, s.Options
	runner.SetLevel(s.Options.LogLevel)
	if err := e.persist(); err != nil {
		return err
	}
	return e.restartIfRunningLocked()
}

// --- proxy ----------------------------------------------------------------------------------------------

func (e *Engine) buildLocked() ([]byte, error) {
	hops, err := e.hopsLocked(e.activeChainLocked(), e.amneziaConfigLocked())
	if err != nil {
		return nil, err
	}
	spec := xrayconf.Spec{Hops: hops, Inbound: e.st.Inbound, Routing: e.st.Routing, LogLevel: e.st.Options.LogLevel, VPN: e.vpnSpecLocked()}
	if e.st.Options.SystemProxy != "" {
		spec.SystemProxyPort = e.st.Options.SystemProxyPort
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
	if amneziaIndex(e.activeChainLocked()) < 0 || e.st.Amnezia.Key == "" {
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
	hops := 0
	if c := e.activeChainLocked(); c != nil {
		hops = len(c.Hops)
	}
	mode := ModeProxy
	if e.vpnFD >= 0 {
		mode = ModeVPN
	}
	runner.Record("info", fmt.Sprintf("proxy started on %s:%d (%d hops, %s mode)", e.st.Inbound.Listen, e.st.Inbound.SocksPort, hops, mode))
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
	e.stopLocked()
}

func (e *Engine) stopLocked() {
	if e.inst != nil {
		e.inst.Close()
		e.inst = nil
		runner.Record("info", "proxy stopped")
	}
	e.vpnFD = -1 // the app closes its descriptor after Stop
}

// Status is shown on the main screen and in the notification.
type Status struct {
	Running        bool      `json:"running"`
	VPN            bool      `json:"vpn"`  // running in VPN mode
	Mode           string    `json:"mode"` // the mode Start should use (Options.Mode)
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
	OwnServer      string    `json:"own_server,omitempty"` // name of the exit hop when it is an own server
	ChainID        string    `json:"chain_id,omitempty"`   // the active saved chain
	ServerID       string    `json:"server_id,omitempty"`  // the active single server (instead of a chain)
	ChainName      string    `json:"chain_name,omitempty"`
	Hops           []HopInfo `json:"hops,omitempty"`            // hops of the active chain, phone first
	AmneziaHop     int       `json:"amnezia_hop,omitempty"`     // 1-based position of Amnezia in the chain, 0 = none
	ChainError     *Error    `json:"chain_error,omitempty"`     // why the active chain cannot run
	DeviceID       string    `json:"device_id,omitempty"`       // installation UUID, as listed in Amnezia's device list
	Chain          []string  `json:"chain,omitempty"`           // server addresses, phone first, exit last
	ChainCountries []string  `json:"chain_countries,omitempty"` // geoip.dat country code per Chain entry, "" if unknown
	LastError      *Error    `json:"last_error,omitempty"`
	// SystemProxy and SystemProxyPort: the desktop OS proxy mode (Options.SystemProxy) and its port.
	SystemProxy     string `json:"system_proxy,omitempty"`
	SystemProxyPort int    `json:"system_proxy_port,omitempty"`
}

// Status returns the current status.
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := Status{
		Running: e.inst != nil, VPN: e.inst != nil && e.vpnFD >= 0, Mode: modeOr(e.st.Options.Mode), Listen: e.st.Inbound.Listen,
		SocksPort: e.st.Inbound.SocksPort, HTTPPort: e.st.Inbound.HTTPPort,
		AuthFailures: runner.AuthFailures(), HasKey: e.st.Amnezia.Key != "",
		HasGatewayKey: len(e.st.Amnezia.GatewayPEMs) > 0, HasConfig: e.st.Amnezia.Last != nil,
		FetchedAt: e.st.Amnezia.FetchedAt, LastError: e.lastErr, DeviceID: e.st.Amnezia.Identity.InstallationUUID,
		SystemProxy: e.st.Options.SystemProxy, SystemProxyPort: e.st.Options.SystemProxyPort,
	}
	if e.inst != nil {
		s.StartedAt = e.startedAt
		s.Up, s.Down = e.inst.Traffic()
	}
	if l := e.st.Amnezia.Last; l != nil {
		s.Country, s.CountryName, s.SubscriptionTo = l.Country, l.CountryName, l.SubscriptionEnd
	}
	c := e.activeChainLocked()
	if c != nil {
		s.ChainID, s.ServerID, s.ChainName, s.AmneziaHop = c.ID, e.st.ActiveServer, e.chainNameLocked(c), amneziaIndex(c)+1
		for _, h := range c.Hops {
			s.Hops = append(s.Hops, e.hopInfoLocked(h))
		}
		if n := len(s.Hops); n > 0 && s.Hops[n-1].Kind == HopServer {
			s.OwnServer = s.Hops[n-1].Name
		}
	}
	s.ChainError = e.validateLocked(c)
	if hops, err := e.hopsLocked(c, e.amneziaConfigLocked()); err == nil {
		s.Chain, _ = xrayconf.Hops(hops)
		if len(s.Chain) > 0 {
			s.ChainCountries = make([]string, len(s.Chain))
			for i, addr := range s.Chain {
				s.ChainCountries[i] = geo.LookupCountry(e.GeoDir(), addr)
			}
		}
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

// TestChain measures a request through the chain. While the proxy runs it uses the running instance;
// otherwise it starts a temporary in-process instance (no listening port) for the active chain. country is
// the geoip.dat country code for ip ("" if the database isn't downloaded or has no match).
func (e *Engine) TestChain(ctx context.Context) (ip string, ms int64, country string, err error) {
	e.mu.Lock()
	if running, url, dir := e.inst, e.st.Options.ExitIPURL, e.GeoDir(); running != nil {
		e.mu.Unlock()
		t0 := time.Now()
		ip, err = running.ExitIP(ctx, url)
		if err != nil {
			return "", 0, "", newErr(CodeNetwork, err)
		}
		return ip, time.Since(t0).Milliseconds(), geo.LookupCountry(dir, ip), nil
	}
	c := e.activeChainLocked()
	e.mu.Unlock()
	return e.testHops(ctx, c)
}

// TestSaved measures a request through a saved chain (chainID) or a single own server (serverID) in a
// temporary instance, without selecting it: the running proxy keeps its chain (see D26).
func (e *Engine) TestSaved(ctx context.Context, chainID, serverID string) (ip string, ms int64, country string, err error) {
	e.mu.Lock()
	var c *Chain
	switch {
	case serverID != "":
		if e.serverLocked(serverID) == nil {
			e.mu.Unlock()
			return "", 0, "", newErr(CodeNoOwnServer, errors.New("unknown server"))
		}
		c = &Chain{Hops: []ChainHop{{Kind: HopServer, Server: serverID}}}
	default:
		c = e.chainLocked(chainID)
	}
	e.mu.Unlock()
	ip, ms, country, err = e.testHops(ctx, c)
	// Remember it for the lists (a cancelled test leaves the previous result).
	if ctx.Err() == nil {
		r := TestResult{At: time.Now(), Ms: ms, IP: ip, Country: country}
		if err != nil {
			r = TestResult{At: time.Now(), Error: Classify(err).Code}
		}
		e.mu.Lock()
		if e.st.Tests == nil {
			e.st.Tests = map[string]TestResult{}
		}
		e.st.Tests[testKey(chainID, serverID)] = r
		e.persist()
		e.mu.Unlock()
	}
	return ip, ms, country, err
}

// testHops measures a request through c in a temporary in-process instance (no listening port).
func (e *Engine) testHops(ctx context.Context, c *Chain) (ip string, ms int64, country string, err error) {
	e.mu.Lock()
	url, dir := e.st.Options.ExitIPURL, e.GeoDir()
	hops, err := e.hopsLocked(c, e.amneziaConfigLocked())
	e.mu.Unlock()
	if err != nil {
		return "", 0, "", Classify(err)
	}
	cfg, err := xrayconf.BuildDialer(hops)
	if err != nil {
		return "", 0, "", Classify(err)
	}
	inst, err := runner.Start(cfg)
	if err != nil {
		return "", 0, "", Classify(err)
	}
	defer inst.Close()
	t0 := time.Now()
	ip, err = inst.ExitIP(ctx, url)
	if err != nil {
		return "", 0, "", newErr(CodeNetwork, err)
	}
	return ip, time.Since(t0).Milliseconds(), geo.LookupCountry(dir, ip), nil
}

// PingCountry fetches a one-off Amnezia config for country and measures a request through the active chain
// with that config in the Amnezia hop (like TestChain). It does not touch the saved state: the
// fetched config is discarded after the measurement, and a running proxy keeps using its current config.
func (e *Engine) PingCountry(ctx context.Context, country string) (ms int64, err error) {
	e.mu.Lock()
	st := e.st
	key, pems, id, url := st.Amnezia.Key, st.Amnezia.GatewayPEMs, st.Amnezia.Identity, st.Amnezia.GatewayURL
	exitURL := st.Options.ExitIPURL
	chain := e.activeChainLocked()
	if chain != nil {
		c := *chain
		chain = &c
	}
	via, viaErr := e.fetchHopsLocked()
	e.mu.Unlock()
	if amneziaIndex(chain) < 0 {
		return 0, &Error{Code: CodeInvalidSettings, Field: "chain", Detail: "the active chain has no Amnezia hop"}
	}
	if viaErr != nil {
		return 0, Classify(viaErr)
	}

	if key == "" {
		return 0, newErr(CodeInvalidKey, errors.New("no subscription key"))
	}
	sub, err := vpnkey.ParseSubscription(key)
	if err != nil {
		return 0, Classify(err)
	}
	if len(pems) == 0 {
		return 0, newErr(CodeNoGatewayKey, errors.New("no gateway public key"))
	}
	var pubs []*rsa.PublicKey
	for _, p := range pems {
		k, err := pubkey.Parse([]byte(p))
		if err != nil {
			return 0, Classify(err)
		}
		pubs = append(pubs, k[0].Pub)
	}
	client := &gateway.Client{BaseURL: url}
	if len(via) > 0 {
		cfg, err := xrayconf.BuildDialer(via)
		if err != nil {
			return 0, Classify(err)
		}
		d, err := runner.Start(cfg)
		if err != nil {
			return 0, Classify(err)
		}
		defer d.Close()
		client.Dial = d.Dial
	}
	res, err := account.Fetch(ctx, client, pubs, account.Request{
		Key: sub, Identity: id, Country: country, OS: e.OS,
	})
	if err != nil {
		return 0, Classify(err)
	}
	e.mu.Lock()
	hops, err := e.hopsLocked(chain, res.XrayConfig)
	e.mu.Unlock()
	if err != nil {
		return 0, Classify(err)
	}
	cfg, err := xrayconf.BuildDialer(hops)
	if err != nil {
		return 0, Classify(err)
	}
	inst, err := runner.Start(cfg)
	if err != nil {
		return 0, Classify(err)
	}
	defer inst.Close()
	t0 := time.Now()
	if _, err = inst.ExitIP(ctx, exitURL); err != nil {
		return 0, newErr(CodeNetwork, err)
	}
	return time.Since(t0).Milliseconds(), nil
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

// GeoProgress is the progress of the running geo update, or nil when none runs.
func (e *Engine) GeoProgress() *geo.Progress {
	if p := e.geoProg.Load(); p != nil {
		c := *p
		return &c
	}
	return nil
}

// UpdateGeo downloads geo files from the selected source. While the proxy runs the download goes through
// it (not visible on the local network) and the proxy is restarted to load the new files. Progress is
// readable with GeoProgress while it runs; a second call meanwhile fails with CodeGeoBusy.
func (e *Engine) UpdateGeo(ctx context.Context) error {
	if !e.geoBusy.TryLock() {
		return newErr(CodeGeoBusy, errors.New("a geo update is already running"))
	}
	defer e.geoBusy.Unlock()
	e.geoProg.Store(&geo.Progress{Count: 2, Total: -1})
	defer e.geoProg.Store(nil)
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
	report := func(p geo.Progress) { e.geoProg.Store(&p) }
	if err := geo.Download(ctx, client, base, e.GeoDir(), src != "custom", report); err != nil {
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

// modeOr returns m, or ModeProxy for states saved before modes existed.
func modeOr(m string) string {
	if m == "" {
		return ModeProxy
	}
	return m
}
