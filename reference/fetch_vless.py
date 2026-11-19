#!/usr/bin/env python3
"""Fetch the Xray/VLESS-Reality config of an Amnezia Premium subscription
straight from Amnezia's gateway, the same way the official client does, and
print a vless:// link (and optionally a 2-hop Xray config through your own VPS).

Protocol source: amnezia-vpn/amnezia-client (subscriptionController.cpp,
gatewayController.cpp) and amnezia-vpn/libagw (gateway/envelope.go).

  pip install cryptography

  # 1. one-time: get the gateway's RSA public key out of YOUR installed app
  python3 -I fetch_vless.py --extract-pubkey "C:/Program Files/AmneziaVPN/AmneziaVPN.exe" -o gateway.pem

  # 2. fetch your VLESS config
  python3 -I fetch_vless.py vpn://YOUR_SUBSCRIPTION_KEY --pubkey gateway.pem

  # 3. or build the chain  Amnezia hop -> your VPS
  python3 -I fetch_vless.py vpn://... --pubkey gateway.pem --chain "vless://your-vps" -o chain.json

NOTE: each fetch registers a client with Amnezia and may use one of your 7 device
slots. The tool therefore keeps ONE installation id + client UUID in
state.json (next to the script) and reuses them. Free slots in the Personal Dashboard.
"""
import argparse, base64, json, os, platform, re, sys, uuid, zlib
import urllib.parse as up
import urllib.request, urllib.error

DEFAULT_GATEWAY = "http://gw.amnezia.org:80/"
STATE_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "state.json")   # next to the script, gitignored
APP_VERSION = "5.1.1.10"     # reported to the gateway like the official client
CLI_NAME = "AmneziaVPN"


# ---------------------------------------------------------------- vpn:// keys
def decode_vpn(key: str) -> dict:
    s = key.strip()
    if s.startswith("vpn://"):
        s = s[6:]
    raw = base64.urlsafe_b64decode(s + "=" * (-len(s) % 4))
    # 4-byte prefix: Qt length header, or the 000000ff signature of subscription keys
    try:
        data = zlib.decompress(raw[4:])
    except zlib.error:
        data = zlib.decompress(raw)
    return json.loads(data)


# ------------------------------------------------------------------- gateway
def _crypto():
    try:
        from cryptography.hazmat.primitives import hashes, padding, serialization
        from cryptography.hazmat.primitives.asymmetric import padding as apad
        from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
        return hashes, padding, serialization, apad, Cipher, algorithms, modes
    except ImportError:
        sys.exit("Missing dependency: pip install cryptography")


def load_pubkey_pems(arg: str) -> list:
    """arg: path to PEM file (may hold several keys), a PEM string, or base64 of a PEM. Returns a list of PEMs."""
    data = open(arg, "rb").read() if os.path.isfile(arg) else arg.encode()
    if b"BEGIN PUBLIC KEY" not in data:
        data = base64.b64decode(data.strip())
    pems = [m.group(0) + b"\n" for m in re.finditer(rb"-----BEGIN PUBLIC KEY-----.*?-----END PUBLIC KEY-----", data, re.S)]
    if not pems:
        sys.exit("That does not look like an RSA public key PEM")
    return pems


def extract_pubkeys(binary_path: str) -> list:
    """The app embeds several gateway keys (production, sandbox, ...). Return all of them."""
    blob = open(binary_path, "rb").read()
    found = [m.group(0) for m in re.finditer(rb"-----BEGIN PUBLIC KEY-----.{100,900}?-----END PUBLIC KEY-----", blob, re.S)]
    for m in re.finditer(rb"LS0tLS1CRUdJTiBQVUJMSUMgS0VZ[A-Za-z0-9+/=]{100,1800}", blob):
        try:
            pem = base64.b64decode(m.group(0))
            if b"END PUBLIC KEY" in pem:
                found.append(pem.split(b"-----END PUBLIC KEY-----")[0] + b"-----END PUBLIC KEY-----")
        except Exception:
            pass
    out = []
    for f in found:
        f = f.strip() + b"\n"
        if f not in out:
            out.append(f)
    if not out:
        sys.exit("No public key found in that file. Try the app's main binary / library "
                 "(Windows: AmneziaVPN.exe; macOS: AmneziaVPN.app/Contents/MacOS/AmneziaVPN). "
                 "For an APK, unzip it and pass the lib*.so inside.")
    return out


