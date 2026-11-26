import SwiftUI

/// Before anything goes to them: who helps carry the user's words to the other person, and what each gets
/// (Guideline 5.1.2(i)). Shown once, after signing up, and to everyone who signed up before it, before the app opens.
/// Not designed yet: built from the sign-up steps' parts.
struct ServicesScreen: View {
    let account: Account

    @State private var saving = false
    @State private var problem: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                Text("How your words travel").font(.pearlScreenTitle).foregroundStyle(Palette.fg)
                    .padding(.top, 52)
                    .accessibilityAddTraits(.isHeader)
                Text("Three services help \(Brand.name) carry what you say to the other person. Each gets only what its part needs, never your name or your ID.")
                    .font(.pearlLead).foregroundStyle(Palette.fg2).padding(.top, 8)

                VStack(alignment: .leading, spacing: 20) {
                    ForEach(Self.services, id: \.name) { service in
                        HStack(alignment: .top, spacing: 14) {
                            Image(systemName: service.symbol).font(.system(size: 20)).foregroundStyle(Palette.them)
                                .frame(width: 28)
                            VStack(alignment: .leading, spacing: 4) {
                                Text(service.name).font(.pearlMessageStrong).foregroundStyle(Palette.fg)
                                Text(service.text).font(.pearlMessage).foregroundStyle(Palette.fg2)
                            }
                        }
                        .accessibilityElement(children: .combine)
                    }
                }
                .padding(.top, 28)

                Text("You can read this again in Profile, under What we keep, and why.")
                    .font(.pearlBody).foregroundStyle(Palette.fg2).padding(.top, 24)
                if let problem {
                    Notice(text: problem).padding(.top, 16)
                }
            }
            .padding(.horizontal, 20)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .safeAreaInset(edge: .bottom) {
            VStack(spacing: 6) {
                Button(action: agree) { saving ? Text("Saving…") : Text("Agree and continue") }
                    .buttonStyle(PrimaryButtonStyle())
                    .disabled(saving)
                // Without them nothing can be translated or spoken, so the way out is to leave.
                Button("Not now, sign out") { Task { await account.signOut() } }
                    .font(.pearlMeta).foregroundStyle(Palette.fg2)
                    .padding(.vertical, 8)
            }
            .padding(.horizontal, 16).padding(.bottom, 8)
        }
        .background(Palette.bg)
    }

    private func agree() {
        saving = true
        problem = nil
        Task {
            do {
                try await account.agreeToServices()
            } catch {
                problem = String(localized: "That couldn't be saved just now. Please try again.")
            }
            saving = false
        }
    }

    /// The same services, in the same words, as the privacy page.
    private static var services: [(name: String, symbol: String, text: String)] {
        [
            (String(localized: "Groq writes down your words"),
             "text.quote",
             String(localized: "Your phone writes down what you say by itself when it can. When it can't, the recording goes to our server, and Groq writes the words down.")),
            (String(localized: "Microsoft translates them"),
             "character.book.closed",
             String(localized: "Microsoft's Azure AI Translator translates your words into the other person's language.")),
            (String(localized: "Fish Audio speaks them in your voice"),
             "waveform",
             String(localized: "Fish Audio makes the translation speak in your voice, from the recording you make of it.")),
        ]
    }
}
