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

- `app/`: the page, its layout, the Pearl tokens (`globals.css`), the icon
- `components/`: the logo, the note parts, the hero's loop, the journey and
  its stage frames, the sample player
- `lib/site.ts`: the name, written only here until its trademark check is
  done (ADR 0021); the copy; the journey's stages

## The sample in the hero

"Hear Rosa say hello" plays two files, one after the other, and appears only
when both exist when the site is built:

- `public/sample/original.mp3`: Rosa saying hello, in Spanish
- `public/sample/translated.mp3`: the same words in English, in her voice,
  as Shinar delivered them

Use real output from Shinar, from a person who agreed to be heard on the
site. Never imitate a known person's voice.

## Hosting

Not hosted yet. `web/out/` is all a host needs: any web server that serves
files, such as the Caddy on the Azure machine (ADR 0023), once there is a
domain.
