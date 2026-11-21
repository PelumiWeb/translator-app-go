# Shinar's website

A Next.js project, exported as plain files (ADR 0028). Its home page tells a
voice note's journey, in the Pearl design.

| Command | What it does |
| --- | --- |
| `make web-dev` | run it on http://localhost:3100, reloading as files change |
| `make web-build` | build it into `web/out/`, checking the types |
| `make web-preview` | build it and serve `web/out/` on http://localhost:3100, as a host would |

Each installs the packages first when `package-lock.json` has changed. Node
20.9 or later is needed.

## Where things are

- `app/`: the home page, its layout, the Pearl tokens (`globals.css`), the
  icon; `privacy/`, `terms/` and `support/`, the pages the stores ask for
  (ADR 0032); `og.png/`, the picture a link to the site shows, drawn while
  building (it fetches IBM Plex Sans from Google Fonts then, as `next/font`
  does)
- `components/`: the logo and its intro, the note parts, the hero's loop,
  the journey and its stage frames, the sample player, who it's for, the
  ways to talk, the waitlist, the questions, the header and footer, the
  layout of the text pages, and `Reveals`, which lets each part marked
  `data-reveal` rise into view; `MotionProvider`, round every page, which
  sets up Motion (ADR 0028, addendum)
- `lib/site.ts`: the name, written only here until its trademark check is
  done (ADR 0021); the site's address; the copy; the journey's stages
- `lib/motion.ts`: Pearl's easing and duration for Motion's animations;
  `lib/motion-features.ts`, Motion's features, loaded after the page is shown

## Motion

CSS keyframes play what loops or runs on its own; Motion (`framer-motion`)
animates what changes with the page's state. Use its `m` components (`m.div`,
not `motion.div`: the provider is `strict`, so the full `motion` component
fails rather than loading every feature up front), and import from
`framer-motion`, not `motion/react`, which would bring all of Motion into the
first load. Reduce Motion is handled by the provider: only fades remain.

## The sample in the hero

"Hear Rosa say hello" plays two files, one after the other, and appears only
when both exist when the site is built:

- `public/sample/original.mp3`: Rosa saying hello, in Spanish
- `public/sample/translated.mp3`: the same words in English, in her voice,
  as Shinar delivered them

Use real output from Shinar, from a person who agreed to be heard on the
site. Never imitate a known person's voice.

## Hosting

On Vercel (ADR 0031), as a project made from this repository with its Root
Directory set to `web`; Vercel finds Next.js and builds it on each push to
`main`. `vercel.json` passes `/v1/waitlist` on to Shinar's server, so the
waitlist form posts to the name that served the page and needs no CORS. It
is the only path passed on: the site needs nothing else from the server.
