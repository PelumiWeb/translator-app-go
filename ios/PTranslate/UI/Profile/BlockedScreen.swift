import Observation
import SwiftUI

/// The people the user has blocked, each with a way to unblock.
@MainActor
@Observable
final class BlockedViewModel {
    private(set) var people: [BlockedPerson] = []
    private(set) var loading = true
    private(set) var problem: String?

    @ObservationIgnored private let client: RoomsClient

    init(client: RoomsClient) {
        self.client = client
    }

    func load() async {
        do {
            people = try await client.blocked()
            problem = nil
        } catch is CancellationError {
        } catch {
            problem = "This list couldn't be loaded just now."
        }
        loading = false
    }

    func unblock(_ person: BlockedPerson) async {
        do {
            try await client.unblock(userID: person.id)
            people.removeAll { $0.id == person.id }
            problem = nil
        } catch {
            problem = "\(person.profile.shownName) couldn't be unblocked just now."
        }
    }
}

struct BlockedScreen: View {
    @State private var viewModel: BlockedViewModel

    init(client: RoomsClient) {
        _viewModel = State(initialValue: BlockedViewModel(client: client))
    }

    var body: some View {
        List {
            if let problem = viewModel.problem {
                Notice(text: problem).listRowBackground(Color.clear)
            }
            Section {
                ForEach(viewModel.people) { person in
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(person.profile.shownName).font(.pearlMessageStrong).foregroundStyle(Palette.fg)
                            if !person.profile.handle.isEmpty {
                                Text("@\(person.profile.handle)").font(.pearlMeta).foregroundStyle(Palette.fg2)
                            }
                        }
                        Spacer()
                        Button("Unblock") { Task { await viewModel.unblock(person) } }
                            .buttonStyle(.bordered)
                            .tint(Palette.you)
                    }
                }
            } footer: {
                Text("Neither of you can send messages to the other until you unblock them. They aren't told.")
                    .font(.pearlMeta).foregroundStyle(Palette.fg2)
            }
            .listRowBackground(Palette.bgRaised)
        }
        .overlay {
            if !viewModel.loading && viewModel.people.isEmpty && viewModel.problem == nil {
                Text("You haven't blocked anyone.").font(.pearlMessage).foregroundStyle(Palette.fg2)
            }
        }
        .font(.pearlLead)
        .foregroundStyle(Palette.fg)
        .scrollContentBackground(.hidden)
        .background(Palette.bg)
        .navigationTitle("Blocked people")
        .navigationBarTitleDisplayMode(.inline)
        .task { await viewModel.load() }
    }
}
