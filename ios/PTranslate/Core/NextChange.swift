import Observation

/// Returns after the next change to anything `value` reads from an `@Observable` object.
///
/// `Observations` does this, and more, from iOS 26; this is for iOS 18. A change made before the call is not seen,
/// so read the value first, then wait: in the same actor, nothing can change in between, since the tracking starts
/// before the first suspension. It runs on the caller's actor, which must be the one the object's changes are made
/// on.
func nextChange(of value: () -> Void) async {
    await withCheckedContinuation { continuation in
        // Called once, as the change is about to be made; by the time the caller resumes, it has been.
        withObservationTracking(value) {
            continuation.resume()
        }
    }
}
