# ADR 0019: The Pearl design in both apps

Status: accepted, 2026-10-31

## Context

On 2026-10-31 the owner chose a design from Claude Design: the "Lagoon /
Pearl" direction, under the working name Timbre. The handoff
(`docs/design/pearl/README.md`) gives the colours (dark designed, light only
proposed), IBM Plex Sans with a type scale, spacing, radii, motion, the record
control, the language chips, the dot waveform, and three screens: recording
your voice, a conversation, and the interpreter. It asks for one visual
identity through each platform's own patterns, not one layout forced onto both.

Milestone 19 builds it into the screens that exist today. The conversation
screen waits for milestone 14, which has the messaging behind it.

## Decision

**Tokens in code, one set per platform.** `Palette` (iOS) and `PearlColors`
(Android) hold the colours, each with dark and light. The design's two meaning
colours, `you` and `them`, are kept to exactly that use. Android does not use
Material's dynamic colour, which would recolour the brand with the wallpaper;
Material's scheme is mapped onto the palette so standard components match.

**Dark only, until light is designed.** The apps are dark whatever the phone's
setting (`UIUserInterfaceStyle` on iOS; the theme and launch window on
Android). The first build followed the phone and showed the handoff's proposed
light palette on the owner's phone, which they rightly said was not the
design. The light tokens are kept, unused, for when light mode is designed.

**IBM Plex Sans, bundled.** Regular and Medium, from IBM's repository, under
the SIL Open Font License, with the licence shipped beside the fonts. The type
roles grow with Dynamic Type and Android's font scale. Scripts Plex does not
cover fall back to the system's fonts; the Plex Arabic, Devanagari, Thai and
CJK families are not bundled yet.

**Icons without a library.** SF Symbols on iOS. On Android, the six outline
icons the design uses are drawn as vector paths after Lucide's shapes, rather
than adding the Material icons library.

**The view model reports a stage; the screen chooses the words.** The
interpreter needs a headline and a line under it for every stage of a run, so
the view models' status text became a typed `InterpreterStage`, and the
wording lives in the screens.

**Two changes underneath, on both phones**, which the design called for:

- The speech reports, when it starts, whose voice it is in (`SpeechVoice`), so
  the interpreter can say that the phone's voice is speaking instead of the
  user's own.
- Tapping the control while it speaks stops the speech
  (`pipeline.stopSpeaking`).

**Each platform's patterns.** The settings button is Liquid Glass on iOS;
settings are a native list, with Advanced as a pushed page; languages are
picked from a searchable sheet on both. The developer controls (where speech
is turned into text, the speech model, the phone's speed, the last result's
details) moved from the interpreter to Settings, under Advanced.

## Consequences

- Both apps look like the design, always dark. Light mode waits for a design
  pass, as the handoff asks.
- The recording rings are a fixed loop. The handoff asks for them to follow the
  microphone's level, which needs the recorders to report it: later.
- The teleprompter advances by time, each line its share of the passage's 20
  seconds, since nothing listens for the words.
- The app's name on the home screen is still PTranslate; Timbre is a working
  name until the owner decides.
- About 400 KB of fonts on each platform.
