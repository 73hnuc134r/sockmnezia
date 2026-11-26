package links

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEndpointRoundTrip(t *testing.T) {
	vm, _ := json.Marshal(map[string]any{"v": "2", "ps": "vm", "add": "vm.example.com", "port": "443", "id": uid,
		"net": "grpc", "tls": "tls", "path": "svc", "type": "multi", "sni": "cdn.example.com"})
	ui := base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:secret"))
	for _, l := range []string{
		"vless://" + uid + "@vps.example.com:443?type=tcp&security=reality&sni=www.example.org&fp=chrome&pbk=PUBKEY&sid=ab12&spx=%2Fx&flow=xtls-rprx-vision#My%20VPS",
		"vless://" + uid + "@192.0.2.1:443?type=ws&security=tls&path=%2Fws&host=cdn.example.com&alpn=h2,http/1.1",
		"vless://" + uid + "@192.0.2.1:443?type=grpc&security=tls&serviceName=svc&mode=multi&authority=a.example.com",
		"vless://" + uid + "@192.0.2.1:443?type=xhttp&security=reality&pbk=K&path=%2Fx&mode=packet-up&extra=%7B%22xmux%22%3A%7B%22maxConnections%22%3A4%7D%7D",
		"vless://" + uid + "@[2001:db8::1]:443?type=tcp&headerType=http&host=example.com&path=%2Fa",
		"trojan://secret@trojan.example.com:443?type=httpupgrade&path=%2Fu#t",
		"vmess://" + base64.StdEncoding.EncodeToString(vm),
		"ss://" + ui + "@ss.example.com:8388#sip002",
		"socks5://user:pass@192.0.2.5:1080#s",
		"https://u:p@proxy.example.com:443",
		`{"protocol":"trojan","settings":{"servers":[{"address":"j.example.com","port":443,"password":"p"}]},"streamSettings":{"network":"tcp","security":"tls","tlsSettings":{"serverName":"j.example.com"}}}`,
	} {
		orig, err := Parse(l)
		if err != nil {
			t.Fatal(l, err)
		}
		e, err := EndpointFromLink(l)
		if err != nil {
			t.Fatalf("%s: %v", l, err)
		}
		link, err := e.Link()
		if err != nil {
			t.Fatalf("%s: %+v: %v", l, e, err)
		}
		back, err := Parse(link)
		if err != nil || !SameOutbound(orig.Outbound, back.Outbound) {
			t.Errorf("%s\n -> %+v\n -> %s: %v", l, e, link, err)
		}
	}
}

func TestEndpointEdits(t *testing.T) {
	e, _ := EndpointFromLink("vless://" + uid + "@vps.example.com:443?type=tcp&security=tls&sni=vps.example.com#vps")
	e.Network, e.Path, e.Port, e.Name = "ws", "/new", 8443, "renamed"
	link, _ := e.Link()
	s, err := Parse(link)
	if err != nil || s.Port != 8443 || s.Network != "ws" || s.Name != "renamed" {
		t.Fatal(link, err)
	}
	if ws := s.Outbound["streamSettings"].(map[string]any)["wsSettings"].(map[string]any); ws["path"] != "/new" {
		t.Fatal(ws)
	}
	// Reality fields are dropped when security is not reality; a missing port is refused.
	e.Security, e.PublicKey = "tls", "leftover"
	if link, _ := e.Link(); strings.Contains(link, "pbk=") {
		t.Fatal(link)
	}
	e.Port = 0
	if _, err := e.Link(); err == nil {
		t.Fatal("port 0 accepted")
	}
	// A raw JSON outbound with settings no link carries is JSON-only.
	if _, err := EndpointFromLink(`{"protocol":"trojan","settings":{"servers":[{"address":"j.example.com","port":443,"password":"p"}]},"mux":{"enabled":true}}`); !errors.Is(err, ErrJSONOnly) {
		t.Fatal(err)
	}
}