class DecryptFail(Exception):
    pass


def build_envelope(payload: bytes, pem: bytes):
    hashes, padding, serialization, apad, Cipher, algorithms, modes = _crypto()
    pub = serialization.load_pem_public_key(pem)
    key, iv, salt = os.urandom(32), os.urandom(32), os.urandom(8)
    keys_json = json.dumps({
        "aes_key": base64.b64encode(key).decode(),
        "aes_iv": base64.b64encode(iv).decode(),     # 32 bytes sent, only first 16 used by CBC
        "aes_salt": base64.b64encode(salt).decode(),  # sent, unused
    }, separators=(",", ":")).encode()
    enc_keys = pub.encrypt(keys_json, apad.PKCS1v15())
    padder = padding.PKCS7(128).padder()
    enc = Cipher(algorithms.AES(key), modes.CBC(iv[:16])).encryptor()
    ct = enc.update(padder.update(payload) + padder.finalize()) + enc.finalize()
    body = json.dumps({"key_payload": base64.b64encode(enc_keys).decode(),
                       "api_payload": base64.b64encode(ct).decode()},
                      separators=(",", ":")).encode()
    return body, key, iv


def aes_decrypt(data: bytes, key: bytes, iv: bytes) -> bytes:
    hashes, padding, serialization, apad, Cipher, algorithms, modes = _crypto()
    dec = Cipher(algorithms.AES(key), modes.CBC(iv[:16])).decryptor()
    pt = dec.update(data) + dec.finalize()
    un = padding.PKCS7(128).unpadder()
    return un.update(pt) + un.finalize()


def gateway_post(gateway: str, path: str, payload: dict, pem: bytes, timeout=20) -> dict:
    body, key, iv = build_envelope(json.dumps(payload, separators=(",", ":")).encode(), pem)
    url = gateway.rstrip("/") + "/" + path.lstrip("/")
    req = urllib.request.Request(url, data=body, method="POST", headers={
        "Content-Type": "application/json", "X-Client-Request-ID": str(uuid.uuid4())})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            raw = r.read()
    except urllib.error.HTTPError as e:
        raw = e.read()
    except Exception as e:
        sys.exit("Gateway unreachable (%s). If the gateway is blocked where you are, route this "
                 "through a proxy/VPN or pass --gateway with a reachable mirror." % e)
    try:
        return json.loads(aes_decrypt(raw, key, iv))
    except Exception:
        raise DecryptFail("HTTP answer not decryptable: %r" % raw[:120])


# --------------------------------------------------------------------- state
def load_state() -> dict:
    try:
        return json.load(open(STATE_FILE))
    except Exception:
        return {}


def save_state(st: dict):
    with open(STATE_FILE, "w") as f:
        json.dump(st, f)
    try:
        os.chmod(STATE_FILE, 0o600)
    except OSError:
        pass


# ------------------------------------------------------ xray / vless helpers
def find_xray_config(vpn_cfg: dict) -> dict:
    for c in vpn_cfg.get("containers", []):
        lc = c.get("xray", {}).get("last_config") if c.get("container") == "amnezia-xray" else None
        if lc:
            return json.loads(lc) if isinstance(lc, str) else lc
    raise SystemExit("Gateway answered, but no amnezia-xray container is in it. Containers: %s"
                     % [c.get("container") for c in vpn_cfg.get("containers", [])])


def main_outbound(x: dict) -> dict:
    v = [o for o in x.get("outbounds", []) if o.get("protocol") == "vless"]
    o = next((o for o in v if o.get("tag") == "proxy"), v[0] if v else None)
    if not o:
        raise SystemExit("No vless outbound in the Xray config")
    return o


