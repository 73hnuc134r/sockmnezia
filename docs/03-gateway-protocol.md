# 03 · Gateway protocol

Source of truth: upstream `amnezia-client` (`client/core/controllers/api/subscriptionController.cpp`,
`gatewayController.cpp`, `core/utils/api/gatewayPayloadBuilder.cpp`) and the Go transport library `libagw`
(`gateway/envelope.go`, `client.go`, `transport.go`). Verified against the live gateway once (a real answer was
received and decoded) and against a mock implementing the same envelope.

## Endpoint

- Production base: `http://gw.amnezia.org:80/` (**plain HTTP**; confidentiality comes from the envelope).
  Hard-coded in the client. Sandbox/dev gateways exist (build-time config).
- Paths used by the client: `v1/config` (get/refresh a config, the important one), `v1/trial`, `v1/services`,
  `v1/account_info`, `v1/revoke_config`, `v1/native_config`, `v1/revoke_native_config`, `v1/renewal_link`,
  `v1/subscriptions`, `v1/restore_subscription`, `v1/news`, `v1/get_subscription_info`.
- HTTP: `POST`, `Content-Type: application/json`, header `X-Client-Request-ID: <uuid4>` (the real client sends it).
- Censorship bypass in the official client: if the direct request looks interfered with (TLS error, undecryptable
  answer, …) it fetches a proxy list from S3-style storage objects, health-checks proxies and retries through them.
  The list is encrypted with a key derived from `SHA-512(PEM text)` (see `libagw` `deriveProxyListKeyIV`). The
  storage URLs are build-time constants. **Not implemented in the reference code.** Users in blocked regions
  can instead route the whole call through any working VPN/proxy.

## Request envelope

Body is JSON with two base64 fields:

```json
{ "key_payload": "<b64 RSA(keys_json)>", "api_payload": "<b64 AES-CBC(payload_json)>" }
```

1. Generate random `key` (32 bytes), `iv` (**32 bytes**), `salt` (8 bytes).
2. `keys_json = {"aes_key": b64(key), "aes_iv": b64(iv), "aes_salt": b64(salt)}` (standard base64, compact JSON).
3. `key_payload = base64( RSA_PKCS1_v1_5_encrypt(gateway_public_key, keys_json) )`.
   Public key is X.509 SPKI PEM (`BEGIN PUBLIC KEY`), RSA. **Not OAEP.** Note: WebCrypto does not implement
   RSAES-PKCS1-v1_5; the browser reference does BigInt modular exponentiation (~15 lines).
4. `api_payload = base64( AES-256-CBC(PKCS#7 pad(payload_json), key, iv[:16]) )`.
   Quirks that the server relies on: **IV is sent as 32 bytes but CBC uses only the first 16**; the 8-byte salt is
   sent and unused.
5. Response body (HTTP status is not reliable, success was 200, errors 500 etc.) is **raw AES-256-CBC ciphertext**
   (no base64) of a JSON document, same `key` and `iv[:16]`, PKCS#7. If decryption fails, the answer was not from
   the gateway (blocked, proxy page, or **wrong public key**: the gateway then answers with a plain-JSON 500
   such as `{"http_status":500,"message":"internal server error"}`).

## `POST v1/config` payload (what the client sends for a Premium/VLESS config)

```json
{
  "os_version": "android",
  "app_version": "5.x.y.z",
  "cli_name": "AmneziaVPN",
  "app_language": "en",
  "installation_uuid": "<uuid4, persistent per install>",
  "user_country_code": "<from key api_config>",
  "server_country_code": "<optional location, e.g. de>",
  "service_type": "<from key api_config, e.g. amnezia-premium>",
  "service_protocol": "vless",
  "public_key": "<client UUID for vless; WG public key for awg>",
  "auth_data": { "api_key": "<from key>" }
}
```

- Empty/absent values are omitted (the official `GatewayPayloadBuilder` skips empty fields).
- For **`vless`, `public_key` is a client-generated UUID** (v4). The gateway registers it as a user on the Xray
  server and returns it in the config. Reuse the same UUID on later calls.
- Optional fields seen in code: `distribution`, `captcha_id` / `captcha_solution`, `is_connect_event`,
  `email` (trial), `migration_code`, `transaction_id` (store purchases).
- The official client refreshes configs by re-posting the same payload (`auth_data` taken from the stored
  server entry), e.g. when the user switches location; `server_country_code` selects the location.

## Response (success)

```json
{
  "config": "vpn://…",
  "http_status": 201,
  "routing_decision": { "relay": {"effective": true, "requested": false},
                         "split": {"effective_options": [], "requested_options": []} },
  "service_info": { "is_ad_visible": false, "is_renewal_available": true }
}
```

- **`http_status: 201` is success** (a naive "http_status present ⇒ error" check is wrong; treat `>= 400` as error).
- `config` decodes per doc 02 to a server entry; see doc 04.

## Errors (JSON, inside the decrypted body)

`{"http_status": N, "message": "…"}`. Mapping from `apiUtils::checkApiResponseErrors`:
409 → config limit reached (device slots; or "trial already used"), 404 → not found, 408 → timeout,
429 → rate limit, 402 → payment required / expired, 422 → unprocessable (e.g. expired subscription message),
501 → not implemented / update the app, CAPTCHA-required answers carry captcha data (`captcha_id`, image,…).
A captcha must be solved (the official client shows it); simplest handling: tell the user to connect once in the
official app.

## Other behaviors worth knowing

- Answer also includes `available_countries` inside the decoded server entry's `api_config`
  (list of selectable locations), `subscription` info (end date, device counts), `server_country_code(_name)`.
- `v1/account_info` returns subscription/device info; `v1/revoke_config` frees a device slot.
- Timeout used by the client: 12 s per attempt.
