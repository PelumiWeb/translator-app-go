import Foundation
import Synchronization
import Testing
@testable import PTranslate

/// Runs the real `BackendClient` against a stub server, so the requests, the event stream and the audio download
/// are tested as they go over the wire. The same cases as the Android tests.
struct RemoteSpeechSynthesizerTests {
    /// Stands in for the player: records what it was given to play.
    private final class Speaker: Sendable {
        private let state = Mutex<(played: [PCMAudio], stops: Int, startedAfter: [Int], startedIn: [SpeechVoice])>(([], 0, [], []))

        var played: [PCMAudio] { state.withLock { $0.played } }
        var stops: Int { state.withLock { $0.stops } }

        /// How many sounds had been played each time the speech said it started, and in what voice.
        var startedAfter: [Int] { state.withLock { $0.startedAfter } }
        var startedIn: [SpeechVoice] { state.withLock { $0.startedIn } }

        func play(_ audio: PCMAudio) {
            state.withLock { $0.played.append(audio) }
        }

        func started(in voice: SpeechVoice) {
            state.withLock {
                $0.startedAfter.append($0.played.count)
                $0.startedIn.append(voice)
            }
        }

        func stop() {
            state.withLock { $0.stops += 1 }
        }
    }

    private let server = StubServer()
    private let speaker = Speaker()

    /// What the user recorded at onboarding.
    private let sample = PCMAudio(samples: (0..<16_000).map { Int16($0 % 100) }).wavData()

    /// What the server makes from it.
    private let spoken = PCMAudio(samples: (0..<8_000).map { Int16($0 % 50) }, sampleRate: 22_050)

    private func synthesizer(sample: Data?) -> RemoteSpeechSynthesizer {
        RemoteSpeechSynthesizer(
            backend: BackendClient(baseURL: server.baseURL, session: server.session),
            voiceSample: { sample },
            play: { [speaker] in speaker.play($0) },
            stopPlaying: { [speaker] in speaker.stop() },
            maxAttempts: 3,
            retryDelay: { _ in .zero } // no waiting in tests
        )
    }

    private var synthesizer: RemoteSpeechSynthesizer { synthesizer(sample: sample) }

    private func enqueueSpeechAccepted() {
        server.enqueue(.json(
            status: 202,
            body: #"{"id":"job-1","status":"queued","events_url":"/v1/jobs/job-1/events","audio_url":"/v1/jobs/job-1/audio"}"#
        ))
    }

    private func enqueueAudio() {
        server.enqueue(.data(status: 200, headers: ["Content-Type": "audio/wav"], body: spoken.wavData()))
    }

    private func sse(_ seq: Int, _ type: String, _ data: String = "{}") -> String {
        "id: \(seq)\nevent: \(type)\ndata: \(data)\n\n"
    }

    private func speak(with synthesizer: RemoteSpeechSynthesizer) async throws(SpeechError) {
        try await synthesizer.speak("buenos días", language: Language("es"), onStart: { [speaker] in speaker.started(in: $0) })
    }

    @Test func sendsTheVoiceSampleAndTheTextThenPlaysWhatTheServerMade() async throws {
        enqueueSpeechAccepted()
        server.enqueue(.events(sse(1, "queued") + sse(2, "processing") + sse(3, "done")))
        enqueueAudio()

        try await speak(with: synthesizer)

        let upload = server.requests[0]
        #expect(upload.method == "POST")
        #expect(upload.path == "/v1/speech")
        #expect(upload.header("Idempotency-Key")?.isEmpty == false, "an idempotency key is sent")
        let form = upload.bodyText
        #expect(form.contains(#"name="text""#) && form.contains("\r\n\r\nbuenos días\r\n"), "text field")
        #expect(form.contains(#"name="lang""#) && form.contains("\r\n\r\nes\r\n"), "lang field")
        #expect(form.contains(#"name="voice"; filename="voice.wav""#), "voice part")
        #expect(server.requests[1].path == "/v1/jobs/job-1/events")
        #expect(server.requests[2].path == "/v1/jobs/job-1/audio")

        #expect(speaker.played == [spoken])
        // Reported as started once, just before it plays: not while the voice is being made.
        #expect(speaker.startedAfter == [0])
        #expect(speaker.startedIn == [.own])
    }

    @Test func reconnectsToTheEventStreamAndStillPlaysTheResult() async throws {
        enqueueSpeechAccepted()
        server.enqueue(.eventsThenDrop(sse(1, "queued")))
        server.enqueue(.events(sse(2, "processing") + sse(3, "done")))
        enqueueAudio()

        try await speak(with: synthesizer)

        #expect(server.requests[2].header("Last-Event-ID") == "1")
        #expect(speaker.played.count == 1)
    }

    @Test func retriesAnUploadAfterAServerFaultWithTheSameIdempotencyKey() async throws {
        server.enqueue(.status(503))
        enqueueSpeechAccepted()
        server.enqueue(.events(sse(1, "done")))
        enqueueAudio()

        try await speak(with: synthesizer)

        let keys = server.requests.prefix(2).map { $0.header("Idempotency-Key") }
        #expect(keys[0] != nil && keys[0] == keys[1])
        #expect(speaker.played.count == 1)
    }

    @Test func aJobThatFailedIsReportedInWordsAndNothingIsPlayed() async {
        enqueueSpeechAccepted()
        server.enqueue(.events(sse(1, "queued") + sse(2, "error", #"{"message": "the voice engine is down"}"#)))

        await #expect(throws: SpeechError(message: "the voice engine is down")) {
            try await speak(with: synthesizer)
        }
        #expect(speaker.played.isEmpty)
        #expect(speaker.startedAfter.isEmpty)
    }

    @Test func aRequestTheServerRefusedIsReportedWithItsReason() async {
        server.enqueue(.json(
            status: 400,
            body: #"{"error":{"code":"text_too_long","message":"the text is too long to speak in one request"}}"#
        ))

        await #expect(throws: SpeechError(message: "the text is too long to speak in one request")) {
            try await speak(with: synthesizer)
        }
        #expect(server.requestCount == 1)
    }

    @Test func aServerWithNoVoiceEngineIsNotAskedAgain() async {
        server.enqueue(.json(
            status: 501,
            body: #"{"error":{"code":"no_voice_engine","message":"this server has no voice engine"}}"#
        ))

        await #expect(throws: SpeechError(message: "this server has no voice engine")) {
            try await speak(with: synthesizer)
        }
        #expect(server.requestCount == 1)
    }

    @Test func withNoVoiceRecordedNothingIsSent() async {
        await #expect(throws: SpeechError(message: "No voice has been recorded")) {
            try await speak(with: synthesizer(sample: nil))
        }
        #expect(server.requestCount == 0)
    }

    @Test func failsWhenTheServerCannotBeReached() async {
        server.shutdown()

        let error = await #expect(throws: SpeechError.self) {
            try await speak(with: synthesizer)
        }
        #expect(error?.message.hasPrefix("Could not reach the server") == true)
    }

    @Test func stopStopsThePlayback() async {
        await synthesizer.stop()

        #expect(speaker.stops == 1)
    }
}
