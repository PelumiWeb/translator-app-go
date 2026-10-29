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

        /// Any status, headers and bytes, for answers the cases above do not cover, such as part of a file.
        case data(status: Int, headers: [String: String] = [:], body: Data)

        /// The same, but the connection is lost once the bytes are sent.
        case dataThenDrop(status: Int, headers: [String: String] = [:], body: Data)
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
        var handler: (@Sendable (Request) -> Response)?
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

    /// Answers every request with what `handler` returns, instead of from the queue: for a server whose answer
    /// depends on the request, like one that serves byte ranges.
    func respond(with handler: @escaping @Sendable (Request) -> Response) {
        state.withLock { $0.handler = handler }
    }

    /// From now on every connection is refused, and nothing more is recorded.
    func shutdown() {
        state.withLock { $0.reachable = false }
    }

    var requests: [Request] { state.withLock { $0.requests } }

    var requestCount: Int { requests.count }

    /// nil when the connection should be refused: the server is shut down, or has nothing left to say.
    fileprivate func respond(to request: Request) -> Response? {
        let next: (handler: (@Sendable (Request) -> Response)?, queued: Response?)? = state.withLock { state in
            guard state.reachable else { return nil }
            state.requests.append(request)
            if let handler = state.handler {
                return (handler, nil)
            }
            return (nil, state.queued.isEmpty ? nil : state.queued.removeFirst())
        }
        // The handler is called outside the lock, so it may use the server itself.
        guard let next else { return nil }
        return next.handler?(request) ?? next.queued
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
            failSoon(.networkConnectionLost)
        case .dropAfterRequest?:
            fail(.networkConnectionLost)
        case .data(let status, let headers, let body)?:
            send(status, headers: headers, body: body)
            client?.urlProtocolDidFinishLoading(self)
        case .dataThenDrop(let status, let headers, let body)?:
            send(status, headers: headers, body: body)
            failSoon(.networkConnectionLost)
        }
    }

    override func stopLoading() {}

    private func send(_ status: Int, contentType: String, body: String) {
        send(status, headers: ["Content-Type": contentType], body: Data(body.utf8))
    }

    private func send(_ status: Int, headers: [String: String], body: Data) {
        let response = HTTPURLResponse(url: request.url!, statusCode: status, httpVersion: "HTTP/1.1", headerFields: headers)!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        if !body.isEmpty {
            client?.urlProtocol(self, didLoad: body)
        }
    }

    private func fail(_ code: URLError.Code) {
        client?.urlProtocol(self, didFailWithError: URLError(code))
    }

    /// Fails a moment after the body was sent, as a real connection does: bytes arrive, then the line drops.
    /// Failing at once is not the same: `URLSession.AsyncBytes` discards bytes it holds but has not handed over
    /// when an error arrives, so the client would see none of them. The failure is delivered on the thread that
    /// started loading, which is where a URLProtocol must talk to its client.
    private func failSoon(_ code: URLError.Code) {
        nonisolated(unsafe) let loop = CFRunLoopGetCurrent()
        DispatchQueue.global().asyncAfter(deadline: .now() + .milliseconds(200)) {
            CFRunLoopPerformBlock(loop, CFRunLoopMode.commonModes.rawValue) { self.fail(code) }
            CFRunLoopWakeUp(loop)
        }
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
