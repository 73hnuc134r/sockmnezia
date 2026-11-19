package links

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

const uid = "00000000-0000-4000-8000-000000000001"

func get(t *testing.T, o any, path ...any) any {
	t.Helper()
	cur := o
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("%v: not an object at %q", path, k)
			}
			cur = m[k]
		case int:
			l, ok := cur.([]any)
			if !ok || len(l) <= k {
				t.Fatalf("%v: not a list at %d", path, k)
			}
			cur = l[k]
		}
	}
	return cur
}

func TestVlessReality(t *testing.T) {
	s, err := Parse("vless://" + uid + "@vps.example.com:443?type=tcp&security=reality&sni=www.example.org&fp=chrome&pbk=PUBKEY&sid=ab12&flow=xtls-rprx-vision#My%20VPS")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "My VPS" || s.Address != "vps.example.com" || s.Port != 443 || s.Network != "tcp" || s.Security != "reality" {
		t.Fatalf("%+v", s)
	}
	o := map[string]any(s.Outbound)
	if get(t, o, "settings", "vnext", 0, "users", 0, "flow") != "xtls-rprx-vision" ||
		get(t, o, "streamSettings", "realitySettings", "publicKey") != "PUBKEY" ||
		get(t, o, "streamSettings", "realitySettings", "shortId") != "ab12" {
		t.Fatalf("%v", o)
	}
}

func TestVlessTransports(t *testing.T) {
	for link, want := range map[string]string{
		"vless://" + uid + "@192.0.2.1:443?type=ws&security=tls&path=%2Fws&host=cdn.example.com":           "wsSettings",
		"vless://" + uid + "@192.0.2.1:443?type=grpc&security=tls&serviceName=svc&mode=multi":              "grpcSettings",
		"vless://" + uid + "@192.0.2.1:443?type=xhttp&security=reality&pbk=K&path=%2Fx":                    "xhttpSettings",
		"vless://" + uid + "@192.0.2.1:443?type=splithttp&security=tls":                                    "xhttpSettings",
		"vless://" + uid + "@192.0.2.1:80?type=httpupgrade&path=%2Fu":                                      "httpupgradeSettings",
		"vless://" + uid + "@[2001:db8::1]:443?type=tcp&headerType=http&host=example.com&path=%2Fa%2C%2Fb": "tcpSettings",
	} {
		s, err := Parse(link)
		if err != nil {
			t.Errorf("%s: %v", link, err)
			continue
		}
		if get(t, map[string]any(s.Outbound), "streamSettings", want) == nil {
			t.Errorf("%s: no %s", link, want)
		}
	}
}

func TestTrojanDefaultsToTLS(t *testing.T) {
	s, err := Parse("trojan://secret@trojan.example.com:443#t")
	if err != nil {
		t.Fatal(err)
	}
	o := map[string]any(s.Outbound)
	if s.Security != "tls" || get(t, o, "streamSettings", "tlsSettings", "serverName") != "trojan.example.com" ||
		get(t, o, "settings", "servers", 0, "password") != "secret" {
		t.Fatalf("%v", o)
	}
}

func TestVmess(t *testing.T) {
	j, _ := json.Marshal(map[string]any{"v": "2", "ps": "vm", "add": "vm.example.com", "port": "8443", "id": uid,
		"aid": "0", "scy": "auto", "net": "ws", "type": "none", "host": "h.example.com", "path": "/p", "tls": "tls", "sni": "s.example.com"})
	s, err := Parse("vmess://" + base64.StdEncoding.EncodeToString(j))
	if err != nil {
		t.Fatal(err)
	}
	o := map[string]any(s.Outbound)
	if s.Name != "vm" || s.Port != 8443 || get(t, o, "streamSettings", "wsSettings", "path") != "/p" ||
		get(t, o, "streamSettings", "tlsSettings", "serverName") != "s.example.com" {
		t.Fatalf("%v", o)
	}
}

