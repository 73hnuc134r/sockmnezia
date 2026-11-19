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
