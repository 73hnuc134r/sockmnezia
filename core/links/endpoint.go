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

// Endpoint is an own server as editable fields (the app's endpoint editor). It maps 1:1 to the share-link
// parameters, so EndpointFromLink and Link go through the same parser: whatever the editor produces is
// validated exactly like a pasted link. Fields that do not apply to the protocol/transport are ignored.
type Endpoint struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"` // vless, vmess, trojan, shadowsocks, socks, http
	Address  string `json:"address"`
	Port     int    `json:"port"`

	ID         string `json:"id,omitempty"`         // vless / vmess user id
	Password   string `json:"password,omitempty"`   // trojan, shadowsocks, socks, http
	User       string `json:"user,omitempty"`       // socks, http
	Method     string `json:"method,omitempty"`     // shadowsocks cipher
	Flow       string `json:"flow,omitempty"`       // vless (e.g. xtls-rprx-vision)
	Encryption string `json:"encryption,omitempty"` // vless "none"; vmess security ("auto", …)

	Network     string `json:"network,omitempty"`  // tcp, ws, grpc, httpupgrade, xhttp
	Security    string `json:"security,omitempty"` // none, tls, reality
	SNI         string `json:"sni,omitempty"`
	Fingerprint string `json:"fp,omitempty"`
	ALPN        string `json:"alpn,omitempty"` // comma-separated
	PublicKey   string `json:"pbk,omitempty"`  // reality
	ShortID     string `json:"sid,omitempty"`  // reality
	SpiderX     string `json:"spx,omitempty"`  // reality
	Path        string `json:"path,omitempty"` // ws, httpupgrade, xhttp, tcp+http header
	Host        string `json:"host,omitempty"`
	ServiceName string `json:"service_name,omitempty"` // grpc
	Mode        string `json:"mode,omitempty"`         // grpc "multi"; xhttp mode
	HeaderType  string `json:"header_type,omitempty"`  // tcp "http"
	Authority   string `json:"authority,omitempty"`    // grpc
	Extra       string `json:"extra,omitempty"`        // xhttp extra (JSON)
}

// ErrJSONOnly means the server has settings no link (and so no editor field) can carry; edit its JSON.
var ErrJSONOnly = fmt.Errorf("%w: this server can only be edited as JSON", ErrUnsupported)

// EndpointFromLink parses a link or outbound JSON into editable fields (ErrJSONOnly when a link cannot
// describe it exactly).
func EndpointFromLink(s string) (Endpoint, error) {
	srv, err := Parse(s)
	if err != nil {
		return Endpoint{}, err
	}
	link, err := Format(srv.Outbound, srv.Name)
	if err != nil {
		return Endpoint{}, ErrJSONOnly
	}
	return decodeLink(link)
}

func decodeLink(link string) (Endpoint, error) {
	if strings.HasPrefix(link, "vmess://") {
		raw, err := b64(link[len("vmess://"):])
		if err != nil {
			return Endpoint{}, err
		}
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			return Endpoint{}, err
		}
		port, _ := strconv.Atoi(str(v["port"]))
		e := Endpoint{Name: str(v["ps"]), Protocol: "vmess", Address: str(v["add"]), Port: port, ID: str(v["id"]),
			Encryption: str(v["scy"]), Network: str(v["net"]), Security: firstNonEmpty(str(v["tls"]), "none"),
			SNI: str(v["sni"]), Fingerprint: str(v["fp"]), ALPN: str(v["alpn"]), Host: str(v["host"]), Path: str(v["path"]),
			HeaderType: str(v["type"])}
		if e.Network == "grpc" {
			e.ServiceName, e.Mode, e.Path, e.HeaderType = e.Path, e.HeaderType, "", ""
		}
		return e, nil
	}
	u, err := url.Parse(link)
	if err != nil {
		return Endpoint{}, err
	}
	port, _ := strconv.Atoi(u.Port())
	e := Endpoint{Name: fragmentName(u), Address: u.Hostname(), Port: port}
	q := u.Query()
	switch u.Scheme {
	case "vless", "trojan":
		e.Protocol = u.Scheme
		if u.Scheme == "vless" {
			e.ID, e.Flow, e.Encryption = u.User.Username(), q.Get("flow"), q.Get("encryption")
		} else {
			e.Password = u.User.Username()
		}
		e.Network, e.Security = q.Get("type"), q.Get("security")
		e.SNI, e.Fingerprint, e.ALPN = q.Get("sni"), q.Get("fp"), q.Get("alpn")
		e.PublicKey, e.ShortID, e.SpiderX = q.Get("pbk"), q.Get("sid"), q.Get("spx")
		e.Path, e.Host, e.ServiceName, e.Mode = q.Get("path"), q.Get("host"), q.Get("serviceName"), q.Get("mode")
		e.HeaderType, e.Authority, e.Extra = q.Get("headerType"), q.Get("authority"), q.Get("extra")
	case "ss":
		e.Protocol = "shadowsocks"
		if raw, err := b64(u.User.Username()); err == nil {
			if p := strings.SplitN(string(raw), ":", 2); len(p) == 2 {
				e.Method, e.Password = p[0], p[1]
			}
		}
	case "socks", "http", "https":
		e.Protocol = u.Scheme
		if u.Scheme == "https" {
			e.Protocol, e.Security = "http", "tls"
		}
		e.User = u.User.Username()
		e.Password, _ = u.User.Password()
	default:
		return Endpoint{}, fmt.Errorf("%w: scheme %q", ErrUnsupported, u.Scheme)
	}
	return e, nil
}

