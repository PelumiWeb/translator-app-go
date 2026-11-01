import Foundation
import Testing
@testable import PTranslate

/// The rooms client over the wire, and how notes are ordered and shown. The same cases as the Android tests.
struct RoomsTests {
    private let server = StubServer()
    private let client: RoomsClient

    init() {
        client = RoomsClient(rest: Rest(baseURL: server.baseURL, session: server.session, token: { "tok" }, sessionEnded: {}))
    }

    @Test func roomsAreReadWithTheirPeopleLastMessageAndRenditions() async throws {
        server.enqueue(.json(status: 200, body: """
            {"rooms":[{"id":"r1","people":[{"id":"u2","display_name":"Rosa","handle":"rosa","language":"es"}],
            "last_message":{"id":"m1","room_id":"r1","sender_id":"u2","mine":false,"lang":"es","transcript":"hola",
            "duration_ms":2400,"created_at":"2026-10-31T09:12:00.123456+01:00","renditions":[
            {"lang":"en","status":"ready","text":"hello","voice":"own","audio_url":"/v1/messages/m1/audio?lang=en"}]},
            "unread":1,"created_at":"2026-10-31T09:00:00Z"}]}
            """))

        let room = try #require(try await client.rooms().first)

        #expect(room.other?.displayName == "Rosa")
        #expect(room.unread == 1)
        let rendition = try #require(room.lastMessage?.renditions.first)
        #expect(rendition == Rendition(lang: "en", status: .ready, text: "hello", voice: .own, audioURL: "/v1/messages/m1/audio?lang=en"))
        #expect(server.requests.first?.header("Authorization") == "Bearer tok")
    }

    @Test func aStatusThisAppDoesNotKnowYetReadsAsOnItsWay() async throws {
        server.enqueue(.json(status: 200, body: #"{"id":"m1","room_id":"r1","sender_id":"u1","mine":true,"lang":"en","transcript":"hi","duration_ms":900,"created_at":"2026-10-31T09:00:00Z","renditions":[{"lang":"es","status":"queued_for_review"}]}"#))
        #expect(try await client.message("m1").renditions.first?.status == .translating)
    }

    @Test func aVoiceNoteIsSentWithItsWordsLengthLanguageAndKey() async throws {
        server.enqueue(.json(status: 201, body: #"{"id":"m1","room_id":"r1","sender_id":"u1","mine":true,"lang":"en","transcript":"hello","duration_ms":2400,"created_at":"2026-10-31T09:00:00Z","renditions":[{"lang":"es","status":"translating"}]}"#))

        _ = try await client.sendMessage(roomID: "r1", wav: Data([1, 2, 3]), transcript: "hello", durationMS: 2400, lang: "en", idempotencyKey: "note-1")

        let request = try #require(server.requests.first)
        #expect(request.path == "/v1/rooms/r1/messages")
        #expect(request.header("Idempotency-Key") == "note-1")
        for part in [#"name="transcript""#, "hello", #"name="duration_ms""#, "2400", #"name="lang""#, #"name="audio""#] {
            #expect(request.bodyText.contains(part))
        }
    }

    @Test func theChangeStreamSaysWhenItIsReadyThenWhatChanged() async throws {
        server.enqueue(.events("event: ready\ndata: {}\n\nevent: room\ndata: {\"room_id\":\"r1\",\"message_id\":\"m1\"}\n\nevent: room\ndata: {\"room_id\":\"r2\"}\n\n"))

        var changes: [RoomChange] = []
        for try await change in try await client.changes() {
            changes.append(change)
        }

        #expect(changes == [.ready, .changed(roomID: "r1", messageID: "m1"), .changed(roomID: "r2", messageID: nil)])
        #expect(server.requests.first?.path == "/v1/me/events")
    }

    private func message(_ id: String, at seconds: TimeInterval, mine: Bool = false, renditions: [Rendition] = []) -> Message {
        let json = """
            {"id":"\(id)","room_id":"r","sender_id":"s","mine":\(mine),"lang":"es","transcript":"hola","duration_ms":12400,
            "created_at":"\(Date(timeIntervalSince1970: seconds).ISO8601Format())","renditions":[]}
            """
        var message = try! Rest.decoder.decode(Message.self, from: Data(json.utf8))
        message.renditions = renditions
        return message
    }

    @Test @MainActor func aNewerCopyOfANoteReplacesTheOlderAndNotesStayInOrder() {
        let first = message("a", at: 1000)
        let second = message("b", at: 2000)
        var translated = second
        translated.renditions = [Rendition(lang: "en", status: .ready, text: "hello")]

        let merged = RoomViewModel.merge([second, first], [translated])

        #expect(merged.map(\.id) == ["a", "b"])
        #expect(merged[1].renditions.first?.text == "hello")
    }

    @Test func theRoomListShowsATranslationWhenReadyAndTheLengthUntilThen() {
        let ready = message("a", at: 0, renditions: [Rendition(lang: "en", status: .ready, text: "hello")])
        let waiting = message("b", at: 0, renditions: [Rendition(lang: "en", status: .speaking, text: "hello")])
        let mine = message("c", at: 0, mine: true)

        #expect(ready.preview(myLanguage: "en") == "hello")
        #expect(waiting.preview(myLanguage: "en") == "Voice note · 0:12")
        #expect(mine.preview(myLanguage: "en") == "You: hola")
        #expect(ready.rendition(for: "fr") == nil)
    }

    @Test func aScannedCodeGivesAnIDOnlyWhenItIsOurs() {
        #expect(handle(fromCode: "shinar://add/Rosa.LM") == "rosa.lm")
        #expect(handle(fromCode: "https://example.com/rosa") == nil)
        #expect(handle(fromCode: "shinar://add/") == nil)
    }
}
