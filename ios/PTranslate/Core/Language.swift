import Foundation

/// A language as a BCP 47 tag such as "en" or "yo".
struct Language: Hashable, Sendable {
    let tag: String

    init(_ tag: String) {
        self.tag = tag
    }

    static let english = Language("en")
    static let spanish = Language("es")
}

// Apple's frameworks list a language once per region ("en-US", "en-GB" ...), and call Norwegian "nb" (Bokmål)
// where Whisper, the server and Android say "no". This finds ours in theirs: the phone's voices, for one.
extension Language {
    /// The entry in one of Apple's lists that stands for this language: the one Apple treats as the language's
    /// default, which needs no region to name it (American English, Brazilian Portuguese), or else the first.
    func match(in apple: [Locale.Language]) -> Locale.Language? {
        let code = tag == "no" ? "nb" : tag
        let candidates = apple.filter { $0.languageCode?.identifier == code }
        return candidates.first { $0.minimalIdentifier == code } ?? candidates.first
    }
}