def outbound_to_link(o: dict, name="amnezia") -> str:
    v = o["settings"]["vnext"][0]; u = v["users"][0]; ss = o.get("streamSettings", {})
    q = {"encryption": u.get("encryption", "none"), "type": ss.get("network", "tcp"),
         "security": ss.get("security", "none")}
    if u.get("flow"):
        q["flow"] = u["flow"]
    r = ss.get("realitySettings")
    if r:
        q.update(sni=r.get("serverName", ""), fp=r.get("fingerprint", "chrome"),
                 pbk=r.get("publicKey", ""), sid=r.get("shortId", ""))
        if r.get("spiderX"):
            q["spx"] = r["spiderX"]
    return "vless://%s@%s:%s?%s#%s" % (u["id"], v["address"], v["port"], up.urlencode(q), up.quote(name))


def link_to_outbound(link: str, tag: str) -> dict:
    u = up.urlsplit(link)
    if u.scheme != "vless":
        raise SystemExit("--chain expects a vless:// link")
    q = dict(up.parse_qsl(u.query))
    net, sec = q.get("type", "tcp"), q.get("security", "none")
    ss = {"network": net, "security": sec}
    if sec == "reality":
        ss["realitySettings"] = {"serverName": q.get("sni", ""), "fingerprint": q.get("fp", "chrome"),
                                 "publicKey": q.get("pbk", ""), "shortId": q.get("sid", ""),
                                 "spiderX": q.get("spx", "")}
    elif sec == "tls":
        ss["tlsSettings"] = {"serverName": q.get("sni", u.hostname), "fingerprint": q.get("fp", ""),
                             **({"alpn": q["alpn"].split(",")} if q.get("alpn") else {})}
    if net == "ws":
        ss["wsSettings"] = {"path": q.get("path", "/"), "headers": {"Host": q.get("host", "")}}
    elif net == "grpc":
        ss["grpcSettings"] = {"serviceName": q.get("serviceName", ""), "multiMode": q.get("mode") == "multi"}
    elif net in ("xhttp", "splithttp"):
        ss["xhttpSettings"] = {"path": q.get("path", "/"), "host": q.get("host", ""), "mode": q.get("mode", "auto")}
    user = {"id": up.unquote(u.username), "encryption": q.get("encryption", "none")}
    if q.get("flow"):
        user["flow"] = q["flow"]
    return {"tag": tag, "protocol": "vless",
            "settings": {"vnext": [{"address": u.hostname, "port": u.port or 443, "users": [user]}]},
            "streamSettings": ss}


def build_config(x: dict, vps_link: str = None) -> dict:
    """Amnezia's config is itself a chain (proxy -> dialerProxy -> proxy-relay), so keep ALL its outbounds.
    With vps_link, your VPS is dialed THROUGH the Amnezia chain, so the exit IP stays your VPS."""
    outs = json.loads(json.dumps(x.get("outbounds", [])))
    main = main_outbound({"outbounds": outs})
    lst = [main] + [o for o in outs if o is not main]
    if vps_link:
        vps = link_to_outbound(vps_link, "hop-vps")
        vps["streamSettings"].setdefault("sockopt", {})["dialerProxy"] = main["tag"]
        lst = [vps] + lst
    if not any(o.get("tag") == "direct" for o in lst):
        lst.append({"tag": "direct", "protocol": "freedom"})
    tags = {o.get("tag") for o in lst}
    for o in lst:
        so = o.get("streamSettings", {}).get("sockopt", {})
        if so.get("dialerProxy") and so["dialerProxy"] not in tags:
            del so["dialerProxy"]
    return {"log": {"loglevel": "warning"},
            "inbounds": [
                {"tag": "socks-in", "port": 10808, "listen": "127.0.0.1", "protocol": "socks",
                 "settings": {"udp": True}, "sniffing": {"enabled": True, "destOverride": ["http", "tls"]}},
                {"tag": "http-in", "port": 10809, "listen": "127.0.0.1", "protocol": "http"}],
            "outbounds": lst}


