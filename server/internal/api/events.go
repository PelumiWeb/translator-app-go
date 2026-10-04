package api

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// handleJobEvents streams a job's events as Server-Sent Events: first the
// ones already recorded, then new ones as they happen, until the job ends.
func (a *API) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	// Checked here because Postgres rejects a malformed uuid with an error,
	// which would otherwise surface as a 500.
	if !uuidPattern.MatchString(jobID) {
		writeError(w, http.StatusNotFound, "job_not_found", "no such job")
		return
	}

	// A reconnecting client sends the id of the last event it received.
	afterSeq := 0
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid_last_event_id", "Last-Event-ID must be a non-negative integer")
			return
		}
		afterSeq = n
	}

	status, err := a.Jobs.Status(r.Context(), jobID)
	if errors.Is(err, queue.ErrJobNotFound) {
		writeError(w, http.StatusNotFound, "job_not_found", "no such job")
		return
	}
	if err != nil {
		a.Logger.Error("reading job status", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the job")
		return
	}

	// Order matters: subscribe first, read the history second. An event
	// recorded between the two then shows up in both and is sent once
	// (see lastSeq below). The other way round, it would be in neither.
	live, unsubscribe := a.Events.Subscribe(jobID)
	defer unsubscribe()

	history, err := a.Jobs.Events(r.Context(), jobID, afterSeq)
	if err != nil {
		a.Logger.Error("reading job events", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the job")
		return
	}

	// From here on the response is a stream: the status line is sent now
	// and can no longer change, so later failures can only end the stream.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	// Go buffers response writes. ResponseController.Flush pushes what has
	// been written to the client now instead of when the handler returns.
	flusher := http.NewResponseController(w)
	if err := flusher.Flush(); err != nil {
		return
	}

	lastSeq := afterSeq
	// send writes one event and reports whether the stream should end,
	// either because the job is over or because the client is gone.
	send := func(e queue.Event) (end bool) {
		if e.Seq <= lastSeq {
			return false // already sent during the replay
		}
		// The SSE wire format: "field: value" lines, blank line to finish.
		// The payload is single-line JSON, so one data line is enough.
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Type, e.Payload); err != nil {
			return true
		}
		if err := flusher.Flush(); err != nil {
			return true
		}
		lastSeq = e.Seq
		return e.Type == queue.EventDone || e.Type == queue.EventError
	}

	for _, e := range history {
		if send(e) {
			return
		}
	}
	// The job ended before this request and the client already has its last
	// event (it reconnected with that event's id). Nothing more will come.
	if status == queue.StatusDone || status == queue.StatusFailed {
		return
	}

	heartbeat := time.NewTicker(a.Heartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done(): // the client disconnected
			return
		case <-a.Stopping: // the server is shutting down
			return
		case e, ok := <-live:
			if !ok {
				// The bus ended the subscription because this client fell
				// too far behind. It can reconnect and catch up.
				return
			}
			if send(e) {
				return
			}
		case <-heartbeat.C:
			// A line starting with ":" is a comment that clients ignore.
			// It keeps proxies from closing a connection that looks idle.
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			if err := flusher.Flush(); err != nil {
				return
			}
		}
	}
}
