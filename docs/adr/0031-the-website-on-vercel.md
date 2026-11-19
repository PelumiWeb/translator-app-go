# ADR 0031: The website on Vercel

Status: accepted, 2026-11-19

## Context

ADR 0030 had the Azure machine's Caddy serve the website beside the API,
shipped by hand with `make web-ship`. The same day, before it was used, the
owner chose to host the site on Vercel instead.

The site is plain files (ADR 0028) except for one thing: the waitlist form
posts to `/v1/waitlist` on the name that served the page, which has to reach
Shinar's server.

## Decision

**A Vercel project made from the repository**, with its Root Directory set
to `web`. Vercel finds Next.js, builds the site on each push to `main`, and
serves `out/`. Its address is the project's `vercel.app` name until there is
a domain, which then points at Vercel.

**`web/vercel.json` passes `/v1/waitlist` on to the server**
(`https://shinar-api.southafricanorth.cloudapp.azure.com/v1/waitlist`), as
an external rewrite: Vercel forwards the request and hands back the server's
answer. To the browser it is the page's own address, so there is still no
CORS and no server address in the page. It is the only path passed on.

**The Azure machine is the API only again.** `deploy/Caddyfile` passes every
request to the server, as before ADR 0030. `deploy/ship-web.sh`,
`deploy/Caddyfile.site`, `make web-ship` and the domain argument of
`deploy/setup.sh` are gone. `make web-preview` stays: it does on the Mac
what `vercel.json` does on Vercel.

## Alternatives considered

**Caddy on the Azure machine** (ADR 0030). Free and already set up, but
shipped by hand, and with no address worth sharing until a domain is bought.

**The page posting straight to the server, with CORS.** No rewrite, but the
page would carry the server's address, and the server would need to allow
each address the site is served from, previews included.

## Consequences

- **Vercel's free plan, Hobby, is for non-commercial, personal use only.**
  Shinar is free during the soft launch but is a product meant to be paid
  for later (calls, milestone 17). Whether Hobby fits until then, or Pro
  ($20 a month) is needed, is the owner's to decide.
- The server sees each sign-up come from Vercel, not from the visitor. Today
  nothing uses the visitor's address; a limit by address (ADR 0030) would
  read the header Vercel adds for it.
- The waitlist form's email passes through Vercel on its way to the server,
  as it would through any host. Vercel keeps nothing of the list.
- If the server moves to another name, `vercel.json` changes with it.
- The machine still has ADR 0030's Caddy file until `deploy/setup.sh` is run
  again. It answers the API the same way, and the rest with nothing.
