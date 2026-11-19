# 08 · Source map (upstream, public)

Repos: `github.com/amnezia-vpn/amnezia-client` (GPL-3.0, C++/Qt) and `github.com/amnezia-vpn/libagw` (Go
transport; no license file seen). Paths as of client ~5.1.x; they move, grep for the identifiers.

## amnezia-client

| What | File / symbol |
|---|---|
| Fetch/refresh a config, payload fields | `client/core/controllers/api/subscriptionController.cpp` (`importServiceFromGateway`, `updateServiceFromGateway`, `extractServerConfigJsonFromResponse`, `publicKeyForProtocol`, `generateProtocolData`) |
| Common payload (`os_version`, `app_version`, `cli_name`, `app_language`, `installation_uuid`, `distribution`) | `client/core/utils/api/gatewayPayloadBuilder.cpp` |
| JSON key names (`api_config`, `auth_data`, `service_type`, `service_protocol`, `server_country_code`, `available_countries`, …) | `client/core/api/apiDefs.h`, `client/core/utils/constants/apiKeys.h` |
| Gateway wrapper, proxy bypass state | `client/core/controllers/gatewayController.cpp` |
| Gateway environments (prod endpoint constant, key defines) | `client/core/repositories/secureAppSettingsRepository.cpp` (`gatewayEnvironment`), `client/CMakeLists.txt` (defines), `.github/workflows/deploy.yml` (secrets used at build time) |
| Error mapping of gateway JSON | `client/core/utils/api/apiUtils.cpp` (`checkApiResponseErrors`) |
| Building a Premium `vpn://` key (signature + zlib, no length header) | `apiUtils.cpp` (`getPremiumV2VpnKey`, `AMNEZIA_CONFIG_SIGNATURE = 000000ff`) |
| `auth_data` model | `client/core/models/api/authData.h` |
| Server entry model (`api_config`, `containers`) | `client/core/models/api/apiV2ServerConfig.cpp` |
| Xray config generation for self-hosted + key names (`last_config`) | `client/core/utils/constants/configKeys.h`, `client/server_scripts/xray/*` |
| Local settings encryption (not needed, FYI: AES-256-CBC, key in OS keychain, disabled on Linux) | `client/secureQSettings.cpp` |
| Bundled Xray bindings recipe | `recipes/amnezia-libxray`, `recipes/amnezia-xray-bindings` |

## libagw

| What | File |
|---|---|
| Envelope crypto (RSA PKCS#1 v1.5, AES-256-CBC, 32-byte IV/first 16 used, 8-byte unused salt) | `gateway/envelope.go` |
| `Client.Post`, failover through proxy pool, working-proxy cache | `gateway/client.go`, `gateway/bypass.go`, `gateway/state.go` |
| HTTP transport, `X-Client-Request-ID`, timeouts | `gateway/transport.go` |
| Proxy-list storage URLs and decoding (key = `SHA-512(PEM)` split) | `gateway/storage.go`, `gateway/envelope.go` (`deriveProxyListKeyIV`) |
| C ABI | `cabi/agw.h` |
| Third-party Go reimplementation (WireGuard-only) | `github.com/asciimoth/wgo/amnesia` (no xray/vless) |

## Docs used

Official docs site `docs.amnezia.org`: connecting via text key, Premium FAQ, Personal Dashboard (where to find the
Subscription Key, device slots, "Reload API config").
