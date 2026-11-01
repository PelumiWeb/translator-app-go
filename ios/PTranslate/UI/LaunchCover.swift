import SwiftUI

/// Covers the app as it opens, picking up where the launch screen leaves off: the same mark, as wide, in the same
/// place. The first time, on the way to the welcome, it plays the intro: the two voices step apart, each speaks
/// once, they meet again and the name appears. Otherwise it only fades. A tap skips it.
struct LaunchCover: View {
    let playsIntro: Bool
    let onFinished: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var apart = 0.0
    @State private var lens = 1.0
    @State private var ripple = 0.0
    @State private var named = false

    var body: some View {
        ZStack {
            Palette.bg
            ShinarMark(apart: apart, lens: lens, ripple: ripple)
                .frame(width: 135)
                .overlay(alignment: .bottom) {
                    // Below the mark, leaving the mark where the launch screen put it.
                    Wordmark(size: 40)
                        .fixedSize()
                        .opacity(named ? 1 : 0)
                        .offset(y: named ? 0 : 8)
                        .alignmentGuide(.bottom) { $0[.top] - 16 }
                }
        }
        .ignoresSafeArea()
        .contentShape(Rectangle())
        .onTapGesture(perform: onFinished)
        .accessibilityHidden(true)
        .task {
            guard playsIntro else { return onFinished() }
            do {
                try await Task.sleep(for: .milliseconds(200))
                if !reduceMotion {
                    withAnimation(Self.ease(0.3)) { apart = 9; lens = 0 }
                    try await Task.sleep(for: .milliseconds(300))
                    withAnimation(Self.ease(0.4)) { ripple = 1 }
                    try await Task.sleep(for: .milliseconds(400))
                    withAnimation(Self.ease(0.3)) { apart = 0; lens = 1 }
                    try await Task.sleep(for: .milliseconds(300))
                }
                withAnimation(Self.ease(0.3)) { named = true }
                try await Task.sleep(for: .milliseconds(300))
                onFinished()
            } catch {
                // Skipped: the cover is already on its way out.
            }
        }
    }

    private static func ease(_ seconds: Double) -> Animation {
        .timingCurve(0.22, 0.61, 0.36, 1, duration: seconds)
    }
}
