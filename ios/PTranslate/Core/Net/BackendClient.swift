import Foundation

/// A failure to get what was asked of the backend.
struct BackendError: LocalizedError {
    let message: String

    /// Whether trying again could succeed. True for a connection problem or a server fault; false when the server
    /// understood the request and refused it, since sending the same thing again gets the same answer.
    let retryable: Bool

    var errorDescription: String? { message }
}

/// One entry in a job's event stream, already parsed.
enum JobEvent: Equatable, Sendable {
    case queued
    case processing
    case partial(String)
    case done(text: String, language: String)
    case failed(String)

    /// A type this version of the app does not know. Its number still counts.
    case unknown
}

/// An event with its position in the job's history. `seq` counts 1, 2, 3 ... and is what a reconnecting client
/// sends back so the server can continue from the right place.
struct JobUpdate: Equatable, Sendable {
    let seq: Int
    let event: JobEvent
}

/// A request to speak, accepted and queued. Its audio is fetched from `audioURL` once the job is done.
struct SpeechJob: Equatable, Sendable {
    let id: String
    let audioURL: String
}

/// One model the backend offers, as listed in its manifest.
struct RemoteModel: Decodable, Equatable, Sendable {
    let id: String
    let version: String
    let size: Int64
    let sha256: String
    let url: String
}

/// An open model download. `partial` is true when the server honoured the requested offset and `bytes` holds only
/// the rest of the file; false when it is sending the whole file from the start.
struct ModelDownload {
    let partial: Bool
    let bytes: URLSession.AsyncBytes
}

/// The only type that knows the backend's URLs and JSON.
final class BackendClient: Sendable {
    private let baseURL: URL
    private let session: URLSession

    /// The session's request timeout is the longest it waits between two pieces of data. The default of 60
    /// seconds suits event streams as well, since the server sends a keep-alive every 15.
    init(baseURL: URL, session: URLSession) {
        self.baseURL = baseURL
        self.session = session
    }

    /// Uploads the audio and returns the job's id.
    ///
    /// `idempotencyKey` identifies this upload across attempts. If a response is lost and the upload is sent again
    /// with the same key, the server returns the job it already created instead of making a second one.
    func createJob(wav: Data, sourceLanguage: Language, idempotencyKey: String) async throws -> String {
        var form = MultipartForm()
        form.addField("source_lang", value: sourceLanguage.tag)
        form.addFile("audio", filename: "audio.wav", contentType: "audio/wav", data: wav)

        var request = URLRequest(url: url("/v1/jobs"))
        request.httpMethod = "POST"
        request.setValue(form.contentType, forHTTPHeaderField: "Content-Type")
        request.setValue(idempotencyKey, forHTTPHeaderField: "Idempotency-Key")
        request.httpBody = form.body()

        let (body, response) = try await send(request)
        guard (200..<300).contains(response.statusCode) else {
            throw Self.refusal(body, response)
        }
        do {
            return try JSONDecoder().decode(CreateJobResponse.self, from: body).id
        } catch {
            throw BackendError(message: "The server sent an unreadable response", retryable: false)
        }
    }

    /// Asks for `text` to be spoken in `language`, in the voice heard in `voiceWAV`. The server queues it like a
    /// transcription: follow `jobEvents` until it is done, then fetch `speechAudio`. `idempotencyKey` works as for
    /// `createJob`.
    func createSpeech(voiceWAV: Data, text: String, language: Language, idempotencyKey: String) async throws -> SpeechJob {
        var form = MultipartForm()
        form.addField("text", value: text)
        form.addField("lang", value: language.tag)
        form.addFile("voice", filename: "voice.wav", contentType: "audio/wav", data: voiceWAV)

        var request = URLRequest(url: url("/v1/speech"))
        request.httpMethod = "POST"
        request.setValue(form.contentType, forHTTPHeaderField: "Content-Type")
        request.setValue(idempotencyKey, forHTTPHeaderField: "Idempotency-Key")
        request.httpBody = form.body()

        let (body, response) = try await send(request)
        guard (200..<300).contains(response.statusCode) else {
            throw Self.refusal(body, response)
        }
        do {
            let created = try JSONDecoder().decode(CreateSpeechResponse.self, from: body)
            return SpeechJob(id: created.id, audioURL: created.audioURL)
        } catch {
            throw BackendError(message: "The server sent an unreadable response", retryable: false)
        }
    }

