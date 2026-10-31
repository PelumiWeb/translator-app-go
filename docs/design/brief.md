# Design brief: a voice app for talking across languages

For Claude Design. Written 2026-10-31. Everything here describes an app that
already works end to end on a real iPhone; it looks like a developer's test
screen, and needs a real design before the next features are built.

## 1. The product in one paragraph

People talk to anyone in the world without sharing a language. You speak in
your language; the other person hears it in theirs, **in your own voice**.
They answer in theirs; you hear it in yours, in their voice. Each person signs
up with their name and a 20-second recording of their voice, and gets an ID
others can find them by. Voice notes come first, text messages next, live
calls later for paying users. There is also a face-to-face interpreter: one
phone, speak, and it says what you said in another language, in your voice.

The promise to design around: **"Talk to anyone. Be heard in your own
voice."** The voice is the hero, not the text.

Working name: PTranslate (to be replaced; propose names if inspired).

## 2. Who it is for

- Families split across languages: a grandmother in Lagos, grandchildren in
  Madrid.
- People who met online, or abroad, and want to keep talking.
- Travellers and expats dealing with everyday life in a new language.
- Small businesses with customers who speak another language.

They are not technical. They use it one-handed, often outdoors, often
emotionally (talking to family). Many will use it in a language other than
English, and some read right to left.

## 3. Platforms and constraints

- **Two native apps, one brand.** iPhone (iOS 26 and later, built in SwiftUI)
  and Android (Jetpack Compose, Material 3, Android 8 and later). Not a web
  app, and not one identical layout forced onto both: one visual identity,
  expressed through each platform's own patterns (iOS 26's Liquid Glass and
  navigation on iPhone; Material 3 on Android).
- Light and dark mode, both first-class.
- Large text: iOS Dynamic Type and Android font scaling, up to the largest
  sizes, without truncating translations.
- VoiceOver and TalkBack: every control labelled; the record button usable
  without sight; haptics when recording starts and stops.
- Text in any script and either direction: Arabic (right to left), Hindi,
  Thai, Chinese, Japanese, Korean. Translations are often longer than the
  original (German, Russian).
- Phone sizes from small Android phones to iPhone Pro Max. Reference device:
  iPhone 14 Pro.
- Please design with native components where they fit, so it can be built
  faithfully in SwiftUI and Compose.

## 4. Feel

Warm, human, calm, trustworthy, global. It carries people's voices and their
families' conversations, so it should feel personal, not like a utility.

Avoid: the generic translator look (flags, blue gradients, robots, chat
bubbles with "A→文" icons), anything that feels like AI showing off, and
clutter.

Ideas to explore, not requirements: sound and waveforms as a motif; two
voices meeting; colour that tells "my side" from "their side"; a record button
that feels alive. Please propose **two or three distinct style directions**
before designing every screen.

## 5. What exists today

Two screens, plain and unstyled:

**Record your voice** (first launch). A title, why the recording is needed, a
note on where it goes, a passage to read aloud (about 20 seconds), a timer and
progress while recording, then listen, record again, or keep it. Under 10
seconds, or silence, cannot be kept, with the reason. "Not now, use the
phone's voice" skips it. Current wording:

> Your translations can be spoken in your own voice. To do that, the app needs
> a short recording of you reading the passage below.
>
> The recording stays on this phone. When a translation is spoken, it goes
> with the text to our server, and from there to Fish Audio, the service that
> makes the speech. Our server deletes it straight after. You can delete it
> from this phone at any time.

The passage:

> When I travel, I like to talk with the people I meet about their food, their
> music, and the places they love. A good conversation makes the world feel
> smaller. I am recording this so that when my words are translated, they can
> still be spoken in my own voice.

**Translate** (the interpreter). From and To language pickers; a choice of
where speech is turned into text (Automatic, This device, Server) with a
line explaining it; the on-device speech model's status (download, progress,
ready) and the phone's measured speed; a Record/Stop button; a status line;
the transcript; the translation; technical details (where the text came from,
how long it took, a confidence figure); a note when the result was sent to
the server and why; errors in red.

Several of those are for developers and should leave the main screen: the
route choice, the model's speed, the confidence and timing details, and the
routing notes belong in Settings, under "Advanced". The model download (57 MB,
once) should become a calm step: during onboarding, or in the background with
a gentle prompt.

## 6. The core moment, step by step

This is the heart of the design. One person speaks; the app turns it into the
other language, in their voice. Every step below happens today, with real
timings. **The person must always know what is happening**, because some
steps take seconds. Today the worst gap is the voice: 2 to 5 seconds of
nothing while it is being made.

