import Testing
@testable import PTranslate

struct PCMAudioLevelsTests {
    @Test func levelsAreTheLoudnessOfEqualSlicesRelativeToTheLoudest() {
        // Four slices of 100 samples: silent, quiet, loud, half as loud. The same case as on Android.
        let samples: [Int16] = (0..<400).map { i in
            let amplitude: Int16 = switch i / 100 {
            case 0: 0
            case 1: 1000
            case 2: 4000
            default: 2000
            }
            return i % 2 == 0 ? amplitude : -amplitude
        }

        #expect(PCMAudio(samples: samples).levels(4) == [0, 0.25, 1, 0.5])
    }

    @Test func silenceAndEmptyAudioHaveNoLevel() {
        #expect(PCMAudio(samples: Array(repeating: 0, count: 300)).levels(3) == [0, 0, 0])
        #expect(PCMAudio(samples: []).levels(2) == [0, 0])
    }
}
