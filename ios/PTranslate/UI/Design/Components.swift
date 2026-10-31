import SwiftUI

/// The main action of a screen: a capsule in the colour that means "you".
struct PrimaryButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.custom("IBMPlexSans-Medm", size: 16, relativeTo: .body))
            .foregroundStyle(Palette.onYou)
            .frame(maxWidth: .infinity, minHeight: 52)
            .padding(.horizontal, 24)
            .background(Palette.you.opacity(isEnabled ? 1 : 0.3), in: Capsule())
            .opacity(configuration.isPressed ? 0.8 : 1)
    }
}

/// A quiet action: underlined text, as "Not now, use the phone's voice" in the design.
struct LinkButton: View {
    let title: String
    let action: () -> Void

    init(_ title: String, action: @escaping () -> Void) {
        self.title = title
        self.action = action
    }

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(.pearlBody)
                .underline()
                .foregroundStyle(Palette.fg.opacity(0.72))
                .multilineTextAlignment(.center)
                .padding(.vertical, 10)
                .padding(.horizontal, 12)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

/// Something the person should know, never styled as an alarm: warm text with an outline icon, as the handoff
/// suggests for errors.
struct Notice: View {
    let text: String

    var body: some View {
        Label {
            Text(text)
                .font(.pearlBody)
                .foregroundStyle(Palette.fg)
        } icon: {
            Image(systemName: "info.circle")
                .foregroundStyle(Palette.fg)
        }
    }
}

/// The design's dot waveform: dots whose size follows `levels`, the played part (`played`, 0 to 1) at full strength
/// and the rest faded.
struct DotWaveform: View {
    let levels: [Float]
    let color: Color
    var played: Double = 1

    var body: some View {
        Canvas { canvas, size in
            guard !levels.isEmpty else { return }
            let step = size.width / CGFloat(levels.count)
            for (index, level) in levels.enumerated() {
                let diameter = 3 + 7 * CGFloat(min(max(level, 0), 1))
                let center = CGPoint(x: step * (CGFloat(index) + 0.5), y: size.height / 2)
                var layer = canvas
                layer.opacity = (Double(index) + 0.5) / Double(levels.count) <= played ? 1 : 0.3
                layer.fill(
                    Path(ellipseIn: CGRect(x: center.x - diameter / 2, y: center.y - diameter / 2, width: diameter, height: diameter)),
                    with: .color(color)
                )
            }
        }
        .frame(minHeight: 20)
        .accessibilityHidden(true)
    }
}

extension Language {
    /// The language's name in itself, as the design asks: "Español", not "Spanish".
    var ownName: String {
        let locale = Locale(identifier: tag)
        guard let name = locale.localizedString(forLanguageCode: tag) else { return tag }
        return name.prefix(1).uppercased(with: locale) + name.dropFirst()
    }

    /// The language's name in the phone's language, to help find it in a list.
    var localName: String {
        guard let name = Locale.current.localizedString(forLanguageCode: tag) else { return tag }
        return name.prefix(1).uppercased() + name.dropFirst()
    }
}

/// "0:07", for timers; the fonts' tabular figures keep it from jittering.
func clock(_ duration: Duration) -> String {
    let seconds = Int(duration.components.seconds)
    return "\(seconds / 60):" + String(format: "%02d", seconds % 60)
}
