> Copied from Claude Design's handoff on 2026-10-31, as the reference for building the design. The brief it
> mentions is `../brief.md`. The HTML prototype (`Style Directions.dc.html` and `support.js`) is not kept in the
> repository. ADR 0019 records how the design was built into the apps.

# Handoff: Timbre (working name, was PTranslate). Visual direction "Lagoon / Pearl"

## Overview
This is the chosen visual direction for a voice-translation app. You speak in your language and the other person hears it in theirs, in your own voice. The full product brief is in `brief.md`; it covers product, platforms, the "core moment" steps, error copy and the v1 screen list.

This package covers the **approved style direction** and three key screens:
1. Onboarding: record your voice (recording state)
2. Conversation with voice notes (their note played, my note preparing, mic idle)
3. Interpreter (preparing-voice state). This one was designed in an earlier round; restyle it with the Pearl rules below.

The other v1 screens in brief §7 (welcome, sign in, name, ID, language, model download, home, add someone, profile/settings) have **not been designed yet**. Build them from the tokens and components here, using native platform patterns.

## About the design files
The files in this bundle are **design references created in HTML**. They are prototypes that show the intended look, not production code. The job is to recreate them natively:
- **iPhone:** SwiftUI, iOS 26+, with Liquid Glass and standard navigation where they fit.
- **Android:** Jetpack Compose, Material 3, Android 8+.

The brief asks for one visual identity expressed through each platform's own patterns, not one identical layout on both.

To view: open `Style Directions.dc.html` in a browser, with `support.js` next to it. The page lists rounds newest first:
- **Turn 3, option 3c "Pearl"** is the approved design.
- 3a and 3b are rejected colour alternatives.
- Turn 2 / 2b is the earlier Lagoon with apricot accents. It is shown for the interpreter layout only; its colours are superseded.

