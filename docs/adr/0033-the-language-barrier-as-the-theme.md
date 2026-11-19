# ADR 0033: The language barrier as the theme

Status: accepted, 2026-11-19

## Context

The owner, on 2026-11-19: "The whole point of the app is to break language
barrier, make sure that's the theme of all the app. It should be included in
the web app, and anywhere that's necessary."

Until then the promise was "Talk to anyone. Be heard in your own voice."
(design brief round 1, 2026-10-31), on both phones' Welcome screen, the
website's hero and title, and the briefs. It said how Shinar is different
(your own voice) but not what it is for: the words "language" or "barrier"
appeared nowhere a newcomer looks first. The marketing brief went further and
told writers to avoid "breaking down language barriers at scale", to keep the
copy concrete.

The name already tells this story. Shinar is the plain of Genesis 11 where
everyone still shared one language, before Babel: the story of how the
language barrier began.

## Decision

**Breaking the language barrier is the theme of everything Shinar says**:
the apps, the website, the marketing and the store listings.

**The promise** becomes "Break the language barrier. Be heard in your own
voice." Its first half is what Shinar is for, in the owner's own word,
"break"; its second keeps what sets it apart. **The sub-line** becomes "Talk
to anyone. You speak your language. They hear theirs, in your voice.", which
keeps the old promise's first half. Both are the same words on the Welcome
screen of both phones and in the website's hero and title
(`web/lib/site.ts`).

**On the website**, beside the hero: the journey is headed "How a voice note
crosses the language barrier" and says that Rosa and her grandson don't
share a language; the first question is "What does Shinar do?", answered
with the barrier; the waitlist asks people to join with someone they don't
share a language with (the soft launch works in pairs); the footer's name
story ends "before Babel and the language barrier".

**In the apps**, only the Welcome screen changes. The screens after it
already tell the story in what they do ("You'll hear them in English, in
their own voice", "hears Español", "Talk to me on Shinar, in your own
language"), and repeating the phrase on every screen would wear it out.

**The tone rule changes**: say "the language barrier" plainly, in those
words, then show it with a concrete example; never in abstractions such as
"at scale", "seamless" or "frictionless".

## Alternatives considered

**Keep the promise and add a tagline about the barrier.** Two slogans on one
screen, and the first thing people read would still not say what the app is
for.

**"Talk to anyone. No language barrier."** Keeps the old opening, but drops
the voice from the promise, and the voice is what no other translator does.

**"No more language barriers."** Sounds like an advertisement, and the
plural makes it abstract.

## Consequences

- Pearl's Welcome screen keeps its design; its headline and sub-line are new
  words, noted in `docs/design/pearl/README.md`. The screenshot shows the old
  ones.
- The motion piece's end card and the website's hero in design brief round 4
  carry the new promise.
- "Break the language barrier" is 26 characters, so it fits the App Store's
  30-character subtitle when there is a listing.
- The promise slightly overreaches for some languages (Yorùbá and Hausa are
  understood less well, Igbo not yet). The website's questions and the
  marketing brief stay honest about that; the promise is what the app is
  for, not a claim that every language works.
- A change to the promise is made on both phones and the website together,
  as with any rule the phones share.
