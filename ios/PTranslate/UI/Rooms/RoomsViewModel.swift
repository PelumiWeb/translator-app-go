import Foundation
import Observation

/// The user's rooms, kept up to date by the feed, and adding someone new.
@MainActor
@Observable
final class RoomsViewModel {
    /// Looking someone up by ID, in the "Add someone" sheet.
    enum Lookup: Equatable {
        case idle
        case searching
        case found(Profile)
        case notFound(String)
    }

    private(set) var loading = true
    private(set) var rooms: [Room] = []
    var query = ""
    private(set) var problem: String?
    private(set) var lookupHandle = ""
    private(set) var lookup = Lookup.idle
    private(set) var opening = false
    /// Set once a room has been opened from the sheet, for the screen to go to it.
    var opened: Room?

    var me: Profile? { account.profile }
    var connected: Bool { feed.connected }

    var shown: [Room] {
        guard !query.isEmpty else { return rooms }
        return rooms.filter { room in
            room.people.contains { $0.displayName.localizedCaseInsensitiveContains(query) || $0.handle.localizedCaseInsensitiveContains(query) }
        }
    }

    @ObservationIgnored private let client: RoomsClient
    @ObservationIgnored private let account: Account
    @ObservationIgnored private let feed: RoomFeed
    @ObservationIgnored private let lookupDelay: Duration
    @ObservationIgnored private var lookupTask: Task<Void, Never>?
    @ObservationIgnored private var openTask: Task<Void, Never>?

    init(client: RoomsClient, account: Account, feed: RoomFeed, lookupDelay: Duration = .milliseconds(400)) {
        self.client = client
        self.account = account
        self.feed = feed
        self.lookupDelay = lookupDelay
    }

    isolated deinit {
        lookupTask?.cancel()
        openTask?.cancel()
    }

    /// Loads the rooms, then again after every change, for as long as the screen shows. Run it from `.task`.
    func follow() async {
        await load()
        for await _ in feed.changes() {
            await load()
        }
    }

    func load() async {
        do {
            rooms = try await client.rooms()
            problem = nil
        } catch is CancellationError {
        } catch let error as BackendError {
            problem = error.status == 0 ? "No connection. Your rooms will appear when you're back online." : "Your rooms couldn't be loaded just now."
        } catch {}
        loading = false
    }

    /// Looks the ID up once typing pauses.
    func lookupChanged(_ input: String) {
        let handle = Handles.normalise(input)
        lookupTask?.cancel()
        lookupHandle = handle
        guard Handles.problem(with: handle) == nil else {
            lookup = .idle
            return
        }
        lookup = .searching
        lookupTask = Task { [weak self, account, lookupDelay] in
            do {
                try await Task.sleep(for: lookupDelay)
                let result: Lookup
                if handle == account.profile?.handle {
                    result = .notFound("That's your own ID.")
                } else {
                    do {
                        result = .found(try await account.findUser(handle))
                    } catch let error as BackendError {
                        result = .notFound(
                            error.code == "user_not_found" ? "No one has the ID @\(handle) yet. Check the spelling with them."
                                : error.status == 0 ? "No connection, so this ID couldn't be looked up."
                                : "This ID couldn't be looked up just now."
                        )
                    }
                }
                guard let self, self.lookupHandle == handle else { return }
                self.lookup = result
            } catch {}
        }
    }

    /// Opens the room with the person found. It opens at once: their first note from you reaches them straight away.
    func addTapped() {
        guard case .found(let person) = lookup, !opening else { return }
        opening = true
        openTask = Task { [self] in
            defer { opening = false }
            do {
                let room = try await client.openRoom(handle: person.handle)
                lookupHandle = ""
                lookup = .idle
                opened = room
                await load()
            } catch {
                lookup = .notFound("The room couldn't be opened just now. Please try again.")
            }
        }
    }

    func sheetClosed() {
        lookupTask?.cancel()
        lookupHandle = ""
        lookup = .idle
    }
}
