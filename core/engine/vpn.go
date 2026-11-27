package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"go4.org/netipx"

	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// Connection modes (Options.Mode).
const (
	ModeProxy = "proxy" // SOCKS/HTTP only (default)
	ModeVPN   = "vpn"   // also the whole phone (or chosen apps) through a VpnService tun (D31)
)

// VPN DNS modes (VPNOptions.DNS), LAN modes (VPNOptions.LAN) and app selection (VPNOptions.Apps).
const (
	VPNDNSPublic = "public" // announce public resolvers; their queries go through the chain as UDP
	VPNDNSDoH    = "doh"    // Xray's DNS module answers port 53 on the tun over DNS-over-HTTPS through the chain
	VPNDNSCustom = "custom" // announce the user's resolvers; through the chain like "public"

	VPNLANBypass = "bypass" // private ranges are not routed into the VPN: LAN devices work directly
	VPNLANTunnel = "tunnel" // everything into the VPN; the locked rules then block private addresses

	VPNAppsAll    = "all"
	VPNAppsOnly   = "only"   // only AppList
	VPNAppsExcept = "except" // all but AppList
)

// VPNOptions are the VPN-mode settings (owner's choices, docs/11 §3).
type VPNOptions struct {
	DNS        string   `json:"dns"`
	DNSServers []string `json:"dns_servers,omitempty"` // VPNDNSCustom: IP addresses
	LAN        string   `json:"lan"`
	Apps       string   `json:"apps"`
	AppList    []string `json:"app_list,omitempty"` // Android package names
}

func defaultVPNOptions() VPNOptions {
	return VPNOptions{DNS: VPNDNSPublic, LAN: VPNLANBypass, Apps: VPNAppsAll}
}

var (
	publicDNS = []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "2001:4860:4860::8888"}
	dohURLs   = []string{"https://1.1.1.1/dns-query", "https://8.8.8.8/dns-query"}
	// The VPN's own addresses: a /30 and a /126 that nothing else uses.
	vpnAddrs = []string{"10.233.233.1/30", "fd6d:6e7a:6973::1/126"}
)

const vpnMTU = 1500

func validateVPN(v *VPNOptions) error {
	if v.DNS == "" {
		*v = mergeVPN(*v)
	}
	switch v.DNS {
	case VPNDNSPublic, VPNDNSDoH:
	case VPNDNSCustom:
		if len(v.DNSServers) == 0 {
			return &Error{Code: CodeInvalidSettings, Field: "vpn_dns", Detail: "add at least one DNS server"}
		}
		for i, s := range v.DNSServers {
			a, err := netip.ParseAddr(strings.TrimSpace(s))
			if err != nil {
				return &Error{Code: CodeInvalidSettings, Field: "vpn_dns", Detail: fmt.Sprintf("%q is not an IP address", s)}
			}
			if v.LAN == VPNLANTunnel && isPrivate(a) {
				return &Error{Code: CodeInvalidSettings, Field: "vpn_dns",
					Detail: fmt.Sprintf("%s is a local address: with the local network through the VPN it would be blocked", s)}
			}
			v.DNSServers[i] = a.String()
		}
	default:
		return &Error{Code: CodeInvalidSettings, Field: "vpn_dns", Detail: "unknown DNS mode"}
	}
	switch v.LAN {
	case VPNLANBypass, VPNLANTunnel:
	default:
		return &Error{Code: CodeInvalidSettings, Field: "vpn_lan", Detail: "unknown local network mode"}
	}
	switch v.Apps {
	case VPNAppsAll:
	case VPNAppsOnly, VPNAppsExcept:
		if v.Apps == VPNAppsOnly && len(v.AppList) == 0 {
			return &Error{Code: CodeInvalidSettings, Field: "vpn_apps", Detail: "choose at least one app"}
		}
	default:
		return &Error{Code: CodeInvalidSettings, Field: "vpn_apps", Detail: "unknown app selection"}
	}
	return nil
}

// mergeVPN fills unset fields with the defaults (states saved before VPN mode existed).
func mergeVPN(v VPNOptions) VPNOptions {
	d := defaultVPNOptions()
	if v.DNS == "" {
		v.DNS = d.DNS
	}
	if v.LAN == "" {
		v.LAN = d.LAN
	}
	if v.Apps == "" {
		v.Apps = d.Apps
	}
	return v
}

func privateSet() *netipx.IPSet {
	var b netipx.IPSetBuilder
	for _, c := range routing.PrivateCIDRs {
		b.AddPrefix(netip.MustParsePrefix(c))
	}
	s, _ := b.IPSet()
	return s
}

