package runner_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"github.com/nuc134r/sockmnezia/core/internal/mockgw"
	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/nuc134r/sockmnezia/core/xrayconf"

	// Server-side features for the fake upstreams.
	_ "github.com/xtls/xray-core/proxy/vless/inbound"
)

const testUUID = "00000000-0000-4000-8000-0000000000aa"

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

// httpTarget answers every request with body.
func httpTarget(t *testing.T, body string) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// udpEcho echoes datagrams with a prefix.
func udpEcho(t *testing.T) int {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, a, err := c.ReadFrom(buf)
			if err != nil {
				return
			}
			c.WriteTo(append([]byte("echo:"), buf[:n]...), a)
		}
	}()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// upstream starts a fake VLESS server. Requests for *.test domains are redirected to redirects[domain];
// everything else leaves through a plain freedom outbound (used when another hop is dialed through it).
func upstream(t *testing.T, network string, redirects map[string]string) (int, func()) {
	t.Helper()
	port := freePort(t)
	stream := map[string]any{"network": network, "security": "none"}
	switch network {
	case "xhttp":
		stream["xhttpSettings"] = map[string]any{"path": "/x"}
	case "ws":
		stream["wsSettings"] = map[string]any{"path": "/ws"}
	}
	// Test upstreams must reach loopback targets; Xray's freedom blocks private IPs for VLESS inbounds by default.
	allow := map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}}
	outs := []any{map[string]any{"tag": "out", "protocol": "freedom", "settings": allow}}
	var rules []any
	i := 0
	for dom, target := range redirects {
		tag := fmt.Sprintf("redir%d", i)
		i++
		outs = append(outs, map[string]any{"tag": tag, "protocol": "freedom", "settings": map[string]any{"redirect": target, "finalRules": allow["finalRules"]}})
		rules = append(rules, map[string]any{"domain": []any{"full:" + dom}, "outboundTag": tag})
	}
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": port, "protocol": "vless",
			"settings":       map[string]any{"clients": []any{map[string]any{"id": testUUID}}, "decryption": "none"},
			"streamSettings": stream}},
		"outbounds": outs,
		"routing":   map[string]any{"rules": rules},
	}
	b, _ := json.Marshal(cfg)
	inst, err := runner.Start(b)
	if err != nil {
		t.Fatalf("upstream: %v", err)
	}
	stop := func() { inst.Close() }
	t.Cleanup(stop)
	return port, stop
}

// amneziaConfig mimics the gateway's chain: proxy (tcp) dialed through proxy-relay (xhttp).
func amneziaConfig(mainPort, relayPort int) json.RawMessage {
	return amneziaConfigRelay(mainPort, relayPort, "xhttp")
}

