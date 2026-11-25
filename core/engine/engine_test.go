package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nuc134r/sockmnezia/core/internal/mockgw"
	"github.com/nuc134r/sockmnezia/core/routing"
)

// xorBox stands in for the Android Keystore box.
type xorBox struct{}

func (xorBox) Seal(p []byte) ([]byte, error) { return xor(p), nil }
func (xorBox) Open(s []byte) ([]byte, error) { return xor(s), nil }
func xor(b []byte) []byte {
	o := make([]byte, len(b))
	for i, c := range b {
		o[i] = c ^ 0x5a
	}
	return o
}

func decoyPEM(t *testing.T) string {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestDefaultsArePrivateAndRandom(t *testing.T) {
	a, err := Open(t.TempDir(), nil, "linux")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Open(t.TempDir(), nil, "linux")
	sa, sb := a.Snapshot(), b.Snapshot()
	in := sa.Inbound
	if in.Listen != "127.0.0.1" || in.SocksPort < portMin || in.SocksPort > portMax || wellKnownPorts[in.SocksPort] {
		t.Fatalf("inbound %+v", in)
	}
	if len(in.User) != 16 || len(in.Pass) != 32 || in.UDP || in.HTTPPort != 0 {
		t.Fatalf("inbound %+v", in)
	}
	if sa.Inbound.Pass == sb.Inbound.Pass || sa.Amnezia.Identity == sb.Amnezia.Identity {
		t.Fatal("two installs share secrets")
	}
}

func TestFullAmneziaFlow(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	dir := t.TempDir()
	e, err := Open(dir, xorBox{}, "android")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetKey("vpn://garbage"); code(err) != CodeInvalidKey {
		t.Fatalf("garbage key: %v", err)
	}
	if err := e.Start(context.Background()); err == nil {
		t.Fatal("started without key/config")
	}
	e.SetKey(mockgw.SubscriptionKey())
	if err := e.Fetch(context.Background(), FetchRequest{}); code(err) != CodeNoGatewayKey {
		t.Fatalf("no gateway key: %v", err)
	}
	if n, err := e.SetGatewayKeysPEM(decoyPEM(t) + string(g.PEM)); err != nil || n != 2 {
		t.Fatalf("pems: %d %v", n, err)
	}
	e.SetGatewayURL(g.URL)
	if !e.NeedsRefresh() {
		t.Fatal("should need a fetch")
	}
	if err := e.Start(context.Background()); err != nil { // fetches first
		t.Fatal(err)
	}
	st := e.Status()
	if !st.Running || !st.HasConfig || st.Country != "de" || st.SocksPort == 0 {
		t.Fatalf("%+v", st)
	}
	if e.Snapshot().Amnezia.GatewayPEMs[0] != string(g.PEM) {
		t.Fatal("working key not moved first")
	}
	// The proxy really listens on loopback with the configured port.
	c, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", itoa(st.SocksPort)))
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	// Switching country re-fetches and restarts.
	if err := e.Fetch(context.Background(), FetchRequest{Country: "nl"}); err != nil {
		t.Fatal(err)
	}
	if s := e.Status(); s.Country != "nl" || !s.Running {
		t.Fatalf("%+v", s)
	}
	if got := len(e.Countries()); got != 2 {
		t.Fatalf("countries %d", got)
	}
	// Same identity on every request.
	reqs := g.Requests()
	if reqs[0].Payload["installation_uuid"] != reqs[len(reqs)-1].Payload["installation_uuid"] {
		t.Fatal("identity changed between fetches")
	}
	e.Stop()
	if e.Status().Running {
		t.Fatal("still running")
	}

	// State survives a reload and is not plaintext on disk.
	raw, _ := os.ReadFile(filepath.Join(dir, stateFile))
	if bytes.Contains(raw, []byte("synthetic-api-key")) || bytes.Contains(raw, []byte(e.Snapshot().Inbound.Pass)) {
		t.Fatal("secrets stored in plaintext")
	}
	e2, err := Open(dir, xorBox{}, "android")
	if err != nil {
		t.Fatal(err)
	}
	if e2.Snapshot().Amnezia.Identity != e.Snapshot().Amnezia.Identity || !e2.Status().HasConfig {
		t.Fatal("state lost")
	}
	if _, err := Open(dir, PlainBox{}, "android"); err == nil {
		t.Fatal("opened with the wrong box")
	}
}

func TestGatewayErrorsAreClassified(t *testing.T) {
	g := mockgw.New()
	defer g.Close()
	e, _ := Open(t.TempDir(), nil, "android")
	e.SetKey(mockgw.SubscriptionKey())
	e.SetGatewayKeysPEM(string(g.PEM))
	e.SetGatewayURL(g.URL)
	for reply, want := range map[string]string{
		"409": CodeDeviceLimit, "422": CodeSubscriptionExpired, "402c": CodeCaptchaRequired, "501": CodeUpdateRequired,
	} {
		r := map[string]map[string]any{
			"409":  {"http_status": 409},
			"422":  {"http_status": 422, "message": "Failed to retrieve subscription information. Is it activated?"},
			"402c": {"http_status": 402, "captcha_id": "c", "captcha_image": "aGk="},
			"501":  {"http_status": 501},
		}[reply]
		g.SetReply(func(mockgw.Request) map[string]any { return r })
		err := e.Fetch(context.Background(), FetchRequest{})
		if code(err) != want {
			t.Errorf("%s: %v", reply, err)
		}
		if want == CodeCaptchaRequired {
			var ee *Error
			errors.As(err, &ee)
			if ee.Captcha == nil || ee.Captcha.ID != "c" {
				t.Error("captcha not passed on")
			}
		}
	}
	e.SetGatewayKeysPEM(decoyPEM(t))
	g.SetReply(nil)
	if err := e.Fetch(context.Background(), FetchRequest{}); code(err) != CodeGatewayWrongKey {
		t.Fatal(err)
	}
}

// PingCountry does not touch the saved config and fails like Fetch would before making any network
// request, when the subscription key or gateway keys are missing.
func TestPingCountryValidatesBeforeFetching(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "android")
	if _, err := e.PingCountry(context.Background(), "nl"); code(err) != CodeInvalidKey {
		t.Fatalf("no key: %v", err)
	}
	e.SetKey(mockgw.SubscriptionKey())
	if _, err := e.PingCountry(context.Background(), "nl"); code(err) != CodeNoGatewayKey {
		t.Fatalf("no gateway key: %v", err)
	}
	// It never persists Amnezia.Last or the active country, unlike Fetch.
	if e.Snapshot().Amnezia.Last != nil || e.Snapshot().Amnezia.Country != "" {
		t.Fatal("PingCountry changed the saved state")
	}
}

