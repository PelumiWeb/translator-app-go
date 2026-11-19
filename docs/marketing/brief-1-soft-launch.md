# Marketing brief, round 1: the soft launch

For Claude, taking over Shinar's marketing, or anyone writing to its first
users. Written 2026-11-19 from the code and the project's records, so every
fact here is what the app does today. When something here and a draft
disagree, this wins; when this is out of date, ask the owner.

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

- The promise: **"Talk to anyone. Be heard in your own voice."**
- The sub-line: "You speak your language. They hear theirs, in your voice."
- The name: Genesis 11. Shinar is the plain where everyone still shared one
  language, before Babel. The app undoes the scattering.

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
| Website | A home page telling a voice note's journey, with a waitlist form. Being put on Vercel; **its address is still to come** (see section 10) |
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
- Concrete, not abstract: a grandmother's note, a garden in August, not
  "breaking down language barriers at scale".
- Honest about the wait, the limits and the test.

The app's words to reuse: room, voice note, message, your ID, record your
voice, "in your voice", "Show original", "the phone's voice", "Try it first".

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
4. **A tester FAQ**, from sections 4 to 6: "Why does the other person need
   it too?", "Why does a note take a few seconds?", "What happens to my
   voice?", "Why did I hear the phone's voice?", "I reached the limit",
   "How do I leave?".
5. **A feedback ask** for after the first week: three or four questions the
   owner can send, about whether people kept talking, which languages, what
   broke, and whether they would want calls.

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

1. **The website's address.** The site and its waitlist are built and
   going onto Vercel, at a `vercel.app` address until a domain is bought.
   Don't publish a link before the owner gives one.
2. **The contact address** for privacy questions and removal from the
   waitlist. Not chosen yet.
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
7. **Age.** No minimum age has been decided. Leave age out until it is.
