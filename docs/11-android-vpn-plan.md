# Plan: VPN mode on Android (next to proxy mode)

Status: proposal (2026-11-26).

## 0. Goal

Today the app is a **proxy**: apps that support SOCKS5/HTTP use it, with a password. **VPN mode** sends the
whole phone's traffic (or chosen apps') through the active chain or server, using Android's `VpnService`, for
apps that have no proxy setting. Proxy mode stays exactly as it is and remains the default.

## 1. How it works

```
apps ──► Android VPN (tun fd) ──► Xray "tun" inbound ──► routing ──► chain outbounds ──► internet
                                     (same Xray instance; the SOCKS/HTTP inbounds keep running beside it)
our own app ──────────────────────────────────────────────────────────► network (excluded from the VPN)
```

- **No extra tun2socks layer.** The pinned Xray (`v26.11.9`) has a TUN inbound with Android support
  (`proxy/tun`): it takes the VPN file descriptor (env `xray.tun.fd`), runs Xray's own lightweight TCP/IP
  stack, and feeds connections into the normal routing. One process, one Xray instance, the same chain and
  fail-closed outbounds as proxy mode.
- **No routing loop:** the app excludes itself from its VPN (`addDisallowedApplication(own package)`), so
  Xray's outbound connections (and the Amnezia gateway fetch) go to the real network. No `protect()` calls.
- **Fail-closed:** while VPN mode runs, the tun stays up even when a hop is down: traffic fails instead of
  leaking around the chain. Android's "Always-on VPN" + "Block connections without VPN" are supported.
- **Local network:** private ranges (10/8, 172.16/12, 192.168/16, 169.254/16, fc00::/7, fe80::/10, multicast)
  are not routed into the VPN, so printers/Chromecast/LAN keep working directly. Routes are computed as the
  complement (minSdk 26 has no `excludeRoute`, which needs API 33).
- **IPv6:** the VPN gets an IPv6 address and `::/0` (minus private), so IPv6 can't leak around the tunnel.
- **DNS:** see §3.

## 2. Changes

### Core (Go)
- `runner/features.go`: link `proxy/tun` (amends D7: the TUN inbound is linked, but `xrayconf.Audit` only
  allows it when the spec is VPN mode).
- `xrayconf.Spec.VPN` (MTU, addresses): adds inbound `tun-in` (sniffing as for SOCKS). `Audit`: exactly one tun
  inbound only in VPN mode; locked rules still first; the private-address block stays for the SOCKS/HTTP
  inbounds. Tests: config shape, audit refusals, a Linux test feeding a real tun fd in a network namespace if
  CI allows, otherwise an fd from a socketpair-backed fake.
- `engine`: `Options.Mode` = `proxy` | `vpn`; `StartVPN(fd)` sets the fd for Xray and starts; status reports the
  mode. Per-app settings stored in `Options` (`VPNApps` mode + package list).
- `mobile`: `StartVPN(fd int)`.

### App (Kotlin)
- `ProxyService` becomes a `VpnService` (still a foreground service): in VPN mode it asks for consent
  (`VpnService.prepare`), builds the interface (address, routes, DNS, MTU 1500, disallowed/allowed apps,
  `setUnderlyingNetworks` follow the default network), and passes the fd to the core. Proxy mode is unchanged.
- Manifest: `BIND_VPN_SERVICE` + `android.net.VpnService` intent filter; always-on support.
- Home: **Proxy / VPN** switch next to the connection block; notification and QS tile show the mode.
- Settings → VPN: DNS (§3), **apps**: all / only selected / all except selected, with an app picker (search,
  system apps toggle).
- Strings, `errorText` for new codes (`vpn_permission`, `vpn_failed`).

## 3. Decisions for the owner
1. **DNS in VPN mode.** Recommended: the VPN announces public resolvers (1.1.1.1, 8.8.8.8 and their IPv6) and
   their queries go through the chain like any traffic (UDP), so nothing resolves locally. Needs every hop to
   carry UDP (VLESS/VMess/Trojan/Shadowsocks/Amnezia do; an HTTP-proxy own hop doesn't, and SOCKS hops may not):
   the app warns for such chains. Alternative: Xray's DNS module with DNS-over-HTTPS through the chain (works
   over any hop, more config, slower first lookups).
2. **Per-app selection in the first version**, or VPN for all apps first and the picker later?
3. **SOCKS/HTTP ports in VPN mode:** keep them running beside the VPN (recommended: apps configured for the
   proxy keep working), or turn them off?
4. **Local network** bypasses the VPN (recommended), or goes through it (then blocked by the locked rules)?

## 4. Milestones
| # | Content |
|---|---|
| V1 | Core: tun inbound in `xrayconf` (+ audit, tests), engine mode, `StartVPN`; AAR |
| V2 | App: VpnService path, consent, routes/DNS/IPv6, Home switch, notification/tile, always-on |
| V3 | Per-app selection (if wanted in v1), warnings for chains without UDP, docs/decisions, release |

Not planned: running VPN and another VPN app at once (Android allows one VPN).