// amneziaConfigRelay is amneziaConfig with the relay on another transport ("xhttp" or "tcp").
func amneziaConfigRelay(mainPort, relayPort int, relayNet string) json.RawMessage {
	x := mockgw.XrayConfig(testUUID)
	outs := x["outbounds"].([]any)
	main := outs[0].(map[string]any)
	main["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)["address"] = "127.0.0.1"
	main["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)["port"] = mainPort
	delete(main["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)["users"].([]any)[0].(map[string]any), "flow")
	ms := main["streamSettings"].(map[string]any)
	ms["security"] = "none"
	delete(ms, "realitySettings")
	relay := outs[1].(map[string]any)
	relay["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)["address"] = "127.0.0.1"
	relay["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)["port"] = relayPort
	rs := relay["streamSettings"].(map[string]any)
	rs["security"] = "none"
	delete(rs, "realitySettings")
	rs["xhttpSettings"] = map[string]any{"path": "/x"}
	if relayNet != "xhttp" {
		rs["network"] = relayNet
		delete(rs, "xhttpSettings")
		delete(relay["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)["users"].([]any)[0].(map[string]any), "flow")
	}
	b, _ := json.Marshal(x)
	return b
}

func ownLink(port int) links.Outbound {
	s, err := links.Parse(fmt.Sprintf("vless://%s@127.0.0.1:%d?type=ws&path=%%2Fws&security=none#own", testUUID, port))
	if err != nil {
		panic(err)
	}
	return s.Outbound
}

type client struct {
	t    *testing.T
	in   xrayconf.Inbound
	inst *runner.Instance
}

func startClient(t *testing.T, spec xrayconf.Spec) *client {
	t.Helper()
	if spec.Inbound.SocksPort == 0 {
		spec.Inbound = xrayconf.Inbound{SocksPort: freePort(t), User: "u-" + t.Name()[:4], Pass: "correct-horse-battery", UDP: spec.Inbound.UDP, HTTPPort: spec.Inbound.HTTPPort}
	}
	cfg, err := xrayconf.Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := runner.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inst.Close() })
	return &client{t: t, in: spec.Inbound, inst: inst}
}

func (c *client) socksAddr() string { return fmt.Sprintf("127.0.0.1:%d", c.in.SocksPort) }

// get fetches http://host/ through the SOCKS5 inbound with the given credentials.
func (c *client) get(host, user, pass string) (string, error) {
	d, err := proxy.SOCKS5("tcp", c.socksAddr(), &proxy.Auth{User: user, Password: pass}, &net.Dialer{Timeout: 3 * time.Second})
	if err != nil {
		return "", err
	}
	hc := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		DialContext:       d.(proxy.ContextDialer).DialContext,
		DisableKeepAlives: true,
	}}
	resp, err := hc.Get("http://" + host + "/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func (c *client) mustGet(host, want string) {
	c.t.Helper()
	got, err := c.get(host, c.in.User, c.in.Pass)
	if err != nil || got != want {
		c.t.Fatalf("GET %s: %q %v, want %q", host, got, err, want)
	}
}

// retryWindow covers Xray's dial retries (vless outbound: 5 attempts with backoff) after a request failed.
const retryWindow = 3 * time.Second

func (c *client) mustFail(host string) {
	c.t.Helper()
	// A failed request leaves Xray retrying the dial in the background for a few seconds. Let those retries
	// finish before this test's instances close and the next test starts one: starting an instance writes
	// process-wide dialer globals that a retrying dial reads, which the race detector reports (D25/D26).
	// Registered after startClient's cleanup, so it runs first.
	c.t.Cleanup(func() { time.Sleep(retryWindow) })
	if got, err := c.get(host, c.in.User, c.in.Pass); err == nil && got != "" {
		c.t.Fatalf("GET %s should fail, got %q", host, got)
	}
}

func TestAmneziaChain(t *testing.T) {
	target := httpTarget(t, "hello-amnezia")
	mainPort, _ := upstream(t, "tcp", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", target)})
	relayPort, stopRelay := upstream(t, "xhttp", nil)
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.AmneziaHop(amneziaConfig(mainPort, relayPort))}})
	c.mustGet("target.test", "hello-amnezia")

	// In-process dialing (exit-IP check, geo downloads) goes the same way.
	resp, err := c.inst.HTTPClient(5 * time.Second).Get("http://target.test/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "hello-amnezia" {
		t.Fatalf("in-process dial: %q", b)
	}
	up, down := c.inst.Traffic()
	if up == 0 || down == 0 {
		t.Errorf("traffic counters: %d/%d", up, down)
	}

	// The main hop is dialed through the relay: without the relay nothing works (no fallback).
	stopRelay()
	c.mustFail("target.test")
}

func TestOwnServerThroughAmnezia(t *testing.T) {
	targetOwn := httpTarget(t, "hello-own")
	targetAmz := httpTarget(t, "hello-amnezia")
	mainPort, stopMain := upstream(t, "tcp", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", targetAmz)})
	relayPort, _ := upstream(t, "xhttp", nil)
	ownPort, _ := upstream(t, "ws", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", targetOwn)})
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.AmneziaHop(amneziaConfig(mainPort, relayPort)), xrayconf.OwnHop(ownLink(ownPort))}})
	c.mustGet("target.test", "hello-own") // exit is the own server
	stopMain()
	c.mustFail("target.test") // own server is only reached through Amnezia: fail-closed
}

func TestThreeHopChain(t *testing.T) {
	// own (entry) -> Amnezia (relay -> main) -> own (exit). Each hop is a separate upstream; only the exit
	// knows the target, so the answer proves the whole chain was used, and stopping any hop breaks it.
	// The relay is plain TCP here: an xhttp outbound dialed through another hop trips data races inside Xray
	// that the race detector reports (benign in practice; see docs/DECISIONS.md D25). The gateway-shaped
	// config with the xhttp relay in the middle is load-checked in TestGatewayShapedConfigLoads.
	targetExit := httpTarget(t, "hello-exit")
	entryPort, _ := upstream(t, "ws", nil)
	mainPort, stopMain := upstream(t, "tcp", nil)
	relayPort, _ := upstream(t, "tcp", nil)
	exitPort, _ := upstream(t, "ws", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", targetExit)})
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{
		xrayconf.OwnHop(ownLink(entryPort)), xrayconf.AmneziaHop(amneziaConfigRelay(mainPort, relayPort, "tcp")), xrayconf.OwnHop(ownLink(exitPort)),
	}})
	c.mustGet("target.test", "hello-exit")
	stopMain()
	c.mustFail("target.test") // the middle hop is gone: no shortcut from entry to exit
}

func TestTemporaryInstanceDoesNotHijackTheChain(t *testing.T) {
	// The running proxy's hops are resolved through Xray's process-wide outbound registry. A temporary
	// instance (location ping, connection check, fetch) must not take it over, neither while it runs nor
	// after it closes.
	target := httpTarget(t, "hello-amnezia")
	mainPort, _ := upstream(t, "tcp", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", target)})
	relayPort, _ := upstream(t, "tcp", nil)
	amz := xrayconf.AmneziaHop(amneziaConfigRelay(mainPort, relayPort, "tcp"))
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{amz}})
	c.mustGet("target.test", "hello-amnezia")

	otherMain, _ := upstream(t, "tcp", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", httpTarget(t, "other"))})
	cfg, err := xrayconf.BuildDialer([]xrayconf.Hop{xrayconf.AmneziaHop(amneziaConfigRelay(otherMain, relayPort, "tcp"))})
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := runner.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.mustGet("target.test", "hello-amnezia") // not through the temporary instance's hops
	resp, err := tmp.HTTPClient(5 * time.Second).Get("http://target.test/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "other" {
		t.Fatalf("temporary instance: %q", b)
	}
	tmp.Close()
	c.mustGet("target.test", "hello-amnezia") // still working after the temporary instance is gone
}

func TestOwnOnlyAndRoutingBlock(t *testing.T) {
	target := httpTarget(t, "hello-own")
	ownPort, _ := upstream(t, "ws", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", target), "ads.test": fmt.Sprintf("127.0.0.1:%d", target)})
	rt := routing.Default()
	rt.Rules = []routing.Rule{{Domain: []string{"domain:ads.test"}, OutboundTag: routing.OutBlock}}
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(ownLink(ownPort))}, Routing: rt})
	c.mustGet("target.test", "hello-own")
	c.mustFail("ads.test")
}

