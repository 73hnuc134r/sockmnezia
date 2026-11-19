# 06 · Risks and open questions

## Account side effects

- **Device slots (≈7 per subscription).** Each distinct `installation_uuid` / client UUID can occupy a slot.
  Generate once, persist (`state.json` in the reference), reuse on every call. Never generate per run.
  A full account answers 409 ("config limit"). Slots are freed in the web Personal Dashboard (or via
  `v1/revoke_config`, untested).
- The device may show up in the dashboard under the `os_version` the client reports (reference sends
  `android`/`windows`). Whether the dashboard shows names or only IDs is unknown; whether arbitrary `os_version`
  strings are accepted is untested.
- Credentials rotate/expire: handle re-fetch on connection failure or an auth-style error (the official client
  re-posts `v1/config` when the user changes location or the config is invalidated).

## Gateway behavior

- **CAPTCHA** can be demanded (`captcha_id` + image). Not handled in the reference. The official app solves it
  in UI; a first version can say "connect once in the official app, then retry". A proper client needs a UI.
- **Gateway reachability.** `gw.amnezia.org:80` may be blocked where users need this most. The official client has
  a proxy/S3 failover (doc 03) whose endpoints are build-time constants that aren't in the repo. Workaround that
  works today: route the fetch through any working VPN/proxy, or an own relay.
- **Protocol drift.** The client version string is sent (`app_version`); the server may gate behavior on it.
  Unknown tolerance for an unofficial `app_version`/`cli_name`. The reference used a plausible recent version.
- `http_status` semantics (201 = success) were learned the hard way; verify new status codes before treating
  as errors.

## Legal / policy

- **Terms of service:** using a subscription key outside the official app may violate Amnezia's terms. Unchecked.
  Be transparent in the README, use only the user's own key, no key sharing/pooling.
- **Licensing:** `amnezia-client` is **GPL-3.0**. Copying its code makes derivatives GPL-3.0. `libagw` (Go
  transport) had **no LICENSE file** at the time of research, so assume all rights reserved: do **not** vendor
  it; reimplement from the protocol description (doc 03) or ask the maintainers for a license.
- Xray-core is MPL-2.0; libXray style bindings have their own licenses. Check before bundling.

## Technical unknowns to resolve early

1. **Does Xray-core actually run the returned config as-is** (chained `dialerProxy`, empty SNI/shortId, xhttp
   with default `xmux` only)? Not tested; milestone 0 in doc 07. Version of Xray-core may matter (xhttp is
   recent; the official client bundles its own build via `recipes/amnezia-libxray`).
2. Behavior of **UDP** through the chain (SOCKS5 UDP ASSOCIATE) and QUIC sniffing.
3. How long a fetched config stays valid and whether `last_config` changes server-side (rotation).
4. Whether the **AmneziaWG** path would be useful instead (also returned by the gateway, `service_protocol: awg`).
   A local SOCKS5 over AWG needs a userspace WireGuard netstack (e.g. amneziawg-go + gVisor netstack), which is
   a bigger build; VLESS/Xray is the pragmatic first target.
5. Android specifics (if targeting Android): a plain local proxy needs no VpnService, but apps must be pointed
   at it; per-app routing UX is the user's problem unless a TUN mode is added later.
6. Premium keys for the old subscription generation (one key per location, no `auth_data`) are a different
   format and are out of scope.

## Security notes for the implementation

- The Subscription Key is a secret: store in the platform keystore if on a device; never log; never put in URLs.
- Loopback-only listeners; consider optional SOCKS auth for shared hosts.
- Parse everything from the gateway as untrusted: the envelope uses AES-CBC **without a MAC**, so response
  authenticity is weak (a MITM on plain HTTP can't read it but could tamper). Validate structure strictly,
  never execute/interpret config fields beyond Xray's own parsing.
