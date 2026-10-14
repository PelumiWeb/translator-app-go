import Testing
@testable import PTranslate

struct ServerSentEventParserTests {
    private func parse(_ stream: String) -> [ServerSentEvent] {
        var parser = ServerSentEventParser()
        return stream.utf8.compactMap { parser.consume($0) }
    }

    @Test func readsEventsSeparatedByEmptyLines() {
        #expect(parse("id: 1\nevent: queued\ndata: {}\n\nid: 2\nevent: done\ndata: {\"text\":\"hi\"}\n\n") == [
            ServerSentEvent(id: "1", type: "queued", data: "{}"),
            ServerSentEvent(id: "2", type: "done", data: #"{"text":"hi"}"#),
        ])
    }

    @Test func acceptsCarriageReturnLineEndings() {
        #expect(parse("id: 7\r\nevent: partial\r\ndata: x\r\n\r\n") == [
            ServerSentEvent(id: "7", type: "partial", data: "x"),
        ])
    }

    @Test func joinsSeveralDataLinesWithNewlines() {
        #expect(parse("data: one\ndata: two\n\n") == [ServerSentEvent(id: nil, type: nil, data: "one\ntwo")])
    }

    @Test func ignoresCommentsAndEventsWithoutData() {
        #expect(parse(": keep-alive\n\nevent: empty\n\n").isEmpty)
    }

    @Test func doesNotDeliverAnEventUntilItsEmptyLineArrives() {
        #expect(parse("id: 1\nevent: queued\ndata: {}\n").isEmpty)
    }

    @Test func takesValuesWithoutTheOptionalSpaceAndIgnoresUnknownFields() {
        #expect(parse("id:3\nretry: 1000\nevent:done\ndata:ok\n\n") == [
            ServerSentEvent(id: "3", type: "done", data: "ok"),
        ])
    }
}