func TestPrivateDestinationsBlocked(t *testing.T) {
	target := httpTarget(t, "secret-local-service")
	ownPort, _ := upstream(t, "ws", nil)
	// Even a user rule sending loopback "direct" cannot reopen it.
	rt := routing.Default()
	rt.Rules = []routing.Rule{{IP: []string{"127.0.0.0/8"}, OutboundTag: routing.OutDirect}}
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(ownLink(ownPort))}, Routing: rt})
	for _, h := range []string{fmt.Sprintf("127.0.0.1:%d", target), fmt.Sprintf("localhost:%d", target)} {
		c.mustFail(h)
	}
	if _, err := c.inst.HTTPClient(3 * time.Second).Get(fmt.Sprintf("http://127.0.0.1:%d/", target)); err == nil {
		t.Fatal("in-process dial reached loopback")
	}
}

func TestAuthRequired(t *testing.T) {
	target := httpTarget(t, "ok")
	ownPort, _ := upstream(t, "ws", map[string]string{"target.test": fmt.Sprintf("127.0.0.1:%d", target)})
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(ownLink(ownPort))}, Inbound: xrayconf.Inbound{HTTPPort: freePort(t)}})
	before := runner.AuthFailures()

	// No-auth greeting: the server must refuse every method (0xFF).
	conn, err := net.Dial("tcp", c.socksAddr())
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte{5, 1, 0})
	reply := make([]byte, 2)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, reply); err != nil || !bytes.Equal(reply, []byte{5, 0xff}) {
		t.Fatalf("no-auth greeting answered %x %v", reply, err)
	}
	conn.Close()

	// SOCKS4 is refused.
	conn, _ = net.Dial("tcp", c.socksAddr())
	conn.Write([]byte{4, 1, 0, 80, 1, 2, 3, 4, 0})
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if n, _ := conn.Read(reply); n > 0 && reply[0] == 0 && reply[1] == 0x5a {
		t.Fatal("socks4 granted")
	}
	conn.Close()

	if _, err := c.get("target.test", c.in.User, "wrong"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err := c.get("target.test", "other", c.in.Pass); err == nil {
		t.Fatal("wrong user accepted")
	}
	c.mustGet("target.test", "ok")

	// HTTP inbound: 407 without credentials, works with them.
	pu, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", c.in.HTTPPort))
	hc := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu)}}
	resp, err := hc.Get("http://target.test/")
	if err != nil || resp.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("http without auth: %v %v", resp, err)
	}
	resp.Body.Close()
	pu.User = url.UserPassword(c.in.User, c.in.Pass)
	hc = &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu)}}
	resp, err = hc.Get("http://target.test/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "ok" {
		t.Fatalf("http with auth: %q", b)
	}
	time.Sleep(100 * time.Millisecond)
	if runner.AuthFailures()-before < 2 {
		t.Errorf("auth failures not counted: %d", runner.AuthFailures()-before)
	}
}

