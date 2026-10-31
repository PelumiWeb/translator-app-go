# Design brief, round 2: signing up, finding someone, and your rooms

For Claude Design. Written 2026-10-31. It follows the first brief
(`brief.md`) and the approved direction, **Lagoon / Pearl** (handoff in
`pearl/README.md`). Pearl is built in both apps, dark only, and the owner has
approved it on a real iPhone. This round designs the screens Pearl did not
cover, in Pearl, so that people can sign up and talk to each other.

## 1. Where the product is

A voice app for talking across languages (working name Timbre; the name is
not final). You speak in your language; the other person hears it in theirs,
in your own voice. That works today in the interpreter, face to face on one
phone.

Now it becomes social. Each person signs up with their name and their voice,
finds someone by their ID, and the two talk in voice notes in a **room**. A
room is a conversation. For now a room has exactly two people; group rooms
come later, so keep room for them (section 5).

How a voice note travels: I record and send. The server translates my words
into the listener's language and speaks them in my voice. The listener's
phone plays it, with the translated text shown and my original words a tap
away. It takes a few seconds, and the design must show that wait honestly.

## 2. What is already designed (do not redesign)

- The visual system: tokens, type, spacing, motion (`pearl/README.md`).
- Record your voice (onboarding), the conversation screen, the interpreter.
- Components: record control, composer mic, their note card, my note card
  in the "preparing your voice" state, dot waveform, conversation header,
  language chips.

Keep the rule of two meaning colours: warm white is you, mint is them.

## 3. Screens to design

Dark mode only. Light mode stays proposed for now. For each screen: iPhone
(SwiftUI, iOS 26, Liquid Glass where it fits) and Android (Compose,
Material 3), each in its own platform's patterns.

1. **Welcome.** The first screen. The promise in a sentence, and a way to
   hear it: a short sample of a voice speaking two languages.

2. **Sign in.**
   - iPhone: Sign in with Apple. Apple's button rules apply (its own button
     styles, its own wording).
   - Android: Sign in with Google. Tapping it opens Android's own account
     sheet, drawn by the system; design the screen around it, not the sheet.
   - Email sign-in comes later; leave room for a third, quieter option.
   - Failure: sign-in cancelled, no connection, could not be checked.

3. **Your name.** Prefilled when Apple or Google gives it. Up to 50
   characters.

4. **Your ID.** The handle others find you by, shown as @handle. 3 to 20
   characters: lower-case letters, digits, dots and underscores. Checked as
   the person types: available, taken (with a suggestion), not allowed (say
   which rule). It can be changed later.

5. **Your language.** The one you speak. A searchable list with each
   language named in its own language ("Español", "Yorùbá").

6. **Your voice: consent, then recording.** The recording screen is designed;
   the consent screen before it is not. It must say, plainly:
   - your voice recording is kept on our server, so that the people you talk
     to hear your messages in your voice;
   - it is used only to speak your own words, never anyone else's, and it is
     never given to other people;
   - you can listen to it, record it again or delete it at any time, and it
     is deleted with your account.
   Two choices: agree and record, or not now. Someone who skips can still
   talk; listeners hear their messages in a standard phone voice, and their
   profile invites them to record later.
   After recording (designed): listen, record again, keep.

7. **Home: your rooms.** A list of the people you talk with. Each row: name,
   their language, the last message (its translated text, or "Voice note
   · 0:12"), when, unread, and the state of my last note if it is still on
   its way. Empty state: invite to add someone, with my own ID and QR code
   to share. From here: add someone, the interpreter (the face-to-face mode
   stays), and my profile. Please propose the navigation: tabs, or a list
   with a toolbar.

8. **Add someone.**
   - By ID: a search field; the result shows name, @handle and the language
     they speak; one action, "Send a voice note", which opens the room.
     Not found: say so kindly.
   - By QR code: show mine, scan theirs (camera permission, and what to do
     when it is refused).
   - The room opens at once: the first voice note reaches them straight
     away, with no request to accept. (Blocking and reporting come later.)

9. **The room, its missing states.** The conversation screen is designed;
   these states are not:
   - my note: sending, translating, preparing your voice (designed),
     delivered, heard, failed (with retry; the transcript stays);
   - their note: new, playing (designed), played;
   - a note in a standard phone voice, because its sender has no recorded
     voice: say so, subtly;
   - an empty room, before the first note;
   - recording in the room: the composer mic while recording, with the
     30-second limit and a way to cancel.

10. **Profile and settings.** Today's settings screen was built from Pearl's
    parts without a design; redesign it with:
    - name, @handle, language, each editable;
    - your voice: play it, record again, delete (with a confirmation that
      says what deleting means);
    - account: sign out; delete account (a confirmation that says
      everything goes: profile, voice, rooms);
    - privacy: what is kept and why, in plain words;
    - advanced (already exists): where speech becomes text (Automatic, This
      device, Server), the speech model, the phone's measured speed.

11. **Notifications.** The screen before the system asks for permission (why
    the app wants to notify), and the wording of a notification for a new
    voice note ("Rosa sent a voice note · 0:14").

12. **Errors and notices.** Not designed yet. Pearl's README suggests warm
    white text with an outline icon, no red alarm styling, never blaming the
    person. Show it for: no connection, sign-in failed, ID taken, voice too
    short or silent, a note that could not be sent, the server out of reach.

## 4. Facts that shape the screens

- A voice note is at most 30 seconds. A voice recording for sign-up is 10 to
  30 seconds, reading a passage.
- From sending to the listener hearing it takes a few seconds: translation,
  then the voice is made, usually 2 to 5 seconds.
- Speech is understood in about 99 languages; the voice speaks more than 80.
  Messages are translated on the server, so both phones offer the same
  languages.
- Sign-in on iPhone is Apple; on Android, Google. One person on two kinds of
  phone has two accounts for now.

## 5. Later: leave room in the system, low detail is fine

- **Group rooms.** Several people, each in their own language. With only two
  meaning colours (you and them), how does a group tell its members apart?
  A suggestion is welcome: names on cards, for instance, rather than a third
  colour.
- **Text messages:** type in your language, they read it in theirs, tap to
  hear it in your voice.
- **Safety:** block and report.
- **Calls** for premium users.

## 6. What we would like back

1. Every screen in section 3, in Pearl dark, on iPhone and on Android, with
   the states listed.
2. New components: the rooms list row, the ID field with its checking
   states, the search result card, QR show and scan, the consent layout,
   the error and notice styles, the remaining states of my note card,
   confirmations.
3. The app icon and wordmark, still missing from round 1.
4. Notes for building it: native components on each platform, and anything
   that needs custom drawing.
