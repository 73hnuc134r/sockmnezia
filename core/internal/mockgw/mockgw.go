// Package mockgw is an in-process mock of the Amnezia gateway for tests. It implements the real envelope
// (docs/03) and answers v1/config with a server entry shaped like the real one (docs/04), using only
// synthetic documentation addresses.
package mockgw

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/nuc134r/sockmnezia/core/gateway"
	"github.com/nuc134r/sockmnezia/core/vpnkey"
)

// Request is one decrypted request seen by the mock.
type Request struct {
	Path    string
	Payload map[string]any
}

// Reply lets a test override the answer: return the JSON object to encrypt.
type Reply func(r Request) map[string]any

// Gateway is a running mock.
type Gateway struct {
	*httptest.Server
	Key   *rsa.PrivateKey
	PEM   []byte
	mu    sync.Mutex
	seen  []Request
	reply Reply
}

// New starts a mock gateway with a fresh RSA key.
func New() *Gateway {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	g := &Gateway{Key: k, PEM: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})}
	g.Server = httptest.NewServer(http.HandlerFunc(g.serve))
	return g
}

// SetReply overrides answers (nil restores the default).
func (g *Gateway) SetReply(r Reply) { g.mu.Lock(); g.reply = r; g.mu.Unlock() }

// Requests returns what the mock received.
func (g *Gateway) Requests() []Request {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]Request(nil), g.seen...)
}

func (g *Gateway) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	sess, pt, err := gateway.OpenRequest(g.Key, body)
	if err != nil {
		// What the real gateway does when the key payload was encrypted to another key.
		w.WriteHeader(500)
		w.Write([]byte(`{"http_status":500,"message":"internal server error"}`))
		return
	}
	var payload map[string]any
	json.Unmarshal(pt, &payload)
	req := Request{Path: r.URL.Path, Payload: payload}
	g.mu.Lock()
	g.seen = append(g.seen, req)
	reply := g.reply
	g.mu.Unlock()
	if reply == nil {
		reply = DefaultReply
	}
	out, _ := json.Marshal(reply(req))
	ct, _ := sess.Encrypt(out)
	w.WriteHeader(200)
	w.Write(ct)
}

// XrayConfig is the synthetic Xray client config returned as last_config (a relay chain, like the real one).
func XrayConfig(clientUUID string) map[string]any {
	return map[string]any{
		"log":      map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": 10808, "protocol": "socks", "settings": map[string]any{"udp": true}}},
		"outbounds": []any{
			map[string]any{
				"tag": "proxy", "protocol": "vless",
				"settings": map[string]any{"vnext": []any{map[string]any{"address": "203.0.113.7", "port": 443,
					"users": []any{map[string]any{"id": clientUUID, "flow": "xtls-rprx-vision", "encryption": "none"}}}}},
				"streamSettings": map[string]any{"network": "tcp", "security": "reality",
					"sockopt":         map[string]any{"dialerProxy": "proxy-relay"},
					"realitySettings": map[string]any{"serverName": "", "fingerprint": "firefox", "publicKey": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "shortId": ""}},
			},
			map[string]any{
				"tag": "proxy-relay", "protocol": "vless",
				"settings": map[string]any{"vnext": []any{map[string]any{"address": "198.51.100.77", "port": 443,
					"users": []any{map[string]any{"id": clientUUID, "encryption": "none"}}}}},
				"streamSettings": map[string]any{"network": "xhttp", "security": "reality",
					"xhttpSettings":   map[string]any{"xmux": map[string]any{"maxConnections": 4}},
					"realitySettings": map[string]any{"serverName": "", "fingerprint": "qq", "publicKey": "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", "shortId": ""}},
			},
			map[string]any{"tag": "dns-out", "protocol": "dns"},
		},
	}
}

// ServerEntry is the decoded "config" of a successful answer.
func ServerEntry(clientUUID, country string) map[string]any {
	if country == "" {
		country = "de"
	}
	last, _ := json.Marshal(XrayConfig(clientUUID))
	return map[string]any{
		"name": "Amnezia Premium", "description": "synthetic", "config_version": 2,
		"hostName": "203.0.113.7", "defaultContainer": "amnezia-xray",
		"server_country_code": country, "server_country_name": "Country " + country,
		"api_config": map[string]any{
			"server_country_code": country,
			"available_countries": []any{
				map[string]any{"server_country_code": "de", "server_country_name": "Germany"},
				map[string]any{"server_country_code": "nl", "server_country_name": "Netherlands"},
			},
			"subscription": map[string]any{"end_date": "2030-01-01T00:00:00Z"},
		},
		"containers": []any{map[string]any{"container": "amnezia-xray",
			"xray": map[string]any{"port": "443", "transport_proto": "tcp", "last_config": string(last)}}},
	}
}

// DefaultReply answers like the real gateway for v1/config.
func DefaultReply(r Request) map[string]any {
	if r.Path != "/v1/config" {
		return map[string]any{"http_status": 404, "message": "not found"}
	}
	pk, _ := r.Payload["public_key"].(string)
	country, _ := r.Payload["server_country_code"].(string)
	cfg, _ := vpnkey.Encode(ServerEntry(pk, country), false)
	return map[string]any{
		"config":           cfg,
		"http_status":      201,
		"routing_decision": map[string]any{"relay": map[string]any{"effective": true, "requested": false}},
		"service_info":     map[string]any{"is_ad_visible": false, "is_renewal_available": true},
	}
}

// SubscriptionKey returns a synthetic subscription key.
func SubscriptionKey() string {
	s, _ := vpnkey.Encode(map[string]any{
		"name": "Amnezia Premium", "config_version": 1,
		"api_config": map[string]any{"service_type": "amnezia-premium", "service_protocol": "awg", "user_country_code": "xx"},
		"auth_data":  map[string]any{"api_key": "synthetic-api-key-for-tests"},
	}, true)
	return s
}