// socks5UDP opens an authenticated UDP association and returns the control connection and relay address.
func socks5UDP(t *testing.T, addr, user, pass string, local *net.UDPAddr) (net.Conn, *net.UDPAddr, byte) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	conn.Write([]byte{5, 1, 2})
	m := make([]byte, 2)
	io.ReadFull(r, m)
	if m[1] != 2 {
		t.Fatalf("method %x", m)
	}
	auth := []byte{1, byte(len(user))}
	auth = append(auth, user...)
	auth = append(auth, byte(len(pass)))
	auth = append(auth, pass...)
	conn.Write(auth)
	io.ReadFull(r, m)
	if m[1] != 0 {
		t.Fatalf("auth status %x", m)
	}
	req := []byte{5, 3, 0, 1}
	req = append(req, local.IP.To4()...)
	req = binary.BigEndian.AppendUint16(req, uint16(local.Port))
	conn.Write(req)
	head := make([]byte, 4)
	if _, err := io.ReadFull(r, head); err != nil {
		t.Fatal(err)
	}
	if head[1] != 0 {
		return conn, nil, head[1]
	}
	ip := make([]byte, 4)
	io.ReadFull(r, ip)
	p := make([]byte, 2)
	io.ReadFull(r, p)
	conn.SetDeadline(time.Time{})
	return conn, &net.UDPAddr{IP: net.IP(ip), Port: int(binary.BigEndian.Uint16(p))}, 0
}

func udpPacket(domain string, port int, data string) []byte {
	b := []byte{0, 0, 0, 3, byte(len(domain))}
	b = append(b, domain...)
	b = binary.BigEndian.AppendUint16(b, uint16(port))
	return append(b, data...)
}

func readUDP(c *net.UDPConn, d time.Duration) (string, bool) {
	c.SetReadDeadline(time.Now().Add(d))
	buf := make([]byte, 2048)
	n, err := c.Read(buf)
	if err != nil {
		return "", false
	}
	return string(buf[:n]), true
}