func isPrivate(a netip.Addr) bool { return privateSet().Contains(a) }

// VPNInterface is what the app's VpnService.Builder needs.
type VPNInterface struct {
	MTU       int      `json:"mtu"`
	Addresses []string `json:"addresses"` // CIDR
	Routes    []string `json:"routes"`    // CIDR
	DNS       []string `json:"dns"`
	Apps      string   `json:"apps"`
	AppList   []string `json:"app_list,omitempty"`
}

// VPNInterface returns the VPN parameters for the current settings. With the local network bypassing the
// VPN the routes are everything except routing.PrivateCIDRs (the ranges the locked rules block), computed
// here because minSdk 26 has no VpnService.Builder.excludeRoute.
func (e *Engine) VPNInterface() VPNInterface {
	e.mu.Lock()
	v := mergeVPN(e.st.Options.VPN)
	e.mu.Unlock()
	vi := VPNInterface{MTU: vpnMTU, Addresses: append([]string(nil), vpnAddrs...), Apps: v.Apps, AppList: v.AppList}
	if v.LAN == VPNLANTunnel {
		vi.Routes = []string{"0.0.0.0/0", "::/0"}
	} else {
		var b netipx.IPSetBuilder
		b.AddPrefix(netip.MustParsePrefix("0.0.0.0/0"))
		b.AddPrefix(netip.MustParsePrefix("::/0"))
		b.RemoveSet(privateSet())
		set, _ := b.IPSet()
		for _, p := range set.Prefixes() {
			vi.Routes = append(vi.Routes, p.String())
		}
	}
	switch v.DNS {
	case VPNDNSCustom:
		vi.DNS = append([]string(nil), v.DNSServers...)
	default:
		// DoH mode announces the same public addresses: their port 53 is answered by Xray's DNS module.
		vi.DNS = append([]string(nil), publicDNS...)
	}
	return vi
}

// StartVPN runs the proxy with the VPN's tun (an open file descriptor owned by the caller; Xray reads a
// duplicate). A running proxy-only instance is restarted with the tun. Settings changes keep the tun.
func (e *Engine) StartVPN(ctx context.Context, fd int) error {
	if fd < 3 {
		return newErr(CodeInvalidSettings, errors.New("invalid VPN file descriptor"))
	}
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
	// Xray's tun inbound reads the descriptor from this variable (xray.tun.fd, XRAY_TUN_FD).
	os.Setenv("xray.tun.fd", strconv.Itoa(fd))
	os.Setenv("XRAY_TUN_FD", strconv.Itoa(fd))
	e.vpnFD = fd
	err := e.restartLocked()
	if err != nil {
		e.vpnFD = -1
	}
	return err
}

// vpnSpecLocked is the xrayconf VPN part while VPN mode runs, nil otherwise.
func (e *Engine) vpnSpecLocked() *xrayconf.VPN {
	if e.tun != nil {
		// Desktop TUN: the system resolver points at the tun, so DNS is always answered over DoH there.
		sys := e.tun.sys
		return &xrayconf.VPN{MTU: vpnMTU, DoH: append([]string(nil), dohURLs...), System: &sys}
	}
	if e.vpnFD < 0 {
		return nil
	}
	v := &xrayconf.VPN{MTU: vpnMTU}
	if mergeVPN(e.st.Options.VPN).DNS == VPNDNSDoH {
		v.DoH = append([]string(nil), dohURLs...)
	}
	return v
}

// Instance is a running proxy: Xray in this process (runner.Instance), or the desktop TUN helper's.
type Instance interface {
	Close() error
	Traffic() (up, down int64)
	HTTPClient(timeout time.Duration) *http.Client
	ExitIP(ctx context.Context, url string) (string, error)
}

// Launcher runs a config somewhere else: the desktop's TUN system service (D40). geoDir holds the geo files
// the rules may use.
type Launcher func(cfg []byte, geoDir string) (Instance, error)

type tunMode struct {
	sys    xrayconf.SystemTUN
	launch Launcher
}

// StartTUN runs the proxy with a desktop TUN (D40): the config, with Xray's tun inbound creating the
// interface and the routes (sys), is run by launch with the rights that needs. The SOCKS/HTTP ports keep
// working. A running proxy is restarted in TUN mode; Stop leaves it.
func (e *Engine) StartTUN(ctx context.Context, sys xrayconf.SystemTUN, launch Launcher) error {
	if launch == nil {
		return newErr(CodeInvalidSettings, errors.New("no TUN launcher"))
	}
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
	e.tun = &tunMode{sys: sys, launch: launch}
	err := e.restartLocked()
	if err != nil {
		e.tun = nil
	}
	return err
}
