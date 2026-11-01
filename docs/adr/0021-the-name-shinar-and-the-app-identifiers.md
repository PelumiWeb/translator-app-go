# ADR 0021: The name Shinar, and the app identifiers

Status: accepted, 2026-11-01

## Context

The app was built as PTranslate, and the design rounds called it Timbre as
a working name. Round 3 of the design (2026-11-01) names it **Shinar**:
Genesis 11:2, the plain where everyone still shared one language before
Babel. It brings the logo, the app icon and the splash screen. A formal
trademark clearance is still to come, so the handoff asks for the name to
sit behind one constant until it is done.

Each app also has an identifier that cannot change once it is published:
the Android application ID in the Play Store, and the bundle ID in the App
Store. Until now they were `com.example.ptranslate` (Android Studio's
placeholder, which Google Play refuses) and `pelumi.PTranslate`. The
identifiers are also what Sign in with Apple checks tokens against
(`APPLE_AUDIENCE`) and what the Google Cloud Android OAuth client is
registered for, so they had to be settled before that client was made.

## Decision

**The identifier is `com.pelumi.shinar`, the same on both phones** (the
owner's choice, 2026-11-01): the owner's own namespace, like the `pelumi.`
prefix the iPhone app already used. It needs no domain, and stays right if
the name changes after the trademark check. The iOS test bundle is
`com.pelumi.shinar.tests`. The server's `APPLE_AUDIENCE` defaults to it.

**The name is written once per phone.** On Android, the `app_name` string
resource, which the launcher, the permission dialogs and every screen read.
On iOS, the `APP_NAME` build setting, which becomes the display name and
the permission prompts in Info.plist, and which the screens read back from
the bundle. The logo's wordmark is the name in lowercase.

**The link in a person's QR code is `shinar://add/<id>`**, replacing
`timbre://`. No codes have been printed or shared outside development.

**The code keeps its names.** The Kotlin package stays
`com.example.ptranslate` (Gradle's `namespace`, now separate from the
application ID), and the Xcode project, target and folders stay
`PTranslate`. People never see them. Renaming them would move every source
file and the JNI function names for no change anyone can see. They can be
renamed on their own later.

## Alternatives considered

**`app.shinar` or `com.shinar.app`.** Reverse domain names, which by
convention belong to whoever owns the domain. The owner owns neither, and
an identifier tied to the name would outlive a name that changed.

**Keeping the old identifiers until launch.** It would have meant
registering the Google OAuth client and Sign in with Apple twice.

## Consequences

- Each phone installs Shinar as a new app beside the old PTranslate, which
  can be deleted. Its storage starts empty, so the person signs in again.
  Apple gives the same user ID to every app of one developer team, so
  signing in with Apple finds the same account.
- The Google Cloud Android OAuth client is registered for
  `com.pelumi.shinar` and the debug key's SHA-1. A release key will need
  its own SHA-1 added when there is one.
- If the trademark check fails, the name changes in two places (and the
  link scheme, if wanted); the identifiers stay.
