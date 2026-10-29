import Testing
@testable import PTranslate

struct RoutingTranscriberTests {
    /// A transcriber that works, sends `partial` if given, then fails or answers.
    private func scripted(
        _ source: Transcript.Source,
        text: String? = nil,
        confidence: Float? = nil,
        partial: String? = nil,
        failure: TranscriptionError? = nil
    ) -> FakeTranscriber {
        var events: [TranscriptEvent] = [.stageChanged(.processing)]
        if let partial {
            events.append(.partial(partial))
        }
        if failure == nil {
            let text = text ?? (source == .onDevice ? "from the device" : "from the cloud")
            events.append(.final(Transcript(text: text, confidence: confidence, source: source)))
        }
        return FakeTranscriber(events: events, error: failure)
    }

    private func router(
        device: FakeTranscriber,
        cloud: FakeTranscriber,
        route: RouteSetting = RouteSetting(.automatic),
        modelReady: Bool = true,
        realTimeFactor: Double? = 0.3
    ) -> RoutingTranscriber {
        RoutingTranscriber(
            onDevice: device,
            cloud: cloud,
            route: route,
            modelReady: { modelReady },
            realTimeFactor: { realTimeFactor },
            thresholds: FallbackThresholds(minConfidence: 0.6, maxRealTimeFactor: 1.0)
        )
    }

    private var confidentDevice: FakeTranscriber { scripted(.onDevice, confidence: 0.9) }
    private var cloud: FakeTranscriber { scripted(.cloud) }
    private let unreachable = TranscriptionError.failed("Could not reach the server")

    private func events(_ router: RoutingTranscriber) async throws -> [TranscriptEvent] {
        var events: [TranscriptEvent] = []
        for try await event in router.transcribe(PCMAudio(samples: Array(repeating: 0, count: 16)), language: .english) {
            events.append(event)
        }
        return events
    }

    private func result(_ router: RoutingTranscriber) async throws -> Transcript? {
        guard case .final(let transcript) = try await events(router).last else { return nil }
        return transcript
    }

    // MARK: The two manual routes

    @Test func theDeviceRouteUsesOnlyTheDeviceWhateverTheResult() async throws {
        let cloud = cloud
        let router = router(device: scripted(.onDevice, confidence: 0.1), cloud: cloud, route: RouteSetting(.onDevice))

        #expect(try await result(router) == Transcript(text: "from the device", confidence: 0.1, source: .onDevice))
        #expect(cloud.calls == 0)
    }

    @Test func theCloudRouteUsesOnlyTheCloud() async throws {
        let device = confidentDevice
        let router = router(device: device, cloud: cloud, route: RouteSetting(.cloud))

        #expect(try await result(router) == Transcript(text: "from the cloud", confidence: nil, source: .cloud))
        #expect(device.calls == 0)
    }

    @Test func followsTheRouteForEachNewRecording() async throws {
        let route = RouteSetting(.onDevice)
        let router = router(device: confidentDevice, cloud: cloud, route: route)
        #expect(try await result(router)?.source == .onDevice)

        route.current = .cloud
        #expect(try await result(router)?.source == .cloud)
    }

    // MARK: Automatic: the device is good enough

    @Test func aConfidentDeviceResultIsUsedAndNothingIsUploaded() async throws {
        let cloud = cloud
        let router = router(device: confidentDevice, cloud: cloud)

        #expect(try await result(router) == Transcript(text: "from the device", confidence: 0.9, source: .onDevice, note: nil))
        #expect(cloud.calls == 0)
    }

    @Test func confidenceExactlyAtTheThresholdIsAccepted() async throws {
        let cloud = cloud
        let router = router(device: scripted(.onDevice, confidence: 0.6), cloud: cloud)

        #expect(try await result(router)?.source == .onDevice)
        #expect(cloud.calls == 0)
    }

    @Test func aResultWithNoConfidenceFigureIsAccepted() async throws {
        let cloud = cloud
        let router = router(device: scripted(.onDevice, confidence: nil), cloud: cloud)

        #expect(try await result(router)?.source == .onDevice)
        #expect(cloud.calls == 0)
    }

    @Test func aDeviceWhoseSpeedWasNeverMeasuredIsGivenTheChance() async throws {
        let router = router(device: confidentDevice, cloud: cloud, realTimeFactor: nil)

        #expect(try await result(router)?.source == .onDevice)
    }

    // MARK: Automatic: reasons to go to the cloud before trying

    @Test func withNoModelTheRecordingGoesStraightToTheCloud() async throws {
        let device = confidentDevice
        let router = router(device: device, cloud: cloud, modelReady: false)

        #expect(try await result(router) == Transcript(text: "from the cloud", confidence: nil, source: .cloud, note: .noModel))
        #expect(device.calls == 0)
    }

