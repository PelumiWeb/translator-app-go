import SwiftUI

/// What the big circle is doing. Each look comes from the design handoff.
enum RecordControlState: Equatable {
    /// Waiting: a microphone, in a still halo.
    case idle

    /// Listening: a stop square, with rings spreading out.
    case recording

    /// Writing down or translating: calm rings, a spinner in the circle.
    case working

    /// Making the voice, 2 to 5 seconds: calm rings around a speaker.
    case preparingVoice

    /// The translation can be heard: a speaker, in a still halo. The sound is the feedback.
    case speaking
}

/// The large record control of the design: a circle in the colour that means "you", inside an area where rings show
/// that something is happening. The rings are a fixed loop for now; the design asks for them to follow the
/// microphone's level later.
struct RecordControl: View {
    let state: RecordControlState
    /// What a tap does now, for VoiceOver ("Start recording").
    let label: String
    /// The state in words, for VoiceOver ("0:12").
    var value: String?
    var enabled = true
    var area: CGFloat = 200
    var circle: CGFloat = 96
    let action: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private var animated: Bool {
        !reduceMotion && state != .idle && state != .speaking
    }

    var body: some View {
        ZStack {
            if animated {
                Rings(period: state == .recording ? 2.4 : 3.2)
            } else {
                Halo(diameter: circle + (area - circle) * 0.45, breathing: reduceMotion && state != .idle)
            }
            Button(action: action) {
                ZStack {
                    Circle().fill(Palette.you)
                    glyph
                }
                .frame(width: circle, height: circle)
                .contentShape(Circle())
            }
            .buttonStyle(.plain)
            .disabled(!enabled)
        }
        .frame(width: area, height: area)
        .accessibilityElement(children: .combine)
        .accessibilityLabel(label)
        .accessibilityValue(value ?? "")
        .accessibilityAddTraits(.isButton)
    }

    @ViewBuilder
    private var glyph: some View {
        switch state {
        case .idle:
            Image(systemName: "mic")
                .font(.system(size: 28, weight: .regular))
                .foregroundStyle(Palette.onYou)
        case .recording:
            RoundedRectangle(cornerRadius: 4)
                .fill(Palette.onYou)
                .frame(width: 22, height: 22)
        case .working:
            ProgressView()
                .tint(Palette.onYou)
        case .preparingVoice, .speaking:
            Image(systemName: "speaker.wave.2")
                .font(.system(size: 26, weight: .regular))
                .foregroundStyle(Palette.onYou)
        }
    }
}

/// Three filled rings that grow and fade, one after another, eased as the design specifies.
private struct Rings: View {
    let period: Double

    private let curve = UnitCurve.bezier(
        startControlPoint: UnitPoint(x: 0.22, y: 0.61),
        endControlPoint: UnitPoint(x: 0.36, y: 1)
    )

    var body: some View {
        TimelineView(.animation) { context in
            Canvas { canvas, size in
                let time = context.date.timeIntervalSinceReferenceDate
                for ring in 0..<3 {
                    let phase = (time / period + Double(ring) / 3).truncatingRemainder(dividingBy: 1)
                    let progress = curve.value(at: phase)
                    let diameter = size.width * (0.48 + 0.52 * progress)
                    var layer = canvas
                    layer.opacity = 0.9 * (1 - progress)
                    layer.fill(
                        Path(ellipseIn: CGRect(
                            x: (size.width - diameter) / 2, y: (size.height - diameter) / 2,
                            width: diameter, height: diameter
                        )),
                        with: .color(Palette.youRing)
                    )
                }
            }
        }
    }
}

/// A still halo; with Reduce Motion on, it fades slowly in and out in place of the rings.
private struct Halo: View {
    let diameter: CGFloat
    let breathing: Bool

    var body: some View {
        TimelineView(.animation(paused: !breathing)) { context in
            let wave = breathing ? (sin(context.date.timeIntervalSinceReferenceDate * 2 * .pi / 3.2) + 1) / 2 : 1
            Circle()
                .fill(Palette.youHalo)
                .opacity(0.5 + 0.5 * wave)
                .frame(width: diameter, height: diameter)
        }
    }
}
