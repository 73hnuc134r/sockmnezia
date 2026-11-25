# Plan: sockmnezia, an Android local proxy for Amnezia Premium (VLESS)

Status: **agreed draft** (decisions in §9). Nothing is built yet. Research is in `docs/01…08`; this file is the build plan.

## 1. What we build

An Android app that

1. takes the user's own Premium Subscription Key (`vpn://…`), fetches the Xray config from the Amnezia gateway
   (doc 03) and keeps the device identity stable (doc 06), and
2. runs that config (or only the user's own server, see modes in §5.1) in an embedded Xray-core and exposes a **hardened local SOCKS5 (+ optional HTTP) proxy**
   for other apps (AdGuard, Termux, browsers with proxy settings, …). No VpnService, no TUN, and
3. optionally chains the user's **own Xray-compatible server** as the exit (`own server -> Amnezia -> relay`).

App and repo: **sockmnezia** (lowercase), package id `io.nuc134r.sockmnezia`, license **MIT**. Distributed via GitHub releases.

Later: the same core in a Windows/Linux app (tray app or CLI).

## 2. Architecture: Go core + native Kotlin shell

```
┌──────────────────── Android app (Kotlin, Jetpack Compose) ───────────────────┐
│ UI: onboarding, status, settings, countries, logs                            │
│ ProxyService (foreground service, specialUse)  ←→  Quick Settings tile        │
│ SecretStore (Android Keystore + EncryptedFile)  · settings (DataStore)        │
└───────────────────────────────┬──────────────────────────────────────────────┘
                                │ gomobile bind → core.aar (small, typed API)
┌───────────────────────────────┴──────── Go core (`core/`) ───────────────────┐
│ vpnkey    decode/encode vpn:// (doc 02)                                       │
│ gateway   envelope, POST v1/config, error mapping (doc 03), pluggable dialer  │
│ pubkey    extract RSA key candidates from an APK/binary, trial, cache (doc 05)│
│ account   payload builder, identity, Fetch(country), Revoke()                 │
│ xrayconf  take last_config, keep the chain, replace inbounds, optional VPS    │
│           exit, security policy (§4), validate                                │
│ runner    embedded xray-core: start/stop/reload, health check                 │
│ cmd/amz   CLI (desktop/Termux, also the dev/test harness)                     │
└──────────────────────────────────────────────────────────────────────────────┘
```

Why: Xray-core is Go, so the core is Go and is reused as-is for Windows/Linux later (CLI first, a desktop UI
on top later). The Android side is native Kotlin as asked; it never parses gateway/Xray JSON itself, it only
calls the core API (`Init`, `Fetch`, `Countries`, `Start(settings)`, `Stop`, `Status`, `Forget`).

Notes:
- Xray-core is a **library dependency** pinned in `go.mod` (not libXray, not v2rayNG's AAR) so we control the
  version and the inbound code we rely on in §4. License: MPL-2.0, fine for an app; `libagw` is **not** used
  (no license, doc 06) — the gateway client is a clean-room implementation of doc 03.
- `gomobile bind -target android -androidapi 26` produces the AAR; ABIs `arm64-v8a`, `armeabi-v7a`, `x86_64`.
- Geo files (`geoip.dat`, `geosite.dat`) are not bundled; they are downloaded on demand for routing (§5.3).

## 3. Repository layout

```
core/                  Go module: packages from §2, mock gateway in Go, tests
  mobile/              the gomobile-facing API (only simple types)
android/               Gradle project (app module, depends on the AAR built from core/)
tool/                  version.sh, changelog.sh (from nux), build-aar.sh
reference/             existing executable spec (Python/Node), stays as is
docs/                  research + this plan + RELEASING.md
CHANGELOG.md  .gitleaks.toml  .github/workflows/
```

## 4. Local proxy security (the main requirement)

### 4.1 What the threat is

In spring 2026 it was disclosed that most Xray/sing-box mobile clients (v2rayNG, Happ, v2RayTun, Hiddify,
NekoBox, …) open an **unauthenticated SOCKS5 on 127.0.0.1:10808**. Any app on the phone (including apps that
are required to ship monitoring code) can scan loopback, find it, connect, and learn the **exit IP** (or bypass
per-app split tunneling). The Android loopback is shared by all apps; there is no per-app isolation.
Mitigations in the write-ups: SOCKS auth, disabling UDP (UDP ASSOCIATE "has no auth"), not using a fixed port,
not exposing management APIs.

### 4.2 What v2rayNG does today (read from its current source, Nov 2026)

- `CoreConfigManager.configureInbounds`: one `socks` inbound (Xray serves HTTP on the same port);
  `listen 127.0.0.1` unless "proxy sharing" is on; `auth: password` **only if** the user set both username and
  password, otherwise `noauth`; `udp` on by default (`DEFAULT_SOCKS_ENABLE_UDP = true`); sniffing http/tls/quic.
- `SettingsManager.getSocksPort`: fixed port from settings (default **10808**) or "dynamic port" =
  `Random.nextInt(10000, 65535)` per start.
- Defaults are therefore the vulnerable ones (fixed well-known port, no auth, UDP on). Users reported the
  auth/random-port options breaking the Hev TUN mode; related issues were closed "not planned".

### 4.3 What Xray-core actually does with UDP (read from `proxy/socks`, current master)

- With `auth: password` the TCP handshake requires credentials before any command, **including UDP ASSOCIATE**.
- UDP ASSOCIATE opens a **new ephemeral UDP port per association** (not the main port), bound to the inbound's
  address, closed when the TCP control connection closes or idles out.
- The UDP socket accepts datagrams only from the expected source: the IP of the TCP peer (or the address given
  in the request), and it **pins the first sender's port** when the request had port 0.
- Residual risk on Android: every app shares 127.0.0.1, so the IP check means nothing; a local app that guessed
  the ephemeral port could race the legitimate client for the first datagram. It cannot open an association
  itself without the password. Low risk, but not zero → UDP stays **off by default** (§4.4).

### 4.4 Our policy (defaults in bold; all enforced in `xrayconf`, not left to the UI)

| Area | Behavior |
|---|---|
| Authentication | **Always on**, cannot be disabled. SOCKS5 username/password (RFC 1929) and HTTP Basic on the HTTP inbound. **Random 16-char username and 32-char password generated on first run**, stored in `SecretStore`. The user can **edit both freely** (any printable ASCII, 1–255 bytes as RFC 1929 allows; a strength hint warns below 12 chars, never blocks), view/copy, or regenerate. |
| Port | **Random port chosen on first run** from 20000–60999 (never 1080/10808/10809/7890/2080 or other well-known client ports), kept stable so client apps stay configured. The user can set any port 1024–65535 or press "random". **Rotation is manual only** (port and credentials change only when the user edits or regenerates them). Port in use → the proxy does not start and the user is told (no silent change). |
| Bind address | **127.0.0.1 only.** "Share on LAN/hotspot" is an explicit opt-in with a warning, binds to the chosen interface address (not 0.0.0.0), still requires auth. |
| UDP | **Off.** When on: SOCKS5 UDP ASSOCIATE only after authenticated TCP, per-association ephemeral port (Xray behavior above); clients are told in the UI to send their real source port in the ASSOCIATE request so pinning is exact. |
| HTTP inbound | **Off** (SOCKS5 only); opt-in, separate random port, Basic auth with the same credentials. |
| Management | **No Xray API / HandlerService / stats / metrics inbound, ever** (one client leaked configs that way). No dokodemo/DNS inbound. The app talks to the core in-process only. |
| Loopback abuse | Routing rule: destinations in `geoip:private`-equivalent ranges (127/8, 10/8, 172.16/12, 192.168/16, 169.254/16, ::1, fc00::/7, fe80::/10) → `blackhole`, so the proxy cannot be used to reach the phone's or LAN's own services (expressed as IP CIDRs, no geo files needed). |
| Sniffing | `routeOnly: true` (sniffed domains only for routing, the real destination is not rewritten). |
| Probing | Wrong/no credentials → connection closed right after the method/auth reply, no error text; failed-auth counter shown in the UI ("someone on this device probed your proxy"). |
| Logs | Xray log level `warning`, access log off; our `Log` redacts keys, UUIDs, addresses, passwords. Logs stay in-app, share only on user action. |
| Secrets at rest | Subscription key, proxy password, client UUID, gateway answer: Android Keystore-backed encryption; `allowBackup=false`, excluded from device-to-device transfer. |
| App components | No exported services/receivers/providers except the launcher activity and the QS tile (permission-protected). No "start proxy" intent other apps can fire. |

What we **cannot** hide, and say so in the README: a full loopback port scan will still find *a* SOCKS5 server
that asks for a password (the protocol requires answering the greeting). With auth always on, a scanner learns
"some proxy exists" but cannot use it to learn the exit IP or route traffic. Closing that last gap needs
per-UID filtering, which Android only allows to the active VpnService (`getConnectionOwnerUid`); that belongs
to a later optional TUN mode (§8).

Every row above gets an automated test (§6): a loopback probe without credentials must fail; UDP to the main
port must get nothing; a datagram from a foreign source port must be dropped; private destinations must be
blackholed; the generated config must contain no `api`, `stats` or extra inbounds.

## 5. Android app

- **Min SDK 26 (Android 8), target current.** Kotlin, Jetpack Compose, Material 3, single activity. Strings in
  `values/` and `values-ru/` (en + ru from day one).
- **Foreground service** (`foregroundServiceType="specialUse"`, sideload/GitHub distribution, no Play policy
  issue) with a persistent notification: state, port, Stop action. Quick Settings tile. Optional start on boot.
- **Onboarding**: paste/scan key → gateway public key (see open question Q1) → fetch → done; shows the
  device-slot warning (doc 06) before the first fetch.
- **Main screen**: on/off, current country, exit check (fetch the public IP *through* the proxy, on demand
  only), traffic counters from the core.
- **Proxy settings** (§4.4): host/port/username/password with copy buttons and a "copy as `socks5://…`"
  link, regenerate buttons, UDP/HTTP/LAN toggles with warnings.
- **Countries**: from `available_countries`; switching re-posts `v1/config` with `server_country_code`.
- **Own server exit** (§5.1), a must-have.
- **Battery checks** (§5.2).
- **Routing** with geo rules and an editor (§5.3).
- **Errors in plain words**: device limit (409), expired (402/422), CAPTCHA ("open the official app once"),
  gateway unreachable (suggest the option below).
- **Fetch the gateway through my own server** (option, off by default): the `v1/config` request is sent via
  the active own server (the core starts a temporary Xray instance with only that outbound and an in-process
  dialer, no listening port). Useful when `gw.amnezia.org` is blocked. A generic upstream SOCKS/HTTP proxy for the
  fetch is available in the same place.
- In-app update check against GitHub releases (like nux).

### 5.1 Chaining to the user's own server

> Superseded by `docs/09-multihop-redesign.md` (2026-11-25): the three modes below became saved chains of
> hops in which Amnezia is one hop kind, at any position. Kept for history.

- Import: share links `vless://` (incl. Reality, xhttp), `vmess://`, `trojan://`, `ss://`, `socks://`/`http://`,
  or a raw **Xray outbound JSON** object (for anything the links can't express). Parsing lives in the core
  (`xrayconf/links`), so the desktop app gets it too. Several saved servers, one active; stored encrypted.
- Chain: the own outbound is put **first** (default outbound) with `streamSettings.sockopt.dialerProxy:
  "proxy"`, so traffic goes `local proxy -> own server <- via Amnezia proxy <- via Amnezia relay`; exit IP = own
  server. Core validates tags, removes conflicting `dialerProxy`, rejects tags that clash with the Amnezia ones.
- Modes: **Amnezia only** / **Amnezia → own server** / **Own server only** (no Amnezia key needed, e.g. after
  the subscription ends; the app works without onboarding a key). Fail-closed: if a hop fails, connections fail;
  there is never a fallback to a shorter chain or to direct.
- Transports for the own hop: TCP-based ones (raw/tcp, ws, grpc, httpupgrade, xhttp; TLS or Reality) are
  supported and covered by the loopback test. UDP-based transports (kcp, QUIC/hysteria) cannot be dialed through
  the chain reliably → import is refused with an explanation. UDP *traffic* (SOCKS UDP, if enabled) goes through
  VLESS/XUDP; tested.
- "Test chain" button: connects through the full chain and reports the exit IP and latency (on demand only).

### 5.2 Battery / background checks

A "Background health" card on the main screen and in Settings, re-checked on every resume; each failed item
explains why it matters and has a button that opens the right system screen:

| Check | API | Fix action |
|---|---|---|
| Battery optimization exemption | `PowerManager.isIgnoringBatteryOptimizations` | `ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` (permission declared; GitHub distribution) |
| Background restriction ("Restricted" battery mode) | `ActivityManager.isBackgroundRestricted` (28+) | app details settings |
| Notifications allowed (FGS notification, Android 13+) | `NotificationManagerCompat.areNotificationsEnabled`, `POST_NOTIFICATIONS` | runtime request / notification settings |
| Data Saver blocking background data | `ConnectivityManager.getRestrictBackgroundStatus` | `ACTION_IGNORE_BACKGROUND_DATA_RESTRICTIONS_SETTINGS` |
| App hibernation / unused-app permission reset | `PackageManagerCompat.getUnusedAppRestrictionsStatus` | `IntentCompat.createManageUnusedAppRestrictionsIntent` |
| Battery Saver currently on (info only) | `PowerManager.isPowerSaveMode` | battery saver settings |
| OEM killers (Xiaomi/MIUI-HyperOS autostart, Huawei/Honor, Samsung sleeping apps, Oppo/Vivo/OnePlus) | `Build.MANUFACTURER` | known OEM intents where they resolve, otherwise instructions + link to dontkillmyapp.com |
| Start on boot works | flag set by `BOOT_COMPLETED` receiver | — (shown only when the option is on) |

Plus a runtime signal: the service records unexpected deaths (started but not stopped by the user) and the
card shows "Android stopped the proxy N times in the last 7 days" with the checklist above. Checks are pure
functions over an injected `DeviceState`, unit-tested with fakes.

### 5.3 Routing (geo rules + editor, modelled on v2rayNG)

v2rayNG keeps an ordered list of rulesets (`RulesetItem`: remarks, `domain[]`, `ip[]`, `port`, `network`,
`protocol[]`, `outboundTag` = proxy/direct/block, `enabled`, `locked`), ships presets (global, bypass
LAN, bypass mainland China, "white" lists for Iran and Russia) and downloads geo files from selectable sources
(`Loyalsoldier/v2ray-rules-dat`, `runetfreedom/russia-v2ray-rules-dat`, `Chocolate4U/Iran-v2ray-rules`). We
do the same:

- **Model** (core, `routing` package): ordered rules with the same fields (domain: `domain:`, `full:`,
  `keyword:`, `regexp:`, `geosite:`; ip: CIDR, `geoip:`; port ranges; network tcp/udp; protocol
  http/tls/quic/bittorrent; outbound `proxy` | `direct` | `block`), `enabled`, `remarks`. Plus
  `domainStrategy` (AsIs / IPIfNonMatch / IPOnDemand) and the default outbound (proxy). The core compiles them
  into Xray `routing.rules`; validation errors point at the rule and field.
- **Locked security rules first**, not editable or reorderable: private/loopback destinations → block (§4.4).
  User rules come after them, so routing cannot reopen that hole. "Direct" means the connection leaves from the
  phone's own network (shown in the editor as a hint).
- **Presets**: Global (everything via proxy), Bypass Russia (`geosite:category-ru`, `geoip:ru` direct;
  default for the `ru` locale, otherwise Global), Block ads (`geosite:category-ads-all`), Bypass BitTorrent,
  Block QUIC (udp/443). Applying a preset replaces or appends (user chooses).
- **Editor UI** (Compose): list with drag-to-reorder, enable switch, swipe to delete; rule screen with chips for
  domains/IPs, a geosite/geoip tag picker that lists tags from the downloaded files, outbound selector. Import /
  export the rule list as JSON (v2rayNG-compatible format, so lists can be copied over) and via clipboard.
- **Geo files**: source picker (the three above + custom URL), download on first use of a `geo*:` rule and on
  "Update now"; optional weekly auto-update over unmetered network (WorkManager). Downloads go through the
  proxy chain when it is running (so the fetch is not visible to the local network), verified against the
  `.sha256sum` published next to each file, atomically replaced; the proxy reloads after an update.
  Rules that use geo tags without files are refused with a "download geo files" action.
- **Test tool**: "Where does X go?" enter a domain/IP, the core shows the matching rule and outbound.
- Same engine for the future desktop app; the CLI accepts the JSON rule list.

## 6. Testing

- **Go**: unit tests for every package; a Go **mock gateway** (port of `reference/tests/mock_gateway.py`) with
  negative cases (wrong RSA key → plain 500, 409, 402, captcha body, `http_status: 201` success).
- **Loopback integration test** (Linux, CI): start a real Xray **server** (VLESS+Reality on loopback with a
  generated x25519 key, plus a relay to mimic the chain) as upstream, start our runner against it, then
  `curl` through SOCKS5 with/without credentials, UDP associate tests from §4.4, private-destination block,
  own-VPS chain. Nothing touches the real gateway.
- **Routing**: compile tests for every rule field and preset; loopback test that `direct`, `block` and `proxy`
  rules send traffic where expected; locked rules always come first. Geo fixtures are tiny generated `.dat` files.
- **Android**: JVM unit tests (view models, settings), one instrumented smoke test on an emulator in CI
  (start service, probe port with/without auth) — emulator job can be added after the first release if slow.
- **Manual milestone 0** (before UI work, needs the real key, never in CI): run a fetched config through
  `cmd/amz run` and confirm `curl --socks5-hostname` works through the chained config (doc 07, milestone 0).
- `secrets.yml` gitleaks on every push; fixtures only use 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24,
  example.com and keys generated at test time.

## 7. CI and releases (same shape as nux)

| Workflow | Trigger | Does |
|---|---|---|
| `tests.yml` | push to `main`, PRs | `go vet`, `staticcheck`, `go test -race ./...` (incl. loopback Xray test); build AAR; `./gradlew lint testDebugUnitTest assembleDebug` (arm64) |
| `secrets.yml` | every push/PR, called by release | gitleaks over full history with `.gitleaks.toml` (+ rules for `vpn://AAAA_3ic…` keys and VLESS UUID+host pairs) |
| `release.yml` | tag `v*` or manual with version | tests + secrets + build AAR → signed universal APK and per-ABI APKs (`sockmnezia-<ver>[-abi].apk`) → GitHub release, notes from `CHANGELOG.md`, `-beta` tags as pre-release |
| `release-notes.yml` | manual | re-sync notes from `CHANGELOG.md` |

- Same secrets as nux: `ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`,
  `ANDROID_KEY_PASSWORD`; without them a warning and debug signing. `docs/RELEASING.md` copied and adapted.
- Toolchain pinned: Go (`go.mod` `toolchain`), JDK 17, Gradle wrapper, Android SDK/NDK version, gomobile
  version; caches for Go modules, Gradle, NDK and the built AAR (keyed on `core/go.sum` + core sources).
- Version from git tag (`tool/version.sh`), `versionCode` = run number.
- Reproducibility: `-trimpath`, `-buildvcs=false`, stripped symbols (`-ldflags "-s -w"`).

## 8. Milestones

0. ~~**Feasibility**~~ **done**: the config generated by `reference/termux.js` was run with Xray and carried
   traffic through the chain (confirmed by the owner).
1. **Core**: `vpnkey`, `gateway`, `pubkey`, `account` + Go mock gateway + tests. CI `tests.yml`,
   `secrets.yml` live from the first commit.
2. **Proxy**: `xrayconf` with the full §4.4 policy, share-link parser and own-server chain (§5.1), routing
   model + geo file handling (§5.3), `runner`,
   loopback integration tests (incl. the chain). CLI usable on
   Linux/Termux (this is already the "desktop" fallback).
3. **Android shell**: AAR build, foreground service, onboarding, main screen, proxy settings (editable
   credentials/port), own-server chain UI, routing editor, battery checks, secrets in Keystore. Debug APK in CI.
4. **Release pipeline**: signing, `release.yml`, CHANGELOG, update check. First `v0.1.0` (beta).
5. **Polish**: countries, QS tile, boot start, error UX, ru translation review, exit-IP check.
6. **Later**: Windows/Linux app on the same core (tray UI), CAPTCHA UI, gateway-fetch via upstream proxy
   list, optional TUN/VpnService mode with per-UID access control for the proxy, AWG backend.

## 9. Decisions

1. Gateway public key: user picks the official Amnezia APK once (key extracted), plus a button that downloads
   the latest official APK from Amnezia's GitHub releases and extracts it. The key is never shipped.
2. Package id `io.nuc134r.sockmnezia`; the UI says it is unofficial and not affiliated with Amnezia.
3. Jetpack Compose.
4. License MIT (most permissive common choice; Xray-core MPL-2.0 is compatible when used as a library).
5. Port and credentials rotate only manually; the user can edit them.
6. Own-server chaining (incl. an own-server-only mode), battery checks and geo routing with an editor are in
   scope for v0.1.
7. Min SDK 26. Languages: English and Russian.
8. Optional: fetch the gateway config through the own server.
