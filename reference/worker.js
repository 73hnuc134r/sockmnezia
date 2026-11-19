// Cloudflare Worker: tiny CORS proxy for the Amnezia gateway, so index.html can call it from a browser.
// The gateway is plain HTTP and sends no CORS headers; this adds them. The proxy only ever sees the
// RSA/AES-encrypted envelope (the AES key never leaves your browser), so it can't read your key or the answer.
//
// Deploy: Workers & Pages -> Create Worker -> paste this file. Optional vars:
//   ALLOW_ORIGIN = https://your-page.example   (default *)
//   GATEWAY      = http://gw.amnezia.org:80    (default)
export default {
  async fetch(req, env) {
    const cors = {
      'Access-Control-Allow-Origin': env.ALLOW_ORIGIN || '*',
      'Access-Control-Allow-Methods': 'POST, OPTIONS',
      'Access-Control-Allow-Headers': 'content-type, x-client-request-id',
    };
    if (req.method === 'OPTIONS') return new Response(null, { headers: cors });
    const { pathname } = new URL(req.url);
    if (req.method !== 'POST' || !/^\/v1\/(config|services|account_info)$/.test(pathname)) {
      return new Response('not found', { status: 404, headers: cors });
    }
    const upstream = await fetch((env.GATEWAY || 'http://gw.amnezia.org:80') + pathname, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Client-Request-ID': req.headers.get('x-client-request-id') || crypto.randomUUID(),
      },
      body: req.body,
    });
    return new Response(upstream.body, {
      status: upstream.status,
      headers: { ...cors, 'Content-Type': 'application/octet-stream' },
    });
  },
};
