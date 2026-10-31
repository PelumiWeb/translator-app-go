# Handoff: Timbre (working name, was PTranslate). Visual direction "Lagoon / Pearl"

## Overview
This is the chosen visual direction for a voice-translation app. You speak in your language and the other person hears it in theirs, in your own voice. The full product brief is in `brief.md`; it covers product, platforms, the "core moment" steps, error copy and the v1 screen list.

This package covers the **approved style direction** and three key screens:
1. Onboarding: record your voice (recording state)
2. Conversation with voice notes (their note played, my note preparing, mic idle)
3. Interpreter (preparing-voice state). This one was designed in an earlier round; restyle it with the Pearl rules below.

**Update (round 2):** `Timbre v1 Screens.dc.html` adds the remaining v1 screens, every voice-note state, and the error/notice styles. See the "v1 screens" section near the end of this README.

Still not designed:
- the speech-model download step (onboarding step 5)
- light mode
- Android-specific frames
- the Pearl recolour of the interpreter
- the app icon

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

| `destructive` | `#F2A08A` | Text colour for destructive actions only ("Delete my voice", "Delete account"). Never used for errors. |
| `line` | `rgba(243,238,228,0.08)` | List separators |
| `outline` | `rgba(243,238,228,0.28)` | Secondary button border |

Rule: **two meaning colours only.** Warm white is you, mint is them. The only other accent is `destructive`. Apricot `#F4A37C` from the earlier round has been **removed**.

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

## v1 screens (round 2): `Timbre v1 Screens.dc.html`
All frames are dark, iPhone, 360 pt wide. Screenshots are in `screenshots/v1/`.

### Shared parts
- **Primary button:** 52 pt high, capsule, `you` fill, `onYou` text at 16/500.
- **Secondary button:** same size, 1 px `outline` border, `fg` text.
- **Text input:** 56–60 pt high, `bgRaised`, radius 12, 18–20 pt text. When focused it gets a 1.5 px `fg` border.
- **Onboarding header:** back chevron (28 pt) plus a 5-segment progress bar (3 pt high, gap 6; done = `fg`, to do = `fg` at 20%). Under it, a step label such as "1 of 5" (12/500, `fg2`), then the title at 28–30/500, -0.02em.
- **Onboarding steps:** Name, ID, Language, Voice (record, then consent), Speech model. ⚠ The round-1 record screen says "5 of 6"; change it to **"4 of 5"**.
- **Grouped list (settings, language list):** `bgRaised`, radius 12, rows 50–56 pt, 16 pt side padding, `line` separators. Group titles 12/500 in `fg2`, inset 16. Use native `List(.insetGrouped)` / Material 3 `ListItem`.
- **Tab bar:** a floating iOS 26 Liquid Glass bar with Rooms, Interpreter and Profile. Use the system `TabView`; don't redraw it. On Android, use a Material 3 `NavigationBar`.

### Screens
1. **Welcome** (screenshot `01-welcome`)
   - "timbre" wordmark placeholder at 18/600.
   - Two-voices animation: a warm-white ring set and a mint ring set, 180 pt each, 80 pt apart so they overlap. Each set has 3 pulsing rings (3 s loop, staggered 1 s, mint offset by 0.5 s) around a 52 pt core.
   - Headline "Talk to anyone. Be heard in your own voice." at 34/500.
   - Sub-line "You speak your language. They hear theirs, in your voice."
   - Sample card (`bgRaised`): mint play button, "Hear Rosa say hello" / "Spanish, heard in English, in her voice".
   - Primary button: "Get started".
2. **Sign in** (`02-sign-in`)
   - Title "Sign in to keep your conversations."
   - Body "This is only to find your account on any phone. We never post anything."
   - Buttons: `ASAuthorizationAppleIDButton` (white style) and the Google Credential Manager button (dark theme). The system draws both logos.
   - "Email sign-in is coming soon." in `fg3`, then the Terms / Privacy line.
3. **Your name** (`03-name`): "What should people call you?", a text field, helper "Shown to the people you talk to." The Continue button sits above the keyboard.
4. **Your ID** (`04-id`)
   - "Pick your ID." / "This is how people find you."
   - The field shows a dim "@" prefix.
   - The check runs as you type, debounced to about 400 ms. When it passes, show a check icon and "Available".
   - Helper: "Letters, numbers, dots and underscores."
   - The taken state is on the notices board.
5. **Your language** (`05-language`)
   - "Which language do you speak?" with a search field "Search 99 languages".
   - A "From your phone" group holds the suggested language, ticked.
   - Then "All languages": each row shows the name in its own language at 16 pt, with the English name on the right at 13 pt in `fg2`.
   - Button: "Continue in {language}".
   - Scripts use the Plex sibling families.
6. **Voice consent** (`06-consent`), shown after recording
   - Title "That's your voice. Keep it?"
   - Playback card in `you`, with a petrol play button and dots.
   - Three plain blocks:
     - **What we keep:** this recording, on our server.
     - **What it's for:** only your own words; Fish Audio makes the speech.
     - **Delete it any time:** in Profile, under Your voice.
   - Buttons: "Keep my voice" (primary), "Record again" (secondary), and the link "Use the phone's voice instead".
   - Have legal and privacy review this copy.
7. **Your rooms** (`07-rooms`)
   - Large title "Rooms" (32/600), a + button (44 pt circle, `bgRaised`) top right, and a search field.
   - Each row has:
     - a 48 pt avatar in `bgRaised` with the initial in `them`
     - the name at 16/500 and the time at 12 in `fg2`
     - the last message at 14 in `fg` at 72%, up to 2 lines
     - "{City} · hears {Language}" at 12 in `them`
     - a 10 pt mint dot when there's something new from them
   - My own last messages start with "You: ".
