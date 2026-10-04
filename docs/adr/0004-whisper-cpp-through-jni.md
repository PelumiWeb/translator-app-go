# ADR 0004: On-device transcription with whisper.cpp through JNI

Status: accepted, 2026-10-04

## Context

The app must transcribe speech on the device, offline, in languages other than
English, and report how confident it is so that a weak result can fall back to
the backend. On-device inference with native code is one of the three things
this project sets out to demonstrate.

## Decision

**Engine.** whisper.cpp, pinned to release v1.9.4 as a shallow git submodule at
`third_party/whisper.cpp`. CMake builds it from source as part of the Gradle
build.

**Packaging.** whisper.cpp and ggml are compiled as static libraries and linked
into one shared library of ours, `libptranslate_whisper.so` (3 MB). CPU only:
no GPU or NNAPI backend. OpenMP is off, so ggml uses its own thread pool and
there is no `libomp.so` to ship.

**Always optimised.** The inference code is compiled with `-O3` in every build
type. A debug build of the app would otherwise run the model unoptimised, many
times slower, and every measurement taken during development would be
meaningless.

**ABI.** `arm64-v8a` only. That covers current phones and the emulator on Apple
silicon. `x86_64` is one line in `app/build.gradle.kts` when someone needs an
Intel emulator; leaving it out halves native build time and APK size.

**16 KB pages.** Built with `ANDROID_SUPPORT_FLEXIBLE_PAGE_SIZES=ON`. Devices
with 16 KB memory pages refuse to load a library aligned for 4 KB, and NDK 27
does not align for 16 KB unless asked.

**JNI surface.** Six static functions, all called from one Kotlin class,
`WhisperContext`, which owns the native pointer:

| Function | Purpose |
| --- | --- |
| `nativeInit(path)` | load a model, return a handle or 0 |
| `nativeFree(handle)` | release it |
| `nativeTranscribe(handle, samples, language, threads)` | run the model, return the text |
| `nativeMeanTokenProbability(handle)` | confidence of the last transcript |
| `nativeNoSpeechProbability(handle)` | the model's estimate that the last audio held no speech |
| `nativeSystemInfo()` | CPU features in use |

The transcript crosses the boundary as UTF-8 bytes, not as a `jstring`. JNI's
`NewStringUTF` expects "modified UTF-8" and can abort on characters outside the
basic multilingual plane.

**Confidence.** The mean probability the model assigned to the tokens it
produced, ignoring control tokens. whisper.cpp has no single confidence value;
this is the simplest signal it does provide. Whether it separates good from bad
transcripts well enough is checked in milestone 6, where the fallback threshold
is set.

**One decoding pass.** whisper.cpp's default is to decode a low-confidence
result again, up to five times, at rising temperature. That is switched off
(`temperature_inc = 0`). On a phone the retries turn a poor recording into a
wait several times longer, and a weak result has a better remedy here: the
backend.

**Silence.** Given silence, Whisper does not return an empty transcript; it
invents a plausible sentence. Two guards in `WhisperTranscriber` report "no
speech" instead:

1. Before the model runs, a loudness check (`PcmAudio.isSilent`, root mean
   square below about -50 dB). It costs a millisecond and catches a muted or
   missing microphone.
2. After the model runs, its own no-speech probability for the first segment,
   above 0.6, the threshold OpenAI's reference implementation uses. This
   catches audio that is loud enough but is not speech.

**Model.** Multilingual `base`, quantised to `q5_1`: 57 MB. On the emulator it
transcribes an 11 second clip in under 4 seconds including loading the model.
The English-only `.en` models are smaller and more accurate for English but
cannot transcribe anything else.

## Alternatives considered

**Android's `SpeechRecognizer`.** A few lines of code, but on most devices it
sends audio to Google, the choice of on-device languages is the vendor's, and
it demonstrates no native work.

**Vosk, or sherpa-onnx.** Both run on device and stream partial results, which
whisper.cpp does not. Whisper is more accurate across many languages from one
model, and the spec names whisper.cpp.

**A prebuilt whisper AAR.** Hides the build, which is the part worth showing,
and ties the project to someone else's release schedule and build flags.

**`tiny` as the default model.** Half the size and roughly twice as fast, with
noticeably worse accuracy outside English. Kept as the option for devices that
the benchmark in milestone 6 finds too slow for `base`.

## Consequences

- A fresh clone needs `git submodule update --init` before the Android build
  works.
- The first native build takes about a minute; later ones are incremental.
- Whisper does not stream. The on-device path shows nothing until the whole
  clip has been processed, unlike the cloud path.
- A `WhisperContext` must be used from one thread at a time and closed when no
  longer needed. The model lives outside the Java heap, so the garbage
  collector neither sees its size nor frees it.
- Timings on the emulator depend heavily on the host. On an 8 GB Mac that is
  swapping, the same 11 second clip took anywhere from 4 to over 100 seconds.
  Only numbers from a real phone mean anything, which is why the app shows the
  real-time factor of every transcription.
- Code that calls the native library cannot be tested on the JVM. It is covered
  by device tests (`make android-device-test`) that transcribe whisper.cpp's
  sample recording.
