# Decisions

Short records of choices made while building, newest last. Each says what was chosen and why, so it can
be revisited deliberately. The product plan is `docs/PLAN.md`; this file records the details below it.

## D1 · Go core, native Kotlin app (2026-11-19)

The core (gateway client, config builder, Xray runner) is one Go module in `core/`; Android calls it through a
gomobile AAR. Xray-core is Go, so this avoids a second language for the protocol work and gives the
Windows/Linux app for free later. Owner's choice: native Android first.

## D2 · License MIT (2026-11-19)

Owner asked for the most permissive option. No GPL code is copied: the gateway client is written from the
protocol description in `docs/03`, `libagw` (no license) is not used. Xray-core (MPL-2.0) is linked as a
library, which MPL allows in an MIT app; its notice goes into the app's licenses screen.

## D3 · Xray-core version (2026-11-19)

Pinned to the commit of tag `v26.11.9` (Go pseudo-version, because Xray's `v26.x` tags are not valid Go module
versions for a v1 module path). That version has the per-association UDP socket for SOCKS5 (`TempUDPConn`)
that §4.3 of the plan relies on; older module releases (`v1.260327.0`) do not. Requires Go 1.27, so the module
says `go 1.27.4` and CI uses `go-version-file`.

## D4 · Client identity reported to the gateway (2026-11-19)

Payload fields are the official client's: `cli_name: AmneziaVPN`, `app_version: 5.1.1.10` (the version used by
the reference scripts, accepted by the live gateway), `os_version: android`. One `installation_uuid` and one
client UUID per install, generated once and reused, so fetches do not use extra device slots. Empty fields are
omitted like the official `GatewayPayloadBuilder`.

## D5 · Gateway answer handling (2026-11-19)

- The HTTP status is ignored; the body is always decrypted. Undecryptable → `ErrWrongKey` (wrong gateway key,
  block page or proxy page); only that error moves on to the next candidate key.
- `http_status < 300` is success (201 is the normal answer). Errors are mapped like the official client:
  409 device limit / trial used, 402 captcha required / invalid / refresh or subscription not active,
  422 "Is it activated?" = expired, 404, 408, 429, 501 = update required.
- CAPTCHA: the answer carries `captcha_id`, `captcha_image` (base64) and `hint`; the retry sends
  `captcha_id` + `captcha_solution`. The core supports it, so the app can show the image instead of sending the
  user to the official app.
- No system proxy is picked up implicitly; a custom dialer can route the call (own server, upstream proxy).

## D6 · Gateway public key extraction (2026-11-19)