func TestOwnServersAndSettings(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	if _, err := e.AddOwnServer("hy2://x@example.com:443", ""); code(err) != CodeUnsupportedLink {
		t.Fatal(err)
	}
	if _, err := e.AddOwnServer("nonsense", ""); code(err) != CodeInvalidLink {
		t.Fatal(err)
	}
	id1, err := e.AddOwnServer("trojan://p@one.example.com:443#one", "")
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := e.AddOwnServer("trojan://p@two.example.com:443", "Second")
	s := e.Snapshot()
	// The first server is selected on its own (no chain), because Amnezia is not set up.
	if s.Own[1].Name != "Second" || len(s.Chains) != 1 || s.ActiveServer != id1 || s.ActiveChain != "" {
		t.Fatalf("%+v", s)
	}
	cid, _ := e.SaveChain(Chain{Name: "pair", Hops: []ChainHop{{Kind: HopServer, Server: id2}, {Kind: HopServer, Server: id1}}})
	if err := e.RemoveOwnServer(id2); code(err) != CodeServerInUse || !strings.Contains(err.(*Error).Detail, "pair") {
		t.Fatal("a server used by a chain was deleted", err)
	}
	e.DeleteChain(cid)
	if err := e.RemoveOwnServer(id2); err != nil {
		t.Fatal(err)
	}

	set := e.GetSettings()
	set.Inbound.UDP = true
	set.Inbound.User = "my-own-user"
	set.Inbound.Pass = "my-own-password!"
	if err := e.SetSettings(set); err != nil {
		t.Fatal(err)
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if !e.Status().Running {
		t.Fatal("not running")
	}
	// Changing the port restarts on the new one.
	set.Inbound.SocksPort = RandomPort("127.0.0.1", set.Inbound.SocksPort)
	if err := e.SetSettings(set); err != nil {
		t.Fatal(err)
	}
	if e.Status().SocksPort != set.Inbound.SocksPort || !e.Status().Running {
		t.Fatal("not restarted")
	}

	bad := set
	bad.Inbound.Pass = "has space"
	var ee *Error
	if err := e.SetSettings(bad); !errors.As(err, &ee) || ee.Field != "pass" {
		t.Fatalf("%v", err)
	}
	bad = set
	bad.Inbound.Listen = "0.0.0.0"
	if err := e.SetSettings(bad); err == nil {
		t.Fatal("0.0.0.0 accepted")
	}
	bad = set
	bad.Routing.Rules = []routing.Rule{{OutboundTag: "direct"}}
	if err := e.SetSettings(bad); err == nil {
		t.Fatal("empty rule accepted")
	}
	bad = set
	bad.Options.GeoSource = "custom"
	bad.Options.GeoCustomURL = "http://example.com"
	if err := e.SetSettings(bad); err == nil {
		t.Fatal("http geo source accepted")
	}

	// Geo rules without files: refused at start with a clear code.
	geoSet := set
	geoSet.Routing, _ = routing.Default().Apply("bypass_ru", true)
	if err := e.SetSettings(geoSet); code(err) != CodeGeoMissing {
		t.Fatalf("geo: %v", err)
	}
	v, err := e.TestRoute("127.0.0.1", 80, "tcp")
	if code(err) != CodeGeoMissing && (err != nil || v.OutboundTag != "block") {
		t.Fatalf("%+v %v", v, err)
	}
}

func TestPortInUse(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	e.AddOwnServer("trojan://p@one.example.com:443", "")
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	set := e.GetSettings()
	set.Inbound.SocksPort = l.Addr().(*net.TCPAddr).Port
	e.SetSettings(set)
	err := e.Start(context.Background())
	if code(err) != CodePortInUse {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(e.Status().LastError.Code, "port") {
		t.Fatal("last error not kept")
	}
}

func TestForget(t *testing.T) {
	e, _ := Open(t.TempDir(), nil, "linux")
	e.SetKey(mockgw.SubscriptionKey())
	id := e.Snapshot().Amnezia.Identity
	e.Forget()
	s := e.Snapshot()
	if s.Amnezia.Key != "" || s.Amnezia.Identity == id {
		t.Fatal("not forgotten")
	}
}

func itoa(i int) string { return strconv.Itoa(i) }
