# ADR 0026: Push notifications through APNs and FCM

Status: accepted, 2026-11-15

## Context

A voice note reaches a phone live only while the app is open: the event
stream (ADR 0020) stops when the app goes to the background. Someone who is
not in the app never learns that a note is waiting. Checkpoint 14.5 is the
notification that tells them.

Every iPhone notification goes through Apple's push service (APNs), and
every Android one through Google's Firebase Cloud Messaging (FCM). The
choice is whether the server talks to each directly, or sends everything
through FCM, which can forward to APNs.

Calls (milestone 17) will need to ring an iPhone. That takes a VoIP push
(PushKit), which only APNs delivers; FCM cannot send one.

## Decision

**The server talks to each service directly** (the owner's choice,
2026-11-15): APNs for iPhones, FCM for Android phones. Both are HTTPS APIs
called with Go's standard library (`internal/push`). The server signs its
own tokens: an ES256 token from the developer account's APNs key for Apple,
and an RS256 token from the Firebase service account for Google, which it
exchanges for an access token. No vendor SDK is needed on the server. On
Android, `firebase-messaging` receives the messages, with the
`google-services` Gradle plugin (both approved 2026-11-15). The iPhone needs
nothing beyond the system's own frameworks.

**On Android the address is the Firebase installation ID.** Firebase
Messaging 26 retires registration tokens: the app calls `register()` on every
launch and is told its installation ID in `onRegistered()`, and the server
sends to it with the `fid` field of FCM's API, in place of `token`. The
manifest turns this on (`firebase_messaging_installation_id_enabled`).

**A phone's address belongs to its session.** A signed-in phone sends its
address (`PUT /v1/sessions/current/push`: `apns`, `apns-sandbox` or `fcm`,
and the token). Signing out deletes the session and the address with it, so
a phone never hears about an account it has left. An address given by a new
session moves to it: the same phone, signed in again. An address the service
says is gone (the app deleted) is forgotten.

**A notification is a job.** When a note becomes ready for the listeners of a
language (at once for those who speak the sender's language, after
translation and speech for the others), the same transaction adds a `push`
job. A worker sends it to every phone of those listeners, with the queue's
leases and retries. A retry sends to every phone again; each notification
carries the message's ID (APNs collapse ID, a tag on Android), so a repeat
replaces the first instead of showing twice.

**What it says** follows the design brief (section 3.11): the sender's name,
then "Sent a voice note · 0:14". Not the words: they would sit on a locked
screen. Android receives a data message and builds the notification itself;
the iPhone receives Apple's alert. Either way the phone stays quiet about a
note in the room that is open on its screen, and opening a room clears its
notifications.

**Permission** is the system's own question, asked the first time the rooms
are shown. The screen before it, which the brief asks for, is not designed
yet; when it is, it goes in front of the same question.

**Keys.** The APNs key (`AuthKey_<id>.p8`) and the Firebase service account
key are secret and stay out of the repository and out of chat: on the Mac in
`~/keys/shinar/`, on the host in `/etc/shinar/` (root:shinar, 640). The
settings name the files: `APNS_KEY_FILE`, `APNS_KEY_ID`, `APNS_TEAM_ID`
(defaults to the team, 4USZRKF452), `FCM_SERVICE_ACCOUNT_FILE`. With none of
them the server runs as before, without notifications. `make push-check`
signs in to both services and sends to a made-up phone, which proves the keys
work without anything arriving anywhere. `google-services.json` is kept out
of git too (it names the Firebase project, and the repository is public); an
Android build without it has no Firebase and so no notifications.

## Alternatives considered

**FCM for both phones.** One API and one key, but the iPhone app would need
the Firebase SDK, and calls would still need APNs directly for the VoIP push,
leaving two paths to Apple.

**A hosted push service** (OneSignal and the like). Another company holding
the phones' addresses and seeing every notification, for something two HTTPS
calls do.

**Sending from the deliverer, after the commit, without a job.** Simpler, but
a push lost to a restart or a busy service would be lost for good, and the
API process, where same-language notes become ready, would need the keys too.

## Consequences

- Notifications work once the keys are in place on the host. The Firebase
  key is (checked with `make push-check`); the APNs key waits on the owner.
- iPhone builds run from Xcode register with Apple's test service
  (`apns-sandbox`), TestFlight and App Store builds with the real one. The
  App ID needs the Push Notifications capability; the GitHub workflow's
  manually made provisioning profile must be made again to include it.
- Group rooms (milestone 20) need no change: the job already reaches every
  listener of a language.
- Calls (milestone 17) add VoIP pushes beside these, to the same APNs client.
- A notification on a locked screen still shows who sent a note. People who
  mind can hide previews in their phone's settings.
