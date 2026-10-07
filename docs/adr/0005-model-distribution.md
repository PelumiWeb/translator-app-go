# ADR 0005: Model distribution with a manifest, resume and checksum

Status: accepted, 2026-10-07

## Context

The Whisper model is 57 MB. It is too large to ship inside the APK
comfortably, it will be replaced by better models over time, and a slow device
may want a smaller one. The app therefore has to fetch it after install, over
mobile connections that drop, and must never load a damaged file: a truncated
or corrupted model crashes native code or, worse, produces wrong transcripts
without an error.

## Decision

**The backend is the source.** It serves a manifest and the files:

- `GET /v1/models/manifest` lists each model's `id`, `version`, `size`,
  `sha256` and `url`.
- `GET /v1/models/{id}` serves the file with HTTP range support and the
  SHA-256 as its `ETag`.

The catalogue is built at server start by hashing the `.bin` files in one
directory. A model's version is the first 12 characters of its hash, so it
cannot drift from the content.

**The app installs in four steps** (`ModelManager`):

1. Read the manifest and find the model.
2. Download into `<id>.part`. If a partial file exists, ask for the rest with
   `Range: bytes=<have>-` and `If-Range: "<sha256>"`.
3. Compute the SHA-256 of the complete file and compare it with the manifest.
4. Rename `<id>.part` to `<id>.bin`.

**A file named `<id>.bin` is always a verified model.** It can only come into
existence through the rename in step 4, which is atomic within a directory.
Code that loads the model needs no flag or database row to know it is safe;
the file's presence is the record.

**Three guards keep a resume from producing a wrong file:**

- `<id>.part.sha256` records which model a partial file belongs to. If the
  manifest now names a different hash, the partial file is deleted.
- `If-Range` makes the server send the whole file, not a tail, if the file
  changed between the manifest request and the download.
- If the server answers `200` to a range request, the app writes from an empty
  file instead of appending.

Each of these could only fail silently, as a file of the right length with
the wrong bytes. The checksum in step 3 would still catch that; the guards
save a 57 MB download that was bound to be rejected.

**After a failure**, a dropped connection keeps the partial file for the next
attempt. A checksum mismatch deletes it: nothing in a corrupt file can be
trusted.

**An installed model is used without contacting the server**, so the app works
offline once it has one.

## Alternatives considered

**Bundle the model in the APK.** No download code at all. The APK grows to
about 90 MB, every model change is an app release, and there is no way to give
a slow device a smaller model.

**Play Asset Delivery.** Google hosts and delivers the file, with resume and
integrity checks built in. It ties the app to Play, cannot be exercised from a
local build, and removes the backend work this project is meant to show.

**Android's `DownloadManager`.** Handles resume and survives the app being
killed. It gives little control over where the file lands, has no checksum
step, and is hard to test; the download loop here is about 30 lines and is
covered by tests against a fake server.

**A signature instead of a checksum.** A SHA-256 from the same server that
serves the file protects against corruption, not against a compromised server.
Signing the manifest with a key pinned in the app would. Left out because
there is no auth anywhere yet; noted as the next step if this were deployed.

## Consequences

- A first run needs the network and about 57 MB of data. Until the download
  finishes, on-device transcription is unavailable and the cloud path is the
  only one.
- The download stops if the app is killed, and continues the next time the app
  asks for the model. Nothing runs in the background.
- The app does not yet notice a newer model on the server once one is
  installed. The manifest's `version` is there for that; the check is not
  written.
- The server reads its model directory once, at start.
- ML Kit's translation language packs are not part of this. ML Kit downloads
  and stores those itself.
