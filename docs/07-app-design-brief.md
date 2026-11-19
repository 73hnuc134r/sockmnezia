# 07 · App design brief

## Recommendation: one Go program

Xray-core is Go; the protocol work is plain HTTP + crypto. A single Go module gives:

- the **gateway client** (clean-room, from doc 03; stdlib `crypto/rsa`, `crypto/aes`, `net/http`),
- **`github.com/xtls/xray-core`** embedded as a library (`core.New(&core.Config{…})` from the JSON config, or the
  `infra/conf` loader), so no external binary and no `v2rayNG`,
- one static binary for Linux/macOS/Windows and **Termux (linux/arm64)**; later an Android `.aar` via
  `gomobile bind` for a UI wrapper.

Alternatives considered: Kotlin/Android with a prebuilt libXray AAR (best UX on phones, more build plumbing);
Node.js (the reference is Node but Xray would have to be a separate binary). Start with Go; the core library
should be UI-agnostic so a mobile shell can be added.

## Command-line shape (suggested)

```
amz init                      # asks for Subscription Key, finds the gateway key (APK path or auto-download)
amz fetch [--country de]      # POST v1/config, stores the config (state dir), prints summary
amz countries                 # lists api_config.available_countries from the last answer
amz run   [--listen 127.0.0.1:10808] [--http 127.0.0.1:10809] [--exit-vless <link>]
amz status                    # subscription end date, device counts, last fetch time, selected country
amz forget                    # delete stored key/config/identity (and optionally revoke the device)
```

- `run` = load stored config -> (optionally refresh if stale/failing) -> build final Xray JSON (doc 04, incl.
  optional own-VPS exit chain) -> start Xray-core with inbounds SOCKS5 + HTTP on loopback -> wait for signals.
- Config/state live in one user-chosen directory (default: next to the binary or `$XDG_STATE_HOME`), file mode 0600,
  never in the repo. Store: subscription key (prefer OS keyring when available), `installation_uuid`, client UUID,
  working gateway public key, last config JSON, last country.

## Modules

| Package | Responsibility |
|---|---|
| `vpnkey` | decode/encode `vpn://` (doc 02) |
| `gateway` | envelope + POST + error mapping (doc 03), pluggable `http.Client` (so users can pass a proxy) |
| `pubkey` | extract candidate RSA keys from APK/binary/PEM; trial against gateway (doc 05) |
| `account` | payload builder, identity persistence, `Fetch(country)`, `Revoke()` |
| `xrayconf` | pick outbounds, keep chain, add inbounds, add own-VPS exit, validate tags/dialerProxy (doc 04) |
| `runner` | embed Xray-core, start/stop/reload, health check through the local SOCKS port |
| `cmd/amz` | CLI |

## Milestones

0. **Prove the config runs** (before writing the app): generate `amnezia.json` with `reference/termux.js`
   (or the Python CLI), run it with a stock `xray run -c amnezia.json`, check
   `curl --socks5-hostname 127.0.0.1:10808 https://ifconfig.me`. Then try the own-VPS chain. If Xray rejects the
   config (version, xhttp, empty SNI), find out why *now* (unsupported field? needs newer Xray?).
1. `vpnkey` + `gateway` packages with tests against a mock gateway (port `reference/tests/mock_gateway.py`
   to Go, or run the Python mock in CI). Include negative tests: wrong key, 409, captcha body, 201 handling.
2. `account.Fetch` against the real gateway (manual, behind a flag/env var; never in CI).
3. `pubkey` extraction + candidate trial (include a test zip with two RSA keys, decoy first).
4. `xrayconf` + `runner`; integration test: local fake upstream (a Xray server on loopback) to validate chaining.
5. UX: `init` wizard, `countries`, `status`, refresh-on-failure, clear error messages (CAPTCHA, device limit,
   gateway blocked: suggest routing through another VPN).
6. Optional: gateway failover via proxy list (needs the S3 endpoints which are not public), CAPTCHA UI,
   Android `.aar`, TUN mode, AWG backend.

## Testing notes

- `reference/tests/` has a **mock gateway** implementing the exact envelope, returning an answer shaped like the
  real one (relay chain, `http_status: 201`, `last_config` as string). It is the executable spec for steps 1-3.
- Never put a real key or real config in tests or fixtures. Use synthetic UUIDs/addresses (`203.0.113.x`,
  `198.51.100.x`, `example.com`).

## Non-goals (first version)

Selling/sharing keys, bypassing subscription limits, reverse-engineering anything beyond the documented client
protocol, scraping other users' data.
