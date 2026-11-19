"""Offline test: runs fetch_vless.py against a local mock gateway that implements
the libagw envelope (RSA PKCS#1 v1.5 + AES-256-CBC, 32-byte IV / first 16 used)."""
import base64, json, os, subprocess, sys, tempfile, threading, zlib, struct
from http.server import BaseHTTPRequestHandler, HTTPServer
from cryptography.hazmat.primitives import padding, serialization
from cryptography.hazmat.primitives.asymmetric import rsa, padding as apad
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "..", "fetch_vless.py")
priv = rsa.generate_private_key(public_exponent=65537, key_size=2048)
PEM = priv.public_key().public_bytes(serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo)
seen = {}

def vpn_encode(obj, sig=b"\x00\x00\x00\xff"):
    comp = zlib.compress(json.dumps(obj).encode(), 6)
    return "vpn://" + base64.urlsafe_b64encode(sig + comp).decode().rstrip("=")

XRAY = {"outbounds": [{"protocol": "vless", "tag": "proxy-relay", "settings": {"vnext": [{"address": "198.51.100.77", "port": 443, "users": [{"id": "r-uuid", "encryption": "none"}]}]}, "streamSettings": {"network": "xhttp", "security": "reality", "realitySettings": {"serverName": "", "fingerprint": "qq", "publicKey": "PBK2", "shortId": ""}, "xhttpSettings": {"xmux": {"maxConnections": 4}}}}, {"tag": "dns-out", "protocol": "dns"}, {"protocol": "vless", "tag": "proxy", "settings": {"vnext": [{"address": "203.0.113.7", "port": 443,
        "users": [{"id": "UUID_PLACEHOLDER", "flow": "xtls-rprx-vision", "encryption": "none"}]}]},
        "streamSettings": {"network": "tcp", "security": "reality", "sockopt": {"dialerProxy": "proxy-relay"}, "realitySettings": {
            "serverName": "www.example.com", "fingerprint": "chrome", "publicKey": "PBK", "shortId": "ab"}}}]}

class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        try:
            self._post()
        except Exception:
            self.send_response(500); self.end_headers(); self.wfile.write(b'{}')

    def _post(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        keys = json.loads(priv.decrypt(base64.b64decode(body["key_payload"]), apad.PKCS1v15()))
        key, iv = base64.b64decode(keys["aes_key"]), base64.b64decode(keys["aes_iv"])
        assert len(key) == 32 and len(iv) == 32 and len(base64.b64decode(keys["aes_salt"])) == 8
        d = Cipher(algorithms.AES(key), modes.CBC(iv[:16])).decryptor()
        pt = d.update(base64.b64decode(body["api_payload"])) + d.finalize()
        u = padding.PKCS7(128).unpadder()
        req = json.loads(u.update(pt) + u.finalize())
        seen["path"], seen["req"] = self.path, req
        x = json.loads(json.dumps(XRAY)); [o for o in x["outbounds"] if o["tag"] == "proxy"][0]["settings"]["vnext"][0]["users"][0]["id"] = req["public_key"]
        cfg = {"containers": [{"container": "amnezia-xray", "xray": {"last_config": json.dumps(x)}}]}
        resp = json.dumps({"config": vpn_encode(cfg, struct.pack(">I", 1)), "http_status": 201}).encode()
        p = padding.PKCS7(128).padder(); e = Cipher(algorithms.AES(key), modes.CBC(iv[:16])).encryptor()
        out = e.update(p.update(resp) + p.finalize()) + e.finalize()
        self.send_response(200); self.end_headers(); self.wfile.write(out)

srv = HTTPServer(("127.0.0.1", 0), H); port = srv.server_address[1]
threading.Thread(target=srv.serve_forever, daemon=True).start()

with tempfile.TemporaryDirectory() as td:
    home = {**os.environ, "HOME": td, "USERPROFILE": td}
    DECOY = rsa.generate_private_key(public_exponent=65537, key_size=2048).public_key().public_bytes(serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo)
    pem_path = os.path.join(td, "gw.pem"); open(pem_path, "wb").write(DECOY + PEM)
    key = vpn_encode({"name": "Amnezia Premium", "config_version": 1,
                      "api_config": {"service_type": "amnezia-premium", "service_protocol": "awg", "user_country_code": "ru"},
                      "auth_data": {"api_key": "SECRET-API-KEY"}})
    import shutil; shutil.copy(SCRIPT, td + "/fetch_vless.py")   # state.json is written next to the script
    run = lambda *a: subprocess.run([sys.executable, "-I", td + "/fetch_vless.py", *a], capture_output=True, text=True, env=home)

    # pubkey extraction from a fake binary (base64 PEM embedded)
    fake = os.path.join(td, "app.bin")
    open(fake, "wb").write(os.urandom(500) + base64.b64encode(DECOY) + b"\x00" + base64.b64encode(PEM) + b"\x00" + os.urandom(500))
    r = run("--extract-pubkey", fake); assert r.stdout.count("BEGIN PUBLIC KEY") == 2, r
    print("extract-pubkey: OK")

    r = run(key, "--pubkey", pem_path, "--gateway", f"http://127.0.0.1:{port}/", "--country", "de")
    assert r.returncode == 0, r.stderr
    assert r.stdout.split("\n")[1].startswith("vless://"), r.stdout
    base = json.loads(r.stdout[r.stdout.index("{"):])
    assert [o["tag"] for o in base["outbounds"][:3]] == ["proxy", "proxy-relay", "dns-out"], base["outbounds"]
    req = seen["req"]
    assert seen["path"] == "/v1/config" and req["service_protocol"] == "vless"
    assert req["auth_data"]["api_key"] == "SECRET-API-KEY" and req["service_type"] == "amnezia-premium"
    assert req["server_country_code"] == "de" and req["user_country_code"] == "ru" and req["public_key"] in r.stdout
    print("fetch + decode:", r.stdout.split("\n")[1][:70], "...")

    r2 = run(key, "--pubkey", pem_path, "--gateway", f"http://127.0.0.1:{port}/",
             "--chain", "vless://aaaa@198.51.100.9:443?type=tcp&security=reality&sni=x.com&fp=chrome&pbk=K&sid=1#vps")
    ch = json.loads(r2.stdout)
    tags = [o["tag"] for o in ch["outbounds"]]
    assert tags[:3] == ["hop-vps", "proxy", "proxy-relay"], tags
    assert ch["outbounds"][0]["streamSettings"]["sockopt"]["dialerProxy"] == "proxy"
    assert ch["outbounds"][1]["streamSettings"]["sockopt"]["dialerProxy"] == "proxy-relay"     # relay chain kept
    assert all(o.get("streamSettings", {}).get("sockopt", {}).get("dialerProxy", "proxy") in tags for o in ch["outbounds"])
    assert req["public_key"] == seen["req"]["public_key"]   # state reused (same uuid)
    print("chain: OK; state reused: OK")