| Step | What happens | Typical time | What the person should see or hear |
| --- | --- | --- | --- |
| Recording | They speak, up to 30 seconds | as long as they talk | that it is listening: level or waveform, a timer, the limit coming |
| Too short | They tapped and let go at once | under 0.3 s | a gentle "hold on a little longer" |
| Turning speech into text | On the phone, or on the server when the phone cannot | about a second on the phone; a few on the server, with words appearing one by one | the words arriving, or progress |
| First use of a language | The phone downloads that language once; on iPhone, the system asks in its own sheet | seconds, once | why, and that it happens only once |
| Translating | | under a second | barely noticeable |
| Making the voice | Their voice saying the translation, made on the server | 2 to 5 seconds | that their voice is being prepared ("in your voice"), not frozen |
| Speaking | The translation plays in their voice | as long as the sentence | what is playing, and a way to stop or replay it |
| Spoken by the phone instead | No voice recorded yet, no internet, or the voice service failed: the phone's built-in voice says it | immediate | subtly, that this is the phone's voice, not theirs |
| Done | Both texts stay on screen: what they said, and the translation | | the original and the translation, clearly paired |

What can go wrong, with today's wording:

- "Microphone permission is needed to record"
- "The recording was silent (−60 dB). Check the microphone"
- "No speech was recognised"
- "The speech could not be made out. Please try again" (the model produced
  nonsense)
- "Connection lost. Reconnecting" (it carries on by itself)
- A failed translation keeps the transcript on screen, with the reason
- "This iPhone has no voice for Klingon" (the text stays; only the sound is
  missing)

In conversations between two people (section 7), the same steps happen, split
between the sender's phone and the listener's.

## 7. Screens to design

### Now: version 1, in this order

1. **Onboarding**
   - Welcome: the promise, in a sentence and a moment of sound.
   - Sign in: Sign in with Apple, Sign in with Google; email later.
   - Your name.
   - Your ID: a unique handle others find you by (for example @pelumi),
     checked as they type.
   - Your language: the one you speak, searchable.
   - Record your voice: the passage, a timer, progress, listen, redo, keep.
     Consent: the voice is now kept on the server, so others can hear your
     messages in it. Plain words: what is kept, what it is used for (only
     your own words), and that you can delete it any time.
   - Optional: download the speech model (57 MB; suggest Wi-Fi).

2. **Home: your conversations.** A list of people, with the last message.
   An empty state that invites adding someone.

3. **A conversation (voice notes).**
   - Their voice notes: play in their voice, in your language. The
     translated text shown; the original one tap away.
   - Your voice notes: what you said, its transcript, and its progress
     (sending, translating, making the voice for them, delivered).
   - The record control: tap to start and stop, or hold to talk (please
     propose which); 30 seconds at most; a way to cancel.
   - Long press on a message: replay, show the original, copy, report.
   - Each person's language visible, so you know what they will hear.

4. **Add someone.** By ID, or by QR code: show mine, scan theirs.

5. **Interpreter (face to face).** Today's Translate screen, redesigned:
   two languages, swap, one big record control, what was said and its
   translation, spoken in your voice. Consider a two-person mode for passing
   the phone across a table.

6. **Profile and settings.**
   - Your name, ID, language.
   - Your voice: play it, record again, delete it.
   - Account: sign out, delete account.
   - Privacy: what is kept and why, in plain words.
   - Advanced: where speech is turned into text (Automatic, This device,
     Server); the speech model (status, size, delete, download again); the
     phone's measured speed.

### Later: keep room for them in the design system, low detail is fine

7. **Text messages:** type in your language, they read it in theirs; tap to
   hear it in your voice.
8. **Safety and discovery:** block, report; finding new people, including at
   random.
9. **Calls for premium users:** live, each side hearing the other in their
   own language and the speaker's voice; an upgrade screen.

## 8. Languages

- Speech is understood in about 99 languages.
- On iPhone, translation covers 22: Arabic, Chinese, Danish, Dutch, English,
  French, German, Hindi, Indonesian, Italian, Japanese, Korean, Norwegian,
  Polish, Portuguese, Russian, Spanish, Swedish, Thai, Turkish, Ukrainian,
  Vietnamese. On Android, 59. Messages between people will be translated on
  the server, with wider and equal coverage on both phones.
- The voice can speak more than 80 languages.

Design language pickers for long lists, search, and names shown in the
person's own language.

## 9. Words

Short, plain, friendly, honest. Say exactly where a voice goes and why, the
way the current onboarding note does. Never blame the person for an error;
say what to do next.

## 10. What we would like back

1. Two or three style directions on a few key screens (onboarding voice
   recording, a conversation, the interpreter), to choose from.
2. For the chosen direction:
   - Design tokens: colour (light and dark), type scale (with large text),
     spacing, corner radius, elevation, motion.
   - Components: the record control in every state (idle, recording, too
     short, working, preparing the voice, speaking), waveform and level
     meter, voice-note bubbles (mine and theirs, in every state of section
     6), language picker and language chip, progress and loading indicators,
     empty states, error and notice styles, buttons.
   - Every version-1 screen in section 7, light and dark, on iPhone and on
     Android.
   - The states of section 6, including errors, as frames or a prototype.
   - Motion notes: recording, the wait while the voice is made, the moment
     it starts speaking.
   - An app icon and a wordmark.
3. Notes for building it: which native components to use on each platform,
   and anything that will need custom drawing.
