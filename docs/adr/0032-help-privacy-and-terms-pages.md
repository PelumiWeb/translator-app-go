# ADR 0032: Help, privacy and terms pages, and a minimum age of 16

Status: accepted, 2026-11-19

## Context

The soft launch has begun (ADR 0030). The marketing work wrote a tester FAQ,
and the owner asked for it on the website, with "all the screens Apple will
ask for" before launch. App Store Connect asks for a privacy policy URL and a
support URL (TestFlight's test information has a field for the privacy
policy too). Apple's guideline 1.2, for apps where people contact each
other, asks for published contact details, a way to report abuse with a
timely answer, a way to block people, and a way to filter what is sent;
reviewers also ask for terms that have no tolerance for abuse. Both
onboarding screens already say "By continuing you agree to the Terms and
Privacy Policy" (milestone 13), as plain text, since there were no pages.

The pages need two facts no record had settled. The owner decided:

- **Shinar is for people 16 and over.** Not 13: a voice recording is treated
  like biometric data in some places, and the app has no way for a parent to
  agree.
- **The pages name "Shinar"**, with the contact address, and no person or
  company for now.

## Decision

**Three pages on the website**, from one layout (`web/components/TextPage`):
`/support` (help: joining, feedback, reporting someone, deleting a voice or
an account, leaving the test or the list), `/privacy` and `/terms`. The home
page gets the questions (`#faq`), the tester FAQ plus who can try it and
what it costs. Every page has the same footer linking all four.

**The privacy page says only what the code does**, checked against the
server and the apps' privacy pages on 2026-11-19: what is kept and why, the
services named as the apps name them (Fish Audio, Azure AI Translator,
Groq, Google ML Kit, Apple and Google for sign-in and notifications,
TestFlight, Vercel), that messages are not end-to-end encrypted, how long
each thing is kept (the interpreter's recordings until the work is done,
its speech an hour, backups up to 35 days), and the minimum age.

**The terms** add what the product needs that a generic page would not:
only your own voice, never someone else's, and never to make anyone believe
someone said something they didn't; no tolerance for abuse; translations can
be wrong, so not for medical, legal or safety matters. Reports come by email
to the contact address, looked into within 24 hours, until the app has a
report button (milestone 16).

**The contact address is required**: a text page will not build without
`CONTACT_EMAIL`. `MINIMUM_AGE` and `PAGES_UPDATED` live in
`web/lib/site.ts` with the name.

## Alternatives considered

**A generated policy from a template service.** Faster, but generic: it
would not say what is kept of a voice, which services see what, or that the
server reads messages, which is the part people need to trust.

**Waiting for milestone 18**, which planned these pages. The soft launch is
happening now, and TestFlight's external testers are outside people.

## Consequences

- **Not enough for the App Store yet.** Guideline 1.2 asks for reporting and
  blocking in the app (milestone 16); email reports cover the soft launch
  only. App Review will also ask the age rating questions and App Privacy
  answers, which follow from the privacy page.
- **The owner's commitments, in public:** reports looked into within 24
  hours; an account deleted on request for someone who can't open the app;
  waitlist addresses kept until Shinar is out and people have been told. The
  server has no command to delete an account by ID yet, so that is by hand
  in the database until there is one.
- **Not decided, and left out of the terms:** which country's law governs
  them, and who runs Shinar. Both before the App Store.
- The apps do not ask anyone's age; the minimum age is stated in the terms
  and will be in the App Store's age rating.
- The onboarding line "By continuing you agree to the Terms and Privacy
  Policy" can now link to these pages, on both phones. Not done: the links
  would carry the `vercel.app` address until there is a domain.
- When what the app keeps, or who helps, changes, the privacy page changes
  in the same milestone, as the apps' privacy pages do.
- The waitlist's hidden field is now called `leave_empty` and labelled
  "Leave this empty", so no browser fills it in as a company name and drops
  the sign-up. The API's field keeps its name, `company`.
