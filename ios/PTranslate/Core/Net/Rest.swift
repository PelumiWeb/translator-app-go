import Foundation

/// JSON requests on behalf of the signed-in user, shared by `AccountClient` and `RoomsClient`. Each request carries
/// the session token from `token`; a refusal comes back as a `BackendError` with the server's error code. A 401 on a
/// request that carried a token means the session has ended, and `sessionEnded` is told.
final class Rest: Sendable {
    let session: URLSession
    private let baseURL: URL
    private let token: @Sendable () async -> String?
    private let sessionEnded: @Sendable () async -> Void

    init(
        baseURL: URL,
        session: URLSession,
        token: @escaping @Sendable () async -> String?,
        sessionEnded: @escaping @Sendable () async -> Void
    ) {
        self.baseURL = baseURL
        self.session = session
        self.token = token
        self.sessionEnded = sessionEnded
    }

    func url(_ path: String) -> URL {
        URL(string: path, relativeTo: baseURL)!.absoluteURL
    }

    /// A request to `path`, with the session token when there is one.
    func request(_ path: String, method: String = "GET") async -> URLRequest {
        var request = URLRequest(url: url(path))
        request.httpMethod = method
        if let token = await token() {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return request
    }

    @discardableResult
    func send(_ request: URLRequest) async throws -> Data {
        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw BackendClient.connectionError(error, "Could not reach the server")
        }
        guard let http = response as? HTTPURLResponse else {
            throw BackendError(message: "The server's answer was not HTTP", retryable: false)
        }
        if http.statusCode == 401, request.value(forHTTPHeaderField: "Authorization") != nil {
            await sessionEnded()
        }
        guard (200..<300).contains(http.statusCode) else {
            throw BackendClient.refusal(data, http)
        }
        return data
    }

    func get<T: Decodable>(_ path: String, as type: T.Type = T.self) async throws -> T {
        try decode(try await send(await request(path)))
    }

    /// Sends `body` as JSON with `method`, and decodes the answer.
    func send<T: Decodable>(_ method: String, _ path: String, json body: some Encodable, as type: T.Type = T.self) async throws -> T {
        var request = await request(path, method: method)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(body)
        return try decode(try await send(request))
    }

    func decode<T: Decodable>(_ data: Data, as type: T.Type = T.self) throws -> T {
        do {
            return try Self.decoder.decode(T.self, from: data)
        } catch {
            throw BackendError(message: "The server sent an unreadable response", retryable: false)
        }
    }

    /// The server writes times as RFC 3339, with fractions of a second; Foundation's ISO 8601 strategy refuses those.
    static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let text = try decoder.singleValueContainer().decode(String.self)
            if let date = try? Date(text, strategy: .iso8601.year().month().day().time(includingFractionalSeconds: true).timeZone(separator: .omitted)) {
                return date
            }
            if let date = try? Date(text, strategy: .iso8601) {
                return date
            }
            throw DecodingError.dataCorrupted(.init(codingPath: decoder.codingPath, debugDescription: "not a date: \(text)"))
        }
        return decoder
    }()
}
