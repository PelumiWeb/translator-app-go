# ADR 0028: The website, in Next.js, exported as plain files

Status: accepted, 2026-11-18

## Context

Shinar has no website. It will need one before it goes public (milestone 18):
the App Store and Google Play ask for a privacy policy, terms and a support
page, and people who hear about the app need somewhere to see what it does.

On 2026-11-18 the owner asked for a motion piece showing a voice note's
journey from one person to another, and for the same idea on a website. The
design brief for both is `docs/design/brief-4-motion.md`. The owner then
asked for the website to be built, in Next.js.

The site needs no server of its own. Everything on it is the same for every
visitor, and nothing is sent from it.

## Decision

**A Next.js project in `web/`,** beside `android/`, `ios/` and `server/`.
App Router, TypeScript, React 19. Commands: `make web-dev`, `make web-build`,
`make web-preview`.

**Exported as plain files.** `output: "export"` makes `next build` write
HTML, CSS and JavaScript into `web/out/`, which any web server can serve.
There is no Node process to run in production. The Azure machine's Caddy
(ADR 0023) can serve the folder once there is a domain, and so can any
static host.

**Few dependencies.** `next`, `react` and `react-dom`, and for development
`typescript` (5.9, the major version Next's own starter uses; 7 is the new
native compiler, which Next does not use yet) and the type packages for
React and Node. No CSS framework, no animation library, no linter, no test
runner. The styles are CSS Modules, one per component, and the animations
are CSS keyframes on SVG and HTML. `next build` checks the types.

**Pearl, dark only,** as the apps are (ADR 0019): the handoff's tokens as
CSS custom properties in `web/app/globals.css`, IBM Plex Sans 400 and 500,
and only the motion the handoff designs (the recording rings, the slower
"preparing" rings, the dot waveform, the welcome screen's two voices). The
light tokens are left out until light mode is designed.

**The font is served by the site.** `next/font/google` downloads IBM Plex
Sans when the site is built and serves it from the site's own files, so a
visitor's browser never contacts Google. The build needs the internet.

**The name is written once,** in `web/lib/site.ts`, until its trademark
check is done (ADR 0021). The wordmark is the name in lowercase.

**Only what the app does today.** The page tells the journey of one voice
note, the app's own example (Rosa in Madrid, her grandson in Lagos), with
its real stages, timings and wording. It does not show calls, prices, a
count of languages, end-to-end encryption, store badges or the names of
the services behind each stage (brief round 4, section 8).

**A sample, when there is real audio.** The hero's "Hear Rosa say hello"
card appears only when `web/public/sample/original.mp3` and
`translated.mp3` exist when the site is built. The audio must be real
output from Shinar, from someone who agreed to it; there is none yet.

## Alternatives considered

**Plain HTML and CSS, without a framework.** Fewer dependencies and nothing
to build. But the site will grow pages (privacy, terms, support, perhaps one
per language), and the owner, a React engineer, chose Next.js.

**Next.js with a server (on Vercel, or Node on the Azure machine).** Nothing
on the site needs one. A static export can still become a server later
without rewriting the pages.

**Tailwind for the styles.** A dependency the Pearl tokens do not need: a
few custom properties cover them.

## Consequences

- Building the site needs Node 20.9 or later. The dev machine has it
  through nvm. `web/node_modules` takes about 380 MB, ignored by git.
- The site is not online yet. The domain and the host are the owner's
  decisions, and so is how visitors are asked to get the app before it is
  on the stores; until then the page says "Coming to iPhone and Android".
- The layout is built from Pearl's approved parts, without a design of its
  own. When Claude Design answers brief round 4, the page follows that
  design.
- There are no automated tests. The page has no logic beyond showing which
  stage is in view, and a test runner would be another dependency.
