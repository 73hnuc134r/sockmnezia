# 01 · Findings summary

1. **A Premium `vpn://` Subscription Key does not contain any server config.** It is an account credential:
   `api_config` (service type, protocol, country) + `auth_data.api_key`. (doc 02)

2. **The official app turns that key into a config by calling Amnezia's gateway**, `POST <gateway>/v1/config`,
   with a small JSON payload. The request is wrapped in an **RSA-PKCS#1 v1.5 + AES-256-CBC envelope**; the
   answer is raw AES-CBC with the same key. (doc 03)

3. **The gateway answer** is JSON with `http_status: 201` and `config: "vpn://…"`, a second `vpn://` blob that
   decodes to the full server entry, including an **Xray client config** at
   `containers[amnezia-xray].xray.last_config`. (doc 03, 04)

4. **That Xray config is a chain, not a single server.** Outbound `proxy` (VLESS+Reality, tcp, xtls-rprx-vision)
   is dialed *through* outbound `proxy-relay` (VLESS+Reality, xhttp) using Xray's `sockopt.dialerProxy`. A single
   `vless://` link can't express that, so the app must run the whole JSON in Xray-core. (doc 04)

5. **To exit via your own VPS**, prepend one more VLESS outbound with `dialerProxy: "proxy"`; Xray then dials
   `own VPS -> proxy -> proxy-relay`. The exit IP is the VPS. (doc 04)

6. **The gateway's RSA public key is not in the open-source repo.** It is injected at build time from CI
   secrets and baked into official binaries (several keys: production, sandbox, …). Practical approach: extract
   it from an official APK/binary at runtime, try every candidate, keep the one the gateway answers. (doc 05)

7. **Device identity matters.** The payload carries `installation_uuid` and (for VLESS) a client-generated
   UUID that the gateway registers on the Xray server. A subscription allows ~7 devices; generate once, persist,
   reuse. (doc 06)

8. **Feasibility:** proven end-to-end at the protocol level (real gateway reached, real config received and
   structurally verified). Not yet proven: actually connecting through the returned config from a custom Xray
   runner. Do that first (doc 07, milestone 0).

## Why not just use the official app's VPN?

The user wants a local SOCKS5/HTTP proxy (works with existing proxy-based setups, no VpnService slot, can be
chained with an own VPS exit). The official client offers VPN mode only. No claim is made about *why*; the
protocol itself has no such restriction since Xray inbounds are just configuration.
