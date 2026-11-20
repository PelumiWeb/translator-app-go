# Marketing brief, round 1: the soft launch

For Claude, taking over Shinar's marketing, or anyone writing to its first
users. Written 2026-11-19 from the code and the project's records, so every
fact here is what the app does today; section 11, on the apps around Shinar,
added 2026-11-20. When something here and a draft disagree, this wins; when
this is out of date, ask the owner.

Read with it: `docs/design/brief-4-motion.md` (the story of a voice note,
the visual language, and what not to show), and `docs/design/pearl/README.md`
(the design system) when making anything visual.

## 1. Shinar in a paragraph

Shinar lets people talk to anyone in the world without sharing a language.
You speak in your language; the other person hears it in theirs, **in your
own voice**. They answer in theirs; you hear it in yours, in their voice.
Each person signs up with their name and a 20-second recording of their
voice. Today people talk in voice notes and text messages, in a **room** of
two people. There is also a face-to-face interpreter on one phone.

**The theme is breaking the language barrier** (the owner, 2026-11-19, ADR
0033). It is the whole point of the app, so every piece says it, in words or
by its example: two people who share no language, talking anyway.

- The promise: **"Break the language barrier. Be heard in your own voice."**
  The same words open the app (its Welcome screen) and the website.
- The sub-line: "Talk to anyone. You speak your language. They hear theirs,
  in your voice."
- The name: Genesis 11. Shinar is the plain where everyone still shared one
  language, before Babel: the story of how the language barrier began. The
  app undoes the scattering.

**The name is not final.** A trademark check is still to come. Keep the name
in one place in anything you make (a variable, one line, the end card), so
it can be replaced without rewriting.

## 2. Where things stand (2026-11-19)

| | Today |
| --- | --- |
| Stage | A soft launch: a small group of invited testers, before the stores |
| iPhone | Testing through **TestFlight**, Apple's app for trying apps before release. Needs iOS 18 or later: the iPhone XS and XR and everything newer |
| Android | Built, but **not offered to testers yet**. Android sign-ups on the waitlist are kept for later. Do not give a date |
| App Store, Google Play | Not yet. No store badges, no "download now" |
| Price | Free while testing. Say nothing about later prices or plans |
| Website | https://web-nine-phi-84.vercel.app, until there is a domain. The home page: the promise; who it's for; a voice note's journey (`/#how`); three ways to talk; what is kept of your voice; the questions (`/#faq`); how joining goes, and the waitlist (`/#join`). Then help, privacy and terms pages (`/support`, `/privacy`, `/terms`). A link to it shows a preview picture: the promise and Rosa's note, Spanish to English |
| How people join | They leave their email on the website's waitlist. The owner invites them in batches, and TestFlight emails each one an invitation |

## 3. Who the first users are

Not yet settled with the owner (see section 10). What the product is built
around, and the example it uses everywhere:

- **Families split by language.** Rosa, in Madrid, speaks Spanish; her
  grandson, in Lagos, speaks English. He hears her notes in English, in her
  voice. Grandparents and grandchildren, in-laws, families who moved.
- People with **friends or partners abroad** whose language they don't
  share.

A soft launch works only in **pairs**: both people need Shinar to talk to
each other. Ask people to join with the person they want to talk to.

The website says it the same way, under "Made for people a language apart":
a grandmother and her grandson, your partner's family, the friend who moved
home. Only Rosa's pair names languages, since only Spanish and English have
been heard in her voice; do the same until other pairs have been heard.

A go-to-market report on the website exists (explee.com, made 2026-11-20).
It sits behind a browser check, so it is not in this brief yet. When the
owner shares its text, its audiences and channels belong here and in
section 10.

## 4. What a tester can do today

On the iPhone, through TestFlight:

- **Sign up** with Sign in with Apple. Choose a name, an ID that others find
  you by (such as @rosa.m), and the language you speak.
- **Record your voice** once: read a short passage aloud, about 20 seconds,
  and agree to it being kept. Or skip it: people then hear your messages in
  their phone's own voice.
