package links

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// Format turns an outbound back into a share link named name: vless://, trojan://, vmess://, ss://,
// socks:// or http(s)://. It only returns a link that parses back to exactly the same outbound, so a link
// never describes a different server than the one that runs; anything it cannot express (e.g. a raw JSON
// outbound with extra settings) is ErrUnsupported, and the caller shares the JSON instead.
func Format(o Outbound, name string) (string, error) {
	link, err := format(o, name)
	if err != nil {
		return "", err
	}
	back, err := Parse(link)
	if err != nil {
		return "", fmt.Errorf("%w: the link does not parse back: %v", ErrUnsupported, err)
	}
	if !SameOutbound(o, back.Outbound) {
		return "", fmt.Errorf("%w: the outbound has settings a link cannot carry", ErrUnsupported)
	}
	return link, nil
}

// SameOutbound reports whether two outbounds describe the same server (compared as JSON, ignoring tag and
// remarks).
func SameOutbound(a, b Outbound) bool {
	norm := func(o Outbound) any {
		c := Outbound{}
		for k, v := range o {
			if k != "tag" && k != "remarks" {
				c[k] = v
			}
		}
		raw, _ := json.Marshal(c)
		var v any
		json.Unmarshal(raw, &v)
		return v
	}
	return reflect.DeepEqual(norm(a), norm(b))
}

func format(o Outbound, name string) (string, error) {
	proto, _ := o["protocol"].(string)
	st, _ := o["settings"].(map[string]any)
	ss, _ := o["streamSettings"].(map[string]any)
	first := func(k string) map[string]any {
		if l, _ := st[k].([]any); len(l) > 0 {
			m, _ := l[0].(map[string]any)
			return m
		}
		return nil
	}
	switch proto {
	case "vless":
		srv := first("vnext")
		user := firstUser(srv)
		if srv == nil || user == nil {
			return "", fmt.Errorf("%w: vless outbound without a server", ErrUnsupported)
		}
		q, err := streamQuery(ss)
		if err != nil {
			return "", err
		}
		if e := str(user["encryption"]); e != "" && e != "none" {
			q.Set("encryption", e)
		}
		if f := str(user["flow"]); f != "" {
			q.Set("flow", f)
		}
		return userLink("vless", str(user["id"]), srv, q, name), nil
	case "trojan":
		srv := first("servers")
		if srv == nil {
			return "", fmt.Errorf("%w: trojan outbound without a server", ErrUnsupported)
		}
		q, err := streamQuery(ss)
		if err != nil {
			return "", err
		}
		return userLink("trojan", str(srv["password"]), srv, q, name), nil
	case "vmess":
		return formatVmess(first("vnext"), ss, name)
	case "shadowsocks":
		srv := first("servers")
		if srv == nil || ss != nil {
			return "", fmt.Errorf("%w: shadowsocks outbound", ErrUnsupported)
		}
		cred := base64.RawURLEncoding.EncodeToString([]byte(str(srv["method"]) + ":" + str(srv["password"])))
		return "ss://" + cred + "@" + hostPortStr(srv) + frag(name), nil
	case "socks", "http":
		srv := first("servers")
		if srv == nil {
			return "", fmt.Errorf("%w: %s outbound without a server", ErrUnsupported, proto)
		}
		scheme := proto
		if proto == "http" && ss != nil && ss["security"] == "tls" {
			scheme = "https"
		}
		u := &url.URL{Scheme: scheme, Host: hostPortStr(srv), Fragment: name}
		if l, _ := srv["users"].([]any); len(l) > 0 {
			m, _ := l[0].(map[string]any)
			u.User = url.UserPassword(str(m["user"]), str(m["pass"]))
		}
		return u.String(), nil
	}
	return "", fmt.Errorf("%w: protocol %q has no share link", ErrUnsupported, proto)
}

func firstUser(srv map[string]any) map[string]any {
	if l, _ := srv["users"].([]any); len(l) > 0 {
		m, _ := l[0].(map[string]any)
		return m
	}
	return nil
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.Itoa(int(x))
	case int:
		return strconv.Itoa(x)
	}
	return ""
}

func hostPortStr(srv map[string]any) string {
	return net.JoinHostPort(str(srv["address"]), str(srv["port"]))
}

func frag(name string) string {
	if name == "" {
		return ""
	}
	return "#" + url.PathEscape(name)
}

