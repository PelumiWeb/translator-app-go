import SwiftUI

/// The app's name and logo (docs/design/pearl, "Logo and splash").
enum Brand {
    /// From the display name, `APP_NAME` in the project's build settings: the one place the name is written, since
    /// it waits on a trademark check (ADR 0021).
    static let name = Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String ?? "Shinar"
}

/// The mark: two voices, mint for them on the left and warm white for you on the right, overlapping above a horizon
/// line. The intro moves the circles `apart` (in the logo's units), fades the overlap (`lens`) and sends a ripple
/// out from each (`ripple`, 0 to 1); at rest it is the logo.
struct ShinarMark: View, Animatable {
    var apart: Double = 0
    var lens: Double = 1
    var ripple: Double = 0

    var animatableData: AnimatablePair<Double, AnimatablePair<Double, Double>> {
        get { AnimatablePair(apart, AnimatablePair(lens, ripple)) }
        set {
            apart = newValue.first
            lens = newValue.second.first
            ripple = newValue.second.second
        }
    }

    var body: some View {
        Canvas { canvas, size in
            // The logo's viewBox is 108 by 50 units, from (6, 16).
            let unit = size.width / 108
            func point(_ x: Double, _ y: Double) -> CGPoint { CGPoint(x: (x - 6) * unit, y: (y - 16) * unit) }
            func circle(_ center: CGPoint, _ radius: Double) -> Path {
                Path(ellipseIn: CGRect(x: center.x - radius, y: center.y - radius, width: radius * 2, height: radius * 2))
            }

            var line = Path()
            line.move(to: point(8, 62))
            line.addLine(to: point(112, 62))
            canvas.stroke(line, with: .color(Palette.fg.opacity(0.5)), style: StrokeStyle(lineWidth: 2.5 * unit, lineCap: .round))

            let radius = 22 * unit
            let left = point(46 - apart, 40)
            let right = point(74 + apart, 40)
            canvas.fill(circle(left, radius), with: .color(Palette.them))
            canvas.fill(circle(right, radius), with: .color(Palette.you))
            if lens > 0 {
                var overlap = canvas
                overlap.clip(to: circle(left, radius))
                overlap.fill(circle(right, radius), with: .color(Palette.lens.opacity(lens)))
            }
            if ripple > 0 && ripple < 1 {
                var rings = canvas
                rings.opacity = 0.6 * (1 - ripple)
                let grown = radius * (1 + 0.6 * ripple)
                rings.stroke(circle(left, grown), with: .color(Palette.them), lineWidth: 1.25 * unit)
                rings.stroke(circle(right, grown), with: .color(Palette.you), lineWidth: 1.25 * unit)
            }
        }
        .aspectRatio(108 / 50, contentMode: .fit)
        .accessibilityHidden(true)
    }
}

/// The name as the logo writes it: lowercase, IBM Plex Sans Medium, tracked in.
struct Wordmark: View {
    let size: CGFloat

    var body: some View {
        Text(Brand.name.lowercased())
            .font(.custom("IBMPlexSans-Medm", size: size, relativeTo: .title))
            .tracking(-0.04 * size)
            .foregroundStyle(Palette.fg)
            .accessibilityLabel(Brand.name)
    }
}

/// The mark and the name side by side, as at the top of the welcome.
struct Lockup: View {
    var markWidth: CGFloat = 40
    var nameSize: CGFloat = 20

    var body: some View {
        HStack(spacing: markWidth * 0.15) {
            ShinarMark().frame(width: markWidth)
            Wordmark(size: nameSize)
        }
        .accessibilityElement(children: .combine)
    }
}