## Fidelity
**High fidelity for the visual system** (colour, type, components, motion) on the screens shown. Treat measurements as final at the 360 pt-wide reference (the brief's reference device is iPhone 14 Pro, 393 pt; scale the layout, not the type). The phone bezel and status bar in the mocks are illustration only; use the real system status bar.

**Dark mode is designed. Light mode is only proposed** (tokens below) and needs a design pass.

---

## Design tokens

### Colour: dark (designed)
| Token | Value | Use |
|---|---|---|
| `bg` | `#0E2426` | Page background (deep petrol) |
| `bgRaised` | `#173539` | Their message card, inactive chips |
| `bgDeep` | `#071416` | Deepest layer (sheet scrims, the mock's bezel) |
| `fg` | `#F3EEE4` | Primary text (warm white) |
| `fg2` | `rgba(243,238,228,0.64)` | Secondary text, meta, timestamps |
| `fg3` | `rgba(243,238,228,0.44)` | Upcoming passage lines |
| `fg4` | `rgba(243,238,228,0.32)` | Already-read passage lines |
| `you` | `#F3EEE4` | **Everything that is "me"**: my message cards (every state), mic, record button, recording rings, my language chip |
| `onYou` | `#0E2426` | Text and icons on `you` fills |
| `youHalo` | `rgba(243,238,228,0.12)` | Static halo around the idle mic |
| `youRing` | `rgba(243,238,228,0.28)` | Animated recording rings (start opacity, see Motion) |
| `them` | `#7FD1C1` | **Everything that is "the other person"**: their play button, their waveform dots, "Show original" link, their language label, the translation text in the interpreter |
| `onThem` | `#0E2426` | Icons on `them` fills |

Rule: **two meaning colours only.** Warm white is you, mint is them. No third accent. Apricot `#F4A37C` from the earlier round has been **removed**.

Errors are not designed yet. Suggestion: warm white text with an outline icon. Avoid red alarm styling and never blame the user (brief §9).

### Colour: light (proposed, not yet designed)
| Token | Value |
|---|---|
| `bg` | `#F3EEE4` |
| `bgRaised` | `#DCEFE9` |
| `fg` | `#0E2426` |
| `them` | `#1C6E62` |
| `you` | `#0E2426` fill with `#F3EEE4` icon/text (the inverse of dark mode, since warm white disappears on a light page) |

Check every pair for at least 4.5:1 contrast for text (3:1 for large type and icons).

### Typography
**IBM Plex Sans** for everything. Plex has sibling families (Arabic, Devanagari, Thai, JP, KR), so use those as fallbacks for those scripts to keep the brand consistent across languages. Weights used: 400 and 500 (600 only in the status bar mock). Use tabular numerals on every timer and duration.

| Role | Size / line height | Weight | Tracking | Example |
|---|---|---|---|---|
| Timer | 46 / 1.0 | 500 | -0.02em | `0:12` |
| Passage, current line | 25 / 1.25 | 500 | -0.01em | "about their food, their music…" |
| Interpreter translation | 28 / 1.2 | 500 | -0.015em | "¿Dónde puedo comprar…?" (`them` colour) |
| Contact name (title) | 22 / 1.1 | 500 | 0 | "Abuela Rosa" |
| Preparing headline | 19 | 500 | 0 | "Preparing your voice…" |
| Message text | 16 / 1.5 | 400 | 0 | translation inside a card |
| Passage, other lines | 16 / 1.4 | 400 | 0 | `fg3` / `fg4` |
| Secondary / link | 13–14 | 400 | 0 | "Not now, use the phone's voice" (underlined, offset 3) |
| Label | 12 | 500 | 0 | "Listening · 5 of 6", "Madrid · hears Español" |
| Meta | 12 | 400 | 0 | "0:14", "In her voice · Show original" |

Large text is required. Map these roles onto **Dynamic Type** (`Font.custom(_, size:, relativeTo:)` / `UIFontMetrics`) and onto **Compose `sp`** with font scaling. Translations must never truncate, so let cards grow and let screens scroll. Translations into German or Russian often run longer than the original.

### Spacing
A 4-point base. Values used: 4, 6, 8, 10, 12, 14, 16, 18, 20, 24, 28.
- Edge padding: 16 on list and conversation screens; 28 on centred reading screens (record, interpreter).
- Card padding: 16. Gap between elements inside a card: 12. Gap between cards: 14.

### Radius
- Message cards: 12
- Language chips: fully rounded (capsule)
- Record, play and mic controls: circles
- Stop glyph inside the record button: a 22×22 square with radius 4

### Elevation
None. Layers are told apart by colour only (`bg`, then `bgRaised`, then fills). Native sheets and menus use their system material.

### Motion
| What | Spec |
|---|---|
| Recording rings | 3 rings, each a filled circle in `youRing`. Animate scale 0.48 → 1.0 and opacity 0.9 → 0. Duration 2.4 s, infinite, staggered 0 / 0.8 / 1.6 s, easing `cubic-bezier(0.22, 0.61, 0.36, 1)`. Ring container is 200×200 behind a 96 pt button. **In production, drive the ring amplitude or spawn rate from the live input level**; the mock uses a fixed loop. |
| Preparing your voice (2–5 s) | The same rings in a slower, calmer loop (suggest 3.2 s) around the speaker glyph, so the screen never looks frozen. They stop when audio starts. |
| Speaking | The played portion of the dot waveform fills to full opacity as playback progresses. |
| Haptics | Light impact when recording starts, success notification when it stops (brief §3). |
| Reduce Motion | Replace the pulsing rings with a static halo and a slow opacity fade. |

---

## Components

### Record control (large, onboarding and interpreter)
- A 200×200 ring area (animated rings in `youRing`) with a 96×96 `you` circle in the centre.
- Idle: mic glyph, 28 pt, stroke 1.75, `onYou`.
- Recording: 22×22 stop square (radius 4) in `onYou`.
- Below it, the secondary link "Not now, use the phone's voice" in 14 pt, `fg` at 72% opacity, underlined.
- Accessibility labels: "Start recording" / "Stop recording". Expose elapsed time as the accessibility value.
- States still to design (brief §10): too short, working, preparing voice, speaking.

### Composer mic (conversation)
- A 96×96 static halo (`youHalo`) with a 68×68 `you` circle in the centre.
- Mic glyph 26 pt, stroke 1.75, `onYou`.
- Bottom padding 10 above the home indicator.
- Proposed interaction: **tap to start, tap to send**, with a cancel affordance and a 30 s limit. This works one-handed and with VoiceOver. A hold-to-talk shortcut is optional.

### Their voice note (card)
- `bgRaised` background, radius 12, padding 16, max width about 290, left-aligned.
- First row: 44×44 `them` play button with a 15 pt filled play triangle in `onThem`, then a dot waveform (flex), then the duration (`0:14`, 12 pt `fg2`, tabular).
- Translation text: 16/1.5, `fg`.
- Meta: "In her voice · " in `fg2`, then "Show original" in `them`, underlined.

### My voice note (card)
- `you` background (warm white), text in `onYou`, radius 12, padding 16, max width about 270, right-aligned.
- Content: my transcript at 16/1.5, then a status row.
- Status row while preparing: a 28×28 circle with a 1.5 px border in `onYou` at 50% and a 14×14 solid dot inside it, followed by "Preparing your voice in Spanish" (13/500).
- The card stays warm white in **every** state. The full sequence is sending, translating, preparing your voice, delivered (brief §7.3); the other three are not designed yet.

### Dot waveform
- 22 dots, spread evenly, vertically centred in a 20 pt-high row.
- Dot diameter 3 to about 10 pt, set by amplitude.
- Played dots at full opacity, unplayed at 0.3.
- Colour comes from the speaker's side: `them` for their notes, `onYou` on my cards if a waveform is shown there.

### Conversation header
- Back chevron on the left (24 pt, stroke 1.5).
- Centred stack: the name in 22/500, then "{City} · hears {Language in its own name}" in 12 pt `them`.
- Use the native navigation bar on each platform.

### Language chips (interpreter)
- Capsules with padding 8×14, 14/500 text.
- Mine: `you` fill with `onYou` text.
- Theirs: `bgRaised` fill with `them` text.
- A swap glyph (18 pt) sits between them.
- Always show language names in their own language ("Español").

---

## Screens

### 1. Record your voice (onboarding step 5 of 6)
Vertical stack, centred text, laid out top to bottom:
1. Label "Listening · 5 of 6" (12/500, `fg`).
2. Timer `0:12` (46/500).
3. "Keep going. A few more seconds." (13, `fg2`).
4. The passage fills the remaining space, vertically centred, as a teleprompter with three lines:
   - the read line at 16 in `fg4`: "When I travel, I like to talk with the people I meet"
   - the current line at 25/500 in `fg`: "about their food, their music, and the places they love."
   - the next line at 16 in `fg3`: "A good conversation makes the world feel smaller."
5. The record control.
6. "Not now, use the phone's voice".

Behaviour (brief §5):
- 10 s minimum and 30 s maximum.
- Recordings under 10 s, or silent ones, cannot be kept; tell the person why.
- After recording: listen, record again, or keep.
- The consent copy (what is kept, what it is used for, delete any time) still needs a screen design.

### 2. Conversation (voice notes)
Laid out top to bottom:
- Header.
- Message list (padding 12 horizontal and 16 at the top).
  - Their card is left-aligned; my card is right-aligned.
  - Mock copy, their card: "Your grandfather would have loved that photo. Come in August, the garden will be full."
  - Mock copy, my card: "I'm sending you the video from the recital tonight."
- The composer mic, centred at the bottom.
- Long press on a message opens replay, show original, copy and report. Use the native context menu.

### 3. Interpreter (face to face)
See turn 2 / option 2b in the HTML for the layout, and recolour it to Pearl. From top to bottom:
1. Language chips row.
2. Centred original text (16, `fg2`).
3. Translation (28/500, `them`).
4. A 220 pt ring stack with an 88 pt `you` circle holding a speaker glyph.
5. "Preparing your voice…" (19/500).
6. "Usually a few seconds" (13, `fg2`).

The brief suggests a two-person mode for passing the phone across a table; it is not designed yet.

---

## State model (from brief §6)
```
RecordState: idle | recording(level, elapsed) | tooShort | transcribing(partialText)
           | translating | preparingVoice | speaking(progress) | phoneVoiceFallback | done | error(kind)
MessageStatus (mine): sending | translating | preparingVoice | delivered | failed(reason)
```
- Every state over about 0.3 s needs a visible indicator. The biggest gap is `preparingVoice`, which takes 2–5 s.
- `phoneVoiceFallback` should say, subtly, that this is the phone's voice and not the person's own.
- Error copy is listed in brief §6. Keep the transcript on screen whenever translation or speech fails.

## Native build notes
- **Custom drawing:** the ring animation, the dot waveform and the live level meter. Use SwiftUI `Canvas` or `TimelineView`, and Compose `Canvas` with `rememberInfiniteTransition`.
- **Native components:**
  - navigation bar / `TopAppBar`
  - lists
  - context menus (`contextMenu` / `DropdownMenu`)
  - language picker: a searchable list in a sheet (`.searchable` / `SearchBar`)
  - sign in: Sign in with Apple, Google Credential Manager
- **Accessibility:**
  - every control labelled
  - the record button works with VoiceOver and TalkBack
  - right-to-left layouts mirror, but the waveform direction follows playback time
- **Fonts:** bundle IBM Plex Sans plus the Plex Arabic, Devanagari, Thai, JP and KR families, or fall back to system fonts for those scripts.

## Assets
- Icons: Lucide-style outlines (mic, play, chevron-left, arrow-left-right, volume-2) at 1.5–1.75 stroke. In production use SF Symbols on iOS and Material Symbols (Outlined) on Android.
- No imagery. App icon and wordmark are not designed yet.

## Files
- `screenshots/01-record-your-voice.png`: approved look for onboarding voice recording (3c, at 3x). The rings are captured mid-animation.
- `screenshots/02-conversation.png`: approved look for the conversation (3c).
- `screenshots/03-interpreter-layout-old-colours.png`: **layout reference only.** It uses the superseded apricot colour; restyle with `you` = warm white as described above.
- `Style Directions.dc.html`: the design reference. Turn 3, option 3c is the approved design. Open it with `support.js` in the same folder.
- `support.js`: the runtime needed to open the HTML file.
- `brief.md`: the full product brief, including flows, timings, error copy and the screen list.
