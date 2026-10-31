import SwiftUI

/// The second action of a screen: the same capsule as `PrimaryButtonStyle`, outlined.
struct SecondaryButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.pearlMessageStrong)
            .foregroundStyle(isEnabled ? Palette.fg : Palette.fg3)
            .frame(maxWidth: .infinity, minHeight: 52)
            .padding(.horizontal, 24)
            .overlay(Capsule().stroke(Palette.fg.opacity(0.28), lineWidth: 1))
            .contentShape(Capsule())
            .opacity(configuration.isPressed ? 0.7 : 1)
    }
}

/// The design's text field: raised petrol, rounded, large text, outlined in warm white while it has focus. `prefix`
/// is shown dim before the text, as the "@" of an ID.
struct PearlTextField<Trailing: View>: View {
    let label: String
    @Binding var text: String
    var placeholder = ""
    var prefix: String?
    var compact = false
    var systemImage: String?
    @ViewBuilder var trailing: Trailing

    @FocusState private var focused: Bool

    var body: some View {
        HStack(spacing: 0) {
            if let systemImage {
                Image(systemName: systemImage).foregroundStyle(Palette.fg2).padding(.trailing, 10)
            }
            if let prefix {
                Text(prefix).foregroundStyle(Palette.fg3)
            }
            TextField(label, text: $text, prompt: Text(placeholder).foregroundStyle(Palette.fg3))
                .focused($focused)
                .foregroundStyle(Palette.fg)
                .tint(Palette.fg)
            trailing
        }
        .font(compact ? .pearlMessage : .pearlField)
        .padding(.horizontal, 16)
        .frame(minHeight: compact ? 46 : 58)
        .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 12))
        .overlay {
            if focused && !compact {
                RoundedRectangle(cornerRadius: 12).stroke(Palette.fg, lineWidth: 1.5)
            }
        }
        .accessibilityLabel(label)
    }
}

extension PearlTextField where Trailing == EmptyView {
    init(label: String, text: Binding<String>, placeholder: String = "", prefix: String? = nil, compact: Bool = false, systemImage: String? = nil) {
        self.init(label: label, text: text, placeholder: placeholder, prefix: prefix, compact: compact, systemImage: systemImage) { EmptyView() }
    }
}

/// The top of an onboarding step: back, then a bar of `total` segments with the first `step` filled, then "2 of 5".
struct OnboardingHeader: View {
    let step: Int
    let total: Int
    let onBack: (() -> Void)?

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            HStack(spacing: 14) {
                if let onBack {
                    Button(action: onBack) {
                        Image(systemName: "chevron.left").font(.system(size: 22, weight: .regular))
                            .foregroundStyle(Palette.fg)
                            .frame(width: 44, height: 44)
                    }
                    .accessibilityLabel("Back")
                }
                HStack(spacing: 6) {
                    ForEach(0..<total, id: \.self) { index in
                        Capsule().fill(index < step ? Palette.fg : Palette.fg.opacity(0.2)).frame(height: 3)
                    }
                }
            }
            Text("\(step) of \(total)").font(.pearlLabel).foregroundStyle(Palette.fg2)
        }
        .accessibilityElement(children: .contain)
    }
}

/// A suggested value to pick, as the free IDs offered when one is taken.
struct SuggestionChip: View {
    let text: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(text).font(.pearlBody).foregroundStyle(Palette.fg)
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Palette.bgRaised, in: Capsule())
        }
        .buttonStyle(.plain)
    }
}

/// A person's initial in a circle: raised petrol and mint for others, warm white for the user.
struct Avatar: View {
    let name: String
    var size: CGFloat = 48
    var fill: Color = Palette.bgRaised
    var content: Color = Palette.them

    var body: some View {
        Text(name.trimmingCharacters(in: .whitespaces).first.map { String($0).uppercased() } ?? "?")
            .font(.custom("IBMPlexSans-Medm", size: size * 0.4, relativeTo: .title3))
            .foregroundStyle(content)
            .frame(width: size, height: size)
            .background(fill, in: Circle())
            .accessibilityHidden(true)
    }
}

// The design's three forms of notice. None is red, and none blames the person: each says what happened and what to
// do next.

/// For a problem that fixes itself: a capsule at the top with a spinner.
struct Toast: View {
    let text: String

    var body: some View {
        HStack(spacing: 10) {
            ProgressView().tint(Palette.fg2).controlSize(.small)
            Text(text).font(.pearlBody).foregroundStyle(Palette.fg)
        }
        .padding(.horizontal, 18).padding(.vertical, 10)
        .background(Palette.bgRaised, in: Capsule())
        .overlay(Capsule().stroke(Palette.fg.opacity(0.08)))
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.updatesFrequently)
    }
}

/// For something the person must do: an icon, a title, a line, and the action.
struct Banner: View {
    let systemImage: String
    let title: String
    let message: String
    let action: String
    let onAction: () -> Void

    var body: some View {
        VStack(spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                Image(systemName: systemImage).font(.system(size: 20)).foregroundStyle(Palette.fg)
                VStack(alignment: .leading, spacing: 2) {
                    Text(title).font(.pearlMessageStrong).foregroundStyle(Palette.fg)
                    Text(message).font(.pearlBody).foregroundStyle(Palette.fg2)
                }
                Spacer(minLength: 0)
            }
            Button(action, action: onAction).buttonStyle(PrimaryButtonStyle())
        }
        .padding(16)
        .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 12))
    }
}

/// For a recording that did not work: a line above the mic, with what to try.
struct Hint: View {
    let text: String
    var systemImage = "arrow.counterclockwise"

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: systemImage).foregroundStyle(Palette.fg)
            Text(text).font(.pearlBody).foregroundStyle(Palette.fg)
        }
        .padding(.horizontal, 16).padding(.vertical, 12)
        .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 12))
        .accessibilityElement(children: .combine)
    }
}

/// The welcome's two voices: a warm white set of rings for you and a mint one for them, overlapping, each breathing
/// on a three-second loop. With Reduce Motion, they stand still.
struct TwoVoices: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        TimelineView(.animation(paused: reduceMotion)) { timeline in
            Canvas { canvas, size in
                let phase = reduceMotion ? 0.3 : timeline.date.timeIntervalSinceReferenceDate.truncatingRemainder(dividingBy: 3) / 3
                let mid = CGPoint(x: size.width / 2, y: size.height / 2)
                func voice(_ center: CGPoint, _ color: Color, _ offset: Double) {
                    let core: CGFloat = 26
                    let ringMax: CGFloat = 90
                    for ring in 0..<3 {
                        let t = (phase + offset + Double(ring) / 3).truncatingRemainder(dividingBy: 1)
                        let radius = core + (ringMax - core) * t
                        var layer = canvas
                        layer.opacity = 0.22 * (1 - t)
                        layer.fill(Path(ellipseIn: CGRect(x: center.x - radius, y: center.y - radius, width: radius * 2, height: radius * 2)), with: .color(color))
                    }
                    canvas.fill(Path(ellipseIn: CGRect(x: center.x - core, y: center.y - core, width: core * 2, height: core * 2)), with: .color(color))
                }
                voice(CGPoint(x: mid.x - 40, y: mid.y), Palette.you, 0)
                voice(CGPoint(x: mid.x + 40, y: mid.y), Palette.them, 1.0 / 6)
            }
        }
        .frame(width: 260, height: 180)
        .accessibilityHidden(true)
    }
}
