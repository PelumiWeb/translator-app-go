# ADR 0024: ML Kit on iOS

Status: accepted, 2026-11-10. Replaces ADR 0012.

## Context

ADR 0012 chose Apple's Translation framework for the iPhone, against the
owner's preference for ML Kit on both phones. It had three costs that grew
once the app became a product:

- 22 languages on the iPhone against 59 on Android, so two people in the
  same room could not always pick the same language.
- It translates without a screen only from iOS 26
  (`TranslationSession(installedSource:target:)`), which held the app at
  iOS 26. Below that, every translation would have to go through a SwiftUI
  view (ADR 0025).
- iOS downloads its language packs itself, behind its own sheet. Apple does
  not publish their sizes; the owner remembers downloads of up to about
  800 MB.

ADR 0012 turned ML Kit down for two reasons that still hold for Google's own
releases: it is published only for CocoaPods, which stops taking new
releases on 2 December 2026, and its binaries have no code for the simulator
on an Apple-silicon Mac, so the app would not build for the simulator.

The owner asked on 2026-11-10 whether those costs could be avoided properly
rather than worked around, and approved the dependencies below.

## Decision

**ML Kit Translate on the iPhone, as on Android**, behind the same
`Translator` protocol (`MLKitTranslator`). It follows Android's
`MlKitTranslator`: the same languages, the same readiness check, downloads
allowed over mobile data, the same messages. Language packs are 31 to 48 MB
(34 MB typical, Japanese the largest), read from Google's own list in the
library; English needs none, and a pair of two other languages needs both.

**Google's files, without CocoaPods.** `ios/scripts/fetch-mlkit.sh`
(`make ios-mlkit`) downloads MLKitTranslate 8.0.0, MLKitNaturalLanguage
10.0.0 and MLKitCommon 14.0.0 from `dl.google.com`, the address in each
version's podspec, and checks each archive against a SHA-256 recorded in the
script. It writes them to `ios/MLKit/Frameworks/`, which git ignores, and does
nothing when they are already there. Nobody else's copy of Google's code is
involved: community repackagings exist, but would make one stranger's
releases part of every build.

**Repackaged as static libraries.** Google ships static frameworks. Xcode
embeds a Swift package's frameworks in the app even when they are static: it
put three empty frameworks in the app, each needing an `Info.plist`, and each
a missing debug-symbol file when the build is uploaded. As static libraries
with their headers, Google's code is linked into the app's own executable and
nothing is embedded. The archive holds one dSYM, the app's.

**The simulator gets a retagged copy.** The simulator on an Apple-silicon Mac
runs the same arm64 instructions as an iPhone; the linker tells the two apart
only by the platform in each object's `LC_BUILD_VERSION` load command (2 for
iOS, 7 for the simulator). `retag-for-simulator.py` changes that one number
in every object of a copy, in place, and refuses anything unexpected. Apple's
`vtool` does the same for a single object, but cannot open archives, and one
of Google's archives holds several objects under the same name, which
unpacking would lose. The iPhone slice is Google's code untouched; the App
Store build never contains the copy. With it, ML Kit translates in the
simulator, which Apple's framework never did, so the real translation test
runs there and on CI.

**One local package links it all.** `ios/MLKit/Package.swift` puts Google's
libraries together with the open-source code they call, at the versions
Google's podspecs ask for, pinned in the project's `Package.resolved`:
GTMSessionFetcher 3.5.0, GoogleDataTransport 10.1.1 (bringing nanopb and
Promises), GoogleUtilities 8.1.5 and ZipArchive 2.6.0, from their own Swift
packages. It links with `-ObjC`, as CocoaPods does: some of Google's object
files hold only Objective-C categories, which the linker would otherwise
leave out.

**GoogleToolboxForMac from source.** ML Kit asks for 4.2.1 up to 5, which has
no Swift package. Its package, from 6.0, cannot be linked by Xcode at all,
because one of its parts is headers only
([swift-package-manager #5706](https://github.com/swiftlang/swift-package-manager/issues/5706);
a fix is open, not merged). The script builds the three parts ML Kit calls
(Logger, StringEncoding, NSData+zlib) from the v4.2.1 tag, checking the
commit, with the same memory-management settings as the podspec.

**Privacy manifests.** Static libraries carry none into the app, so the
script puts Google's manifest in `MLKitTranslate_resource.bundle`, which the
app copies. MLKitTranslate's declares everything the other two do, and more;
`privacy-manifest-covers.py` stops the script if that changes. The Swift
packages bring their own. GoogleToolboxForMac's Logger declares one reason,
`UserDefaults` for the app's own use (CA92.1), which the app's manifest
already gives.

## Alternatives considered

**Keep Apple's translator.** No dependency and no added size, but 22
languages, iOS 26 only, and iOS's own large downloads.

**ML Kit through CocoaPods.** Google's supported route, but it adds Ruby,
CocoaPods and a workspace weeks before CocoaPods freezes, and the app would
not build for the simulator.

**A community Swift package of ML Kit** (such as `d-date/google-mlkit-swiftpm`).
The same result with less work, but every update would be someone else's
repackaging of Google's binaries.

**GoogleToolboxForMac 6.0.1 from its package.** Its three parts ML Kit uses
have the same headers as in 4.2.1, but Xcode cannot link the package (above).

**Translating on the server.** Rejected by ADR 0003 for the interpreter.

## Consequences

- The same 59 languages, and the same translations, on both phones.
- The iPhone app grows by about 31 MB, 29 MB of it ML Kit's translator
  (measured from the link map of a Release build; the App Store build is
  34 MB in all). Android's grows by 16 MB. The App Store encrypts the app's
  code, which then barely compresses, so the download grows by about as much.
- ML Kit sends Google usage and diagnostic data, not linked to the user, as
  its manifest declares. The App Store privacy answers and the privacy policy
  must say so (milestone 18), as the Play listing must for Android.
- Crash reports show no function names inside Google's code: Google ships no
  debug symbols.
- ML Kit logs a warning that several of its usage uploaders were created.
  It appears with a single translator too, so it comes from ML Kit itself.
- Moving to a new ML Kit version means new versions and checksums in the
  script, and checking the podspecs for what the new version needs. Once
  Google publishes a Swift package, the script and the local package can give
  way to it, behind the same `Translator` protocol.
- If a future Xcode refuses the retagged simulator copy, translation stops
  working in the simulator only, as it was with Apple's framework.
- The iPhone no longer needs a screen to download a language pack, so the
  root view's download hook is gone, and the keyboard of milestone 11 can
  translate on its own.
