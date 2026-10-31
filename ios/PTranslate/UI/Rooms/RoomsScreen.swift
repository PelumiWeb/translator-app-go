import SwiftUI

/// The people the user talks with. `addHandle` is an ID from a link (timbre://add/...) to look up in the "Add
/// someone" sheet straight away.
struct RoomsScreen: View {
    let viewModel: RoomsViewModel
    @Binding var addHandle: String?
    let onOpenRoom: (Room) -> Void

    @State private var adding = false

    var body: some View {
        Group {
            if !viewModel.loading && viewModel.rooms.isEmpty {
                EmptyRooms(me: viewModel.me, onAdd: { adding = true })
            } else {
                list
            }
        }
        .background(Palette.bg)
        .overlay(alignment: .top) {
            if !viewModel.connected {
                Toast(text: "Connection lost. Reconnecting…").padding(.top, 8)
            }
        }
        .task { await viewModel.follow() }
        .sheet(isPresented: $adding, onDismiss: viewModel.sheetClosed) {
            if let me = viewModel.me {
                AddSomeoneSheet(viewModel: viewModel, me: me, onClose: { adding = false })
                    .presentationDragIndicator(.visible)
            }
        }
        .onChange(of: addHandle, initial: true) { _, handle in
            guard let handle else { return }
            adding = true
            viewModel.lookupChanged(handle)
            addHandle = nil
        }
        .onChange(of: viewModel.opened) { _, room in
            guard let room else { return }
            adding = false
            onOpenRoom(room)
            viewModel.opened = nil
        }
    }

    private var list: some View {
        @Bindable var viewModel = viewModel
        return List {
            if let problem = viewModel.problem {
                Notice(text: problem).listRowBackground(Color.clear)
            }
            ForEach(viewModel.shown) { room in
                Button { onOpenRoom(room) } label: { RoomRow(room: room, myLanguage: viewModel.me?.language) }
                    .buttonStyle(.plain)
                    .listRowBackground(Palette.bg)
                    .listRowSeparatorTint(Palette.fg.opacity(0.08))
                    .alignmentGuide(.listRowSeparatorLeading) { _ in 62 }
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .navigationTitle("Rooms")
        .searchable(text: $viewModel.query, prompt: "Search")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button { adding = true } label: { Image(systemName: "plus") }
                    .accessibilityLabel("Add someone")
            }
        }
        .refreshable { await viewModel.load() }
    }
}

private struct RoomRow: View {
    let room: Room
    let myLanguage: String?

    var body: some View {
        let name = room.other?.shownName ?? "Deleted account"
        HStack(alignment: .top, spacing: 14) {
            Avatar(name: name)
            VStack(alignment: .leading, spacing: 2) {
                HStack {
                    Text(name).font(.pearlMessageStrong).foregroundStyle(Palette.fg).lineLimit(1)
                    Spacer()
                    Text(roomTime(room.lastMessage?.createdAt ?? room.createdAt)).font(.pearlMeta).foregroundStyle(Palette.fg2)
                }
                HStack(alignment: .top) {
                    Text(room.lastMessage?.preview(myLanguage: myLanguage) ?? "No messages yet")
                        .font(.custom("IBMPlexSans", size: 15, relativeTo: .subheadline))
                        .foregroundStyle(Palette.fg.opacity(0.72))
                        .lineLimit(2)
                    Spacer(minLength: 8)
                    if room.unread > 0 {
                        Circle().fill(Palette.them).frame(width: 10, height: 10).padding(.top, 6)
                            .accessibilityLabel("\(room.unread) new")
                    }
                }
                if let language = room.other?.lang {
                    Text("hears \(language.ownName)").font(.pearlMeta).foregroundStyle(Palette.them).padding(.top, 2)
                }
            }
        }
        .padding(.vertical, 8)
        .contentShape(Rectangle())
    }
}

private struct EmptyRooms: View {
    let me: Profile?
    let onAdd: () -> Void

    var body: some View {
        VStack(spacing: 0) {
            Spacer()
            TwoVoices()
            Text("No one here yet.").font(.pearlTitle).foregroundStyle(Palette.fg).padding(.top, 28)
            Text("Add someone by their ID or QR code. You'll hear them in \(me?.lang?.localName ?? "your language"), in their own voice.")
                .font(.pearlLead).foregroundStyle(Palette.fg2)
                .multilineTextAlignment(.center)
                .padding(.top, 10).padding(.bottom, 24)
            Button("Add someone", action: onAdd).buttonStyle(PrimaryButtonStyle())
            if let me, !me.handle.isEmpty {
                ShareLink(item: addLink(me.handle), message: Text("Talk to me on Timbre, in your own language: my ID is @\(me.handle)")) {
                    Text("Share my ID, @\(me.handle)")
                }
                .buttonStyle(SecondaryButtonStyle())
                .padding(.top, 12)
            }
            Spacer()
        }
        .padding(.horizontal, 28)
        .navigationTitle("Rooms")
    }
}
