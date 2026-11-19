#!/usr/bin/env node
// Interactive Amnezia Premium -> VLESS fetcher. Zero dependencies: just Node.js (Termux: pkg install nodejs).
//
//   pkg install -y nodejs curl && curl -fsSLo ~/amz.js <raw-url-of-this-file> && node ~/amz.js
//
// Tip: turn the Amnezia VPN on first if the gateway (gw.amnezia.org) is blocked on your network.
// Optional flags (skip the prompts): --key vpn://.. --pubkey FILE|PEM|B64 --apk FILE --gateway URL
//                                    --country de --chain "vless://.." --out chain.json
'use strict';
const fs = require('fs'), os = require('os'), path = require('path'), zlib = require('zlib'), crypto = require('crypto');
const readline = require('readline/promises');

const DEFAULT_GATEWAY = 'http://gw.amnezia.org:80/';
// Everything lives next to this script (state.json, gateway.pem, amnezia.json, chain.json); all gitignored.
// ~/storage/downloads is only ever READ, to find the APK.
const DIR = __dirname, STATE = path.join(DIR, 'state.json'), PEMFILE = path.join(DIR, 'gateway.pem');
const LEGACY = path.join(os.homedir(), '.amnezia-vless');
for (const f of ['state.json', 'gateway.pem']) {            // one-time move from the old hidden folder
  const old = path.join(LEGACY, f), now = path.join(DIR, f);
  if (fs.existsSync(old) && !fs.existsSync(now)) { try { fs.copyFileSync(old, now); fs.unlinkSync(old); } catch (e) {} }
}
try { fs.rmdirSync(LEGACY); } catch (e) {}

// ---- args
const args = {}; for (let i = 2; i < process.argv.length; i++) if (process.argv[i].startsWith('--')) args[process.argv[i].slice(2)] = process.argv[++i];
const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
const ask = async (q, def) => { const a = (await rl.question(q + (def ? ` [${def}]` : '') + ': ')).trim(); return a || def || ''; };
const say = s => console.log(s);

// ---- vpn:// decoding
function decodeVpn(key) {
  let s = key.trim().replace(/\s+/g, ''); if (s.startsWith('vpn://')) s = s.slice(6);
  const raw = Buffer.from(s, 'base64url');
  try { return JSON.parse(zlib.inflateSync(raw.subarray(4)).toString()); } catch (e) { return JSON.parse(zlib.inflateSync(raw).toString()); }
}

// ---- gateway public key: pull it out of an installed app / APK / binary
function* zipEntries(buf) {
  let i = buf.length - 22; while (i >= 0 && buf.readUInt32LE(i) !== 0x06054b50) i--;
  if (i < 0) return;
  const n = buf.readUInt16LE(i + 10); let off = buf.readUInt32LE(i + 16);
  for (let k = 0; k < n && buf.readUInt32LE(off) === 0x02014b50; k++) {
    const method = buf.readUInt16LE(off + 10), csize = buf.readUInt32LE(off + 20), nlen = buf.readUInt16LE(off + 28),
          elen = buf.readUInt16LE(off + 30), clen = buf.readUInt16LE(off + 32), lho = buf.readUInt32LE(off + 42);
    yield { name: buf.toString('utf8', off + 46, off + 46 + nlen), method, csize, lho };
    off += 46 + nlen + elen + clen;
  }
}
function entryData(buf, e) {
  const s = e.lho + 30 + buf.readUInt16LE(e.lho + 26) + buf.readUInt16LE(e.lho + 28), d = buf.subarray(s, s + e.csize);
  return e.method === 0 ? d : zlib.inflateRawSync(d);
}
function scanForPems(buf) {
  const s = buf.toString('latin1'), found = [];
  for (const m of s.matchAll(/-----BEGIN PUBLIC KEY-----[A-Za-z0-9+/=\s]{100,900}?-----END PUBLIC KEY-----/g)) found.push(m[0]);
  for (const mm of s.matchAll(/LS0tLS1CRUdJTiBQVUJMSUMgS0VZ[A-Za-z0-9+/=]{100,1800}/g)) {
    try { const pem = Buffer.from(mm[0], 'base64').toString('latin1'); const e = pem.indexOf('-----END PUBLIC KEY-----');
      if (e > 0) found.push(pem.slice(0, e + 24) + '\n'); } catch (e) {}
  }
  return found;
}
// The app embeds several gateway keys (production, sandbox, ...). Return every valid RSA one; the caller tries them.
function extractPubkeys(file) {
  const buf = fs.readFileSync(file), out = [], seen = new Set();
  const add = list => { for (const t of list) { try { const k = crypto.createPublicKey(t); if (k.asymmetricKeyType !== 'rsa') continue;
      const fp = fingerprint(t); if (!seen.has(fp)) { seen.add(fp); out.push(t); } } catch (e) {} } };
  if (buf.readUInt16LE(0) === 0x4b50) {                       // zip / APK
    const es = [...zipEntries(buf)].filter(e => /\.so$|\.dll$|\.exe$/i.test(e.name));
    es.sort((a, b) => (/amnezia/i.test(b.name) - /amnezia/i.test(a.name)) || (/arm64/i.test(b.name) - /arm64/i.test(a.name)));
    for (const e of es) { try { add(scanForPems(entryData(buf, e))); } catch (e2) {} if (out.length) break; }
  } else add(scanForPems(buf));
  return out;
}
const fingerprint = t => crypto.createHash('sha256').update(crypto.createPublicKey(t).export({ type: 'spki', format: 'der' })).digest('hex').slice(0, 8);
function normalizePem(x) {
  let t = fs.existsSync(x) ? fs.readFileSync(x, 'latin1') : x;
  if (!t.includes('BEGIN PUBLIC KEY')) t = Buffer.from(t.trim(), 'base64').toString('latin1');
  if (!t.includes('BEGIN PUBLIC KEY')) throw new Error('not a public key PEM');
  crypto.createPublicKey(t);                                   // validates
  return t;
}

