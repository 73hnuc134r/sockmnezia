# sockmnezia

Android app (Windows/Linux later): a hardened local SOCKS5/HTTP proxy that runs an Amnezia Premium
subscription's VLESS config in embedded Xray-core, optionally chained to the user's own server.
Plan: `docs/PLAN.md`. Decisions: `docs/DECISIONS.md` (add an entry for every non-obvious choice).
Research and protocol: `docs/01…08`; chains (multihop) and sharing: `docs/09`; desktop app plan: `docs/10`; Android VPN mode plan: `docs/11`. `reference/` is a frozen executable spec, not product code.

## Layout
- `core/` — Go module (`github.com/nuc134r/sockmnezia/core`):
  - `vpnkey`, `gateway`, `pubkey`, `account` — subscription key → gateway → Xray config
  - `links` (own-server share links: parse and `Format`), `routing` (rules, presets, tester), `geo` (geo files)
  - `xrayconf` — builds the config from a chain of hops and enforces the security policy (`Audit`);
    `runner` — embedded Xray (one dialer registry for all live instances, D26)
  - `engine` — state (chains, servers), persistence, start/stop, share/import (used by the app binding and
    `cmd/sockmnezia`)
  - `internal/mockgw` mock gateway, `internal/geotest` tiny geo files for tests
  - `runner/integration_test.go` runs real Xray upstreams on loopback (chain, auth, UDP, blocking)
  - `mobile` — gomobile API for the app (JSON strings in/out; errors are `engine.Error` JSON)
  - `cmd/sockmnezia` — CLI
- `android/` — Kotlin + Jetpack Compose app (`io.nuc134r.sockmnezia`): `core/` (Go bridge, models, Keystore box),
  `service/` (foreground + VPN service, QS tile, boot receiver, geo worker), `health/` (battery checks), `ui/`
- `tool/` — `build-aar.sh` (Go core → `android/app/libs/*.aar`), `strings.py` (UI strings, en + ru),
  `version.sh`, `changelog.sh`
- `.github/workflows/` — `tests.yml`, `secrets.yml` (gitleaks), `release.yml`, `release-notes.yml`;
  releasing: `docs/RELEASING.md`

## Rules
- **No personal data in the repo**: no real keys, subscription keys, UUIDs, hosts or IPs. Use `example.com`,
  `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`, `2001:db8::/32`. Generate keys at test time.
- Never contact the real gateway from tests or CI.
- Secrets never go into logs, `String()` methods or error messages.
- User-visible changes get a line in `CHANGELOG.md` under the next version.
- UI strings live in `tool/strings.py` (English and Russian); run `python3 tool/strings.py` after editing.
  Never hardcode UI text in Kotlin. Error codes from the core are translated in `ui/Common.kt` `errorText`.
- After changing `core/mobile`, rebuild the AAR (`tool/build-aar.sh`) before building the app.

- Security behaviour (auth, listen address, locked rules, fail-closed chain) belongs in `xrayconf`, with a test.
- New Xray features must be imported explicitly in `runner/features.go` (never `main/distro/all`).

## Commands
```bash
cd core && gofmt -l . && go vet ./... && go test -race ./...
ANDROID_HOME=… bash tool/build-aar.sh
cd android && ./gradlew lintDebug testDebugUnitTest assembleDebug
```
