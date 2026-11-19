// Package links turns share links (vless://, vmess://, trojan://, ss://, socks://, http(s)://) or a raw Xray
// outbound JSON object into an Xray outbound, for the user's own server (plan §5.1).
package links

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Outbound is an Xray outbound object.
type Outbound = map[string]any

// Server is a parsed own server.
type Server struct {
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Network  string   `json:"network"`
	Security string   `json:"security"`
	Outbound Outbound `json:"outbound"`
}

// ErrUnsupported is returned for schemes/transports that cannot be used.
var ErrUnsupported = errors.New("unsupported")

// Parse accepts one share link or a JSON outbound (or a full config, whose first proxy outbound is used).
func Parse(s string) (*Server, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		return parseJSON(s)
	}
	i := strings.Index(s, "://")
	if i < 0 {
		return nil, errors.New("not a link: expected scheme://…")
	}
	scheme := strings.ToLower(s[:i])
	var (
		srv *Server
		err error
	)
	switch scheme {
	case "vless":
		srv, err = parseVlessTrojan(s, "vless")
	case "trojan":
		srv, err = parseVlessTrojan(s, "trojan")
	case "vmess":
		srv, err = parseVmess(s)
	case "ss":
		srv, err = parseSS(s)
	case "socks", "socks5", "socks5h":
		srv, err = parseSocksHTTP(s, "socks")
	case "http", "https":
		srv, err = parseSocksHTTP(s, scheme)
	case "hysteria", "hysteria2", "hy2", "tuic", "wireguard", "wg", "ssr":
		return nil, fmt.Errorf("%w: %s:// servers use UDP-based transports that cannot be chained through Amnezia", ErrUnsupported, scheme)
	default:
		return nil, fmt.Errorf("%w: scheme %q", ErrUnsupported, scheme)
	}
	if err != nil {
		return nil, err
	}
	return srv, finish(srv)
}

func finish(srv *Server) error {
	o := srv.Outbound
	ss, _ := o["streamSettings"].(map[string]any)
	srv.Network, srv.Security = "tcp", "none"
	if ss != nil {
		if n, _ := ss["network"].(string); n != "" {
			srv.Network = n
		}
		if v, _ := ss["security"].(string); v != "" {
			srv.Security = v
		}
	}
	if srv.Protocol == "" {
		srv.Protocol, _ = o["protocol"].(string)
	}
	if srv.Name == "" {
		srv.Name = net.JoinHostPort(srv.Address, strconv.Itoa(srv.Port))
	}
	return CheckChainable(o)
}

// CheckChainable reports whether the outbound can be dialed through another outbound (dialerProxy):
// only TCP-based transports qualify.
func CheckChainable(o Outbound) error {
	p, _ := o["protocol"].(string)
	switch p {
	case "vless", "vmess", "trojan", "shadowsocks", "socks", "http":
	default:
		return fmt.Errorf("%w: protocol %q", ErrUnsupported, p)
	}
	ss, _ := o["streamSettings"].(map[string]any)
	if ss == nil {
		return nil
	}
	n, _ := ss["network"].(string)
	switch n {
	case "", "tcp", "raw", "ws", "grpc", "httpupgrade", "xhttp", "splithttp":
	case "kcp", "mkcp", "quic":
		return fmt.Errorf("%w: transport %q runs over UDP and cannot be chained through Amnezia", ErrUnsupported, n)
	default:
		return fmt.Errorf("%w: transport %q", ErrUnsupported, n)
	}
	for _, k := range []string{"tlsSettings", "realitySettings"} {
		if t, _ := ss[k].(map[string]any); t != nil {
			if alpn, _ := t["alpn"].([]any); alpn != nil {
				for _, a := range alpn {
					if a == "h3" {
						return fmt.Errorf("%w: HTTP/3 (QUIC) cannot be chained through Amnezia", ErrUnsupported)
					}
				}
			}
		}
	}
	return nil
}

