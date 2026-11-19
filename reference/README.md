# reference/

Working but throw-away implementations of the documented protocol. Use as an executable spec.

| File | What |
|---|---|
| `termux.js` | Interactive Node.js script (zero dependencies): subscription key -> gateway -> `amnezia.json` / `chain.json`. Finds the gateway public key inside an APK. |
| `fetch_vless.py` | Same flow as a Python CLI (`pip install cryptography`). |
| `index.html` | Single-file browser version (BigInt RSA PKCS#1 v1.5 + WebCrypto AES-CBC). Needs a CORS proxy for the plain-HTTP gateway. |
| `worker.js` | Cloudflare Worker CORS proxy for the page. |
| `tests/mock_gateway.py` | Mock gateway implementing the exact envelope and an answer shaped like the real one. |
| `tests/test_mock_gateway.py` | Python CLI vs. the mock (decoy gateway keys included). |
| `tests/test_page_core.js` | Browser-core JS vs. the mock (run with Node 20+, needs `python3`). |

Run the tests from `reference/`: `python3 -I tests/test_mock_gateway.py` and `node tests/test_page_core.js`.
All values in tests are synthetic. Do not add real keys, UUIDs or addresses.
