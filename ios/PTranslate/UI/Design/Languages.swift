import SwiftUI

/// The two languages of the interpreter: the speaker's in the colour that means "you", the listener's in the colour
/// that means "them", with a swap between.
struct LanguageChips: View {
    let source: Language
    let target: Language
    let enabled: Bool
    let swapEnabled: Bool
    let onSourceTapped: () -> Void
    let onTargetTapped: () -> Void
    let onSwapTapped: () -> Void

    var body: some View {
        HStack(spacing: 8) {
            chip(source.ownName, fill: Palette.you, text: Palette.onYou, action: onSourceTapped)
                .accessibilityLabel("You speak \(source.localName)")
                .accessibilityHint("Changes the language you speak")
            Button(action: onSwapTapped) {
                Image(systemName: "arrow.left.arrow.right")
                    .font(.system(size: 17))
                    .foregroundStyle(Palette.fg)
                    .frame(width: 44, height: 44)
            }
            .buttonStyle(.plain)
            .disabled(!enabled || !swapEnabled)
            .opacity(enabled && swapEnabled ? 1 : 0.4)
            .accessibilityLabel("Swap the languages")
            chip(target.ownName, fill: Palette.bgRaised, text: Palette.them, action: onTargetTapped)
                .accessibilityLabel("They hear \(target.localName)")
                .accessibilityHint("Changes the language they hear")
        }
        .disabled(!enabled)
    }

    private func chip(_ title: String, fill: Color, text: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(.pearlChip)
                .foregroundStyle(text)
                .padding(.vertical, 8)
                .padding(.horizontal, 14)
                .background(fill, in: Capsule())
        }
        .buttonStyle(.plain)
    }
}

/// A searchable list of languages, in a sheet. Each shows its own name, and the phone's name for it.
struct LanguagePicker: View {
    let title: String
    let options: [Language]
    let selected: Language
    let onSelect: (Language) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var query = ""

    private var shown: [Language] {
        let sorted = options.sorted { $0.localName.localizedStandardCompare($1.localName) == .orderedAscending }
        guard !query.isEmpty else { return sorted }
        return sorted.filter { $0.ownName.localizedStandardContains(query) || $0.localName.localizedStandardContains(query) }
    }

    var body: some View {
        NavigationStack {
            List(shown, id: \.self) { language in
                Button {
                    onSelect(language)
                    dismiss()
                } label: {
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(language.ownName)
                                .font(.pearlMessage)
                                .foregroundStyle(Palette.fg)
                            if language.localName != language.ownName {
                                Text(language.localName)
                                    .font(.pearlSmall)
                                    .foregroundStyle(Palette.fg2)
                            }
                        }
                        Spacer()
                        if language == selected {
                            Image(systemName: "checkmark")
                                .foregroundStyle(Palette.them)
                                .accessibilityLabel("Selected")
                        }
                    }
                    .contentShape(Rectangle())
                }
                .listRowBackground(Palette.bgRaised)
            }
            .scrollContentBackground(.hidden)
            .background(Palette.bg)
            .searchable(text: $query)
            .navigationTitle(title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }
}