8. **Rooms, empty** (`08-rooms-empty`)
   - The two-voices animation, then "No one here yet."
   - Body: "Add someone by their ID or QR code. You'll hear them in English, in their own voice."
   - Buttons: "Add someone" and "Share my ID, @pelumi.a".
9. **Add someone, by ID** (`09-add-by-id`)
   - A sheet with a grabber, a close button and the title "Add someone".
   - A segmented control: By ID / QR code.
   - The ID field. When it finds someone, show a result card: avatar, name, "@id · speaks {Language}" (language in `them`), and the line "You'll hear Rosa in English, in her voice. She'll hear you in Spanish, in yours." Then "Add Rosa".
10. **Add someone, QR** (`10-add-by-qr`)
    - My QR code on a 236 pt warm-white tile (radius 20). It's a placeholder: generate a real QR code with dark modules on light.
    - Under it: my name, "@id · speaks English", and the hint "Let them scan this with their phone's camera, or the app."
    - Buttons: "Scan their code" (opens the camera) and "Share my code".
11. **Profile and settings** (`11-profile`, full scroll)
    - Header: a 76 pt avatar in `you`, the name at 22/500, and "@id · speaks English".
    - Groups:
      - **Your voice:** Play your voice (0:21 · recorded date), Record again, Delete my voice (`destructive`). Footer: "Used only to speak your own words. Without it, your messages use the phone's voice."
      - **You:** Name, ID, Language.
      - **Privacy:** What we keep, and why.
      - **Advanced:** Speech to text (Automatic / This device / Server), Speech model (Ready · 57 MB; push to status, delete, download again), This phone's speed.
      - **Account:** Sign out, Delete account (`destructive`).
    - Destructive actions must ask for confirmation in a native alert.

### Voice-note states (`12-voice-note-states`)
Mine (a `you` card, 270 pt, radius 12, padding 16):
- **M1 Recording (composer):**
  - a cancel X (44 pt, outline)
  - live dots in `fg`
  - a 64 pt stop button with a 10 pt halo
  - the line "0:07 · tap to send"
  - 30 s maximum; from 25 s on, show the seconds left
- **M2 Too short:** a `bgRaised` hint above the mic: "That was very short. Tap, say something, then tap again to send." Nothing is sent.
- **M3 Speech to text:** words arrive one by one. Words not yet confirmed are in `onYou` at 44%, followed by a caret. Status: "Writing down what you said…"
- **M4 Translating:** a 4-segment bar (sent, translating, voice, delivered) and the status "Translating into {language}".
- **M5 Preparing your voice:** the breathing-ring indicator and "Preparing your voice in {language}".
- **M6 Delivered:** play button, dots and duration above the text. Meta line: check icon, "Delivered in your voice · 9:14".
- **M7 Failed:** the transcript stays. Below a hairline: an alert icon, "Couldn't translate this. Your words are kept." and **Retry**.
- **M8 Phone's voice:** "Delivered in the phone's voice · Record yours". This is not styled as an error.

Theirs (a `bgRaised` card, 290 pt):
- **T1 New:** dots at 45% and a mint unread dot. Meta: "New · 0:14 · in her voice".
- **T2 Playing:** pause glyph; the dots fill as it plays and the text lights up to match. Meta: "Playing in her voice" in mint.
- **T3 Done with original:** the translation, a hairline, then "Original · {Language}" in mint and the original text at 15 pt in `fg` at 72%. Link: "Hide original".
- **T4 Phone's voice:** an outline mint play button, dots in `fg` at 40%, and "Spoken by your phone's voice" with a phone icon.
- **T5 First use of a language:** a 3 pt mint progress bar and "Getting {language} ready on this phone. This happens only once." On iPhone, the system shows its own sheet.

### Errors and notices (`13-errors-notices`)
There are three forms. None of them uses red, and the copy never blames the person.

1. **Toast:** a top capsule in `bgRaised` with a spinner, for problems that fix themselves: "Connection lost. Reconnecting…"
2. **Banner:** a `bgRaised` card with an icon, a title, a body and a primary action, used when the person needs to act. Example: "Timbre needs your microphone" / "To record, allow microphone access in Settings." / [Open Settings]. It replaces the mic until permission is granted.
3. **Hint above the mic:** after a failed recording.
   - Silent: "We didn't hear anything. Check nothing is covering the microphone, then try again." The dB figure goes to Advanced and the logs only.
   - No speech: "No words came through. Try again, a little closer to the phone."
   - Unclear: "We couldn't quite make that out. Please say it again."

Two more cases are handled in place, not as one of the three forms:
- **In the message:** "This iPhone has no voice for {language}. The words are all here; only the sound is missing."
- **In the form:** "@pelumi is taken. These are free:" followed by 3 suggestion chips (capsule, `bgRaised`, 14 pt).

## Files
- `Timbre v1 Screens.dc.html`: round 2, all remaining v1 screens, voice-note states and notices. Open it with `support.js` in the same folder.
- `screenshots/v1/`: one image per screen plus the two boards (2x).
- `screenshots/01-record-your-voice.png`: approved look for onboarding voice recording (3c, at 3x). The rings are captured mid-animation.
- `screenshots/02-conversation.png`: approved look for the conversation (3c).
- `screenshots/03-interpreter-layout-old-colours.png`: **layout reference only.** It uses the superseded apricot colour; restyle with `you` = warm white as described above.
- `Style Directions.dc.html`: the design reference. Turn 3, option 3c is the approved design. Open it with `support.js` in the same folder.
- `support.js`: the runtime needed to open the HTML file.
- `brief.md`: the full product brief, including flows, timings, error copy and the screen list.
