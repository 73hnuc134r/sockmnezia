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
