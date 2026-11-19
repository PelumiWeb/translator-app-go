# Design brief, round 4: a voice note's journey, in motion and on the website

For Claude Design, or any Claude session without access to the code.
Written 2026-11-18. It follows `brief.md` (round 1, the product),
`brief-2-social.md` (round 2, signing up and rooms) and round 3 (the name,
logo and splash). The approved visual direction is **Lagoon / Pearl**; its
handoff is `pearl/README.md`. Everything here must be drawn in Pearl.

Attach with this brief:

- `pearl/README.md`: the tokens, type, motion and components (binding)
- `pearl/assets/*.svg`: the mark, on dark, on light and in one colour
- `pearl/brand/01-logo.png`, `pearl/brand/02-splash.png`
- `pearl/02-conversation.png`, `pearl/v1/01-welcome.png`,
  `pearl/v1/12-voice-note-states.png`

## 1. Shinar in a paragraph

Shinar lets people talk to anyone in the world without sharing a language.
You speak in your language; the other person hears it in theirs, **in your
own voice**. They answer in theirs; you hear it in yours, in their voice.
Each person signs up with their name and a 20-second recording of their
voice. Today they talk in voice notes, in a **room** of two people. There is
also a face-to-face interpreter on one phone.

**The theme is breaking the language barrier** (the owner, 2026-11-19, ADR
0033). Two people who share no language talk anyway, each in their own; the
piece and the page should leave no doubt that this is what Shinar is for.

The promise: **"Break the language barrier. Be heard in your own voice."**
(It was "Talk to anyone. Be heard in your own voice." until 2026-11-19.) The
barrier is the reason; the voice is the hero, not the text.

The name: Genesis 11:2. Shinar is the plain where everyone still shared one
language, before Babel scattered them: the story of how the language barrier
began. The app reverses the scattering. The
mark is two overlapping circles (two voices) over a horizon line (the
plain).

## 2. What we want

Two pieces, from one idea:

1. **A motion piece**: one voice note travelling from one person to another.
   It starts as speech in the sender's language, goes through each stage,
   and arrives on the listener's phone in the listener's language, in the
   sender's voice.
2. **The same journey on the website**: the centrepiece of Shinar's home
   page, as a short looping hero and as a story told while scrolling.

The two should share their scenes, their timing and their drawing, so that
one is a cut of the other.

## 3. The journey, as it really works

Every stage below happens in the app today, with these timings. The motion
can compress time, but it should not pretend there is no wait: a note takes a
few seconds to be ready, and the app's design shows that wait honestly.

| # | Stage | What really happens | Where | Real time | The app's own words |
| --- | --- | --- | --- | --- | --- |
| 1 | Speak | The sender taps the mic and talks, up to 30 seconds | sender's phone | as long as they talk | "0:07 · tap to send" |
| 2 | Write down | The phone turns the speech into text, word by word (the server does it when the phone cannot) | sender's phone | about a second | "Writing down what you said…" |
| 3 | Send | The recording, its words and its language go to Shinar's server | network | under a second | "Sending" |
| 4 | Translate | The server translates the words into the listener's language | server | under a second | "Translating into English" |
| 5 | Prepare the voice | The server speaks the translation in the sender's own voice, made from the voice sample they recorded and agreed to keep | server | 2 to 5 seconds, the longest wait | "Preparing your voice in English" |
| 6 | Deliver | The listener's phone is told: live if the room is open, a notification if not | listener's phone | about a second | notification: "Rosa" / "Sent a voice note · 0:14" |
| 7 | Hear | The listener plays it: their language, the sender's voice. The translated words light up as it plays; the original words are a tap away | listener's phone | as long as the note | "In her voice · Show original" |
| 8 | Reply (optional ending) | The listener answers, and the same journey runs the other way | both | | |

Stage 5 is the heart of the piece: the words have changed language, but the
voice is still theirs.

Show the stages, not the machinery. No server racks, cloud icons, or the
names of the services behind each stage. They are suppliers and can change.

## 4. The example to tell it with

Use the people and words the app's design already uses:

- **Rosa**, in Madrid, speaks Spanish (Español). She sends a note to her
  grandson.
- **Her grandson**, in Lagos, speaks English. He hears her in English, in her
  voice.

Rosa says: "A tu abuelo le habría encantado esa foto. Ven en agosto, el
jardín estará lleno."
He hears: "Your grandfather would have loved that photo. Come in August, the
garden will be full."

His reply, if the piece ends on it: "I'm sending you the video from the
recital tonight." She hears: "Te mando el vídeo del recital de esta noche."

