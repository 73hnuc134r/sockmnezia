# sockmnezia

Android app (Windows/Linux later): a hardened local SOCKS5/HTTP proxy that runs an Amnezia Premium
subscription's VLESS config in embedded Xray-core, optionally chained to the user's own server.
Plan: `docs/PLAN.md`. Decisions: `docs/DECISIONS.md` (add an entry for every non-obvious choice).
Research and protocol: `docs/01…08`. `reference/` is a frozen executable spec, not product code.

## Layout
- `core/` — Go module (`github.com/nuc134r/sockmnezia/core`):
  - `vpnkey`, `gateway`, `pubkey`, `account` — subscription key → gateway → Xray config
  - `links` (own-server share links), `routing` (rules, presets, tester), `geo` (geo files)
  - `xrayconf` — builds the config and enforces the security policy (`Audit`); `runner` — embedded Xray
  - `engine` — state, persistence, start/stop (used by the app binding and `cmd/sockmnezia`)
  - `internal/mockgw` mock gateway, `internal/geotest` tiny geo files for tests
  - `runner/integration_test.go` runs real Xray upstreams on loopback (chain, auth, UDP, blocking)
- `.github/workflows/` — `tests.yml`, `secrets.yml` (gitleaks).

## Rules
- **No personal data in the repo**: no real keys, subscription keys, UUIDs, hosts or IPs. Use `example.com`,
  `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`, `2001:db8::/32`. Generate keys at test time.
- Never contact the real gateway from tests or CI.
- Secrets never go into logs, `String()` methods or error messages.
- User-visible changes get a line in `CHANGELOG.md` under the next version.

- Security behaviour (auth, listen address, locked rules, fail-closed chain) belongs in `xrayconf`, with a test.
- New Xray features must be imported explicitly in `runner/features.go` (never `main/distro/all`).

## Commands
```bash
cd core && gofmt -l . && go vet ./... && go test -race ./...
```
