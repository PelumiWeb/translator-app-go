# ADR 0022: TestFlight builds from GitHub Actions

Status: accepted, 2026-11-02

## Context

On 2026-11-02 the owner asked for a pipeline that sends the iPhone app to
TestFlight, so they can invite testers through TestFlight groups instead of
installing builds from this Mac. The code is on GitHub. The owner chose
GitHub Actions over Xcode Cloud.

Two things make an App Store build harder than a build for a connected
phone. It has to be signed for distribution on a machine that starts empty
every time. And App Store Connect asks two questions of every build: whether
it uses encryption beyond the standard kind, and why it calls the APIs Apple
asks apps to explain (the privacy manifest).

## Decision

**One workflow, `.github/workflows/testflight.yml`**, on GitHub's Apple
silicon runner (`macos-26`). It runs on every push to `main` that changes the
iPhone app (`ios/`, the whisper.cpp submodule, the workflow itself), and by
hand from the Actions tab. It downloads the Whisper model, runs the iOS
tests, and only if they pass, signs, archives and uploads. A newer push
cancels a run that has not finished.

**The upload is a script, `ios/scripts/testflight.sh` (`make
ios-testflight`)**, so it runs the same on a Mac. It is plain `xcodebuild`:
archive, then export with the destination "upload". Xcode gives each build
the next build number App Store Connect expects, so builds from the pipeline
and from Xcode never collide.

**Signing is manual, from secrets.** An Apple Distribution certificate (a
`.p12`) and an App Store provisioning profile for `com.pelumi.shinar` are
kept as repository secrets. Each run puts the certificate in a keychain made
for that run, as GitHub documents. An App Store Connect API key, with the
App Manager role, is used only to upload. Automatic signing with the API key
alone ("cloud signing") was the simpler set-up, but on a fresh machine Xcode
finds a development certificate whose private key is not there and stops;
developers report it on Apple's forums, and nothing confirms a fix.

**The server address is a repository variable, `BACKEND_URL`.** For now it
is the Mac's address on the owner's Wi-Fi. A hosted server (milestone 18)
changes the variable, not the code. (Changed on 2026-11-11, see the
addendum.)

**Every build answers App Store Connect's questions itself:**
`ITSAppUsesNonExemptEncryption` is false in Info.plist (the app's only
encryption is the system's: HTTPS and the Keychain), and
`PrivacyInfo.xcprivacy` gives Apple's reasons for the two APIs the app uses
that need one: `UserDefaults` (CA92.1, the app's own settings) and file
attributes (C617.1, the size of the app's own files). The data the app
collects goes in the manifest with the privacy policy (milestone 18).

## Setting it up

Once, by the owner:

1. **The app in App Store Connect.** Apps, then the plus button, New App:
   iOS, a name (it must be unique on the App Store, and can change later),
   the primary language, bundle ID `com.pelumi.shinar`, and any SKU, such as
   `shinar-ios`.
2. **A distribution certificate.** In Xcode, Settings, Accounts, the team,
   Manage Certificates, then the plus button, Apple Distribution. Then in
   Keychain Access, under My Certificates, export "Apple Distribution: ..."
   as a `.p12` with a password.
3. **A provisioning profile.** On developer.apple.com, Certificates, IDs &
   Profiles, Profiles, then the plus button: App Store Connect, the App ID
   `com.pelumi.shinar`, the Apple Distribution certificate, and a name such
   as "Shinar App Store". Download it.
4. **An API key.** In App Store Connect, Users and Access, Integrations, App
   Store Connect API, Team Keys, then the plus button: a name such as
   "GitHub Actions" and the App Manager role. Download the `.p8` (it can be
   downloaded only once) and note the Key ID and the Issuer ID.
5. **The GitHub settings.** In the repository, Settings, Secrets and
   variables, Actions:

   | Kind | Name | Value |
   | --- | --- | --- |
   | Secret | `IOS_DIST_CERT_P12` | the `.p12` as base64: `base64 -i cert.p12 \| pbcopy` |
   | Secret | `IOS_DIST_CERT_PASSWORD` | the `.p12` password |
   | Secret | `IOS_PROFILE` | the profile as base64: `base64 -i Shinar_App_Store.mobileprovision \| pbcopy` |
   | Secret | `ASC_KEY_P8` | the `.p8` file's text: `pbcopy < AuthKey_XXXX.p8` |
   | Secret | `ASC_KEY_ID` | the Key ID |
   | Secret | `ASC_ISSUER_ID` | the Issuer ID |
   | Variable | `BACKEND_URL` | optional since 2026-11-11: only to send TestFlight builds to a server other than the hosted one |

6. **Testers.** In App Store Connect, the app, TestFlight: an internal
   group, for people added to the team in Users and Access, with automatic
   distribution on, so each new build reaches them; or an external group,
   for anyone by email or a public link, where the first build of each
   version goes through Apple's beta review.

## Alternatives considered

**Xcode Cloud.** Apple's CI: 25 hours a month with the membership, and it
signs for us. Set up in Xcode and App Store Connect rather than in the
repository, and iPhone only. The owner chose GitHub Actions.

**Cloud signing with only the API key.** One secret instead of four, but it
fails on fresh machines, as above.

**fastlane.** The usual tool for this, with `match` to share certificates
through a private repository. A Ruby toolchain and a second repository for
what `xcodebuild` and `security` already do.

## Consequences

- The certificate and the profile expire after a year. Renewing them means
  replacing `IOS_DIST_CERT_P12`, `IOS_DIST_CERT_PASSWORD` and `IOS_PROFILE`.
- GitHub counts macOS minutes ten times against a private repository's free
  allowance. A run (tests, archive, upload) takes about a quarter of an
  hour, so roughly ten runs a month are free.
- Testers install the build anywhere, but the app can only sign in and load
  anything where it reaches the server: today, on the owner's Wi-Fi while
  the Mac runs it. Apple's beta review for external groups would fail
  against that, so external testing waits for a reachable server.
- Builds are not uploaded for pull requests or other branches. A pull
  request check, and the server and Android tests, can be added as jobs of
  their own.

## Addendum, 2026-11-11: Release builds talk to the hosted server

The first TestFlight builds were archived in Xcode, which reads the address
from the project, not from the workflow's variable. The project's Release
configuration still named the Mac (`http://OgundipePelumi.local:8080`), so
on testers' phones the app reached no server at all.

The Release configuration now holds the hosted server's address
(`https://shinar-api.southafricanorth.cloudapp.azure.com`, ADR 0023), so an
archive from Xcode and one from the workflow talk to the same server.
`BACKEND_URL` became an optional override. `ios/scripts/testflight.sh`
reads the address back from the archived app and refuses to upload one that
is not `https://`. Android's `release` build type got the same address, so
the two apps keep one rule: debug builds talk to the Mac, release builds to
the hosted server.
