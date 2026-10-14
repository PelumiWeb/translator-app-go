/// A language as a BCP 47 tag such as "en" or "yo".
struct Language: Hashable, Sendable {
    let tag: String

    init(_ tag: String) {
        self.tag = tag
    }

    static let english = Language("en")
}