// Link turns the fields into a share link; callers validate it with Parse (AddOwnServer and friends do).
func (e Endpoint) Link() (string, error) {
	e.Address = strings.TrimSpace(e.Address)
	if e.Address == "" {
		return "", errors.New("missing server address")
	}
	if e.Port < 1 || e.Port > 65535 {
		return "", errors.New("missing or invalid port")
	}
	host := net.JoinHostPort(e.Address, strconv.Itoa(e.Port))
	set := func(q url.Values, k, v string) {
		if v = strings.TrimSpace(v); v != "" {
			q.Set(k, v)
		}
	}
	switch e.Protocol {
	case "vless", "trojan":
		q := url.Values{}
		set(q, "type", e.Network)
		set(q, "security", e.Security)
		set(q, "sni", e.SNI)
		set(q, "fp", e.Fingerprint)
		set(q, "alpn", e.ALPN)
		if e.Security == "reality" {
			set(q, "pbk", e.PublicKey)
			set(q, "sid", e.ShortID)
			set(q, "spx", e.SpiderX)
		}
		switch e.Network {
		case "grpc":
			set(q, "serviceName", e.ServiceName)
			set(q, "authority", e.Authority)
			set(q, "mode", e.Mode)
		case "xhttp":
			set(q, "path", e.Path)
			set(q, "host", e.Host)
			set(q, "mode", e.Mode)
			set(q, "extra", e.Extra)
		case "ws", "httpupgrade":
			set(q, "path", e.Path)
			set(q, "host", e.Host)
		case "", "tcp", "raw":
			if e.HeaderType == "http" {
				set(q, "headerType", "http")
				set(q, "path", e.Path)
				set(q, "host", e.Host)
			}
		}
		secret := e.Password
		if e.Protocol == "vless" {
			secret = e.ID
			set(q, "flow", e.Flow)
			if e.Encryption != "" && e.Encryption != "none" {
				set(q, "encryption", e.Encryption)
			}
		}
		u := &url.URL{Scheme: e.Protocol, User: url.User(strings.TrimSpace(secret)), Host: host, RawQuery: q.Encode(), Fragment: e.Name}
		return u.String(), nil
	case "vmess":
		v := map[string]any{"v": "2", "ps": e.Name, "add": e.Address, "port": strconv.Itoa(e.Port), "id": strings.TrimSpace(e.ID),
			"scy": e.Encryption, "net": firstNonEmpty(e.Network, "tcp"), "tls": e.Security, "sni": e.SNI, "fp": e.Fingerprint,
			"alpn": e.ALPN, "host": e.Host, "path": e.Path, "type": e.HeaderType}
		if e.Security == "none" {
			v["tls"] = ""
		}
		if e.Network == "grpc" {
			v["path"], v["type"] = e.ServiceName, e.Mode
		}
		b, _ := json.Marshal(v)
		return "vmess://" + base64.StdEncoding.EncodeToString(b), nil
	case "shadowsocks":
		cred := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(e.Method) + ":" + e.Password))
		return "ss://" + cred + "@" + host + frag(e.Name), nil
	case "socks", "http":
		scheme := e.Protocol
		if e.Protocol == "http" && e.Security == "tls" {
			scheme = "https"
		}
		u := &url.URL{Scheme: scheme, Host: host, Fragment: e.Name}
		if e.User != "" || e.Password != "" {
			u.User = url.UserPassword(e.User, e.Password)
		}
		return u.String(), nil
	}
	return "", fmt.Errorf("%w: protocol %q", ErrUnsupported, e.Protocol)
}