func userLink(scheme, secret string, srv map[string]any, q url.Values, name string) string {
	u := &url.URL{Scheme: scheme, User: url.User(secret), Host: hostPortStr(srv), RawQuery: q.Encode(), Fragment: name}
	return u.String()
}

func strList(v any) string {
	l, _ := v.([]any)
	var out []string
	for _, x := range l {
		out = append(out, str(x))
	}
	return strings.Join(out, ",")
}

// streamQuery is the inverse of stream: streamSettings -> share-link parameters.
func streamQuery(ss map[string]any) (url.Values, error) {
	q := url.Values{}
	if ss == nil {
		return q, nil
	}
	n := str(ss["network"])
	if n == "" {
		n = "tcp"
	}
	q.Set("type", n)
	sec := str(ss["security"])
	if sec == "" {
		sec = "none"
	}
	q.Set("security", sec)
	switch sec {
	case "tls":
		t, _ := ss["tlsSettings"].(map[string]any)
		if v := str(t["serverName"]); v != "" {
			q.Set("sni", v)
		}
		if v := str(t["fingerprint"]); v != "" {
			q.Set("fp", v)
		}
		if v := strList(t["alpn"]); v != "" {
			q.Set("alpn", v)
		}
	case "reality":
		r, _ := ss["realitySettings"].(map[string]any)
		q.Set("pbk", str(r["publicKey"]))
		q.Set("fp", str(r["fingerprint"]))
		if v := str(r["serverName"]); v != "" {
			q.Set("sni", v)
		}
		if v := str(r["shortId"]); v != "" {
			q.Set("sid", v)
		}
		if v := str(r["spiderX"]); v != "" {
			q.Set("spx", v)
		}
		if v := str(r["mldsa65Verify"]); v != "" {
			q.Set("pqv", v)
		}
	}
	switch n {
	case "tcp", "raw":
		if t, _ := ss["tcpSettings"].(map[string]any); t != nil {
			h, _ := t["header"].(map[string]any)
			req, _ := h["request"].(map[string]any)
			q.Set("headerType", str(h["type"]))
			q.Set("path", strList(req["path"]))
			if hd, _ := req["headers"].(map[string]any); hd != nil {
				q.Set("host", strList(hd["Host"]))
			}
		}
	case "ws", "httpupgrade":
		w, _ := ss[n+"Settings"].(map[string]any)
		q.Set("path", str(w["path"]))
		if v := str(w["host"]); v != "" {
			q.Set("host", v)
		}
	case "grpc":
		g, _ := ss["grpcSettings"].(map[string]any)
		q.Set("serviceName", str(g["serviceName"]))
		if g["multiMode"] == true {
			q.Set("mode", "multi")
		}
		if v := str(g["authority"]); v != "" {
			q.Set("authority", v)
		}
	case "xhttp":
		x, _ := ss["xhttpSettings"].(map[string]any)
		q.Set("path", str(x["path"]))
		if v := str(x["mode"]); v != "" {
			q.Set("mode", v)
		}
		if v := str(x["host"]); v != "" {
			q.Set("host", v)
		}
		if e, _ := x["extra"].(map[string]any); e != nil {
			b, _ := json.Marshal(e)
			q.Set("extra", string(b))
		}
	default:
		return nil, fmt.Errorf("%w: transport %q", ErrUnsupported, n)
	}
	return q, nil
}

func formatVmess(srv map[string]any, ss map[string]any, name string) (string, error) {
	user := firstUser(srv)
	if srv == nil || user == nil {
		return "", fmt.Errorf("%w: vmess outbound without a server", ErrUnsupported)
	}
	q, err := streamQuery(ss)
	if err != nil {
		return "", err
	}
	v := map[string]any{
		"v": "2", "ps": name, "add": str(srv["address"]), "port": str(srv["port"]), "id": str(user["id"]),
		"scy": str(user["security"]), "net": q.Get("type"), "tls": q.Get("security"),
		"sni": q.Get("sni"), "fp": q.Get("fp"), "alpn": q.Get("alpn"), "host": q.Get("host"),
		"path": q.Get("path"), "type": q.Get("headerType"),
	}
	if v["net"] == "" {
		v["net"] = "tcp"
	}
	if v["tls"] == "none" {
		v["tls"] = ""
	}
	if q.Get("type") == "grpc" {
		v["path"], v["type"] = q.Get("serviceName"), q.Get("mode")
	}
	b, _ := json.Marshal(v)
	return "vmess://" + base64.StdEncoding.EncodeToString(b), nil
}
