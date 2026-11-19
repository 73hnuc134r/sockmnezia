// Runs the page's JS core against tests/mock_gateway.py (libagw-compatible envelope).
const { spawn } = require('child_process'), fs = require('fs'), path = require('path'), os = require('os');
const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
const m = { exports: {} };
new Function('module', 'require', html.match(/<script>([\s\S]*)<\/script>/)[1])(m, require);
const c = m.exports;
(async () => {
  const decoy = require('crypto').generateKeyPairSync('rsa', { modulusLength: 2048 }).publicKey.export({ type: 'spki', format: 'pem' });
  const pemFile = path.join(os.tmpdir(), 'mock_gw.pem'), port = 18765;
  const srv = spawn('python3', ['-I', path.join(__dirname, 'mock_gateway.py'), String(port), pemFile], { stdio: 'inherit' });
  await new Promise(r => setTimeout(r, 1200));
  try {
    // a Premium-style subscription key: 000000ff + zlib(JSON)
    const zlib = require('zlib');
    const keyObj = { name: 'Amnezia Premium', config_version: 1, api_config: { service_type: 'amnezia-premium', service_protocol: 'awg', user_country_code: 'ru' }, auth_data: { api_key: 'SECRET' } };
    const key = 'vpn://' + Buffer.concat([Buffer.from('000000ff', 'hex'), zlib.deflateSync(JSON.stringify(keyObj))]).toString('base64url');
    const uuid = '11111111-2222-3333-4444-555555555555';
    const o = await c.fetchXrayConfig({ key, pem: decoy + fs.readFileSync(pemFile, 'utf8'), gateway: `http://127.0.0.1:${port}/`, country: 'de', protocol: 'vless', installationUuid: crypto.randomUUID(), xrayUuid: uuid });
    const link = c.outboundToLink(c.mainOutbound(o), 'amnezia');
    if (!link.startsWith(`vless://${uuid}@203.0.113.7:443?`)) throw new Error('bad link ' + link);
    console.log('fetch+decode OK:', link.slice(0, 64) + '…');
    const ch = c.buildConfig(o, 'vless://aaaa@198.51.100.9:443?type=tcp&security=reality&sni=x.com&fp=chrome&pbk=K&sid=1#vps');
    const tags = ch.outbounds.map(x => x.tag).slice(0, 3).join();
    if (tags !== 'hop-vps,proxy,proxy-relay' || ch.outbounds[0].streamSettings.sockopt.dialerProxy !== 'proxy' || ch.outbounds[1].streamSettings.sockopt.dialerProxy !== 'proxy-relay') throw new Error('chain ' + tags);
    console.log('chain OK');
  } finally { srv.kill(); }
})().catch(e => { console.error('FAIL', e); process.exit(1); });