On the audio:

- The voices should be real output from Shinar: a real person who agrees,
  recorded through the app, and the note the app actually delivered. Then the
  piece shows what the product does, not a mock-up of it. The person can
  still be called Rosa.
- Never imitate a real, known person's voice.
- Other language pairs are welcome later. English to and from Yorùbá
  translates well, but how the voice sounds in Yorùbá has not been checked;
  hear it before showing it.

## 5. The visual language (from Pearl, binding)

**Dark only.** Light mode is proposed in the handoff, not designed. Do not
use it.

**Colour.** Two meaning colours, and no others:

| Token | Value | Meaning |
| --- | --- | --- |
| `bg` | `#0E2426` | the page, deep petrol |
| `bgRaised` | `#173539` | raised layers, their cards |
| `bgDeep` | `#071416` | the deepest layer |
| `fg` | `#F3EEE4` | text, warm white |
| `you` | `#F3EEE4` | everything that is "me": my card, my mic, my rings |
| `them` | `#7FD1C1` | everything that is "the other person": their play button, their dots, their language |
| lens | `#B9E0D3` | the overlap of the two circles, in the mark only |

No red, no apricot (removed in round 3), no gradients the handoff does not
have. No shadows: layers are told apart by colour.

"You" and "them" depend on whose phone we are looking at. On Rosa's phone
her note is a warm-white card (hers); on her grandson's phone the same note
arrives as a raised card with mint (theirs). That change of colour, from
warm white to mint, can be the visual turn of the journey: **the words change
language, the colour changes side, the voice stays the same.**

**Type.** IBM Plex Sans, weights 400 and 500, tabular numerals on every
time. The handoff's type scale applies. Language names are always written in
their own language ("Español", "Yorùbá"). No flags: a language is not a
country.

**Motion vocabulary, already designed.** Reuse these, do not invent new ones:

- **Recording rings**: three filled circles, scale 0.48 to 1.0, opacity 0.9
  to 0, 2.4 s, staggered 0, 0.8 and 1.6 s.
- **Preparing your voice**: the same rings, slower and calmer (3.2 s), round
  a speaker glyph. They stop when the sound starts.
- **Dot waveform**: 22 dots, 3 to 10 pt by loudness; played dots at full
  opacity, unplayed at 0.3. Mint for their note, petrol on my card.
- **Words arriving**: during "Writing down", words appear one by one, the
  unconfirmed ones dimmer, with a caret.
- **The mark's intro** (splash, first launch): the two circles part by 9
  units and the lens fades out; one ripple per circle (scale 1 to 1.6); they
  return and the lens fades in; the wordmark rises 8 pt as it fades in.
- **Two voices** (Welcome screen): a warm-white and a mint ring set, 180 pt
  each, overlapping, pulsing on a 3 s loop.
- **Easing**: `cubic-bezier(0.22, 0.61, 0.36, 1)` throughout.

Translation should read as a change of words, not a glitch: cross-fade word
by word or phrase by phrase. No letter scrambling, no "hacker" effects.

**No imagery.** The handoff keeps Shinar free of photos. Stay abstract:
phones, cards, rings, dots, words, the two circles and the horizon line.

A starting idea, not a requirement: the horizon line of the mark is the
path the note travels, from one phone to the other, and each stage is a
point along it. The piece ends with the two circles meeting over the line,
the lens appearing between them (two people understood each other), and the
wordmark. The mark has mint on the left and warm white on the right, so
decide which side the sender starts on, or tell the story vertically, one
phone above the other, which also suits a portrait frame.

## 6. The motion piece

Proposed (the owner has not decided these):

- A main cut of 25 to 30 seconds, and a 6-second loop of the core turn
  (stages 4 and 5) for the website and social posts.
- In 16:9 and in 9:16 (portrait, for phones).
- Sound: Rosa's real voice, then her voice in English; quiet music, or
  none. People often watch with the sound off, so the words on screen must
  tell the story alone: her Spanish words, then the English ones.
- It ends on the mark, the wordmark "shinar" and the promise line.

A possible timeline for the main cut:

| Time | Scene |
| --- | --- |
| 0 to 4 s | Rosa speaks; warm-white rings follow her voice; the timer runs |
| 4 to 7 s | Her words are written down in Spanish; she taps send; the card lifts away |
| 7 to 11 s | Translating: the Spanish words become English, phrase by phrase |
| 11 to 17 s | Preparing her voice: the calm rings, then the dots take her voice's shape |
| 17 to 20 s | Delivered: the note arrives on her grandson's phone; "Rosa · Sent a voice note · 0:14" |
| 20 to 26 s | He plays it: English, in her voice; the mint dots fill; "Show original" brings back her Spanish |
| 26 to 30 s | The two circles meet over the horizon; the lens; "shinar"; "Break the language barrier. Be heard in your own voice." |

