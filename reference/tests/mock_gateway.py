"""Standalone mock gateway (same envelope as libagw). Usage: mock_gateway.py PORT PEM_OUT
Implements
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
            self.send_response(500); self.end_headers(); self.wfile.write(b'{"error":"decrypt failed"}')

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



if __name__ == "__main__":
    port, pem_out = int(sys.argv[1]), sys.argv[2]
    open(pem_out, "wb").write(PEM)
    HTTPServer(("127.0.0.1", port), H).serve_forever()
