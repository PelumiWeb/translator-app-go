import Foundation
import Synchronization

/// A fake backend for `URLSession`, the counterpart of OkHttp's MockWebServer on Android. Responses are queued
/// with `enqueue` and served in order, and every request is recorded.
///
/// Each server answers under a host name of its own, so tests that run in parallel do not share one.
final class StubServer: Sendable {
    enum Response: Sendable {
        case json(status: Int, body: String)
        case status(Int, body: String = "")

        /// An event stream that sends `body` and then closes, as the real server does.
        case events(String)

        /// An event stream that sends `body` and then loses the connection.
        case eventsThenDrop(String)

        /// The request arrives, then the connection dies before any response, so the client cannot know whether
        /// it was acted on.
        case dropAfterRequest
    }

    struct Request: Sendable {
        let method: String
        let path: String
        let headers: [String: String]
        let body: Data

        func header(_ name: String) -> String? {
            headers.first { $0.key.caseInsensitiveCompare(name) == .orderedSame }?.value
        }

        var bodyText: String { String(decoding: body, as: UTF8.self) }
    }

    let baseURL: URL
    let session: URLSession

    private struct State {
        var queued: [Response] = []
        var requests: [Request] = []
        var reachable = true
    }

    private let state = Mutex(State())

    init() {
        let host = "stub-\(UUID().uuidString.lowercased()).test"
        baseURL = URL(string: "http://\(host)")!
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubProtocol.self]
        session = URLSession(configuration: configuration)
        // Never removed: a few small objects per test run is not worth the bookkeeping.
        StubProtocol.servers.withLock { $0[host] = self }
    }

    func enqueue(_ response: Response) {
        state.withLock { $0.queued.append(response) }
    }

    /// From now on every connection is refused, and nothing more is recorded.
    func shutdown() {
        state.withLock { $0.reachable = false }
    }

    var requests: [Request] { state.withLock { $0.requests } }

    var requestCount: Int { requests.count }

    /// nil when the connection should be refused: the server is shut down, or has nothing left to say.
    fileprivate func respond(to request: Request) -> Response? {
        state.withLock { state in
            guard state.reachable else { return nil }
            state.requests.append(request)
            return state.queued.isEmpty ? nil : state.queued.removeFirst()
        }
    }
}

/// Plugged into a `URLSession` through its configuration; hands each request to the server that owns its host.
private final class StubProtocol: URLProtocol, @unchecked Sendable {
    static let servers = Mutex<[String: StubServer]>([:])

    override class func canInit(with request: URLRequest) -> Bool { true }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        guard
            let url = request.url,
            let host = url.host(),
            let server = Self.servers.withLock({ $0[host] })
        else {
            fail(.cannotFindHost)
            return
        }

        let recorded = StubServer.Request(
            method: request.httpMethod ?? "GET",
            path: url.path(),
            headers: request.allHTTPHeaderFields ?? [:],
            body: requestBody()
        )
        switch server.respond(to: recorded) {
        case nil:
            fail(.cannotConnectToHost)
        case .json(let status, let body)?:
            send(status, contentType: "application/json", body: body)
            client?.urlProtocolDidFinishLoading(self)
        case .status(let status, let body)?:
            send(status, contentType: "application/json", body: body)
            client?.urlProtocolDidFinishLoading(self)
        case .events(let body)?:
            send(200, contentType: "text/event-stream", body: body)
            client?.urlProtocolDidFinishLoading(self)
        case .eventsThenDrop(let body)?:
            send(200, contentType: "text/event-stream", body: body)
            fail(.networkConnectionLost)
        case .dropAfterRequest?:
            fail(.networkConnectionLost)
        }
    }

    override func stopLoading() {}

    private func send(_ status: Int, contentType: String, body: String) {
        let response = HTTPURLResponse(
            url: request.url!,
            statusCode: status,
            httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": contentType]
        )!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        if !body.isEmpty {
            client?.urlProtocol(self, didLoad: Data(body.utf8))
        }
    }

    private func fail(_ code: URLError.Code) {
        client?.urlProtocol(self, didFailWithError: URLError(code))
    }

    /// URLSession hands a protocol the body as a stream, not as `httpBody`.
    private func requestBody() -> Data {
        if let body = request.httpBody {
            return body
        }
        guard let stream = request.httpBodyStream else { return Data() }
        stream.open()
        defer { stream.close() }
        var body = Data()
        var chunk = [UInt8](repeating: 0, count: 16 * 1024)
        while stream.hasBytesAvailable {
            let read = stream.read(&chunk, maxLength: chunk.count)
            guard read > 0 else { break }
            body.append(chunk, count: read)
        }
        return body
    }
}
