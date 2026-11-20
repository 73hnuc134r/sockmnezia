# sockmnezia

An Android app that turns your **Amnezia Premium** subscription into a **local SOCKS5/HTTP proxy** for other
apps (no VPN slot, no TUN), optionally exiting through **your own Xray-compatible server** chained behind
Amnezia. Unofficial; not affiliated with Amnezia. Use only your own subscription key.

- Takes your Subscription Key (`vpn://…`), talks to Amnezia's gateway like the official client and runs the
  returned VLESS/Reality relay chain in embedded Xray-core.
- Hardened local proxy: password always required (random by default, editable), random non-standard port,
  loopback only (LAN sharing opt-in), UDP off by default, no management API, private/loopback destinations
  always blocked, fail-closed chain. Details: [`docs/PLAN.md` §4](docs/PLAN.md) and
  [`docs/DECISIONS.md`](docs/DECISIONS.md).
- Modes: Amnezia · Amnezia → own server · own server only (vless, vmess, trojan, shadowsocks, socks, http links
  or Xray outbound JSON).
- v2rayNG-style routing rules with geosite/geoip, presets, import/export and a "where does it go" tester.
- Background health screen for battery optimization, Data Saver, OEM app killers and more.

Downloads: [GitHub releases](https://github.com/nuc134r/sockmnezia/releases) (`arm64-v8a` fits most phones).
A desktop/Termux CLI with the same core is in `core/cmd/sockmnezia`.

## Repository

| Path | What |
|---|---|
| `core/` | Go core: gateway client, config builder and security policy, embedded Xray, CLI, gomobile API |
| `android/` | Kotlin/Compose app |
| `docs/01…08` | Research: key format, gateway protocol, Xray config shape, risks |
| `docs/PLAN.md`, `docs/DECISIONS.md` | Plan and decision log |
| `docs/RELEASING.md` | CI and releases |
| `reference/` | Throw-away reference implementations of the protocol (executable spec) |

## Ground rules

- Never commit keys, configs, UUIDs or server addresses (gitleaks runs on every push).
- Only a user's **own** subscription key is ever used; nothing here shares or scrapes keys.
- The gateway is contacted only on user action or when a cached config is older than 24 h; one identity per install.

License: MIT. Xray-core is MPL-2.0.
