# ADR 0035: Block, report, and acting on reports

Status: accepted, 2026-11-23

## Context

The owner is taking the iPhone app to the App Store (2026-11-21). Apple's
Guideline 1.2 asks four things of apps where people contact each other: a
way to filter objectionable material, a way to report it with a timely
answer, a way to block abusive users, and published contact details. The
contact details have been on the help page since ADR 0032, which also
promised that a report is looked into within 24 hours. Reports came by
email; there was no blocking at all.

In Shinar anyone with your ID can open a room with you and send at once,
with no request to accept (the owner's choice, 2026-10-31). Each room has
two people (group rooms are milestone 20).

## Decision

**A block stops the room of two, both ways.** While either member has
blocked the other, neither can send into it, nor ask for a failed note to be
tried again. The blocker sees the room as blocked (`you_blocked`) and can
unblock. A resend of a note kept before the block is answered as before, as
the daily limits do.

**The person blocked is not told.** Their sends are refused with
`403 cannot_send` and no reason, which their phone shows as a note that
could not be sent. Their room looks as it did, and the room's live stream
tells only the blocker that it changed. Telling the blocked person invites
them to come back under another account.

**A report is about a person or one of their messages, and blocks them
too**, unless the reporter unticks it. The reasons are a short fixed list
(spam, harassment, hate, sexual content, violence, someone at risk, other)
with an optional note. People can report only someone they share a room
with, so there is no reporting a stranger whose ID one guessed.

**A report keeps its own copy of the reported message**: the words, their
translation into the reporter's language, and a copy of the recording, in a
folder of its own (`REPORTS_DIR`). Deleting the message with its sender's
account does not delete what was reported, which would let an abuser
destroy the evidence by deleting their account. The copy is cleared when
the owner closes the report; the report itself stays, with the reason and
what was done.

**The owner hears of each report at once.** Recording a report adds a job,
`report`, on the existing queue, in the same transaction. A worker sends a
push notification to every phone the owner's account is signed in on
(`REPORTS_NOTIFY_HANDLE`): "New report", "A voice note · harassment". Never
the words, which the push services would see. Through the queue, like every
other notification, it is retried when Apple's service is busy.

**The owner acts with commands on the machine**, as with the waitlist
(ADR 0030): `make reports-list`, `make report-close ID=... NOTE=...`,
`make account-suspend HANDLE=...`, and `make account-delete HANDLE=...`,
which only says what would go until it is run again with `YES=1`. A
suspended account is signed out everywhere, cannot sign in
(`403 account_suspended`), and cannot be found by its ID; its messages stay
until the owner decides. Deleting an account on request (ADR 0032 promised
it) goes through the same code as deleting one in the app.
`deploy/waitlist.sh` becomes `deploy/command.sh`, since it now runs both
kinds.

**The filter is the translator's own.** Azure AI Translator is asked to mask
offensive words (`profanityAction=Marked`, with asterisks), in the
listener's language. The translation the listener reads shows `***`; the
voice made from it leaves those words out, and a translation with nothing
left to say goes to the listener's phone instead of the voice engine. The
owner agreed to this on 2026-11-21.

## Alternatives considered

**Telling the person blocked, as a message or a state of the room.** Clearer
for them, and some apps do. But it gives the abuser a signal to make another
account, and Apple asks for blocking, not for announcing it.

**Message requests**: a first note from someone new waits until it is
accepted. A stronger filter, but it reverses the owner's choice of
2026-10-31 that a room opens at once, and the stores ask for blocking, not
for this. It stays possible with discovery (the rest of milestone 16), when
strangers can find each other.

**Keeping the reported audio by not deleting it**, instead of a copy:
`ForgetSender` would skip files that an open report points at. No second
folder, but the rule would be spread over two places, and a report made
while its sender's account is being deleted could lose the file. A copy is
the report's own, whatever happens to the message.

**A word list of our own, or a moderation service** (Azure AI Content
Safety). A list would cover few of the app's 59 languages; a service is a new
dependency and a cost per message. The translator already reads every
translated message, in both languages, at no extra cost.

**Sending the owner's notification from the API process, at once.** No new
job kind, but no retry, and the API process does not hold the push keys
when the server runs as two processes.

## Consequences

- The privacy pages, on the website and in the app, have to say that a
  reported message is kept with its report until the report is closed, even
  if its sender deletes their account. That lands in checkpoint 16.2, in the
  same milestone.
- The filter covers translations only. In a room where both speak the same
  language nothing is translated, so nothing is filtered; blocking and
  reporting cover that. The original words, a tap away, are not filtered
  either. Switching the translator back to Google (ADR 0020) would lose the
  filter, which Google's API does not offer.
- A phone reading a translation aloud in its own voice would read the
  asterisks; the iPhone has to leave them out too (16.2).
- Group rooms (milestone 20) need a rule of their own: there a block will
  hide one person, not stop the room.
- The owner's phone hears of reports only if `REPORTS_NOTIFY_HANDLE` is set
  in `/etc/shinar/env`, and only while the owner is signed in with push on.
  `make reports-list` is the record either way.
- Android has none of this yet: the server refuses its sends in a blocked
  room like any other, and the Android app shows a failed note. Its screens
  come with milestone 22.
