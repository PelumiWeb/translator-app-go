import Foundation
import Testing
@testable import PTranslate

/// Runs the real `BackendClient` against a stub server, so the HTTP requests and the event-stream parsing are
/// tested as they go over the wire. The same cases as the Android tests.
struct RemoteTranscriberTests {
    private let server = StubServer()
    private let transcriber: RemoteTranscriber
    private let audio = PCMAudio(samples: Array(repeating: 0, count: 1600))
    private let final = TranscriptEvent.final(Transcript(text: "hello world", confidence: nil, source: .cloud))

    init() {
        transcriber = RemoteTranscriber(
            backend: BackendClient(baseURL: server.baseURL, session: server.session),
            maxAttempts: 3,
            retryDelay: { _ in .zero } // no waiting in tests
        )
    }

    private func enqueueJobAccepted() {
        server.enqueue(.json(status: 202, body: #"{"id":"job-1","status":"queued","events_url":"/v1/jobs/job-1/events"}"#))
    }

    private func sse(_ seq: Int, _ type: String, _ data: String = "{}") -> String {
        "id: \(seq)\nevent: \(type)\ndata: \(data)\n\n"
    }

    private func transcribe(_ language: Language = .english) async throws -> [TranscriptEvent] {
        var events: [TranscriptEvent] = []
        for try await event in transcriber.transcribe(audio, language: language) {
            events.append(event)
        }
        return events
    }

    private func transcribeFails() async -> TranscriptionError? {
        await #expect(throws: TranscriptionError.self) { try await transcribe() }
    }

    @Test func mapsTheJobsEventsToTranscriptEvents() async throws {
        enqueueJobAccepted()
        server.enqueue(.events(
            sse(1, "queued")
                + sse(2, "processing", #"{"attempt": 1}"#)
                + ": keep-alive\n\n"
                + sse(3, "partial", #"{"text": "hello"}"#)
                + sse(4, "partial", #"{"text": "hello world"}"#)
                + sse(5, "done", #"{"text": "hello world", "language": "en"}"#)
        ))

        #expect(try await transcribe() == [
            .stageChanged(.uploading),
            .stageChanged(.queued),
            .stageChanged(.processing),
            .partial("hello"),
            .partial("hello world"),
            final,
        ])
    }

    @Test func uploadsTheAudioAsAWAVFileWithTheSourceLanguage() async throws {
        enqueueJobAccepted()
        server.enqueue(.events(sse(1, "done", #"{"text": "ok"}"#)))

        _ = try await transcribe(Language("yo"))

        let upload = server.requests[0]
        #expect(upload.method == "POST")
        #expect(upload.path == "/v1/jobs")
        #expect(upload.header("Content-Type")?.hasPrefix("multipart/form-data") == true)
        let body = upload.bodyText
        #expect(body.contains(#"name="source_lang""#) && body.contains("\r\n\r\nyo\r\n"), "source_lang field")
        #expect(body.contains(#"name="audio""#), "audio part")
        #expect(body.contains("RIFF") && body.contains("WAVE"), "WAV signature")

        let stream = server.requests[1]
        #expect(stream.method == "GET")
        #expect(stream.path == "/v1/jobs/job-1/events")
        #expect(stream.header("Last-Event-ID") == nil, "a first connection has nothing to resume from")
    }

    @Test func skipsEventTypesItDoesNotKnow() async throws {
        enqueueJobAccepted()
        server.enqueue(.events(sse(1, "something-new", #"{"x": 1}"#) + sse(2, "done", #"{"text": "ok"}"#)))

        let events = try await transcribe()

        #expect(events.count == 2) // uploading, and the final transcript
        #expect(events.last == .final(Transcript(text: "ok", confidence: nil, source: .cloud)))
    }

    // MARK: Reconnecting

    @Test func reconnectsFromTheLastEventItReceivedAndCarriesOn() async throws {
        enqueueJobAccepted()
        // The connection drops after three events...
        server.enqueue(.events(sse(1, "queued") + sse(2, "processing") + sse(3, "partial", #"{"text": "hello"}"#)))
        // ...and the second connection continues from the fourth.
        server.enqueue(.events(
            sse(4, "partial", #"{"text": "hello world"}"#)
                + sse(5, "done", #"{"text": "hello world", "language": "en"}"#)
        ))

        #expect(try await transcribe() == [
            .stageChanged(.uploading),
            .stageChanged(.queued),
            .stageChanged(.processing),
            .partial("hello"),
            .stageChanged(.reconnecting),
            .partial("hello world"),
            final,
        ])
        #expect(server.requests[2].header("Last-Event-ID") == "3")
    }

    @Test func showsNothingTwiceIfTheServerReplaysEventsItAlreadySent() async throws {
        enqueueJobAccepted()
        server.enqueue(.events(sse(1, "queued") + sse(2, "partial", #"{"text": "hello"}"#)))
        // A server that ignores Last-Event-ID and starts from the beginning.
        server.enqueue(.events(
            sse(1, "queued") + sse(2, "partial", #"{"text": "hello"}"#)
                + sse(3, "done", #"{"text": "hello world", "language": "en"}"#)
        ))

        #expect(try await transcribe() == [
            .stageChanged(.uploading),
            .stageChanged(.queued),
            .partial("hello"),
            .stageChanged(.reconnecting),
            final,
        ])
    }

    @Test func reconnectsWhenTheConnectionIsCutInTheMiddleOfTheStream() async throws {
        enqueueJobAccepted()
        server.enqueue(.eventsThenDrop(sse(1, "queued") + sse(2, "processing")))
        server.enqueue(.events(sse(1, "queued") + sse(2, "processing") + sse(3, "done", #"{"text": "hello world"}"#)))

        let events = try await transcribe()

        #expect(events.last == final)
        #expect(events.count { if case .final = $0 { true } else { false } } == 1)
    }

    @Test func givesUpAfterReconnectingSeveralTimesWithoutAnyProgress() async throws {
        enqueueJobAccepted()
        server.enqueue(.events(sse(1, "queued")))
        server.enqueue(.events("")) // maxAttempts is 3: two more tries, both empty
        server.enqueue(.events(""))

        let error = await transcribeFails()

        #expect(error == .failed("The connection to the server was lost"))
        #expect(server.requestCount == 4) // one upload and three streams
    }

    @Test func keepsGoingAsLongAsEachReconnectBringsSomethingNew() async throws {
        enqueueJobAccepted()
        // Five connections of one event each: more than maxAttempts, but none of them a failure without progress.
        server.enqueue(.events(sse(1, "queued")))
        server.enqueue(.events(sse(2, "processing")))
        server.enqueue(.events(sse(3, "partial", #"{"text": "hello"}"#)))
        server.enqueue(.events(sse(4, "partial", #"{"text": "hello world"}"#)))
        server.enqueue(.events(sse(5, "done", #"{"text": "hello world"}"#)))

        #expect(try await transcribe().last == final)
    }

    @Test func doesNotReconnectWhenTheJobItselfFailed() async throws {
        enqueueJobAccepted()
        server.enqueue(.events(sse(1, "queued") + sse(2, "error", #"{"message": "provider exploded"}"#)))

        let error = await transcribeFails()

        #expect(error == .failed("provider exploded"))
        #expect(server.requestCount == 2)
    }

    @Test func doesNotReconnectWhenTheServerSaysTheJobDoesNotExist() async throws {
        enqueueJobAccepted()
        server.enqueue(.status(404, body: #"{"error":{"code":"job_not_found","message":"no such job"}}"#))

        let error = await transcribeFails()

        #expect(error == .failed("The server answered 404"))
        #expect(server.requestCount == 2)
    }

    // MARK: Uploading

    @Test func retriesAnUploadWhoseResponseWasLostWithTheSameIdempotencyKey() async throws {
        server.enqueue(.dropAfterRequest)
        enqueueJobAccepted()
        server.enqueue(.events(sse(1, "done", #"{"text": "hello world"}"#)))

        #expect(try await transcribe().last == final)

        let firstKey = server.requests[0].header("Idempotency-Key")
        let secondKey = server.requests[1].header("Idempotency-Key")
        #expect(firstKey?.isEmpty == false, "a key is sent")
        #expect(firstKey == secondKey, "both attempts carry the same key")
    }

    @Test func usesANewIdempotencyKeyForEachRecording() async throws {
        for _ in 0..<2 {
            enqueueJobAccepted()
            server.enqueue(.events(sse(1, "done", #"{"text": "ok"}"#)))
            _ = try await transcribe()
        }

        #expect(server.requests[0].header("Idempotency-Key") != server.requests[2].header("Idempotency-Key"))
    }

    @Test func retriesAnUploadAfterAServerFault() async throws {
        server.enqueue(.status(503))
        enqueueJobAccepted()
        server.enqueue(.events(sse(1, "done", #"{"text": "hello world"}"#)))

        #expect(try await transcribe().last == final)
    }

    @Test func doesNotRetryAnUploadTheServerRefused() async throws {
        server.enqueue(.status(400, body: #"{"error":{"code":"unsupported_audio","message":"audio must be a WAV file"}}"#))

        let error = await transcribeFails()

        #expect(error == .failed("audio must be a WAV file"))
        #expect(server.requestCount == 1)
    }

    @Test func failsWhenTheServerCannotBeReached() async throws {
        server.shutdown()

        let error = await transcribeFails()

        #expect(error?.errorDescription?.hasPrefix("Could not reach the server") == true)
    }
}
