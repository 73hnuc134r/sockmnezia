package links

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestFormatRoundTrip(t *testing.T) {
	vm, _ := json.Marshal(map[string]any{"v": "2", "ps": "vm", "add": "vm.example.com", "port": "443", "id": uid,
		"net": "ws", "tls": "tls", "path": "/v", "host": "cdn.example.com", "sni": "cdn.example.com"})
	ui := base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:secret"))
	in := []string{
		"vless://" + uid + "@vps.example.com:443?type=tcp&security=reality&sni=www.example.org&fp=chrome&pbk=PUBKEY&sid=ab12&flow=xtls-rprx-vision#My%20VPS",
		"vless://" + uid + "@192.0.2.1:443?type=ws&security=tls&path=%2Fws&host=cdn.example.com",
		"vless://" + uid + "@192.0.2.1:443?type=grpc&security=tls&serviceName=svc&mode=multi&alpn=h2,http/1.1&fp=firefox",
		"vless://" + uid + "@192.0.2.1:443?type=xhttp&security=reality&pbk=K&path=%2Fx&extra=%7B%22xmux%22%3A%7B%22maxConnections%22%3A4%7D%7D",
		"vless://" + uid + "@192.0.2.1:80?type=httpupgrade&path=%2Fu",
		"vless://" + uid + "@[2001:db8::1]:443?type=tcp&headerType=http&host=example.com&path=%2Fa%2C%2Fb",
		"trojan://secret@trojan.example.com:443#t",
		"vmess://" + base64.StdEncoding.EncodeToString(vm),
		"ss://" + ui + "@ss.example.com:8388#sip002",
		"socks5://user:pass@192.0.2.5:1080#s",
		"socks5://192.0.2.5:1080",
		"https://u:p@proxy.example.com:443",
		"http://proxy.example.com:3128",
	}
	for _, l := range in {
		s, err := Parse(l)
		if err != nil {
			t.Fatalf("%s: %v", l, err)
		}
		out, err := Format(s.Outbound, "Renamed hop")
		if err != nil {
			t.Errorf("%s: %v", l, err)
			continue
		}
		back, err := Parse(out)
		if err != nil || !SameOutbound(s.Outbound, back.Outbound) || back.Name != "Renamed hop" {
			t.Errorf("%s -> %s: %+v %v", l, out, back, err)
		}
	}
}

func TestFormatRefusesWhatALinkCannotCarry(t *testing.T) {
	s, _ := Parse("vless://" + uid + "@vps.example.com:443?type=tcp&security=tls")
	s.Outbound["mux"] = map[string]any{"enabled": true}
	if _, err := Format(s.Outbound, "x"); !errors.Is(err, ErrUnsupported) {
		t.Fatal("mux settings would be lost", err)
	}
	if _, err := Format(Outbound{"protocol": "freedom"}, "x"); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	// Names with link syntax stay intact.
	s, _ = Parse("trojan://p@example.com:443")
	out, err := Format(s.Outbound, "a#b?c d/е")
	if err != nil || !strings.HasPrefix(out, "trojan://") {
		t.Fatal(out, err)
	}
	if back, _ := Parse(out); back.Name != "a#b?c d/е" {
		t.Fatal(back.Name)
	}
}
