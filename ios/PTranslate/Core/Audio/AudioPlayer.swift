protocol AudioPlayer: AnyObject {
    /// Plays the audio and returns when it has finished or `stop()` was called.
    func play(_ audio: PCMAudio) async throws

    /// Stops what is playing. Does nothing if nothing is.
    func stop()
}