func hostPort(u *url.URL) (string, int, error) {
	h := u.Hostname()
	if h == "" {
		return "", 0, errors.New("missing server address")
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil || p < 1 || p > 65535 {
		return "", 0, errors.New("missing or invalid port")
	}
	return h, p, nil
}

func fragmentName(u *url.URL) string {
	n, _ := url.PathUnescape(u.Fragment)
	return strings.TrimSpace(n)
}

func split(s string) []any {
	var out []any
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// stream builds streamSettings from the common share-link parameters.
func stream(q url.Values, host string, defSecurity string) (map[string]any, error) {
	net := q.Get("type")
	if net == "" {
		net = "tcp"
	}
	sec := q.Get("security")
	if sec == "" {
		sec = defSecurity
	}
	ss := map[string]any{"network": net, "security": sec}
	switch sec {
	case "none", "":
		ss["security"] = "none"
	case "tls":
		t := map[string]any{"serverName": firstNonEmpty(q.Get("sni"), q.Get("peer"), host)}
		if fp := q.Get("fp"); fp != "" {
			t["fingerprint"] = fp
		}
		if a := split(q.Get("alpn")); a != nil {
			t["alpn"] = a
		}
		if q.Get("allowInsecure") == "1" || q.Get("insecure") == "1" {
			return nil, fmt.Errorf("%w: links that disable TLS certificate checks", ErrUnsupported)
		}
		ss["tlsSettings"] = t
	case "reality":
		pbk := q.Get("pbk")
		if pbk == "" {
			return nil, errors.New("reality link without pbk (public key)")
		}
		r := map[string]any{"serverName": q.Get("sni"), "fingerprint": firstNonEmpty(q.Get("fp"), "chrome"),
			"publicKey": pbk, "shortId": q.Get("sid")}
		if spx := q.Get("spx"); spx != "" {
			r["spiderX"] = spx
		}
		if pqv := q.Get("pqv"); pqv != "" {
			r["mldsa65Verify"] = pqv
		}
		ss["realitySettings"] = r
	default:
		return nil, fmt.Errorf("%w: security %q", ErrUnsupported, sec)
	}
	path := q.Get("path")
	hostHdr := q.Get("host")
	switch net {
	case "tcp", "raw":
		if q.Get("headerType") == "http" {
			req := map[string]any{"path": split(firstNonEmpty(path, "/"))}
			if hostHdr != "" {
				req["headers"] = map[string]any{"Host": split(hostHdr)}
			}
			ss["tcpSettings"] = map[string]any{"header": map[string]any{"type": "http", "request": req}}
		}
	case "ws":
		w := map[string]any{"path": firstNonEmpty(path, "/")}
		if hostHdr != "" {
			w["host"] = hostHdr
		}
		ss["wsSettings"] = w
	case "httpupgrade":
		w := map[string]any{"path": firstNonEmpty(path, "/")}
		if hostHdr != "" {
			w["host"] = hostHdr
		}
		ss["httpupgradeSettings"] = w
	case "grpc":
		g := map[string]any{"serviceName": firstNonEmpty(q.Get("serviceName"), path)}
		if q.Get("mode") == "multi" {
			g["multiMode"] = true
		}
		if a := q.Get("authority"); a != "" {
			g["authority"] = a
		}
		ss["grpcSettings"] = g
	case "xhttp", "splithttp":
		x := map[string]any{"path": firstNonEmpty(path, "/"), "mode": firstNonEmpty(q.Get("mode"), "auto")}
		if hostHdr != "" {
			x["host"] = hostHdr
		}
		if extra := q.Get("extra"); extra != "" {
			var e map[string]any
			if json.Unmarshal([]byte(extra), &e) == nil {
				x["extra"] = e
			}
		}
		ss["network"] = "xhttp"
		ss["xhttpSettings"] = x
	case "kcp", "mkcp", "quic", "h2", "http":
		return nil, fmt.Errorf("%w: transport %q (use tcp, ws, grpc, httpupgrade or xhttp)", ErrUnsupported, net)
	default:
		return nil, fmt.Errorf("%w: transport %q", ErrUnsupported, net)
	}
	return ss, nil
}

func parseVlessTrojan(s, proto string) (*Server, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("bad %s link: %v", proto, err)
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	secret := u.User.Username()
	if secret == "" {
		return nil, fmt.Errorf("%s link without user id/password", proto)
	}
	q := u.Query()
	def := "none"
	if proto == "trojan" {
		def = "tls"
	}
	ss, err := stream(q, host, def)
	if err != nil {
		return nil, err
	}
	var settings map[string]any
	if proto == "vless" {
		user := map[string]any{"id": secret, "encryption": firstNonEmpty(q.Get("encryption"), "none")}
		if f := q.Get("flow"); f != "" {
			user["flow"] = f
		}
		settings = map[string]any{"vnext": []any{map[string]any{"address": host, "port": port, "users": []any{user}}}}
	} else {
		settings = map[string]any{"servers": []any{map[string]any{"address": host, "port": port, "password": secret}}}
	}
	return &Server{Name: fragmentName(u), Protocol: proto, Address: host, Port: port,
		Outbound: Outbound{"protocol": proto, "settings": settings, "streamSettings": ss}}, nil
}

func b64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "=")
	for _, enc := range []*base64.Encoding{base64.RawStdEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("bad base64")
}

func parseVmess(s string) (*Server, error) {
	raw, err := b64(strings.TrimPrefix(s[len("vmess://"):], "//"))
	if err != nil {
		return nil, errors.New("bad vmess link (expected base64 JSON)")
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, errors.New("bad vmess link JSON")
	}
	str := func(k string) string {
		switch x := v[k].(type) {
		case string:
			return x
		case float64:
			return strconv.Itoa(int(x))
		}
		return ""
	}
	port, err := strconv.Atoi(str("port"))
	if err != nil || port < 1 || port > 65535 || str("add") == "" || str("id") == "" {
		return nil, errors.New("vmess link without address, port or id")
	}
	q := url.Values{}
	q.Set("type", firstNonEmpty(str("net"), "tcp"))
	q.Set("security", firstNonEmpty(str("tls"), "none"))
	q.Set("sni", str("sni"))
	q.Set("fp", str("fp"))
	q.Set("alpn", str("alpn"))
	q.Set("path", str("path"))
	q.Set("host", str("host"))
	q.Set("headerType", str("type"))
	if str("net") == "grpc" {
		q.Set("serviceName", str("path"))
		q.Set("mode", str("type"))
	}
	ss, err := stream(q, str("add"), "none")
	if err != nil {
		return nil, err
	}
	user := map[string]any{"id": str("id"), "security": firstNonEmpty(str("scy"), "auto")}
	return &Server{Name: str("ps"), Protocol: "vmess", Address: str("add"), Port: port,
		Outbound: Outbound{"protocol": "vmess", "streamSettings": ss, "settings": map[string]any{
			"vnext": []any{map[string]any{"address": str("add"), "port": port, "users": []any{user}}}}}}, nil
}

func parseSS(s string) (*Server, error) {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.Port() == "" {
		// Legacy: ss://base64(method:password@host:port)#name
		body := s[len("ss://"):]
		name := ""
		if i := strings.Index(body, "#"); i >= 0 {
			name, _ = url.PathUnescape(body[i+1:])
			body = body[:i]
		}
		dec, err := b64(body)
		if err != nil {
			return nil, errors.New("bad ss link")
		}
		u, err = url.Parse("ss://" + string(dec))
		if err != nil {
			return nil, errors.New("bad ss link")
		}
		u.Fragment = name
	}
	if u.Query().Get("plugin") != "" {
		return nil, fmt.Errorf("%w: shadowsocks plugins", ErrUnsupported)
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	method, pass := u.User.Username(), ""
	if p, ok := u.User.Password(); ok {
		pass = p
	} else if dec, err := b64(method); err == nil { // SIP002: base64(method:password)
		parts := strings.SplitN(string(dec), ":", 2)
		if len(parts) == 2 {
			method, pass = parts[0], parts[1]
		}
	}
	if method == "" || pass == "" {
		return nil, errors.New("ss link without method or password")
	}
	return &Server{Name: fragmentName(u), Protocol: "shadowsocks", Address: host, Port: port,
		Outbound: Outbound{"protocol": "shadowsocks", "settings": map[string]any{"servers": []any{
			map[string]any{"address": host, "port": port, "method": method, "password": pass}}}}}, nil
}

func parseSocksHTTP(s, scheme string) (*Server, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	if scheme == "socks" && user != "" && pass == "" { // socks://base64(user:pass)@host:port (v2rayN)
		if dec, err := b64(user); err == nil {
			if p := strings.SplitN(string(dec), ":", 2); len(p) == 2 {
				user, pass = p[0], p[1]
			}
		}
	}
	proto := "socks"
	if scheme != "socks" {
		proto = "http"
	}
	server := map[string]any{"address": host, "port": port}
	if user != "" {
		server["users"] = []any{map[string]any{"user": user, "pass": pass}}
	}
	o := Outbound{"protocol": proto, "settings": map[string]any{"servers": []any{server}}}
	if scheme == "https" {
		o["streamSettings"] = map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"serverName": host}}
	}
	return &Server{Name: fragmentName(u), Protocol: proto, Address: host, Port: port, Outbound: o}, nil
}