- **Add someone** by their ID, or by scanning their QR code. A room for the
  two of you opens straight away; there is no request to accept.
- **Send voice notes** of up to 30 seconds. While recording: pause, resume,
  stop and listen back, delete or send. The other person hears it in their
  language, in your voice. The words light up as it plays, and the original
  words are a tap away.
- **Send text messages.** They are translated for the other person, who can
  tap to hear them in your voice.
- **Get a notification** when a message arrives and the room isn't open. It
  shows the sender's name and, for a voice note, its length. Never the words.
- **Use the interpreter**: two people face to face, one phone, each speaking
  their own language.
- **Change** your name, ID, language or voice; **delete** your voice; **delete**
  your account and everything in it.

The room has a call button. It says calls are coming. Calls are not built;
do not mention them (section 7).

## 5. What testers should expect

Tell people these plainly, before they find them out:

- **A voice note takes a few seconds to arrive.** It is written down,
  translated, then spoken in the sender's voice, which is the longest wait,
  2 to 5 seconds. It is not a live call.
- **Some languages work better than others.** Shinar understands speech in
  many languages, but recognition is weaker in some, Yorùbá and Hausa among
  them, and Igbo is not in the list yet. Translation between two languages
  that are not English goes through English on the interpreter, so it is
  plainer. Do not give a number of languages.
- **Sometimes the phone's voice speaks instead of theirs.** When their voice
  can't be made, or they chose not to keep one, the listener's phone reads
  the words aloud. The card says so.
- **Daily limits.** Each person can send up to **10 minutes of voice notes
  and 100 text messages in any 24 hours**. Each voice note is up to 30
  seconds; each message up to 1,000 characters. Past the limit the app says
  so, and the note waits on its card to be sent later.
- **A shared monthly allowance.** During testing, translation runs on a
  small allowance shared by every tester. If it runs out before the month
  ends, translation stops for everyone until the next month. Say this
  honestly as "it's a test"; the owner can tell you if it gets close.
- **First-time downloads.** The phone downloads a speech model (57 MB) to
  write down speech by itself, and the interpreter downloads a language pack
  (about 30 to 50 MB) for each language. Wi-Fi helps.
- **Only people you know.** You can't be found unless someone has your ID,
  and there is no block or report button yet. For now, talk with people you
  know.
- **TestFlight builds expire after 90 days.** New versions arrive through
  TestFlight; testers should keep its notifications on.
- **Rough edges.** It is a test. Things will break, and that is what the
  owner wants to hear about.

## 6. What happens to people's data

In plain words, from the privacy page in the app (Profile, then Privacy).
Use these facts; don't make new promises.

- **Your account:** your name, ID and language, so people can find you and
  hear you in their language. Your email only if Apple shares it, to
  recognise your account.
- **Your voice:** the recording you made, kept on Shinar's server with the
  date you agreed. Used only to speak your own words when someone plays a
  message you sent. Never given to other people.
- **Your messages:** what you record or type in a room, the words written
  down, their translation, and the translation in your voice. Kept so the
  people in the room can read and play them.
- **Writing down your words:** your phone does it by itself when it can;
  otherwise the recording goes to Shinar's server to be written down.
- **Deleting:** delete your voice in Profile, and your messages use the
  phone's voice. Delete your account, and your profile, voice and everything
  you said in your rooms go with it.
- **Not end-to-end encrypted.** The server reads the words in order to
  translate them. Never claim otherwise, in words or with lock icons.
- **The waitlist:** the email (and an optional first name) is used only to
  invite the person and to tell them when Shinar is out. For an iPhone it is
  given to Apple, whose TestFlight sends the invitation. It is never given to
  anyone else. People can ask to be taken off the list.
- **TestFlight** shows the owner each tester's name, email, when they
  installed, how often they open the app, and crash reports. That is Apple's
  and the tester's arrangement, but say so if asked.

The services behind each stage (translation, the voice, writing down speech)
are named on the app's privacy page. Don't name them in marketing: they are
suppliers and can change. If a tester asks, answer truthfully and point to
the privacy page.

## 7. What not to say or show

