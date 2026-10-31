import Foundation
import Observation

/// The stream of changes to the user's rooms, kept open while any screen listens and reopened when it drops,
/// waiting longer after each failure. One connection serves every screen.
@MainActor
@Observable
final class RoomFeed {
    /// False while the stream is down and being reopened, for the "Reconnecting" notice.
    private(set) var connected = true

    @ObservationIgnored private let rooms: RoomsClient
    @ObservationIgnored private let sleep: @Sendable (Duration) async throws -> Void
    @ObservationIgnored private var listeners: [UUID: AsyncStream<RoomChange>.Continuation] = [:]
    /// Owned by the feed: started with the first listener, cancelled after the last.
    @ObservationIgnored private var connection: Task<Void, Never>?

    init(rooms: RoomsClient, sleep: @escaping @Sendable (Duration) async throws -> Void = { try await Task.sleep(for: $0) }) {
        self.rooms = rooms
        self.sleep = sleep
    }

    /// The changes, for one screen. Stop iterating, and it stops listening.
    func changes() -> AsyncStream<RoomChange> {
        let id = UUID()
        let (stream, continuation) = AsyncStream.makeStream(of: RoomChange.self, bufferingPolicy: .bufferingNewest(32))
        continuation.onTermination = { [weak self] _ in
            Task { @MainActor in self?.stopListening(id) }
        }
        listeners[id] = continuation
        if connection == nil { connect() }
        return stream
    }

    private func stopListening(_ id: UUID) {
        listeners[id] = nil
        if listeners.isEmpty {
            connection?.cancel()
            connection = nil
        }
    }

    private func connect() {
        connection = Task { [weak self, rooms, sleep] in
            var attempt = 0
            while !Task.isCancelled {
                do {
                    for try await change in try await rooms.changes() {
                        if change == .ready {
                            attempt = 0
                            self?.connected = true
                        }
                        self?.broadcast(change)
                    }
                } catch is CancellationError {
                    return
                } catch {
                    // Reopened below, like a stream the server closed.
                }
                self?.connected = false
                do {
                    try await sleep(.milliseconds(min(30_000, 1000 << min(attempt, 5))))
                } catch {
                    return
                }
                attempt += 1
            }
        }
    }

    private func broadcast(_ change: RoomChange) {
        for listener in listeners.values {
            listener.yield(change)
        }
    }
}
