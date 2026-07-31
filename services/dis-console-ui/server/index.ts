// Production server for dis-console-ui (Bun). It serves the built SPA from
// ../dist and, when DIS_CONSOLE_API is set, proxies /api to the dis-console
// fleet API on the same origin — so the browser needs no CORS, and this is the
// natural place to add auth later. In dev we still use the Vite dev server
// (HMR); this server is the production/preview equivalent of what nginx did.

const distDir = `${import.meta.dir}/../dist`;
const port = Number(Bun.env.PORT ?? 8080);
const apiTarget = Bun.env.DIS_CONSOLE_API?.replace(/\/$/, '');

const server = Bun.serve({
  port,
  async fetch(req) {
    const url = new URL(req.url);

    if (url.pathname.includes('..')) {
      return new Response('Bad Request', { status: 400 });
    }

    // Runtime config: the data mode is decided when the container runs, not
    // when the image builds — live whenever an API backend is configured,
    // mock otherwise; DIS_CONSOLE_USE_MOCK overrides explicitly. The built
    // dist/ carries a fallthrough config.js; this route shadows it.
    if (url.pathname === '/config.js') {
      const useMock = Bun.env.DIS_CONSOLE_USE_MOCK
        ? Bun.env.DIS_CONSOLE_USE_MOCK === 'true'
        : !apiTarget;
      return new Response(`window.__DIS_CONSOLE__ = { useMock: ${useMock} };\n`, {
        headers: { 'Content-Type': 'application/javascript', 'Cache-Control': 'no-store' },
      });
    }

    // Same-origin API proxy (no CORS); only when a backend is configured.
    if (url.pathname.startsWith('/api')) {
      if (!apiTarget) {
        return new Response('No API backend configured (set DIS_CONSOLE_API)', { status: 502 });
      }
      const headers = new Headers(req.headers);
      headers.delete('host');
      const init: RequestInit & { duplex?: 'half' } = {
        method: req.method,
        headers,
        redirect: 'manual',
        signal: AbortSignal.timeout(30_000),
      };
      if (req.body) {
        init.body = req.body;
        init.duplex = 'half';
      }
      try {
        return await fetch(`${apiTarget}${url.pathname}${url.search}`, init);
      } catch (err) {
        const timedOut = err instanceof DOMException && err.name === 'TimeoutError';
        return new Response(timedOut ? 'Upstream API timed out' : 'Upstream API unreachable', {
          status: timedOut ? 504 : 502,
        });
      }
    }

    // Static assets from the Vite build.
    const rel = url.pathname === '/' ? '/index.html' : url.pathname;
    const asset = Bun.file(`${distDir}${rel}`);
    if (await asset.exists()) {
      const headers = rel.startsWith('/assets/')
        ? { 'Cache-Control': 'public, max-age=31536000, immutable' }
        : undefined;
      return new Response(asset, headers ? { headers } : undefined);
    }

    // SPA fallback.
    return new Response(Bun.file(`${distDir}/index.html`), {
      headers: { 'Content-Type': 'text/html' },
    });
  },
});

console.log(
  `dis-console-ui on :${server.port}` +
    (apiTarget ? ` · proxying /api -> ${apiTarget}` : ' · static only (no API backend)'),
);
