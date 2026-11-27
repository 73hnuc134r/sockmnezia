package routing

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// PAC returns a proxy auto-config script for the desktop's PAC mode (docs/10 §3): addresses the rules send
// directly bypass the local proxy, everything else goes to proxy ("127.0.0.1:port"), where Xray applies the
// rules again (so a proxy or block rule needs no special case here).
//
// The script follows the rules in order, first match wins, as Xray does, but only as far as it can do so
// exactly: at the first enabled rule it cannot express (geosite:, geoip:, regexp:, ports, networks, detected
// protocols, IP rules that would need DNS) it stops, and everything not decided by then goes to the proxy.
// So the script never sends something directly that Xray would not. Local names and private addresses go
// directly: the proxy blocks them (locked rules), the PAC must not route them into it.
func PAC(s Settings, proxy string) string {
	var b strings.Builder
	b.WriteString("// sockmnezia PAC: generated from the routing rules.\n")
	b.WriteString("function FindProxyForURL(url, host) {\n")
	b.WriteString("  host = host.toLowerCase();\n")
	b.WriteString("  var v4 = /^\\d+\\.\\d+\\.\\d+\\.\\d+$/.test(host);\n")
	b.WriteString("  var v6 = host.indexOf(\":\") >= 0;\n")
	// Local names and private addresses.
	b.WriteString("  if (isPlainHostName(host)")
	for _, d := range PrivateDomains {
		if c := domainCond(d); c != "" {
			b.WriteString(" || " + c)
		}
	}
	for _, p := range PrivateCIDRs {
		if c := ipCond(p); c != "" {
			b.WriteString(" || " + c)
		}
	}
	b.WriteString(" || (v6 && (host === \"::1\" || /^\\[?(fc|fd|fe8|fe9|fea|feb)/.test(host)))) return \"DIRECT\";\n")

	direct := "\"DIRECT\""
	proxied := strconv.Quote("PROXY " + proxy)
rules:
	for _, r := range s.Rules {
		if !r.IsEnabled() || r.Locked {
			continue
		}
		if r.Port != "" || r.Network != "" || len(r.Protocol) > 0 {
			break // not visible to a PAC script
		}
		var conds []string
		for _, d := range r.Domain {
			c := domainCond(d)
			if c == "" {
				break rules
			}
			conds = append(conds, c)
		}
		for _, ip := range r.IP {
			// With AsIs, IP rules only match IP literals; other strategies resolve names, which a PAC
			// script must not do (dnsResolve would leak the lookup to the local resolver).
			c := ipCond(ip)
			if c == "" || s.DomainStrategy != AsIs && s.DomainStrategy != "" {
				break rules
			}
			conds = append(conds, c)
		}
		if len(conds) == 0 {
			break // matches everything: nothing after it matters
		}
		target := proxied
		if r.OutboundTag == OutDirect {
			target = direct
		}
		fmt.Fprintf(&b, "  if (%s) return %s;\n", strings.Join(conds, " || "), target)
	}
	fmt.Fprintf(&b, "  return %s;\n}\n", proxied)
	return b.String()
}

// domainCond is the JavaScript test for one domain entry, or "" when a PAC script cannot express it.
func domainCond(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	q := func(s string) string { return strconv.Quote(s) }
	switch {
	case strings.HasPrefix(d, "full:"):
		return "host === " + q(d[5:])
	case strings.HasPrefix(d, "domain:"):
		v := d[7:]
		return fmt.Sprintf("(host === %s || dnsDomainIs(host, %s))", q(v), q("."+v))
	case strings.HasPrefix(d, "keyword:"):
		return fmt.Sprintf("host.indexOf(%s) >= 0", q(d[8:]))
	case strings.Contains(d, ":"):
		return "" // geosite:, regexp:, dotless: …
	default: // plain: substring, as Xray matches it
		return fmt.Sprintf("host.indexOf(%s) >= 0", q(d))
	}
}

// ipCond is the JavaScript test for one IPv4 address or CIDR (literal hosts only), or "" otherwise. IPv6
// entries never match an IPv4 literal, so they are skipped rather than ending the script.
func ipCond(ip string) string {
	if strings.HasPrefix(ip, "!") || strings.Contains(ip, "geoip:") {
		return ""
	}
	if !strings.Contains(ip, "/") {
		ip += "/32"
		if strings.Contains(ip, ":") {
			ip = strings.TrimSuffix(ip, "/32") + "/128"
		}
	}
	p, err := netip.ParsePrefix(ip)
	if err != nil {
		return ""
	}
	if p.Addr().Is6() {
		return "false"
	}
	bits := p.Bits()
	mask := uint32(0xffffffff) << (32 - bits)
	if bits == 0 {
		mask = 0
	}
	m := fmt.Sprintf("%d.%d.%d.%d", mask>>24, mask>>16&0xff, mask>>8&0xff, mask&0xff)
	return fmt.Sprintf("(v4 && isInNet(host, %q, %q))", p.Masked().Addr().String(), m)
}
