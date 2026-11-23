import SwiftUI

/// Reporting a message, or the person: why, anything else to say, and whether to block them too. Not designed yet:
/// built from Pearl's list rows, as Profile is.
struct ReportSheet: View {
    let target: RoomViewModel.ReportTarget
    /// The person reported, as the room names them.
    let name: String
    /// Sends the report; returns what went wrong, or nil once it is sent.
    let onSend: (ReportReason, String, Bool) async -> String?

    @Environment(\.dismiss) private var dismiss
    @State private var reason: ReportReason?
    @State private var note = ""
    @State private var alsoBlock = true
    @State private var sending = false
    @State private var problem: String?

    /// As the server allows.
    static let maxNote = 500

    var body: some View {
        NavigationStack {
            List {
                Section {
                    ForEach(ReportReason.allCases) { choice in
                        Button { reason = choice } label: {
                            HStack {
                                Text(choice.words).foregroundStyle(Palette.fg)
                                Spacer()
                                if reason == choice {
                                    Image(systemName: "checkmark").foregroundStyle(Palette.you)
                                }
                            }
                            .contentShape(Rectangle())
                        }
                        .accessibilityAddTraits(reason == choice ? .isSelected : [])
                    }
                } header: {
                    sectionHeader("What's wrong?")
                }
                .listRowBackground(Palette.bgRaised)

                Section {
                    TextField("Anything else we should know (optional)", text: $note, axis: .vertical)
                        .lineLimit(2...5)
                        .onChange(of: note) { if note.count > Self.maxNote { note = String(note.prefix(Self.maxNote)) } }
                    Toggle("Also block \(name)", isOn: $alsoBlock).tint(Palette.you)
                } footer: {
                    Text(footer).font(.pearlMeta).foregroundStyle(Palette.fg2)
                }
                .listRowBackground(Palette.bgRaised)

                if let problem {
                    Notice(text: problem).listRowBackground(Color.clear)
                }
            }
            .font(.pearlLead)
            .foregroundStyle(Palette.fg)
            .scrollContentBackground(.hidden)
            .background(Palette.bg)
            .navigationTitle(title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    if sending {
                        ProgressView()
                    } else {
                        Button("Send") { send() }.disabled(reason == nil)
                    }
                }
            }
            .disabled(sending)
        }
        .presentationBackground(Palette.bg)
    }

    private var title: String {
        switch target {
        case .message(let message): message.isText ? "Report this message" : "Report this voice note"
        case .person: "Report \(name)"
        }
    }

    private var footer: String {
        let kept = switch target {
        case .message: "We keep the message you report with your report until we've looked into it, even if they delete their account. "
        case .person: ""
        }
        return kept + "We look at every report within a day. \(name) isn't told who reported them."
    }

    private func send() {
        guard let reason else { return }
        sending = true
        problem = nil
        Task {
            problem = await onSend(reason, note, alsoBlock)
            sending = false
        }
    }

    private func sectionHeader(_ text: String) -> some View {
        Text(text).font(.pearlMeta).foregroundStyle(Palette.fg2).textCase(nil)
    }
}

extension ReportReason {
    /// How each reason reads on the sheet.
    var words: String {
        switch self {
        case .spam: "Spam or a scam"
        case .harassment: "Harassment or bullying"
        case .hate: "Hate"
        case .sexual: "Sexual content"
        case .violence: "Threats or violence"
        case .selfHarm: "Someone may be at risk"
        case .other: "Something else"
        }
    }
}
