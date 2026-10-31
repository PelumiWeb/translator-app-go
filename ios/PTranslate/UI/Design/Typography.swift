import SwiftUI

/// The design's type roles in IBM Plex Sans, bundled (SIL Open Font License, Resources/Fonts). Each grows with
/// Dynamic Type relative to a system style. Scripts Plex does not cover, such as Arabic or Thai, fall back to the
/// system's fonts.
extension Font {
    static let pearlTimer = medium(46, relativeTo: .largeTitle)
    static let pearlPassageCurrent = medium(25, relativeTo: .title)
    static let pearlPassageOther = regular(16, relativeTo: .body)
    static let pearlTranslation = medium(28, relativeTo: .title)
    static let pearlTitle = medium(22, relativeTo: .title2)
    static let pearlHeadline = medium(19, relativeTo: .title3)
    static let pearlMessage = regular(16, relativeTo: .body)
    static let pearlBody = regular(14, relativeTo: .subheadline)
    static let pearlSmall = regular(13, relativeTo: .footnote)
    static let pearlChip = medium(14, relativeTo: .subheadline)
    static let pearlLabel = medium(12, relativeTo: .caption)
    static let pearlMeta = regular(12, relativeTo: .caption)

    // The PostScript names inside the bundled files.
    private static func regular(_ size: CGFloat, relativeTo style: TextStyle) -> Font {
        .custom("IBMPlexSans", size: size, relativeTo: style)
    }

    private static func medium(_ size: CGFloat, relativeTo style: TextStyle) -> Font {
        .custom("IBMPlexSans-Medm", size: size, relativeTo: style)
    }
}