class DecryptFail extends Error { constructor(status, preview) { super('HTTP ' + status + ' ' + preview); this.status = status; this.preview = preview; } }
// ---- gateway envelope (libagw): RSA PKCS#1 v1.5 + AES-256-CBC (iv: 32 bytes sent, first 16 used)
async function gatewayPost(base, route, payload, pem) {
  const key = crypto.randomBytes(32), iv = crypto.randomBytes(32), salt = crypto.randomBytes(8);
  const keys = JSON.stringify({ aes_key: key.toString('base64'), aes_iv: iv.toString('base64'), aes_salt: salt.toString('base64') });
  const encKeys = crypto.publicEncrypt({ key: pem, padding: crypto.constants.RSA_PKCS1_PADDING }, Buffer.from(keys));
  const c = crypto.createCipheriv('aes-256-cbc', key, iv.subarray(0, 16));
  const ct = Buffer.concat([c.update(JSON.stringify(payload)), c.final()]);
  const body = JSON.stringify({ key_payload: encKeys.toString('base64'), api_payload: ct.toString('base64') });
  let res;
  try { res = await fetch(base.replace(/\/+$/, '') + '/' + route, { method: 'POST', body, signal: AbortSignal.timeout(20000),
          headers: { 'Content-Type': 'application/json', 'X-Client-Request-ID': crypto.randomUUID() } }); }
  catch (e) { throw new Error('Gateway unreachable (' + (e.cause?.code || e.message) + '). Turn the Amnezia VPN ON (or any working VPN) and retry.'); }
  const raw = Buffer.from(await res.arrayBuffer());
  try { const d = crypto.createDecipheriv('aes-256-cbc', key, iv.subarray(0, 16)); return JSON.parse(Buffer.concat([d.update(raw), d.final()]).toString()); }
  catch (e) { throw new DecryptFail(res.status, JSON.stringify(raw.toString('latin1').replace(/[^\x20-\x7e]/g, '.').slice(0, 160))); }
}

