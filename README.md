# sockmnezia

An Android app that routes your traffic through **chains of proxy servers**: your **Amnezia Premium**
subscription and/or **your own Xray-compatible servers**, in any order. Use it as a hardened **local
SOCKS5/HTTP proxy** for apps that support one, or as a **VPN** for the whole phone or chosen apps.
Unofficial; not affiliated with Amnezia. Use only your own subscription key.

## What it does

- **Chains of hops.** Build a chain like *home server → Amnezia → VPS*: each hop is reached through the one
  before it, and the last one is the exit websites see. Up to 5 hops; Amnezia can be anywhere in the chain or
  not used at all. Save several chains, or pick a single server on its own.
- **Proxy or VPN mode.**
  - *Proxy*: apps set up for SOCKS5/HTTP use it, with a password.
  - *VPN*: everything, or only the apps you choose, goes through the chain, including apps with no proxy
    setting. DNS is configurable (public resolvers through the chain, DNS-over-HTTPS, or your own servers),
    the local network can bypass the VPN, and Android's Always-on VPN works. The proxy ports keep running
    beside it.
- **Fail-closed.** If a hop is down, traffic stops; it never falls back to a shorter chain or a direct
  connection.
- **Amnezia Premium.** Takes your subscription key (`vpn://…`), talks to Amnezia's gateway like the official
  client and runs the returned VLESS/Reality relay chain. Location picker with "find fastest".
- **Your own servers.** vless (incl. Reality, xhttp), vmess, trojan, shadowsocks, socks and http links, or an
  Xray outbound JSON. Add or edit them field by field or as a link/JSON.
- **Test without switching.** Measure the exit address and ping of any chain or server while the current
  one keeps running.
- **Sharing.** Share a server as a standard link or QR code, or a whole chain as links (Amnezia is a
  placeholder: whoever imports it uses their own subscription) or as Xray JSON. Import several links or a
  shared chain at once, with a preview.
- **Routing.** v2rayNG-style rules with geosite/geoip, presets, import/export and a "where does it go" tester;
  geo files download with progress and can update weekly.
- **Background health.** Checks battery optimization, background restrictions, Data Saver and more, and
  shows a card only when a setting needs changing.

## Hardened by default

Password always required on the proxy ports (random, editable) · random non-standard port · loopback only
(LAN sharing opt-in) · UDP off by default · no management API · private and loopback destinations always
blocked · secrets encrypted with the Android Keystore and never logged. The rules live in the config builder
(`core/xrayconf`), checked on every start, not in the UI. Details: [`docs/PLAN.md` §4](docs/PLAN.md),
[`docs/DECISIONS.md`](docs/DECISIONS.md).

## Get it

[GitHub releases](https://github.com/73hnuc134r/sockmnezia/releases): `arm64-v8a` fits most phones, the
universal APK fits all. The app checks for updates on GitHub. English and Russian.

**Desktop (Windows, Linux):** the same core with a desktop UI: a servers-and-chains grid, the tray, the
system proxy (or PAC) set and restored for you, and a TUN mode for all traffic. Releases are tagged `desktop-v…`
([releases](https://github.com/73hnuc134r/sockmnezia/releases)); see [`desktop/README.md`](desktop/README.md).

A command-line version with the same core (chains, servers, sharing, routing, geo files) is in
`core/cmd/sockmnezia` for Linux, Windows, macOS and Termux.

## Repository

| Path | What |
|---|---|
| `core/` | Go core: gateway client, chain and config builder with the security policy, embedded Xray-core, share links, engine, CLI, gomobile API |
| `android/` | Kotlin / Jetpack Compose app |
| `desktop/` | Desktop app (Wails: Go + Svelte in the OS webview) |
| `docs/01…08` | Research: key format, gateway protocol, Xray config shape, risks |
| `docs/09…11` | Chains and sharing; desktop app plan; Android VPN mode |
| `docs/PLAN.md`, `docs/DECISIONS.md` | Plan and decision log |
| `docs/RELEASING.md` | CI and releases |
| `reference/` | Throw-away reference implementations of the protocol (executable spec) |

Build: `cd core && go test -race ./...`, then `bash tool/build-aar.sh` (needs the Android NDK) and
`cd android && ./gradlew assembleDebug`. See [`docs/RELEASING.md`](docs/RELEASING.md).

## Ground rules

- Never commit keys, configs, UUIDs or server addresses (gitleaks runs on every push).
- Only a user's **own** subscription key is ever used; nothing here shares or scrapes keys, and a shared chain
  never contains the Amnezia config.
- The gateway is contacted only on user action or when a cached config is older than 24 h; one identity per
  install.

License: MIT. Xray-core is MPL-2.0; ZXing (QR codes) is Apache-2.0.
