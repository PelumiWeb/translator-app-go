package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
)

func event(seq int, eventType, payload string) queue.Event {
	return queue.Event{JobID: testJobID, Seq: seq, Type: eventType, Payload: json.RawMessage(payload)}
}

// stream is an open SSE response read one frame at a time.
type stream struct {
	t    *testing.T
	resp *http.Response
	r    *bufio.Reader
}

// openStream starts a real HTTP server for the API and connects to a job's
// event stream. A real server is used, not a ResponseRecorder, because these
// tests are about what arrives while the handler is still running.
func openStream(t *testing.T, a *API, lastEventID string) *stream {
	t.Helper()
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)

	// Every stream gives up after 5s, so a handler that fails to end shows
	// as a test failure instead of a hang.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/jobs/"+testJobID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return &stream{t: t, resp: resp, r: bufio.NewReader(resp.Body)}
}

// next returns the next frame (the lines up to a blank line), or ok == false
// when the server has closed the stream.
func (s *stream) next() (frame string, ok bool) {
	s.t.Helper()
	var lines []string
	for {
		line, err := s.r.ReadString('\n')
		if err == io.EOF {
			return "", false
		}
		if err != nil {
			s.t.Fatalf("reading stream: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		if line == "" {
			return strings.Join(lines, "\n"), true
		}
		lines = append(lines, line)
	}
}

func (s *stream) expect(want string) {
	s.t.Helper()
	got, ok := s.next()
	if !ok {
		s.t.Fatalf("stream closed, want frame:\n%s", want)
	}
	if got != want {
		s.t.Fatalf("frame:\n%s\nwant:\n%s", got, want)
	}
}

func (s *stream) expectClosed() {
	s.t.Helper()
	if got, ok := s.next(); ok {
		s.t.Fatalf("stream still open, got frame:\n%s", got)
	}
}

func TestEventsReplaysHistoryAndClosesWhenJobIsDone(t *testing.T) {
	a := newTestAPI()
	a.Jobs = &fakeJobs{status: queue.StatusDone, events: []queue.Event{
		event(1, "queued", `{}`),
		event(2, "processing", `{"attempt": 1}`),
		event(3, "partial", `{"text": "hello"}`),
		event(4, "done", `{"text": "hello world", "language": "en"}`),
	}}

	s := openStream(t, a, "")

	if got := s.resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	s.expect("id: 1\nevent: queued\ndata: {}")
	s.expect("id: 2\nevent: processing\ndata: {\"attempt\": 1}")
	s.expect("id: 3\nevent: partial\ndata: {\"text\": \"hello\"}")
	s.expect("id: 4\nevent: done\ndata: {\"text\": \"hello world\", \"language\": \"en\"}")
	s.expectClosed()
}

func TestEventsResumesAfterLastEventID(t *testing.T) {
	a := newTestAPI()
	a.Jobs = &fakeJobs{status: queue.StatusDone, events: []queue.Event{
		event(1, "queued", `{}`),
		event(2, "processing", `{}`),
		event(3, "done", `{"text": "hi"}`),
	}}

	s := openStream(t, a, "2")

	s.expect("id: 3\nevent: done\ndata: {\"text\": \"hi\"}")
	s.expectClosed()
}

// A client that already has the final event and reconnects anyway must get
// an empty stream that closes, not one that waits forever.
func TestEventsClosesAtOnceWhenClientAlreadyHasEverything(t *testing.T) {
	a := newTestAPI()
	a.Jobs = &fakeJobs{status: queue.StatusDone, events: []queue.Event{
		event(1, "queued", `{}`),
		event(2, "done", `{}`),
	}}

	s := openStream(t, a, "2")

	s.expectClosed()
}

func TestEventsStreamsLiveEventsUntilTerminal(t *testing.T) {
	a := newTestAPI()
	bus := queue.NewBus()
	a.Events = bus
	a.Jobs = &fakeJobs{status: queue.StatusQueued, events: []queue.Event{event(1, "queued", `{}`)}}

	s := openStream(t, a, "")
	// Once the replayed frame has arrived the handler is subscribed, so
	// anything published now must reach the client.
	s.expect("id: 1\nevent: queued\ndata: {}")

	bus.Publish(event(2, "processing", `{}`))
	s.expect("id: 2\nevent: processing\ndata: {}")

	bus.Publish(event(3, "partial", `{"text": "hel"}`))
	s.expect("id: 3\nevent: partial\ndata: {\"text\": \"hel\"}")

	bus.Publish(event(4, "error", `{"message": "boom"}`))
	s.expect("id: 4\nevent: error\ndata: {\"message\": \"boom\"}")
	s.expectClosed()
}

// An event recorded while the handler is reading the history is both in the
// history and on the bus. It must be sent once.
func TestEventsSendsNoDuplicatesAcrossReplayAndLive(t *testing.T) {
	a := newTestAPI()
	bus := queue.NewBus()
	a.Events = bus
	a.Jobs = &fakeJobs{
		status: queue.StatusProcessing,
		events: []queue.Event{event(1, "queued", `{}`), event(2, "processing", `{}`)},
		// Runs inside Events, after the handler subscribed: event 2 "just
		// happened", and event 3 follows it.
		onEvents: func() {
			bus.Publish(event(2, "processing", `{}`))
			bus.Publish(event(3, "done", `{}`))
		},
	}

	s := openStream(t, a, "")

	s.expect("id: 1\nevent: queued\ndata: {}")
	s.expect("id: 2\nevent: processing\ndata: {}")
	s.expect("id: 3\nevent: done\ndata: {}")
	s.expectClosed()
}

func TestEventsSendsHeartbeats(t *testing.T) {
	a := newTestAPI()
	a.Heartbeat = 10 * time.Millisecond
	a.Jobs = &fakeJobs{status: queue.StatusQueued, events: []queue.Event{event(1, "queued", `{}`)}}

	s := openStream(t, a, "")

	s.expect("id: 1\nevent: queued\ndata: {}")
	s.expect(": keep-alive")
	s.expect(": keep-alive")
}

func TestEventsClosesStreamOnShutdown(t *testing.T) {
	a := newTestAPI()
	stopping := make(chan struct{})
	a.Stopping = stopping
	a.Jobs = &fakeJobs{status: queue.StatusQueued, events: []queue.Event{event(1, "queued", `{}`)}}

	s := openStream(t, a, "")
	s.expect("id: 1\nevent: queued\ndata: {}")

	close(stopping)
	s.expectClosed()
}

func TestEventsClosesStreamWhenClientFallsBehind(t *testing.T) {
	a := newTestAPI()
	bus := queue.NewBus()
	a.Events = bus
	a.Jobs = &fakeJobs{
		status: queue.StatusProcessing,
		events: []queue.Event{event(1, "queued", `{}`)},
		// Flood the subscription while the handler is busy reading history,
		// so its buffer overflows and the bus cuts it off.
		onEvents: func() {
			for seq := 2; seq < 200; seq++ {
				bus.Publish(event(seq, "partial", `{}`))
			}
		},
	}

	s := openStream(t, a, "")

	frames := 0
	for {
		if _, ok := s.next(); !ok {
			break
		}
		frames++
	}
	// The exact count depends on the buffer size; what matters is that the
	// stream ended instead of skipping events and carrying on.
	if frames == 0 || frames >= 199 {
		t.Errorf("received %d frames, want some but not all", frames)
	}
}

func TestEventsHandlerReturnsWhenClientDisconnects(t *testing.T) {
	a := newTestAPI()
	a.Jobs = &fakeJobs{status: queue.StatusQueued, events: []queue.Event{event(1, "queued", `{}`)}}

	// Wrap the handler to learn when it returns.
	returned := make(chan struct{})
	inner := a.Handler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r)
		close(returned)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/jobs/"+testJobID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer resp.Body.Close()
	if _, err := bufio.NewReader(resp.Body).ReadString('\n'); err != nil {
		t.Fatalf("reading first line: %v", err)
	}

	cancel() // the client hangs up

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was still running 2s after the client disconnected")
	}
}

func TestEventsErrors(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		lastEventID string
		jobStatus   string
		wantStatus  int
		wantCode    string
	}{
		{"unknown job", "/v1/jobs/" + testJobID + "/events", "", "", http.StatusNotFound, "job_not_found"},
		{"malformed id", "/v1/jobs/not-a-uuid/events", "", "queued", http.StatusNotFound, "job_not_found"},
		{"bad Last-Event-ID", "/v1/jobs/" + testJobID + "/events", "abc", "queued", http.StatusBadRequest, "invalid_last_event_id"},
		{"negative Last-Event-ID", "/v1/jobs/" + testJobID + "/events", "-1", "queued", http.StatusBadRequest, "invalid_last_event_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAPI()
			a.Jobs = &fakeJobs{status: tt.jobStatus}

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.lastEventID != "" {
				req.Header.Set("Last-Event-ID", tt.lastEventID)
			}
			rec := httptest.NewRecorder()
			a.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decoding error body %q: %v", rec.Body, err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}
