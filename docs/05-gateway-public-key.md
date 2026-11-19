# 05 · The gateway RSA public key

## The problem

Every request is encrypted to the gateway's RSA public key (doc 03). **The key is not in the open-source repo.**
`secureAppSettingsRepository.cpp` builds the production environment from a compile-time define
`PROD_AGW_PUBLIC_KEY_B64` (base64 of a PEM), set from a CI secret in the release workflows. Sandbox
(`SANDBOX_AGW_PUBLIC_KEY_B64`) and dev keys exist too. So the key only exists inside **official release
binaries** (and in whatever the maintainers publish).

## How the reference code gets it

Scan an official binary for the key and try every candidate against the gateway:

- Format found in binaries: either a literal `-----BEGIN PUBLIC KEY-----…-----END PUBLIC KEY-----` or the
  **base64 of that PEM** (begins `LS0tLS1CRUdJTiBQVUJMSUMgS0VZ`).
- Android APK is a zip: the native lib (`lib/<abi>/libAmneziaVPN…so`) contains it; entries are deflated, so
  unzip first (a minimal zip central-directory reader is ~20 lines, see `reference/termux.js`). Windows
  `.exe`/macOS binary: scan raw bytes.
- **The binary embeds several RSA keys** (production, sandbox, …). The first match may be wrong (this happened).
  Collect all candidates, send the real request with each, keep the first whose answer decrypts. A wrong key gives a
  *plain* JSON 500 (`internal server error`), which is a reliable "wrong key" signal.
- Cache the working key (`gateway.pem`, next to the app). If it stops working, rescan a newer APK (the key can
  rotate between releases; an old APK from the 4.8.x line produced a wrong candidate first but still contained the
  working key as a second candidate).

## Options for the product (pick one, document it)

1. **User supplies an APK/binary path** (reference behavior). Simple, robust, no redistribution.
2. **App downloads the latest official release** from the upstream GitHub releases page and extracts the key
   itself (needs network; pin to release assets, verify it is a zip/APK, never execute anything).
3. **Ask the user to paste the PEM.** Lowest effort, worst UX.
4. Ship a key in the app: *discouraged*; it is a deployment secret of the service, may rotate, and bundling it is
   a policy decision for the maintainers of this project, not a technical one.

The key is a public key (it cannot decrypt anything), but it is configuration the vendor chose not to publish in
source form; treat extraction from the user's own copy of the official app as the safe baseline.