Owner's choice: the user picks the official APK once, plus a button that downloads it. The extractor scans
native libraries inside the zip (Amnezia's own library first, arm64 first) in 1 MiB chunks with overlap
(bounded memory on phones), finds PEM blocks (also with escaped `\n`) and base64-encoded PEMs, de-duplicates and
keeps order. All candidates are tried against the gateway; the one that answers is cached.

## D7 · Only the Xray features we use are linked (2026-11-19)

`core/runner/features.go` imports dispatcher, proxyman, router, dns, log, policy, stats, the outbound
protocols (vless, vmess, trojan, shadowsocks, socks, http, freedom, blackhole), the socks/http inbounds and the
TCP-based transports. No commander/API, metrics, observatory, reverse, dokodemo, tun or wireguard: a config that
asks for them does not load, so a management port cannot be opened even by mistake. Side effect: a smaller binary
(CLI ~31 MB stripped).

## D8 · Generated config shape (2026-11-19)

- Outbound tags: Amnezia outbounds get an `amz-` prefix (`amz-proxy`, `amz-proxy-relay`, dialerProxy rewritten);
  the own server is `own`; plus `direct` (freedom) and `block` (blackhole). Only vless/vmess/trojan/shadowsocks
  outbounds are taken from the gateway answer; its `dns-out`, inbounds, routing and anything else are dropped.
- The head of the chain is always the first outbound (Xray's default), so traffic without a matching rule never
  leaves directly: fail-closed. A dialerProxy cycle is rejected.
- Inbounds: SOCKS5 with `auth: password` always, optional HTTP inbound with the same account; `listen` is the
  chosen address (loopback by default, a private/link-local address for LAN sharing; never 0.0.0.0/::).
  The SOCKS `ip` (UDP relay address) is the listen address. Sniffing http/tls/quic with `routeOnly`.
- `stats` is enabled for the in-app traffic counters only (read in-process; there is no API inbound).
- `xrayconf.Audit` re-checks every generated config (allowed top-level keys, auth, listen address, locked rules
  first, head is not freedom/blackhole) and Build fails if it does not pass.

## D9 · Locked rules use literal ranges, not geo files (2026-11-19)

The two locked rules (private/loopback/link-local/multicast/CGNAT IPv4+IPv6 ranges, and `localhost`, `.local`,
`.localdomain`, `.home.arpa`) are written out, so they work without geo files and cannot be weakened by a
different geo source. They are compiled before user rules and verified by the audit and by integration tests
(including a user rule trying to send `127.0.0.0/8` direct). Note: recent Xray also blocks private targets in
`freedom` for VLESS/VMess/Trojan inbounds (server side, `finalRules`); our client side has SOCKS/HTTP inbounds,
where that default does not apply, hence our own rules.

## D10 · SOCKS UDP behaviour is tested, and off by default (2026-11-19)

Integration tests (`core/runner/integration_test.go`) prove with the pinned Xray: UDP ASSOCIATE needs the
password; the relay is a per-association port (never the main port); a datagram from another local socket to the
relay port is dropped when the client declared its source address; datagrams to the main port get nothing; the
association dies with its TCP connection; with UDP off, ASSOCIATE is refused. The UI tells users who enable UDP
that clients should declare their source port (most do).

## D11 · Credentials (2026-11-19)

Generated: 16-character lowercase username, 32-character password (57-symbol alphabet without look-alikes,
~187 bits), crypto/rand. User-editable (owner's request): 1–255 bytes (RFC 1929 limit) of printable ASCII
without spaces, so they work in every client's settings and in `socks5://user:pass@host:port` links. No minimum
length is enforced; the UI warns below 12 characters. Port: random 20000–60999, skipping well-known proxy client
ports (1080, 7890, 10808, …) and busy ports; editable 1024–65535. Rotation is manual only (owner's choice).

## D12 · State storage (2026-11-19)

All state (subscription key, identity, gateway keys, last config, own servers, credentials, settings) is one JSON
document written atomically as `state.bin` (0600) in the app's private directory, passed through a `SecretBox`.
Android implements the box with an AES-GCM key in the Android Keystore; the CLI uses no encryption and relies on
file permissions (desktop keyrings later). One blob keeps writes atomic and the Kotlin side simple.

## D13 · When the Amnezia config is fetched (2026-11-19)

On first start, when the user changes location, on "Refresh", and on start when the cached config is older than
24 h (setting). A failed refresh of a stale config falls back to the cached one; the gateway is never polled in
the background. Same identity on every request.

## D14 · Probe counter (2026-11-19)

Failed SOCKS authentications (no-auth greeting, wrong credentials, SOCKS4) are counted through a hook on Xray's
log handler and shown in the app. Xray's HTTP inbound answers 407 without logging, so HTTP probes are not counted.

## D15 · Routing details (2026-11-19)

Rule JSON is v2rayNG's `RulesetItem` shape (import/export compatible; imported `locked` rules are dropped
because ours are fixed). `proxy` in a rule means the chain head. `geosite:` tags are checked against the file when
rules are compiled (start, rule tester), not when saved, so rules can be written before the files are downloaded;
start refuses with `geo_missing` until they are. Default `domainStrategy` is `AsIs`: `IPIfNonMatch`/`IPOnDemand`
make Xray resolve domains with the system resolver, visible on the local network (the editor says so). The
rule tester uses Xray's own geodata matchers, so answers match the proxy's behaviour.

## D16 · Geo files (2026-11-19)

Sources: runetfreedom (default), Loyalsoldier, Chocolate4U, or a custom https URL. Known sources must publish
`<file>.sha256sum` and the download must match; a custom source may lack it. Files are parsed before use, both are
replaced atomically only when both succeeded, and while the proxy runs the download goes through it.

## D17 · Checks that touch the network are on demand only (2026-11-19)

Exit-IP check (`https://api.ipify.org` by default, configurable) and "Test chain" run only when the user taps
them. "Test chain" uses a temporary in-process Xray without any listening port (`xrayconf.BuildDialer` +
`core.Dial`); the same mechanism sends the gateway request through the own server when "fetch via own server"
is on.

## D18 · Android stack (2026-11-20)

Kotlin + Jetpack Compose (Material 3, navigation-compose), single activity, no DI framework: one `Core` object
bridges to Go on `Dispatchers.IO` and exposes a status `StateFlow` and a revision counter screens reload on.
AGP 9.4 (built-in Kotlin), Kotlin 2.4, Gradle 9.8, compileSdk/targetSdk 37 (current AndroidX requires it),
minSdk 26. JSON between Kotlin and Go via kotlinx.serialization with the Go field names. The Go core is a
gomobile AAR built in CI by `tool/build-aar.sh` and never committed; `-checklinkname=0` is needed because an
Xray dependency (`wlynxg/anet`) uses `go:linkname`. Native libraries are stored compressed in the APK
(`useLegacyPackaging`): ~12 MB instead of ~34 MB per ABI to download.

## D19 · Android service model (2026-11-20)

The proxy runs in the app process; a `specialUse` foreground service (not exported) keeps the process alive and
shows address and traffic; its Stop action and the QS tile are the only controls outside the app. If the
service goes away the proxy is stopped, so it never runs without the notification. "Should run" is remembered;
a new process while it is set and the boot count (`Settings.Global.BOOT_COUNT`) is the one recorded at start
counts once as "Android stopped the proxy" for the health screen (reboots do not count). Start on boot needs both the setting and "was running".

## D20 · Battery/background checks (2026-11-20)

Checks are a pure function over `DeviceState` (unit-tested), read on every resume: battery-optimization
exemption (`REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` is declared — the app is not on Google Play and needs it),
background restriction (API 28+), notifications, Data Saver, unused-app hibernation, Battery Saver (info), known
aggressive OEMs (direct intents to their autostart screens with fallback to app details and a link to
dontkillmyapp.com), and the count of unexpected stops in the last 7 days.

## D21 · Gateway key download (2026-11-20)

"Download from GitHub" asks the GitHub API for the latest `amnezia-vpn/amnezia-client` release, takes the
arm64 APK (or any APK), streams it to a temp file in the app's private directory (600 MB cap), scans it and
deletes it. It goes through the proxy when the proxy runs. Choosing a local APK copies it to the cache first
(the core needs a seekable file) and deletes the copy.

## D22 · Strings (2026-11-20)

UI strings (English, Russian) are generated from `tool/strings.py` into `values*/strings.xml`; CI fails if the
XML is stale. Russian plurals use one/few/many/other.

## D23 · Connection bar, route and device ID (2026-11-21)

Home has a bottom bar (state, route, exit address, ping, start/stop). The route is `xrayconf.Hops` (server
addresses, phone first) in `Status.chain`. Exit address and ping are NOT polled: one "tap the bar" check
(`TestChain`, which uses the running instance when the proxy runs) so the IP lookup service is contacted only on
demand, as before. The device ID shown is the installation UUID (what Amnezia's device list shows); the client
UUID is the Xray user id and stays secret. The notification title is live speed • total traffic.

## D24 · Fastest-location ping (2026-11-22)

"Find fastest" pings every location returned by the last Amnezia answer. Each ping is a real, one-off
`account.Fetch` for that country followed by a request through a temporary Xray instance built from the
result (`Engine.PingCountry`/`mobile.PingCountryJSON`), chained to the active own server like `TestChain`
when the mode calls for it — a plain network ping would not reflect the actual exit path. It never writes
`Amnezia.Last`/`Amnezia.Country` or restarts the running proxy; only picking a result from the dialog does
that (the existing `Fetch` call). Up to 3 attempts per location run concurrently (bounded) in the app; the
lowest successful round trip is shown as soon as the first attempt succeeds and keeps improving until all 3
finish, so a slow or failing gateway for one location doesn't block the others.

## D25 · Chains replace the three modes (2026-11-25)

Plan: `docs/09-multihop-redesign.md`. The state (v2) keeps named chains of hops, phone first; a hop is
Amnezia or an own server by ID, and one chain is active. `xrayconf.Outbounds` dials hop i+1 through hop i
(`sockopt.dialerProxy`); for Amnezia that is the outbound its relay chain dials first (the "tail"), so Amnezia
can be the entry, in the middle or the exit. Rules, each in `xrayconf` with a test: 1–5 hops; Amnezia at most
once (one subscription, one config, one device slot); a hop that is not the entry must be TCP-dialable
(`links.CheckChainable`). Own servers are tagged `hop<N>` by position, never by name or address.

- Migration: v1 modes become "Amnezia", "Amnezia → <server>", "<server>" built from the server that was
  active; the one matching the old mode is active. "Fetch through my own server" becomes "fetch through
  <that server>" (`Options.FetchVia`).
- Gateway requests (fetch, location ping) go through the active chain's hops before Amnezia when there are any:
  fetching around them would expose what the user put in front of Amnezia. Otherwise `FetchVia`, otherwise
  direct.
- A server used by a chain cannot be deleted (owner's choice): the error lists the chains.
- Known upstream issue: an xhttp outbound dialed through another hop (Amnezia's relay with a hop before it)
  trips two data races inside Xray (`splithttp`'s `httptrace` hook reaching the inner dial, and `redirect()`
  appending to a shared session slice). Both writers store the same values, so they are harmless in practice,
  but `go test -race` reports them; the 3-hop integration test uses a TCP relay, and the gateway-shaped
  config with the xhttp relay in the middle is load-checked by Xray in `TestGatewayShapedConfigLoads`.

## D26 · One dialer registry for all Xray instances (2026-11-25)

Xray resolves `dialerProxy` through process-wide globals (`internet.InitSystemDialer`: outbound manager and
DNS client) that every `core.New` overwrites. A temporary instance next to the running proxy (location ping,
connection check when stopped, gateway fetch through earlier hops) therefore took over the proxy's hop
lookups, and after it closed they pointed at a closed instance: "Find fastest" while connected left the proxy
broken until a restart. `runner` now registers every live instance and installs a manager that looks a tag up
in all of them (re-installed after each `core.New`); `xrayconf.BuildDialer` gives temporary configs a unique
tag prefix (`t<N>-`) so a lookup never lands in the wrong instance. `TestTemporaryInstanceDoesNotHijackTheChain`
fails without it. The globals are still written without a lock by `core.New` itself (Xray's own behaviour).

## D27 · Sharing servers and chains (2026-11-25)

A server is shared as a standard link made by `links.Format`, which only returns a link that `links.Parse`
turns back into exactly the same outbound (otherwise the outbound JSON is shared): a link never describes a
different server than the one that runs. A chain is text: `sockmnezia://chain?v=1&name=…`, then one line per
hop (phone first), Amnezia as `sockmnezia://amnezia` (owner's choice). Its config is never exported: it holds
this subscription's per-device credentials; the receiver's subscription fills the hop. Other apps import the
links as separate servers. "Xray JSON" (outbounds with `dialerProxy`) is offered only for chains without
Amnezia. QR codes encode the same text (ZXing core, encoder only). Import previews first (dry run), reuses saved
servers with the same outbound instead of duplicating them, and validates the chain before writing anything.
Shared text contains credentials: the clipboard entry is marked sensitive and it never goes to logs.

## D28 · One selection sheet; single servers without a chain (2026-11-25)

Owner's request. What the proxy runs is either a saved chain (`ActiveChain`) or a single own server
(`ActiveServer`, at most one of the two set): a server no longer needs a one-hop chain, so adding the first
server selects it directly and the v1 "own server only" mode migrates to it. Deleting the selected server or
chain falls back to the first chain (a running proxy switches, or stops when nothing is left). The app picks
it in one bottom sheet (Home card button, or a small button above start/stop) with headed lists for chains and
servers. Each row has Test, which runs `Engine.TestSaved` in a temporary instance (D26 keeps the running
proxy's hops intact) and never changes the selection, and a ⋮ menu: edit (chains) or rename (servers), share,
delete after a confirmation. With "New chain" and "Add server" in the sheet, the Servers tab was removed.

## D29 · Selection screen and endpoint editor (2026-11-26)

Owner's request: the D28 sheet became a regular screen (`Routes.SELECT`), and servers get an editor
(`Routes.SERVER`) with two modes, fields or link/JSON text. The fields are `links.Endpoint`, which maps 1:1
to share-link parameters: `EndpointFromLink` goes through `Parse` and `Format`, and `Endpoint.Link()` builds a
link that the app saves through the same `Parse` as a pasted one, so there is one validation path. A server
whose outbound has settings no link can carry (raw JSON with extras) is edited as JSON only (`json_only`).
`UpdateOwnServer` keeps the ID, so chains follow an edit; it is refused when a chain using the server would
no longer be valid. Rename is the editor's Name field. Pasting several links or a chain stays a separate
"Import links" action with its preview (D27).

Tests: the runner integration tests wait out Xray's dial retries after a request that is expected to fail
(`mustFail`), so the next test starting an instance doesn't race a still-retrying dial (seen as a flaky
`TestUDP` under `-race`).

## D30 · Desktop app decisions (2026-11-26)

Owner's answers to `docs/10-desktop-plan.md` §8, for when the desktop work starts: Wails (Go core in-process,
OS webview); Windows and Linux first, macOS later; no subscriptions. System proxy: OS proxy settings cannot
carry credentials, so while the system-proxy mode is on the app opens a separate HTTP port on 127.0.0.1
without authentication (loopback only, never on LAN, off by default, warned in the UI); the SOCKS/HTTP ports
keep their password. This exception is built in `xrayconf` with a test, like every security rule.

## D31 · VPN / TUN mode uses Xray's own tun inbound (2026-11-26)

Android VPN mode (`docs/11`) and desktop TUN mode (`docs/10` §3a, owner's addition) both use the `tun` inbound
of the pinned Xray in the same instance as the SOCKS/HTTP inbounds: no tun2socks library, no second process,
the same chains and fail-closed outbounds. This amends D7: `proxy/tun` is linked, and `xrayconf.Audit` allows
exactly one tun inbound only when the spec is in VPN/TUN mode. Android excludes the app itself from its VPN
(no routing loop, no `protect()`); desktop binds Xray's sockets to the physical interface
(`autoOutboundsInterface`). Owner's choices for Android: DNS configurable (public resolvers through the chain
by default, DoH in Xray, or custom servers), LAN configurable (bypass by default), per-app selection in the
first version, SOCKS/HTTP ports keep running beside the VPN.

## D32 · In-app updates (2026-11-26)

The update card downloads and installs the newer release instead of only linking to it. The core picks the
release APK for the device's ABIs (`Build.SUPPORTED_ABIS`, then the universal APK), downloads it only from this
repository's release downloads (through the proxy while it runs), and requires a matching `SHA256SUMS` entry
from the same release; the app installs it with a `PackageInstaller` session (no FileProvider). Authenticity
beyond HTTPS + checksums is Android's: an APK signed with another key does not install over the app. Installing
kills the process; `MY_PACKAGE_REPLACED` starts the proxy/VPN again when it was running (independent of
"start on boot"). Needs `REQUEST_INSTALL_PACKAGES`.

## D33 · Android and desktop are released separately from one repository (2026-11-27)

One repository (the apps share the Go core), but separate versions and tags: Android `v1.2.3` (as before),
desktop `desktop-v0.1.0`, each with its own workflow and changelog (`CHANGELOG.md`, `desktop/CHANGELOG.md`).
A desktop-only change does not make an Android release and vice versa; a core change that matters to both is
released in both. Each app's update check only looks at its own tag prefix (Android builds up to 1.1.5 read
`desktop-v…` as 0.0.0, so they never offer a desktop release). Desktop releases are published as not "latest",
so GitHub's latest release stays the Android one.

## D34 · Desktop app structure (2026-11-27)

- `desktop/` is its own Go module (`replace` → `../core`), so Wails and its cgo (GTK/WebKit on Linux) never
  reach the core module or the Android build.
- Wails **v3** (3.1): it has the system tray, single instance, autostart and multiple windows built in (v2 has
  no tray). Linux uses GTK 4 + WebKitGTK 6 (Wails 3's default; Ubuntu 24.04+, Debian 13+, Fedora 39+).
  Windows uses WebView2 and needs no cgo, so both platforms build on a Linux runner.
- The bound service (`desktop/core.go`) returns typed values (engine types) instead of the Android binding's
  JSON strings; TypeScript types are generated (`tool/desktop-bindings.sh`, checked in CI). Errors are
  `*engine.Error`, which reaches the UI as the call error's `cause`. Logic both front ends need moved into
  `engine` (`ServerViews`, `Redact`, endpoint conversions) instead of being duplicated.
- UI: Svelte 5 + TypeScript + Vite, no component library; strings from `tool/strings.py` (shared table plus
  desktop-only `D` entries) as JSON.
- State lives per user in `%LOCALAPPDATA%\sockmnezia\desktop` / `$XDG_DATA_HOME/sockmnezia`, separate from the
  CLI's state directory, so both can be used without overwriting each other.

## D35 · Desktop system proxy and PAC (2026-11-27)

Implements D30. `Options.SystemProxy` ("", "system", "pac") and a stable random `SystemProxyPort`;
`xrayconf.Spec.SystemProxyPort` adds the `sysproxy-in` HTTP inbound on 127.0.0.1 without authentication, and
`Audit` allows exactly that one exception (HTTP, IPv4 loopback, at most one, never the only inbound). The
desktop app applies the OS settings only while the proxy runs: Windows WinINET (registry + refresh), Linux
GSettings and/or KDE kioslaverc; other desktops get manual steps. The previous settings are saved to
`sysproxy-restore.json` before the first change and put back on stop, quit and the next start after a crash (superseded by D38).
Local names and private ranges are in the bypass list, since the proxy blocks them (locked rules).

PAC (`routing.PAC`): served on a separate loopback port, generated from the rules on every request. It follows
the rules in order but stops at the first rule a PAC script cannot evaluate exactly (geosite/geoip/regexp,
ports, networks, protocols, IP rules with a resolving domain strategy); everything undecided goes to the proxy,
where Xray applies the rules. So PAC never sends anything direct that Xray would not, and never calls
`dnsResolve` (no lookups through the local resolver).

Close to tray defaults to on for Windows and off for Linux, where not every desktop shows tray icons
(StatusNotifierItem). Autostart registers the app with `--hidden`, which opens to the tray when closing to the
tray is on.

## D36 · Desktop releases and updates (2026-11-27)

- Packages: Windows per-user NSIS installer (`%LOCALAPPDATA%\Programs`, no administrator rights, matching the
  per-user system proxy and autostart) and a portable zip; Linux `.deb` and `.rpm` (nfpm, depending on GTK 4
  and WebKitGTK 6) and a `.tar.gz`. No AppImage for now: it would have to bundle WebKitGTK, which is large and
  fragile across distributions; the packages use the distribution's.
- The update check only shows the newer `desktop-v…` release and opens its page; the app does not download or
  run installers. The plan's "verify a signature before installing" needs a release-signing key, which does
  not exist yet; until then the user installs from the release page (HTTPS, SHA256SUMS published).
- Binaries are not code-signed (no certificate); SmartScreen warns on first run (docs/RELEASING.md).

## D37 · Desktop TUN mode: an elevated helper, prompt or no-prompt (2026-11-27) — superseded by D40

Owner's choice after comparing v2rayN (which elevates the whole app on Windows, with an elevated Task Scheduler
logon task for autostart, and on Linux pipes the user's sudo password, kept in memory, into `sudo -S`).

- Only a helper runs with administrator/root rights: the same binary with `--tun-helper`; the window and the
  web view never do. The app builds the config as for every start (engine, `xrayconf`) and sends it over a
  local pipe; the helper runs it in embedded Xray, whose tun inbound creates the interface, the routes
  (`autoSystemRoutingTable`), binds Xray's own sockets to the physical interface (`autoOutboundsInterface`),
  and on Windows adds the WFP leak filters (`dns`, `misconfigtun`); on Linux systemd-resolved is pointed at the
  tun when it runs. DNS is answered over DoH through the chain. The SOCKS/HTTP ports keep working (they live in
  the helper's instance; the app's own requests go through them).
- The helper runs only configs that pass `xrayconf.Audit` (now also: no log files, the tun name and its options
  fixed) and contain the tun inbound; it stops the TUN when the app disconnects (stop, quit or crash), and
  exits unless it was started at logon.
- Transport and identity: Windows: named pipe `sockmnezia-tun-<user SID>`, DACL for that user and SYSTEM only;
  the app checks the pipe's server process is elevated. Linux: `/run/user/<uid>/sockmnezia-tun.sock`, the
  helper takes the uid from `PKEXEC_UID` (its command line has no arguments), both sides check `SO_PEERCRED`
  (client = that user, server = root).
- Default: a prompt each time TUN starts (UAC via ShellExecute "runas"; polkit via `pkexec`). Opt-in
  "TUN without prompts" (one prompt to set up): Windows registers the Task Scheduler task "sockmnezia TUN"
  (logon trigger, highest privileges, `--persistent`), which the app also runs on demand; Linux installs a
  polkit rule allowing exactly `<binary> --tun-helper` for that local, active user without a password. The
  trade-off, shown in the UI: any program the user runs could then use the helper too (within the audit).
- No password is stored. Windows ships `wintun.dll` (0.14.1, pinned SHA-256) next to the exe; Linux packages
  recommend pkexec and remove the polkit rules on uninstall.
- While TUN runs, the app's temporary test instances (Test, Test all, location ping) go through the TUN, i.e.
  through the active chain; the running chain's own test uses its SOCKS port.

## D38 · The desktop system proxy stays set: fail-closed (2026-11-27)

Owner: leaking around the proxy is not acceptable. Replaces the "restore on stop, quit and after a crash" part
of D35. While System proxy or PAC is chosen, the OS settings point at sockmnezia whether the proxy runs or
not; stopping, quitting or a crash leave them, so apps that use the system proxy fail instead of going direct.
The settings from before the first change are kept in `sysproxy-restore.json` and put back only when the user
chooses Off (or TUN). A failure to set them never falls back to removing them.

## D39 · PAC mode dropped (2026-11-27)

A PAC script is served by the app; when sockmnezia isn't running, browsers and Windows treat the unreachable
script as "no proxy" and go direct, which breaks the fail-closed rule (D38). Serving it as a file is not
reliable on Windows. A saved "pac" mode is read as the system proxy. `routing.PAC` is removed.

## D40 · Desktop TUN mode runs in a system service, fail-closed (2026-11-27)

Owner: drop the per-start helper (D37); install a service only when TUN is chosen; nothing may leak around the
proxy (D38). Portable builds stay.

- Choosing TUN installs the service (one administrator prompt: UAC / pkexec); choosing another mode removes
  it (one prompt; if declined, TUN mode stays). Windows: a LocalSystem service with restart-on-failure;
  Linux: a systemd unit (`Restart=always`, ordered before `network-pre.target`). The service binary is a
  copy in an administrators-only directory (`%ProgramData%\sockmnezia\bin`, `/usr/local/libexec/sockmnezia`):
  a SYSTEM/root service must never run a file the user can change. A version mismatch reinstalls (prompt).
  Portable builds install the same way. Package removal / the Windows uninstaller remove the service too.
- The service runs the engine's configs (audited, with the tun inbound) and keeps the last one in an
  administrators-only state file, so TUN continues when the app quits and starts at boot. The app attaches on
  launch without restarting it (an identical config is not restarted).
- Fail-closed: Stop in TUN mode makes the service run the same config with everything blocked
  (`xrayconf.BlockAll`): the tun and its routes stay, internet traffic is dropped until Start or another mode.
  Windows: Xray's WFP filters `allexceptlan` + `misconfigtun` let nothing but Xray leave outside the tun
  while the service runs. Linux: an nftables table (`inet sockmnezia`) drops all output except loopback, the
  tun, the local network and sockets with Xray's mark (`SO_MARK` on every outbound in TUN mode), from the
  service start until the uninstall, so it also covers a crash/restart of the service and the boot until
  Xray is up. Windows has a gap of about a second when the service crashes (restarted by Windows) and at boot
  before the service starts. Inbound connections from the internet to this computer's own servers get no
  answer while the kill switch is on (local network excepted).
- Linux DNS without systemd-resolved: name lookups still go to the system's resolver; the UI says so (no
  change, owner's choice). Linux TUN needs systemd and nftables.
- Pipe/socket and identity as in D37, now one per machine for the installing user: Windows
  `\\.\pipe\sockmnezia-tun` (DACL: that user and SYSTEM; the app checks the server is LocalSystem), Linux
  `/run/sockmnezia/tun.sock` (owner = installing user, 0600; SO_PEERCRED both ways).

## D41 · Traffic statistics (2026-11-27)

Per day (local date), per selected chain or single server, plus "direct": what the rules sent around the
proxy. The engine samples Xray's counters (inbound totals and the `direct` outbound, `statsOutbound*` now on)
with the status and when the proxy stops or restarts; through-the-proxy = inbound − direct. Counting per hop
would count every byte once per hop. Stored in `stats.json` next to the state (not secret: IDs, names and
byte counts), a year long, deleted by Reset and by "Delete all data". A deleted chain/server keeps the name it
had. Traffic Xray makes itself (DNS-over-HTTPS in VPN/TUN mode, tests) isn't counted.

## D42 · Windows system proxy through the per-connection WinINET options (2026-11-28)

Writing `ProxyEnable` / `ProxyServer` under `Internet Settings` plus `INTERNET_OPTION_SETTINGS_CHANGED` does not
change `Connections\DefaultConnectionSettings`, which the Settings app, Edge, Chrome and WinHTTP read: those apps
went direct. The desktop now uses `InternetSetOption(INTERNET_OPTION_PER_CONNECTION_OPTION)` on the LAN
connection (flags, server, bypass, auto-config URL), as v2rayN does, and `InternetQueryOption` to save the
previous settings; settings saved in the old registry form are still restored. Not yet verified on Windows.

## D43 · Material 3 redesign: seeded teal, islands, thumb row (2026-11-28)

Both apps follow one set of tokens, approved as mockups first.

- **Colour.** A fixed Material 3 tonal scheme generated from the brand teal `#00695C`, light and dark. Android 12+
  wallpaper colours (dynamic colour) are off by default and offered as a switch in Settings: wallpaper palettes
  made the app look muddy and unlike itself, and the security cues (red "none" pill, Block) need stable colours.
- **Font.** The system font (Roboto on Android, Segoe UI / system sans on desktop); nothing bundled, so no size
  cost.
- **Layout.** "Islands": rounded (28) blocks on a slightly darker screen colour, holding groups of rows with tight
  inner corners (20/6). Surfaces lighten towards the front in both themes (`LocalLevels` on Android, CSS variables
  on desktop) instead of using card shadows.
- **Thumb row.** The three most common actions sit together at the bottom right for one-handed use: the
  connection status (tap = check exit IP and country), Switch, and Start/Stop in the corner. Screens with a main
  action (Save, Add, Test all, Presets) put it there too. Home has no title bar.
- **Mode switch.** Proxy / VPN moved from Home to Settings: it is set once, and Home stays for the thumb actions.
- **Server list.** Chains first, then one island per host with its servers (a host's servers share an address, so
  they share a country). Protocol, security, transport and port are pills everywhere; ports have no leading ":".
  Pings are shown with the country code.
- **Country.** From the local geo database only (core `geo.LookupCountry`), never an online service; so a host
  given as a domain name has no country until a test reports the exit's.

