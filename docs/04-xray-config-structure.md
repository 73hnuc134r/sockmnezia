# 04 · Structure of the returned Xray config

`config` (a `vpn://` blob, doc 02) decodes to a server entry:

```
{
  "name", "description", "config_version",
  "hostName", "dns1", "dns2", "defaultContainer": "amnezia-xray",
  "server_country_code", "server_country_name", "server_country_code_l10n",
  "api_config": { "public_key", "available_countries", "use_cases", "subscription",
                  "server_country_code", "stack_type", … },
  "containers": [ { "container": "amnezia-xray",
                    "xray": { "port": "443", "transport_proto": "tcp", "last_config": "<JSON STRING>" } } ]
}
```

`containers[].xray.last_config` is a **JSON string** (not an object) holding a complete Xray client config:
`{ log, inbounds, outbounds }` (no `routing`).

## What the real answer looked like (anonymized)

```
inbounds : [ socks  127.0.0.1:10808, udp enabled, sniffing http/tls/quic ]
outbounds:
  1. tag "proxy"        vless  tcp   reality  flow xtls-rprx-vision  fingerprint firefox
                        streamSettings.sockopt.dialerProxy = "proxy-relay"     <-- chained
  2. tag "proxy-relay"  vless  xhttp reality  (no flow)  fingerprint qq   xhttpSettings: {xmux:{maxConnections:4}}
  3. tag "dns-out"      dns
```

- Each VLESS outbound uses the **`vnext[0].users[0]`** form: `address`, `port` (443), `id` (the client UUID we
  sent as `public_key`), `encryption: "none"`, `flow`.
- `realitySettings`: `publicKey` (43-char x25519 key), `fingerprint`, and **`serverName` / `shortId` are empty
  strings** in the observed answer (spiderX empty or absent). That is what the official app runs, so it works
  with Xray-core; some third-party clients mishandle empty SNI, which is one reason to run the JSON in Xray-core
  directly instead of converting to a `vless://` link.
- **The two VLESS outbounds are a relay chain**: traffic leaves the client through `proxy-relay` (xhttp
  Reality), which dials `proxy` (tcp Reality, vision). The gateway answer also said
  `routing_decision.relay.effective: true`.
- Therefore a single `vless://` link built from outbound 1 **skips the relay** and likely won't work or is
  not what the service intends. Always keep all outbounds and run the JSON.

## Chaining the user's own VPS as the exit

Goal: public IP = own VPS, but the censored path to it goes via Amnezia.

```
outbounds = [
  { tag: "hop-vps", protocol: "vless", settings: {vnext:[<own VPS>]},
    streamSettings: { …own transport…, sockopt: { dialerProxy: "proxy" } } },   // dial the VPS THROUGH Amnezia
  <proxy (dialerProxy: proxy-relay)>, <proxy-relay>, <dns-out>, { tag:"direct", protocol:"freedom" }
]
```

- Xray uses the **first outbound as the default**, so put `hop-vps` first; the local inbounds then exit via the VPS.
- `dialerProxy` chains nest: `hop-vps -> proxy -> proxy-relay`. Works for TCP-based inner transports; UDP over
  the chain needs testing.
- Remove any `dialerProxy` pointing to a tag that does not exist (Xray refuses to start).
- Reference implementation: `buildConfig` in `reference/termux.js` and `build_config` in `reference/fetch_vless.py`.
  Verified structurally against the real answer, **not yet by actually running Xray**.

## Local inbounds

Replace/add inbounds as needed: SOCKS5 on `127.0.0.1:10808` (UDP enabled) and HTTP on `127.0.0.1:10809` are the
defaults used in the reference. Bind to loopback only; no authentication is built in.