// ---- xray / vless
function findXrayConfig(cfg) {
  for (const c of cfg.containers || []) if (c.container === 'amnezia-xray' && c.xray?.last_config)
    return typeof c.xray.last_config === 'string' ? JSON.parse(c.xray.last_config) : c.xray.last_config;
  throw new Error('No amnezia-xray container in the answer. Containers: ' + (cfg.containers || []).map(c => c.container));
}
function mainOutbound(x) {
  const v = (x.outbounds || []).filter(o => o.protocol === 'vless');
  const o = v.find(o => o.tag === 'proxy') || v[0];
  if (!o) throw new Error('No vless outbound in the Xray config');
  return o;
}
function outboundToLink(o, name = 'amnezia') {
  const v = o.settings.vnext[0], u = v.users[0], ss = o.streamSettings || {}, q = new URLSearchParams();
  q.set('encryption', u.encryption || 'none'); q.set('type', ss.network || 'tcp'); q.set('security', ss.security || 'none');
  if (u.flow) q.set('flow', u.flow);
  const r = ss.realitySettings;
  if (r) { q.set('sni', r.serverName || ''); q.set('fp', r.fingerprint || 'chrome'); q.set('pbk', r.publicKey || ''); q.set('sid', r.shortId || ''); if (r.spiderX) q.set('spx', r.spiderX); }
  return `vless://${u.id}@${v.address}:${v.port}?${q}#${encodeURIComponent(name)}`;
}
function linkToOutbound(link, tag) {
  const u = new URL(link.trim()); if (u.protocol !== 'vless:') throw new Error('Expected a vless:// link');
  const q = Object.fromEntries(u.searchParams), net = q.type || 'tcp', sec = q.security || 'none', ss = { network: net, security: sec };
  if (sec === 'reality') ss.realitySettings = { serverName: q.sni || '', fingerprint: q.fp || 'chrome', publicKey: q.pbk || '', shortId: q.sid || '', spiderX: q.spx || '' };
  else if (sec === 'tls') { ss.tlsSettings = { serverName: q.sni || u.hostname, fingerprint: q.fp || '' }; if (q.alpn) ss.tlsSettings.alpn = q.alpn.split(','); }
  if (net === 'ws') ss.wsSettings = { path: q.path || '/', headers: { Host: q.host || '' } };
  else if (net === 'grpc') ss.grpcSettings = { serviceName: q.serviceName || '', multiMode: q.mode === 'multi' };
  else if (net === 'xhttp' || net === 'splithttp') ss.xhttpSettings = { path: q.path || '/', host: q.host || '', mode: q.mode || 'auto' };
  const user = { id: decodeURIComponent(u.username), encryption: q.encryption || 'none' }; if (q.flow) user.flow = q.flow;
  return { tag, protocol: 'vless', settings: { vnext: [{ address: u.hostname, port: parseInt(u.port || '443', 10), users: [user] }] }, streamSettings: ss };
}
// Amnezia's config is itself a chain (proxy -> dialerProxy -> proxy-relay), so keep ALL its outbounds.
// With vpsLink: your VPS is dialed THROUGH the Amnezia chain, so your exit IP stays the VPS.
function buildConfig(x, vpsLink) {
  const outs = JSON.parse(JSON.stringify(x.outbounds || [])), main = mainOutbound({ outbounds: outs });
  let list = [main, ...outs.filter(o => o !== main)];
  if (vpsLink) {
    const vps = linkToOutbound(vpsLink, 'hop-vps');
    vps.streamSettings.sockopt = { ...(vps.streamSettings.sockopt || {}), dialerProxy: main.tag };
    list = [vps, ...list];
  }
  if (!list.some(o => o.tag === 'direct')) list.push({ tag: 'direct', protocol: 'freedom' });
  const tags = new Set(list.map(o => o.tag));
  for (const o of list) { const d = o.streamSettings?.sockopt?.dialerProxy; if (d && !tags.has(d)) delete o.streamSettings.sockopt.dialerProxy; }
  return { log: { loglevel: 'warning' },
    inbounds: [{ tag: 'socks-in', port: 10808, listen: '127.0.0.1', protocol: 'socks', settings: { udp: true }, sniffing: { enabled: true, destOverride: ['http', 'tls'] } },
               { tag: 'http-in', port: 10809, listen: '127.0.0.1', protocol: 'http' }],
    outbounds: list };
}

