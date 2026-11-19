# amnezia-socks5

**Goal:** a small app that is both

1. an **Amnezia Premium API client**: takes the user's own Subscription Key (`vpn://…`), talks to Amnezia's
   gateway the way the official client does, and obtains the Xray (VLESS Reality) client config, and
2. a **local Xray proxy**: runs that config and exposes a plain **SOCKS5 / HTTP proxy on `127.0.0.1`**
   (no VpnService, no TUN), so any proxy-aware tool can use Amnezia Premium, optionally with the user's own
   VPS as the exit IP (chain: `own VPS -> Amnezia proxy -> Amnezia relay`).

The official Amnezia app only offers a full-device VPN. This project exists because a local proxy is often
what people need (per-app proxies, AdGuard/Privoxy setups, routers, scripts, Termux).

## State of the research

Everything below was worked out by reading the open-source clients and testing against a **mock** gateway.
The real gateway was reached once from a phone and answered with a valid config, which confirmed the protocol
and the response structure. See `docs/` in this order:

| Doc | Contents |
|---|---|
| [`docs/01-findings-summary.md`](docs/01-findings-summary.md) | The 2-minute version. Start here. |
| [`docs/02-subscription-key-format.md`](docs/02-subscription-key-format.md) | What a `vpn://` key is and what is (not) inside. |
| [`docs/03-gateway-protocol.md`](docs/03-gateway-protocol.md) | Endpoints, request envelope, crypto, payload, responses, errors. |
| [`docs/04-xray-config-structure.md`](docs/04-xray-config-structure.md) | Shape of the returned Xray config (a chain!) and how to chain a VPS. |
| [`docs/05-gateway-public-key.md`](docs/05-gateway-public-key.md) | The one awkward dependency: the gateway RSA public key. |
| [`docs/06-risks-and-open-questions.md`](docs/06-risks-and-open-questions.md) | Device slots, CAPTCHA, ToS/licensing, unknowns. |
| [`docs/07-app-design-brief.md`](docs/07-app-design-brief.md) | Suggested architecture and a build plan for the app. |
| [`docs/08-source-map.md`](docs/08-source-map.md) | Where to look in the upstream open-source repos. |
| [`docs/PLAN.md`](docs/PLAN.md) | **Build plan**: Android app, Go core, local proxy hardening, CI. |

`reference/` holds working throw-away implementations of the protocol (Node.js interactive script, Python CLI,
single-file HTML page, CORS worker) and an offline test-suite with a **mock gateway** that implements the
documented envelope. Use them as an executable spec, not as the product.

## Ground rules for whoever builds this

- Never commit keys, configs, UUIDs or server addresses. `.gitignore` already excludes the usual files.
- Only a user's **own** subscription key is ever used; nothing here shares or scrapes keys.
- Treat the live gateway politely: one fetch per run, reuse the stored device identity, back off on errors.
