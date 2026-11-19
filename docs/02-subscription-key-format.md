# 02 · `vpn://` key format

Amnezia uses the `vpn://` scheme for several things. All share one encoding:

```
vpn://  +  base64url( 4-byte prefix + zlib_stream )   (padding may be omitted)
```

- Decode: strip `vpn://`, base64url-decode (add `=` padding), **drop the first 4 bytes**, `zlib.decompress` (zlib
  wrapper, i.e. Qt `qUncompress` minus its length header), `json.loads`.
- The 4-byte prefix is either Qt's `qCompress` big-endian uncompressed length, or, for **subscription keys**, the
  fixed signature `00 00 00 ff` (`AMNEZIA_CONFIG_SIGNATURE`, then the length header is stripped).
  Base64 of that signature makes subscription keys start with `AAAA_3ic…` (`AAAA` + zlib header `78 9c`).
- Fallback: if decompression of `raw[4:]` fails, try `raw` (some producers omit the prefix).

## Kinds of keys

| Kind | Where it comes from | Has server config inside? |
|---|---|---|
| **Premium Subscription Key** | order email / personal dashboard | **No.** Only account data (below). |
| **Gateway answer config** | `config` field of `POST /v1/config` response | **Yes**, full server entry (doc 04). |
| Self-hosted guest key | app "Share" on a self-hosted server | Yes, static `containers[]`. |
| Free (Telegram bot) key | bot | Typically like Premium (API based). |

## Premium Subscription Key payload (decoded JSON)

```json
{
  "name": "Amnezia Premium",
  "description": "…",
  "config_version": 1,
  "api_config": {
    "service_type": "amnezia-premium",
    "service_protocol": "awg",
    "user_country_code": "xx"
  },
  "auth_data": { "api_key": "<secret>" }
}
```

- `auth_data.api_key` is the bearer-style secret. **Anyone holding the key can use the subscription** (7 device
  slots). Never log or commit it.
- `service_protocol` here is just what the key was issued for; the client asks for `vless` explicitly by sending
  `service_protocol: "vless"` (the app's "VLESS" toggle does the same).
- `user_country_code` is the user's region as seen at purchase.
- Keys may be reissued by support if compromised.

## Where to obtain the key (user side)

Order e-mail ("Subscription Key"), the web Personal Dashboard (login with the key), or restored automatically
in the app for App Store / Play purchases (no e-mail in that case).