From brief round 4, section 8, with what the soft launch adds:

- **Not instant**, and not a call. No "real-time", no "live".
- **Not end-to-end encrypted.**
- **No number of languages.**
- **No prices, plans or "premium".**
- **Nothing not in testers' hands:** calls, group rooms, finding new people,
  Android testing. Leave them out rather than saying "coming soon". The one
  exception is the waitlist's own line for Android: "Leave your email too,
  and we'll write when it's ready for Android."
- **No supplier names** (section 6).
- **No store badges**, no "on the App Store".
- **No made-up voices.** Any sample of Shinar speaking must be real output,
  from a real person who agreed to it. Never imitate a known person's voice.
  There is no real sample yet.
- **Not "clone", "deepfake" or "AI voice"** (a recommendation, to confirm
  with the owner). Shinar's own words are "in your own voice", "your voice,
  made from the recording you gave", "with your agreement". The consent is
  part of the product; keep it in the story.

## 8. Voice and tone

The app's own writing is the model:

- Plain, warm and short. Sentences a grandparent would read without help.
- Sentence case everywhere, including buttons and headings.
- Language names in their own language: Español, Yorùbá, Deutsch. No flags:
  a language is not a country.
- "They" for anyone whose pronouns you don't know, including in examples.
- Name the language barrier plainly, then show it: Rosa speaks Spanish, her
  grandson speaks English, and a voice note crosses between them. "The
  language barrier" is a phrase everyone knows; say it in those words, not
  in abstractions such as "at scale", "seamless" or "frictionless".
- Concrete, not abstract: a grandmother's note, a garden in August.
- Honest about the wait, the limits and the test.

The app's words to reuse: room, voice note, message, your ID, record your
voice, "in your voice", "Show original", "the phone's voice", "Try it first",
"break the language barrier".

When the store listing is written (not yet, section 2), "Break the language
barrier" fits the App Store's 30-character subtitle.

Lines that set Shinar apart from the apps around it (section 11), without
naming them:

- "For the people you love who speak another language." Not travel phrases,
  not customer service.
- "You don't have to be awake at the same time. Send a note; they hear it in
  their language, in your voice, when they're free."
- "Translated, and still in your voice."

## 9. What we'd like from you

1. **The waitlist announcement**: a short post for each channel the owner
   uses (section 10), pointing to the website, and a version for messaging
   friends and family directly. Ask people to join in pairs.
2. **The invitation's companion text.** The invitation email itself comes
   from Apple and can't be changed, but the owner can write the app's beta
   description and "What to Test" in App Store Connect, which testers see in
   TestFlight. Draft both (under 4,000 characters each): what Shinar is,
   who to invite, what to try first, how to send feedback.
3. **A welcome note** for each batch, sent by the owner: how to install
   TestFlight and accept the invitation, how to sign up, and to invite the
   person they want to talk to.