func TestUDP(t *testing.T) {
	echo := udpEcho(t)
	ownPort, _ := upstream(t, "ws", map[string]string{"echo.test": fmt.Sprintf("127.0.0.1:%d", echo)})
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(ownLink(ownPort))}, Inbound: xrayconf.Inbound{UDP: true}})

	legit, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer legit.Close()
	ctl, relay, status := socks5UDP(t, c.socksAddr(), c.in.User, c.in.Pass, legit.LocalAddr().(*net.UDPAddr))
	if status != 0 {
		t.Fatalf("associate refused: %d", status)
	}
	defer ctl.Close()
	if relay.Port == c.in.SocksPort {
		t.Fatal("UDP relay must be a per-association port, not the main port")
	}

	// Another local socket races to the relay port: dropped (the association is pinned to the declared source).
	intruder, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer intruder.Close()
	intruder.WriteToUDP(udpPacket("echo.test", 7, "intruder"), relay)
	if got, ok := readUDP(intruder, 700*time.Millisecond); ok {
		t.Fatalf("intruder got an answer: %q", got)
	}

	legit.WriteToUDP(udpPacket("echo.test", 7, "ping"), relay)
	got, ok := readUDP(legit, 3*time.Second)
	if !ok || !strings.HasSuffix(got, "echo:ping") {
		t.Fatalf("legit client: %q %v", got, ok)
	}

	// Datagrams to the main SOCKS port go nowhere.
	stray, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer stray.Close()
	stray.WriteToUDP(udpPacket("echo.test", 7, "stray"), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: c.in.SocksPort})
	if got, ok := readUDP(stray, 700*time.Millisecond); ok {
		t.Fatalf("main port answered UDP: %q", got)
	}

	// Closing the control connection closes the association.
	ctl.Close()
	time.Sleep(300 * time.Millisecond)
	legit.WriteToUDP(udpPacket("echo.test", 7, "after"), relay)
	if got, ok := readUDP(legit, 700*time.Millisecond); ok {
		t.Fatalf("association outlived its TCP connection: %q", got)
	}
}

func TestUDPDisabledByDefault(t *testing.T) {
	ownPort, _ := upstream(t, "ws", nil)
	c := startClient(t, xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(ownLink(ownPort))}})
	local, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer local.Close()
	conn, relay, status := socks5UDP(t, c.socksAddr(), c.in.User, c.in.Pass, local.LocalAddr().(*net.UDPAddr))
	defer conn.Close()
	if status == 0 || relay != nil {
		t.Fatalf("UDP associate granted while UDP is off (%v)", relay)
	}
}

func TestGatewayShapedConfigLoads(t *testing.T) {
	// The real answer: reality on both hops, empty serverName/shortId, xhttp relay with xmux, vision flow.
	in := xrayconf.Inbound{SocksPort: 20001, User: "user", Pass: "password"}
	x, _ := json.Marshal(mockgw.XrayConfig(testUUID))
	own, _ := links.Parse("vless://" + testUUID + "@vps.example.com:443?type=xhttp&security=reality&pbk=CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC&sni=www.example.com&fp=chrome")
	amz, ownHop := xrayconf.AmneziaHop(x), xrayconf.OwnHop(own.Outbound)
	for name, hops := range map[string][]xrayconf.Hop{
		"amnezia": {amz}, "amnezia-own": {amz, ownHop}, "own-amnezia": {ownHop, amz}, "own-amnezia-own": {ownHop, amz, ownHop},
	} {
		cfg, err := xrayconf.Build(xrayconf.Spec{Hops: hops, Inbound: in})
		if err != nil {
			t.Fatal(err)
		}
		if err := runner.Check(cfg); err != nil {
			t.Fatalf("%s: Xray rejects the config: %v", name, err)
		}
	}
}

func TestPortInUse(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	ownPort, _ := upstream(t, "ws", nil)
	cfg, _ := xrayconf.Build(xrayconf.Spec{Hops: []xrayconf.Hop{xrayconf.OwnHop(ownLink(ownPort))},
		Inbound: xrayconf.Inbound{SocksPort: l.Addr().(*net.TCPAddr).Port, User: "u", Pass: "p"}})
	if inst, err := runner.Start(cfg); err == nil {
		inst.Close()
		t.Fatal("started on a busy port")
	}
}

var _ = context.Background