    /// The audio a finished speech job produced, as a WAV file.
    func speechAudio(_ audioURL: String) async throws -> Data {
        var request = URLRequest(url: url(audioURL))
        // It is the user's voice: never kept in a cache on the phone either.
        request.cachePolicy = .reloadIgnoringLocalCacheData
        let (body, response) = try await send(request)
        guard (200..<300).contains(response.statusCode) else {
            throw Self.refusal(body, response)
        }
        return body
    }

    /// Fetches the list of models the backend offers.
    func modelManifest() async throws -> [RemoteModel] {
        var request = URLRequest(url: url("/v1/models/manifest"))
        request.cachePolicy = .reloadIgnoringLocalCacheData
        let (body, response) = try await send(request)
        guard (200..<300).contains(response.statusCode) else {
            throw Self.refusal(body, response)
        }
        do {
            return try JSONDecoder().decode(ManifestResponse.self, from: body).models
        } catch {
            throw BackendError(message: "The server sent an unreadable model list", retryable: false)
        }
    }

    /// Opens a model for download, starting at `fromByte`. The caller reads the bytes as they arrive.
    ///
    /// A resume sends two headers. `Range` asks for the rest of the file. `If-Range` adds a condition, "only if the
    /// file is still this one", using the checksum the server publishes as its ETag. If the file was replaced
    /// meanwhile, the server sends the whole new one instead.
    func openModel(_ model: RemoteModel, fromByte: Int64) async throws -> ModelDownload {
        var request = URLRequest(url: url(model.url))
        request.cachePolicy = .reloadIgnoringLocalCacheData
        if fromByte > 0 {
            request.setValue("bytes=\(fromByte)-", forHTTPHeaderField: "Range")
            request.setValue("\"\(model.sha256)\"", forHTTPHeaderField: "If-Range")
        }

        let bytes: URLSession.AsyncBytes
        let response: URLResponse
        do {
            (bytes, response) = try await session.bytes(for: request)
        } catch {
            throw Self.connectionError(error, "Could not reach the server")
        }
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard status == 200 || status == 206 else {
            throw BackendError(message: "The server answered \(status)", retryable: status >= 500)
        }
        return ModelDownload(partial: status == 206, bytes: bytes)
    }

    /// Opens a job's event stream. Its events can then be read as they arrive, until the server closes the
    /// stream; cancelling the reading task closes the connection.
    ///
    /// `afterSeq` is the number of the last event already received, or 0 for the whole history. It is sent as
    /// Last-Event-ID, the standard header for resuming an event stream.
    func jobEvents(jobID: String, afterSeq: Int = 0) async throws -> JobEvents {
        var request = URLRequest(url: url("/v1/jobs/\(jobID)/events"))
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        if afterSeq > 0 {
            request.setValue(String(afterSeq), forHTTPHeaderField: "Last-Event-ID")
        }

        let bytes: URLSession.AsyncBytes
        let response: URLResponse
        do {
            (bytes, response) = try await session.bytes(for: request)
        } catch {
            throw Self.connectionError(error, "Could not reach the server")
        }
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard (200..<300).contains(status) else {
            // The server answered and said no, e.g. 404 for an unknown job.
            throw BackendError(message: "The server answered \(status)", retryable: status >= 500)
        }
        return JobEvents(bytes: bytes)
    }

    private func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw Self.connectionError(error, "Could not reach the server")
        }
        guard let http = response as? HTTPURLResponse else {
            throw BackendError(message: "The server's answer was not HTTP", retryable: false)
        }
        return (data, http)
    }

    private func url(_ path: String) -> URL {
        URL(string: path, relativeTo: baseURL)!.absoluteURL
    }

    /// What a failed connection becomes. A cancelled task stays cancelled, so that it is never retried.
    fileprivate static func connectionError(_ error: any Error, _ prefix: String) -> any Error {
        if error is CancellationError || (error as? URLError)?.code == .cancelled {
            return CancellationError()
        }
        return BackendError(message: "\(prefix): \(error.localizedDescription)", retryable: true)
    }

    /// A server fault is worth retrying; a request the server understood and refused is not.
    private static func refusal(_ body: Data, _ response: HTTPURLResponse) -> BackendError {
        BackendError(
            message: errorMessage(in: body) ?? "The server answered \(response.statusCode)",
            retryable: response.statusCode >= 500
        )
    }

    private static func errorMessage(in body: Data) -> String? {
        try? JSONDecoder().decode(ErrorResponse.self, from: body).error.message
    }
}