func TestShadowsocks(t *testing.T) {
	ui := base64.RawURLEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:cGFzc3dvcmQxMjM0NTY3OA=="))
	for _, l := range []string{
		"ss://" + ui + "@ss.example.com:8388#sip002",
		"ss://" + base64.StdEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:cGFzc3dvcmQxMjM0NTY3OA==@ss.example.com:8388")) + "#legacy",
	} {
		s, err := Parse(l)
		if err != nil {
			t.Fatalf("%s: %v", l, err)
		}
		o := map[string]any(s.Outbound)
		if get(t, o, "settings", "servers", 0, "method") != "2022-blake3-aes-128-gcm" ||
			get(t, o, "settings", "servers", 0, "password") != "cGFzc3dvcmQxMjM0NTY3OA==" || s.Port != 8388 {
			t.Fatalf("%s: %v", l, o)
		}
	}
	if _, err := Parse("ss://" + ui + "@ss.example.com:8388?plugin=obfs-local"); !errors.Is(err, ErrUnsupported) {
		t.Fatal("plugin accepted")
	}
}

func TestSocksHTTP(t *testing.T) {
	s, err := Parse("socks5://user:pass@192.0.2.5:1080")
	if err != nil || get(t, map[string]any(s.Outbound), "settings", "servers", 0, "users", 0, "pass") != "pass" {
		t.Fatalf("%v %v", s, err)
	}
	s, err = Parse("socks://" + base64.StdEncoding.EncodeToString([]byte("u:p")) + "@192.0.2.5:1080")
	if err != nil || get(t, map[string]any(s.Outbound), "settings", "servers", 0, "users", 0, "user") != "u" {
		t.Fatalf("%v %v", s, err)
	}
	s, err = Parse("https://proxy.example.com:443")
	if err != nil || s.Protocol != "http" || s.Security != "tls" {
		t.Fatalf("%v %v", s, err)
	}
}

func TestJSON(t *testing.T) {
	s, err := Parse(`{"tag":"x","remarks":"json","protocol":"vless","settings":{"vnext":[{"address":"j.example.com","port":443,"users":[{"id":"` + uid + `","encryption":"none"}]}]},"streamSettings":{"network":"ws","sockopt":{"dialerProxy":"y"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	o := map[string]any(s.Outbound)
	if s.Name != "json" || s.Address != "j.example.com" || s.Port != 443 || o["tag"] != nil ||
		get(t, o, "streamSettings", "sockopt", "dialerProxy") != nil {
		t.Fatalf("%+v", s)
	}
	s, err = Parse(`{"outbounds":[{"protocol":"freedom"},{"protocol":"trojan","settings":{"servers":[{"address":"192.0.2.9","port":443,"password":"p"}]}}]}`)
	if err != nil || s.Protocol != "trojan" {
		t.Fatalf("%v %v", s, err)
	}
}

func TestRejected(t *testing.T) {
	for _, l := range []string{
		"hy2://pass@example.com:443",
		"hysteria2://pass@example.com:443",
		"tuic://u:p@example.com:443",
		"vless://" + uid + "@example.com:443?type=kcp",
		"vless://" + uid + "@example.com:443?type=quic",
		"vless://" + uid + "@example.com:443?type=xhttp&security=tls&alpn=h3",
		"vless://" + uid + "@example.com:443?security=tls&allowInsecure=1",
		`{"protocol":"vless","streamSettings":{"network":"kcp"}}`,
		`{"protocol":"wireguard"}`,
	} {
		if _, err := Parse(l); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: want ErrUnsupported, got %v", l, err)
		}
	}
	for _, l := range []string{"", "nonsense", "vless://@example.com:443", "vless://" + uid + "@example.com", "vless://" + uid + "@example.com:443?security=reality", "vmess://???"} {
		if _, err := Parse(l); err == nil {
			t.Errorf("%q accepted", l)
		}
	}
}