# ---------------------------------------------------------------------- main
def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("key", nargs="?", help="vpn:// Subscription Key (or path to a file with it)")
    ap.add_argument("--pubkey", help="gateway RSA public key: PEM file/string or base64 PEM")
    ap.add_argument("--extract-pubkey", metavar="APP_BINARY", help="pull the gateway public key out of your installed app")
    ap.add_argument("--gateway", default=DEFAULT_GATEWAY, help="gateway base URL or reachable proxy (default %(default)s)")
    ap.add_argument("--country", help="server_country_code, e.g. de, nl (location); omit for default")
    ap.add_argument("--protocol", default="vless", help="service_protocol to request (default vless)")
    ap.add_argument("--chain", metavar="VLESS_LINK", help="your own VPS vless:// link as the exit, dialed through Amnezia")
    ap.add_argument("-o", "--out", help="output file (pubkey PEM or chain JSON)")
    a = ap.parse_args()

    if a.extract_pubkey:
        pem = b"".join(extract_pubkeys(a.extract_pubkey))
        if a.out:
            open(a.out, "wb").write(pem); print("wrote", a.out, "(%d key(s); the right one is picked automatically)" % pem.count(b"BEGIN PUBLIC KEY"))
        else:
            sys.stdout.write(pem.decode())
        return
    if not a.key or not a.pubkey:
        ap.error("need KEY and --pubkey (or use --extract-pubkey first)")

    key = open(a.key).read() if os.path.isfile(a.key) else a.key
    k = decode_vpn(key)
    api_cfg, auth = k.get("api_config", {}), k.get("auth_data", {})
    if not auth.get("api_key"):
        sys.exit("This key has no auth_data.api_key. Top-level keys: %s\n"
                 "If it already contains containers, use the offline decoder instead." % list(k))
    pems = load_pubkey_pems(a.pubkey)

    st = load_state()
    st.setdefault("installation_uuid", str(uuid.uuid4()))
    st.setdefault("xray_uuid", str(uuid.uuid4()))
    save_state(st)

    payload = {
        "os_version": platform.system().lower() or "linux",
        "app_version": APP_VERSION, "cli_name": CLI_NAME, "app_language": "en",
        "installation_uuid": st["installation_uuid"],
        "user_country_code": api_cfg.get("user_country_code", ""),
        "service_type": api_cfg.get("service_type", ""),
        "service_protocol": a.protocol,
        "public_key": st["xray_uuid"],          # for vless the "public key" is the client UUID
        "auth_data": {"api_key": auth["api_key"]},
    }
    if a.country:
        payload["server_country_code"] = a.country
    payload = {k2: v for k2, v in payload.items() if v not in ("", None)}

    resp, last = None, None
    for i, pem in enumerate(pems):
        try:
            resp = gateway_post(a.gateway, "v1/config", payload, pem)
            break
        except DecryptFail as e:
            last = e
            print("key #%d not accepted: %s" % (i + 1, e), file=sys.stderr)
    if resp is None:
        sys.exit("No gateway key worked (the app/APK may be old, or something blocked the reply).")
    if resp.get("http_status", 0) >= 400 or "config" not in resp:
        print("Gateway error / unexpected answer:", json.dumps(resp, indent=2, ensure_ascii=False), file=sys.stderr)
        if "captcha" in json.dumps(resp).lower():
            print("The gateway asked for a CAPTCHA; solve it once in the official app (connect there), then retry.", file=sys.stderr)
        sys.exit(1)

    x = find_xray_config(decode_vpn(resp["config"]))
    if a.chain:
        out = json.dumps(build_config(x, a.chain), indent=2)
    else:
        out = json.dumps(build_config(x), indent=2)
        print("# Main-hop link only (skips Amnezia's relay hop; prefer the JSON as a custom config):")
        print(outbound_to_link(main_outbound(x)))
        print("# Xray config JSON below / in -o file: import as CUSTOM CONFIG in v2rayN/v2rayNG\n")
    if a.out:
        open(a.out, "w").write(out); print("wrote", a.out)
    else:
        print(out)


if __name__ == "__main__":
    main()
