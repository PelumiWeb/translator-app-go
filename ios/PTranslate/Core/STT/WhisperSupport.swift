import Darwin

/// Whether this phone can run the Whisper build at all.
///
/// whisper.cpp is compiled for the A13 and later, whose processors have the dot-product and half-precision
/// instructions its fast paths use (ADR 0004, addendum). iOS 18 also runs on the iPhone XS and XR, whose A12 lacks
/// the dot-product ones: Whisper would stop the app there the moment it ran. On such a phone, nothing loads the
/// model, and every recording goes to the server.
enum WhisperSupport {
    static let processorSupported = has("hw.optional.arm.FEAT_DotProd") && has("hw.optional.arm.FEAT_FP16")

    /// Reads one of the processor features iOS lists; ggml checks the same ones. A feature iOS does not know
    /// counts as missing.
    private static func has(_ feature: String) -> Bool {
        var value: Int32 = 0
        var size = MemoryLayout<Int32>.size
        return sysctlbyname(feature, &value, &size, nil, 0) == 0 && value == 1
    }
}
