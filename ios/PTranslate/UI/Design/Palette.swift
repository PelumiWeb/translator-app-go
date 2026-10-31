import SwiftUI
import UIKit

/// The "Pearl" palette from the design handoff (docs/design/pearl). Two colours carry meaning, and only two: `you` for
/// everything that is the person holding the phone, `them` for the person they talk to. Layers are told apart by
/// colour, not by shadows.
///
/// Only dark is designed, so the app is dark whatever the phone's setting (`UIUserInterfaceStyle` in Info.plist).
/// The light values are the handoff's proposal, kept for when light mode is designed; they are not shown.
enum Palette {
    static let bg = dynamic(light: warmWhite, dark: petrol)
    static let bgRaised = dynamic(light: UIColor(hex: 0xDCEFE9), dark: UIColor(hex: 0x173539))
    static let bgDeep = dynamic(light: UIColor(hex: 0xE5DFD3), dark: UIColor(hex: 0x071416))
    static let fg = dynamic(light: petrol, dark: warmWhite)
    /// Secondary text, meta, timestamps.
    static let fg2 = dynamic(light: petrol.withAlphaComponent(0.64), dark: warmWhite.withAlphaComponent(0.64))
    /// Upcoming lines of the passage.
    static let fg3 = dynamic(light: petrol.withAlphaComponent(0.44), dark: warmWhite.withAlphaComponent(0.44))
    /// Lines of the passage already read.
    static let fg4 = dynamic(light: petrol.withAlphaComponent(0.32), dark: warmWhite.withAlphaComponent(0.32))
    static let you = dynamic(light: petrol, dark: warmWhite)
    static let onYou = dynamic(light: warmWhite, dark: petrol)
    static let youHalo = dynamic(light: petrol.withAlphaComponent(0.10), dark: warmWhite.withAlphaComponent(0.12))
    static let youRing = dynamic(light: petrol.withAlphaComponent(0.22), dark: warmWhite.withAlphaComponent(0.28))
    static let them = dynamic(light: UIColor(hex: 0x1C6E62), dark: UIColor(hex: 0x7FD1C1))
    static let onThem = dynamic(light: warmWhite, dark: petrol)

    private static let petrol = UIColor(hex: 0x0E2426)
    private static let warmWhite = UIColor(hex: 0xF3EEE4)

    private static func dynamic(light: UIColor, dark: UIColor) -> Color {
        Color(uiColor: UIColor { $0.userInterfaceStyle == .dark ? dark : light })
    }
}

private extension UIColor {
    convenience init(hex: UInt32) {
        self.init(
            red: CGFloat((hex >> 16) & 0xFF) / 255,
            green: CGFloat((hex >> 8) & 0xFF) / 255,
            blue: CGFloat(hex & 0xFF) / 255,
            alpha: 1
        )
    }
}