func parseJSON(s string) (*Server, error) {
	var o map[string]any
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		return nil, fmt.Errorf("bad JSON: %v", err)
	}
	if outs, ok := o["outbounds"].([]any); ok {
		o = nil
		for _, x := range outs {
			m, _ := x.(map[string]any)
			if p, _ := m["protocol"].(string); p != "" && p != "freedom" && p != "blackhole" && p != "dns" {
				o = m
				break
			}
		}
		if o == nil {
			return nil, errors.New("config has no proxy outbound")
		}
	}
	p, _ := o["protocol"].(string)
	if p == "" {
		return nil, errors.New("JSON is not an Xray outbound (no protocol)")
	}
	delete(o, "tag")
	delete(o, "proxySettings")
	if ss, _ := o["streamSettings"].(map[string]any); ss != nil {
		if so, _ := ss["sockopt"].(map[string]any); so != nil {
			delete(so, "dialerProxy")
		}
	}
	srv := &Server{Protocol: p, Outbound: o}
	if name, _ := o["remarks"].(string); name != "" {
		srv.Name = name
		delete(o, "remarks")
	}
	srv.Address, srv.Port = jsonAddress(o)
	return srv, finish(srv)
}

func jsonAddress(o map[string]any) (string, int) {
	st, _ := o["settings"].(map[string]any)
	for _, k := range []string{"vnext", "servers"} {
		if l, _ := st[k].([]any); len(l) > 0 {
			if m, _ := l[0].(map[string]any); m != nil {
				a, _ := m["address"].(string)
				p, _ := m["port"].(float64)
				return a, int(p)
			}
		}
	}
	a, _ := st["address"].(string)
	p, _ := st["port"].(float64)
	return a, int(p)
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