4. **A tester FAQ** (done, and now on the website's home page), from sections 4 to 6: "Why does the other person need
   it too?", "Why does a note take a few seconds?", "What happens to my
   voice?", "Why did I hear the phone's voice?", "I reached the limit",
   "How do I leave?".
5. **A feedback ask** for after the first week: three or four questions the
   owner can send, about whether people kept talking, which languages, what
   broke, and whether they would want calls.

**Link previews.** A link to the website shows a picture (1200 by 630): the
promise, and Rosa's note going from Spanish to English. It is drawn from the
site's own words, so it changes when they do. Before a post goes out, paste
the link in a chat to yourself and look at the preview; WhatsApp and X keep
an old preview for a while after a change.

**How testers send feedback:** in the TestFlight app, open Shinar and choose
"Send Beta Feedback", or take a screenshot in Shinar and share it as beta
feedback. It reaches the owner in App Store Connect, with the screenshot.

**How someone joins, step by step** (for the welcome note):

1. Leave your email on the website, choosing iPhone.
2. When your turn comes, an email arrives from TestFlight. Open it on your
   iPhone.
3. Install TestFlight from the App Store if you don't have it, then accept
   the invitation and install Shinar.
4. Open Shinar, sign in with Apple, and choose your name, ID and language.
5. Record your voice (about 20 seconds), or use the phone's voice instead.
6. Add the person you want to talk to by their ID, or scan their code, and
   send your first voice note.

## 10. Still open, for the owner

Ask before relying on any of these:

1. **The website's address** is https://web-nine-phi-84.vercel.app until a
   domain is bought. Keep it in one place in anything you make, as with the
   name, so it can be replaced.
2. **The contact address** for privacy questions and removal from the
   waitlist is the one shown under the website's waitlist form. Use that
   one, and ask the owner before giving any other.
3. **The audience and the channels**: who first, and where (WhatsApp groups,
   Instagram, X, LinkedIn, family and friends).
4. **The first batch's size.** On the shared free allowance (section 5) a
   small start is safer: about 20 to 30 people, in pairs, then more as the
   owner watches how much is used.
5. **Apple's beta review.** The first build for outside testers goes through
   Apple's review, which usually takes a day or two. Don't promise a date
   for invitations before it has passed.
6. **A real voice sample** for the website and any video: whose voice plays
   Rosa, from someone who agrees.
7. **Age.** Shinar is for people 16 and over (decided 2026-11-19). Say so
   where age comes up, and don't aim anything at younger people.

## 11. The apps around Shinar

Looked at on 2026-11-20, from their websites (https://voicelink.tech,
https://www.1voice.ai). Shinar is not the only app that speaks across
languages. Know what the others say, so that Shinar says something else.
Never name them in public posts.

| | Voicelink | 1Voice | Shinar |
| --- | --- | --- | --- |
| Who it is for | Travel, family and friends, work | Businesses: customer service, sales, bookings | People who already know each other and don't share a language |
| How people talk | Live, face to face: "manual" (record, then hear it) or "autopilot" (it listens and translates as you go) | Phone calls, with an AI voice answering | Voice notes and text messages between two people anywhere; an interpreter face to face |
| Whose voice the listener hears | The site doesn't say; the app "plays the sentence" in the other language | "Human-like" AI voices | **The speaker's own**, made from a recording they agreed to keep |
| What its site claims | "Speak. Listen. Understand.", real time, a list of languages | 60+ languages, under 50 ms, 99.9% accuracy, 1M+ users, compliance badges, customer logos | Only what the app does today (section 7) |
| Price | 10 free minutes a day, ads for more, an unlimited plan; on the App Store and Google Play | A free trial, then sales | Free while testing; iPhone, by invitation |

**What Shinar has that neither site says:**

1. **Your own voice, with your agreement.** Neither promises that the
   listener hears the speaker's voice. That is Shinar's whole promise; lead
   with it, and keep the consent in the story.
2. **Two people anywhere, not at the same time.** They sell "instant" and
   "real time"; Shinar can't, and shouldn't pretend to. A voice note waits
   until the other person is free, so a grandmother and a grandson in
   different time zones can still talk. Say the wait plainly (a few seconds),
   and say why notes suit families.
3. **For people you know.** Not phrases for a holiday, not a call centre.
   You add someone by their ID; nobody can find you without it.
4. **Plain and honest.** Their pages are full of numbers and badges.
   Shinar's has a real example, real limits, and a privacy page that names
   who helps. That difference is worth keeping.

**Borrowed for the website, 2026-11-20** (all within section 7):

- A start in three steps, as Voicelink has ("download, choose two languages,
  start speaking"): the waitlist now says how joining goes, with the pair
  first.
- Who it's for, by relationship (Voicelink's travel, family, work): only the
  people part.
- The ways to talk, as Voicelink shows its two modes: voice notes, text
  messages and the interpreter.
- One short line under the button on what trying it takes, as both have
  ("Free minutes every day", "No credit card required"): "Free while we
  test. On iPhone, by invitation."
- A preview picture for links (section 9).

**Not borrowed, and not to be** (section 7 still holds): "real time",
"instant", a time in milliseconds; a number of languages, users or
countries; accuracy figures; "trusted by" logos and compliance badges; store
badges; plans, ads for minutes, "unlimited"; calls.