    @Test func aDeviceThatIsTooSlowIsSkipped() async throws {
        let device = confidentDevice
        let router = router(device: device, cloud: cloud, realTimeFactor: 1.8)

        let transcript = try await result(router)

        #expect(transcript?.note == .slowDevice)
        #expect(transcript?.source == .cloud)
        #expect(device.calls == 0)
    }

    @Test func speedExactlyAtTheLimitStillCountsAsFastEnough() async throws {
        let router = router(device: confidentDevice, cloud: cloud, realTimeFactor: 1.0)

        #expect(try await result(router)?.source == .onDevice)
    }

    @Test func aSlowDeviceIsUsedAfterAllWhenTheCloudCannotBeReached() async throws {
        let router = router(device: confidentDevice, cloud: scripted(.cloud, failure: unreachable), realTimeFactor: 1.8)

        let transcript = try await result(router)

        #expect(transcript?.source == .onDevice)
        #expect(transcript?.note == .cloudUnavailable)
    }

    // MARK: Automatic: reasons to go to the cloud after trying

    @Test func aLowConfidenceResultIsShownAsADraftAndReplacedByTheClouds() async throws {
        let router = router(
            device: scripted(.onDevice, text: "wreck a nice beach", confidence: 0.42),
            cloud: scripted(.cloud, text: "recognise speech")
        )

        #expect(try await events(router) == [
            .stageChanged(.processing), // the device working
            .partial("wreck a nice beach"), // its doubtful result, as a draft
            .stageChanged(.processing), // the cloud working
            .final(Transcript(text: "recognise speech", confidence: nil, source: .cloud, note: .lowConfidence)),
        ])
    }

    @Test func aLowConfidenceResultIsKeptWhenTheCloudCannotBeReached() async throws {
        let router = router(
            device: scripted(.onDevice, text: "wreck a nice beach", confidence: 0.42),
            cloud: scripted(.cloud, failure: unreachable)
        )

        #expect(try await result(router) == Transcript(
            text: "wreck a nice beach", confidence: 0.42, source: .onDevice, note: .cloudUnavailable
        ))
    }

    @Test func soundWithoutWordsOnTheDeviceIsGivenToTheCloud() async throws {
        let router = router(device: scripted(.onDevice, failure: .noSpeech("No speech was recognised")), cloud: cloud)

        #expect(try await result(router) == Transcript(
            text: "from the cloud", confidence: nil, source: .cloud, note: .noSpeechOnDevice
        ))
    }

    @Test func noWordsOnTheDeviceAndNoCloudReportsThatNoSpeechWasRecognised() async {
        let router = router(
            device: scripted(.onDevice, failure: .noSpeech("No speech was recognised")),
            cloud: scripted(.cloud, failure: unreachable)
        )

        // The device's verdict, not the network error: it is the more useful one.
        await #expect(throws: TranscriptionError.noSpeech("No speech was recognised")) {
            try await events(router)
        }
    }

    @Test func aGarbledDeviceResultIsDiscardedAndTheCloudIsAsked() async throws {
        let router = router(
            device: scripted(.onDevice, failure: .garbled("The speech could not be made out. Please try again")),
            cloud: cloud
        )

        let all = try await events(router)

        guard case .final(let transcript) = all.last else {
            Issue.record("no final result in \(all)")
            return
        }
        #expect(transcript.note == .garbledOnDevice)
        // Unlike a low-confidence result, the nonsense is never shown as a draft.
        #expect(!all.contains { if case .partial = $0 { true } else { false } })
    }

    @Test func aGarbledResultWithNoCloudAsksTheUserToTryAgain() async {
        let garbled = TranscriptionError.garbled("The speech could not be made out. Please try again")
        let router = router(device: scripted(.onDevice, failure: garbled), cloud: scripted(.cloud, failure: unreachable))

        await #expect(throws: garbled) {
            try await events(router)
        }
    }

    @Test func aModelThatFailsToRunFallsBackToTheCloud() async throws {
        let router = router(device: scripted(.onDevice, failure: .failed("The on-device model failed to run")), cloud: cloud)

        let transcript = try await result(router)

        #expect(transcript?.note == .onDeviceFailed)
        #expect(transcript?.source == .cloud)
    }

    @Test func aSilentRecordingIsNeverUploaded() async {
        let silent = TranscriptionError.silentAudio("The recording was silent (-100 dB). Check the microphone")
        let cloud = cloud
        let router = router(device: scripted(.onDevice, failure: silent), cloud: cloud)

        await #expect(throws: silent) {
            try await events(router)
        }
        #expect(cloud.calls == 0)
    }

    @Test func withNoModelAndNoCloudTheCloudsErrorIsReported() async {
        let device = confidentDevice
        let router = router(device: device, cloud: scripted(.cloud, failure: unreachable), modelReady: false)

        await #expect(throws: unreachable) {
            try await events(router)
        }
        #expect(device.calls == 0)
    }
}