**The name is not final.** A trademark check is still to come. Keep the
end card (mark, wordmark) a separate scene that can be replaced without
remaking the rest.

## 7. The website

A website will be needed before the app goes public: the app stores ask for
a privacy policy, terms and a support page. No domain has been chosen. This
round designs the home page, with the journey at its centre.

A first version is already built, in Next.js and exported as plain files
(`web/`, ADR 0028), from Pearl's parts and without a design of its own: a
looping hero card, the seven stages while scrolling, each drawn as the screen
it happens on, a section on what is kept of your voice, and the two voices
before the footer. Treat it as a sketch to improve on, not a constraint.
Whatever you design should be buildable with SVG, CSS and a little
JavaScript; there is no animation library.

**The home page.**

1. **Hero**: the mark and wordmark, the promise line "Break the language
   barrier. Be heard in your own voice.", the sub-line "Talk to anyone. You
   speak your language. They hear theirs, in your voice.", and the 6-second
   loop beside or behind it.
2. **Hear it**: a sample player, like the Welcome screen's card: "Hear Rosa
   say hello", her Spanish, then her voice in English. The best proof is
   the real sound.
3. **The journey, while scrolling**: the scenes of the motion piece, one
   stage per step of the scroll, each with one plain sentence. Scrolling
   back plays it backwards; it never takes over the scroll.
4. **Your voice is yours**: in plain words, what is kept and why: the voice
   sample is kept on Shinar's server, only to speak that person's own words,
   never given to anyone else, and deleted whenever they choose or with
   their account. Taken from the consent screen; legal review is still to
   come.
5. **Get the app**: see the open decisions (section 9).
6. **Footer**: privacy, terms, support (pages to come).

**Building notes.**

- Mobile first: most people will arrive on a phone.
- Draw the hero and the scroll story in SVG and CSS, not as a video, so the
  page stays light and sharp; the full cut can be a video file further down.
- Honour `prefers-reduced-motion`: the rings become a static halo with a
  slow fade, and the scroll story becomes still frames.
- Sound never plays until the person presses play.
- Contrast of at least 4.5:1 for text, 3:1 for large type and icons.
- Fonts: IBM Plex Sans, served from the web.
- Some people read right to left; leave room for that, it is not needed in
  this round.

## 8. What not to say or show

The piece and the page must only show what Shinar does today.

- **Not instant.** A note takes a few seconds. Do not show a live call: calls
  are planned for later, not built.
- **Not end-to-end encrypted.** The server reads the words to translate them.
  Do not claim it, in words or with lock icons.
- **No language count.** Speech is understood in about 99 languages and the
  voice speaks more than 80, but the number a person can actually use is the
  overlap, not yet checked. Leave the number out.
- **No prices, plans or "premium".** Billing comes much later.
- **Nothing not yet in people's hands:** group rooms, calls (the app only
  says they are coming), finding strangers. Leave them out, rather than
  saying "coming soon". Text messages were built on 2026-11-18 but not yet
  tried on two phones; leave them out of this round too.
- **No supplier names** (see section 3).
- **No store badges** until the app is on the App Store and Google Play;
  Apple's and Google's badge rules apply then.

## 9. Open decisions, for the owner

Design around these with a sensible default, and mark which one you chose:

1. Length and frames of the motion piece (proposed in section 6).
2. Whose real voice plays Rosa, and the grandson.
3. The website's call to action while the app is not on the stores: a
   waitlist (it would collect email addresses, so it needs a privacy note and
   somewhere to keep them), a TestFlight invitation, or simply "Coming soon
   to iPhone and Android".
4. The domain, and where the site is hosted. These do not change the design.
   (It is built with Next.js, decided on 2026-11-18.)

## 10. What we would like back

1. **A storyboard**: every scene, its timing, its frame, what moves and how.
2. **The motion itself** as an HTML prototype in Pearl, like the earlier
   rounds, that can be recorded to video and reused on the website. Main cut
   and 6-second loop; 16:9 and 9:16.
3. **The website's home page**, on a phone and on a desktop, with the hero,
   the sample player, the scroll story and its reduced-motion form.
4. **Notes for building**: which parts are SVG and CSS, which are video,
   timings and easings per scene, and the reduced-motion version of each.
