import Foundation
import zlib

/// Above this, text repeats itself too much to be speech. The value is the one OpenAI's reference Whisper uses for
/// the same purpose.
private let maxCompressionRatio: Float = 2.4

/// How many times smaller the text gets when compressed. Ordinary sentences come out between 1 and 2. Text made of
/// one phrase repeated compresses to almost nothing, so its ratio is very high.
///
/// zlib at its default level, as Android's `Deflater` and OpenAI's Python use, so all three agree on the number.
func compressionRatio(_ text: String) -> Float {
    let bytes = Array(text.utf8)
    guard !bytes.isEmpty else { return 1 }

    var compressedSize = compressBound(uLong(bytes.count))
    var compressed = [UInt8](repeating: 0, count: Int(compressedSize))
    guard compress2(&compressed, &compressedSize, bytes, uLong(bytes.count), Z_DEFAULT_COMPRESSION) == Z_OK else {
        return 1
    }
    return Float(bytes.count) / Float(compressedSize)
}

/// Whether a transcript is a repetition loop: the model stuck on one token or phrase ("I, I, I, I, ..."), a known
/// way for Whisper to fail.
///
/// The model's confidence cannot be used to catch this. It is typically high for such output, because each
/// repeated token is, given the ones before it, exactly what the model expects next. Compressibility measures the
/// text itself instead. A few repeated words do not trip it: short text does not compress well enough to pass the
/// limit.
func looksLikeRepetitionLoop(_ text: String) -> Bool {
    compressionRatio(text) > maxCompressionRatio
}

/// Whether a transcript contains no words, only Whisper's labels for other sounds: "[Music]", "[BLANK_AUDIO]",
/// "(applause)". The model was trained on subtitles, which are full of these, and writes them when it hears
/// something that is not speech. They are not something the speaker said.
///
/// A label inside real speech ("Hello [laughs] there") does not count: there are words to keep.
func isOnlySoundLabels(_ text: String) -> Bool {
    // One or more labels in square or round brackets, and nothing else.
    text.wholeMatch(of: /(\s*(\[[^\]]*\]|\([^)]*\))\s*)+/) != nil
}