// ---- main
async function main() {
  say('\n== Amnezia Premium -> VLESS ==\n');

  // 1. subscription key
  const keyIn = args.key || await ask('1) Paste your Amnezia Subscription Key (vpn://...)');
  const k = decodeVpn(fs.existsSync(keyIn) ? fs.readFileSync(keyIn, 'utf8') : keyIn);
  const api = k.api_config || {}, auth = k.auth_data || {};
  if (!auth.api_key) throw new Error('This key has no auth_data.api_key. Top-level keys: ' + Object.keys(k));
  say('   key OK (service: ' + (api.service_type || '?') + ')');

  // 2. gateway public key candidates: saved one, or every RSA key found in the app
  const country = args.country ?? (args.key ? '' : null);
  let countryAns = country;
  const gateway = args.gateway || DEFAULT_GATEWAY;
  let st = {}; try { st = JSON.parse(fs.readFileSync(STATE, 'utf8')); } catch (e) {}
  st.installation_uuid ||= crypto.randomUUID(); st.xray_uuid ||= crypto.randomUUID();
  fs.writeFileSync(STATE, JSON.stringify(st), { mode: 0o600 });

  const getCandidates = async (fromSaved) => {
    if (args.pubkey) return [normalizePem(args.pubkey)];
    if (fromSaved && fs.existsSync(PEMFILE)) { say('2) Using saved gateway key (' + PEMFILE + ')'); return [normalizePem(PEMFILE)]; }
    say('\n2) I need Amnezia\'s gateway public key. It is built into the official app.');
    say('   Download the LATEST Amnezia APK (amnezia.org or github.com/amnezia-vpn/amnezia-client/releases) to your');
    say('   Downloads folder (run `termux-setup-storage` once so I can read it), then give me its path.');
    for (;;) {
      const p = args.apk || await ask('   Path to APK/binary, or paste the key (PEM / base64), Enter = ~/storage/downloads', '');
      try {
        let src = p;
        if (!p || (fs.existsSync(p) && fs.statSync(p).isDirectory())) {
          const d = p || path.join(os.homedir(), 'storage', 'downloads');
          const apks = fs.existsSync(d) ? fs.readdirSync(d).filter(f => /amnezia.*\.apk$/i.test(f)) : [];
          if (!apks.length) throw new Error('no Amnezia*.apk found in ' + d);
          src = path.join(d, apks.sort().pop()); say('   found ' + src);
        }
        if (fs.existsSync(src)) {
          say('   scanning (can take a minute)...'); const c = extractPubkeys(src);
          if (!c.length) throw new Error('no public key found in that file');
          say('   found ' + c.length + ' candidate key(s): ' + c.map(fingerprint).join(', ')); return c;
        }
        return [normalizePem(src)];
      } catch (e) { say('   ! ' + e.message); if (args.apk) throw e; }
    }
  };

  let resp, pem, fromSaved = true;
  for (;;) {
    const cands = await getCandidates(fromSaved);
    if (countryAns === null) countryAns = await ask('3) Server location code, e.g. de, nl, us (Enter = default)', '');
    say('\nContacting gateway...');
    const payload = { os_version: 'android', app_version: '5.1.1.10', cli_name: 'AmneziaVPN', app_language: 'en',
      installation_uuid: st.installation_uuid, user_country_code: api.user_country_code, service_type: api.service_type,
      service_protocol: 'vless', public_key: st.xray_uuid, auth_data: { api_key: auth.api_key } };
    if (countryAns) payload.server_country_code = countryAns;
    for (const x of Object.keys(payload)) if (!payload[x]) delete payload[x];
    let lastFail;
    for (const [i, cand] of cands.entries()) {
      try { resp = await gatewayPost(gateway, 'v1/config', payload, cand); pem = cand; break; }
      catch (e) { if (!(e instanceof DecryptFail)) throw e; lastFail = e; say(`   key #${i + 1} (${fingerprint(cand)}) not accepted: ${e.message}`); }
    }
    if (pem) break;
    if (args.pubkey || args.apk) throw new Error('No candidate key worked. Last answer: ' + lastFail.message);
    say('\n   None of those keys worked (the APK may be old, or the gateway answered with an error). Try the newest APK.');
    try { fs.unlinkSync(PEMFILE); } catch (e) {}
    fromSaved = false;
  }
  fs.writeFileSync(PEMFILE, pem); say('   gateway key ' + fingerprint(pem) + ' works; saved to ' + PEMFILE);
  if (resp.http_status >= 400 || !resp.config) {
    throw new Error('Gateway answer: ' + JSON.stringify(resp) + (/captcha/i.test(JSON.stringify(resp)) ? '\nIt wants a CAPTCHA: connect once in the official Amnezia app, then retry.' : ''));
  }
  const x = findXrayConfig(decodeVpn(resp.config));
  const link = outboundToLink(mainOutbound(x));
  const amz = path.join(DIR, 'amnezia.json');
  fs.writeFileSync(amz, JSON.stringify(buildConfig(x), null, 2));
  say('\n== Amnezia Xray config written to: ' + amz + ' ==');
  say('Import it as a CUSTOM CONFIG in v2rayNG/v2rayN (not as a link): Amnezia routes through a relay hop,');
  say('which a single vless:// link cannot express. SOCKS 127.0.0.1:10808, HTTP 10809.');
  say('\n(Main-hop link only, it skips the relay, so use the JSON above unless you know you need this:)\n' + link + '\n');

  // 4. optional chain: your VPS as the exit, dialed through Amnezia
  const vps = args.chain ?? await ask('4) Your own VPS vless:// link to use as the EXIT through Amnezia (Enter = skip)', '');
  if (vps) {
    const out = args.out || path.join(DIR, 'chain.json');
    fs.writeFileSync(out, JSON.stringify(buildConfig(x, vps), null, 2));
    say('\nWrote chain config: ' + out + '\n(your VPS is dialed through Amnezia, so your exit IP stays the VPS; import as a custom config)');
  }
  say('\nDone. state.json (device identity, reused so no extra slots are used) and gateway.pem are in ' + DIR);
}
main().then(() => rl.close(), e => { console.error('\nERROR: ' + e.message); rl.close(); process.exit(1); });
