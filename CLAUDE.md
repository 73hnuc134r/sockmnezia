# sockmnezia

Android app (Windows/Linux later): a hardened local SOCKS5/HTTP proxy that runs an Amnezia Premium
subscription's VLESS config in embedded Xray-core, optionally chained to the user's own server.
Plan: `docs/PLAN.md`. Decisions: `docs/DECISIONS.md` (add an entry for every non-obvious choice).
Research and protocol: `docs/01…08`. `reference/` is a frozen executable spec, not product code.

## Layout
- `core/` — Go module (`github.com/nuc134r/sockmnezia/core`): `vpnkey`, `gateway`, `pubkey`, `account`;
  `internal/mockgw` is the mock gateway used by tests.
- `.github/workflows/` — `tests.yml`, `secrets.yml` (gitleaks).

## Rules
- **No personal data in the repo**: no real keys, subscription keys, UUIDs, hosts or IPs. Use `example.com`,
  `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`, `2001:db8::/32`. Generate keys at test time.
- Never contact the real gateway from tests or CI.
- Secrets never go into logs, `String()` methods or error messages.
- User-visible changes get a line in `CHANGELOG.md` under the next version.

## Commands
```bash
cd core && gofmt -l . && go vet ./... && go test -race ./...
```