/// A job's events in the order the server sent them. A failure while reading is a `BackendError`.
struct JobEvents: AsyncSequence {
    typealias Element = JobUpdate

    fileprivate let bytes: URLSession.AsyncBytes

    func makeAsyncIterator() -> AsyncIterator {
        AsyncIterator(bytes: bytes.makeAsyncIterator())
    }

    struct AsyncIterator: AsyncIteratorProtocol {
        fileprivate var bytes: URLSession.AsyncBytes.AsyncIterator
        private var parser = ServerSentEventParser()

        fileprivate init(bytes: URLSession.AsyncBytes.AsyncIterator) {
            self.bytes = bytes
        }

        mutating func next() async throws -> JobUpdate? {
            while true {
                let byte: UInt8?
                do {
                    byte = try await bytes.next()
                } catch {
                    throw BackendClient.connectionError(error, "The event stream broke")
                }
                guard let byte else { return nil } // the server closed the stream
                guard let event = parser.consume(byte) else { continue }
                // Every event the server sends carries its number as the id.
                guard let seq = event.id.flatMap({ Int($0) }) else { continue }
                return JobUpdate(seq: seq, event: try JobEvent(type: event.type, data: event.data))
            }
        }
    }
}

private extension JobEvent {
    init(type: String?, data: String) throws {
        let json = Data(data.utf8)
        let decoder = JSONDecoder()
        do {
            switch type {
            case "queued": self = .queued
            case "processing": self = .processing
            case "partial": self = .partial(try decoder.decode(TextPayload.self, from: json).text)
            case "done":
                let done = try decoder.decode(DonePayload.self, from: json)
                self = .done(text: done.text ?? "", language: done.language ?? "")
            case "error": self = .failed(try decoder.decode(MessagePayload.self, from: json).message)
            default: self = .unknown // a type added later; older apps skip it
            }
        } catch {
            throw BackendError(message: "The server sent an unreadable \(type ?? "") event", retryable: false)
        }
    }
}

/// A `multipart/form-data` body, the form encoding the upload endpoint takes.
private struct MultipartForm {
    private let boundary = "Boundary-\(UUID().uuidString)"
    private var data = Data()

    var contentType: String { "multipart/form-data; boundary=\(boundary)" }

    mutating func addField(_ name: String, value: String) {
        append("--\(boundary)\r\nContent-Disposition: form-data; name=\"\(name)\"\r\n\r\n\(value)\r\n")
    }

    mutating func addFile(_ name: String, filename: String, contentType: String, data file: Data) {
        append("--\(boundary)\r\n")
        append("Content-Disposition: form-data; name=\"\(name)\"; filename=\"\(filename)\"\r\n")
        append("Content-Type: \(contentType)\r\n\r\n")
        data.append(file)
        append("\r\n")
    }

    func body() -> Data {
        data + Data("--\(boundary)--\r\n".utf8)
    }

    private mutating func append(_ text: String) {
        data.append(Data(text.utf8))
    }
}

private struct ManifestResponse: Decodable {
    let models: [RemoteModel]
}

private struct CreateJobResponse: Decodable {
    let id: String
}

private struct CreateSpeechResponse: Decodable {
    let id: String
    let audioURL: String

    enum CodingKeys: String, CodingKey {
        case id
        case audioURL = "audio_url"
    }
}

private struct TextPayload: Decodable {
    let text: String
}

/// A speech job ends with an empty "done": its result is the audio, fetched separately.
private struct DonePayload: Decodable {
    let text: String?
    let language: String?
}

private struct MessagePayload: Decodable {
    let message: String
}

private struct ErrorResponse: Decodable {
    struct Detail: Decodable {
        let message: String
    }

    let error: Detail
}
