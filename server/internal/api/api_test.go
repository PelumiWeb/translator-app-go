package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
)

// Fakes shared by the tests in this package. None of these tests needs a
// database.

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

// fakeJobs is an in-memory JobStore.
type fakeJobs struct {
	status string        // returned by Status; empty means the job is unknown
	events []queue.Event // returned by Events, filtered by afterSeq

	enqueueErr error
	// onEvents, if set, runs inside Events. By then the handler has already
	// subscribed, which lets a test publish an event "during" the replay.
	onEvents func()

	mu       sync.Mutex
	enqueued []string // "lang key" per Enqueue call
}

func (f *fakeJobs) Enqueue(_ context.Context, sourceLang, audioKey string) (string, error) {
	if f.enqueueErr != nil {
		return "", f.enqueueErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueued = append(f.enqueued, sourceLang+" "+audioKey)
	return testJobID, nil
}

func (f *fakeJobs) Status(context.Context, string) (string, error) {
	if f.status == "" {
		return "", queue.ErrJobNotFound
	}
	return f.status, nil
}

func (f *fakeJobs) Events(_ context.Context, _ string, afterSeq int) ([]queue.Event, error) {
	if f.onEvents != nil {
		f.onEvents()
	}
	var out []queue.Event
	for _, e := range f.events {
		if e.Seq > afterSeq {
			out = append(out, e)
		}
	}
	return out, nil
}

// fakeAudio is an in-memory AudioStore.
type fakeAudio struct {
	mu      sync.Mutex
	blobs   map[string]string
	deleted []string
}

func (a *fakeAudio) Put(key string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blobs == nil {
		a.blobs = map[string]string{}
	}
	a.blobs[key] = string(data)
	return nil
}

func (a *fakeAudio) Delete(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.blobs, key)
	a.deleted = append(a.deleted, key)
	return nil
}

func (a *fakeAudio) keys() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	keys := make([]string, 0, len(a.blobs))
	for k := range a.blobs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

const testJobID = "5b1c0c1e-0000-4000-8000-000000000001"

// newTestAPI returns an API wired to fakes. Tests replace the fields they
// care about before calling Handler.
func newTestAPI() *API {
	return &API{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:             fakePinger{},
		Jobs:           &fakeJobs{},
		Events:         queue.NewBus(),
		Audio:          &fakeAudio{},
		Wake:           func() {},
		Stopping:       make(chan struct{}),
		MaxUploadBytes: 1 << 20,
		Heartbeat:      time.Hour, // effectively off unless a test lowers it
	}
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   string
	}{
		{"database up", nil, http.StatusOK, `{"status":"ok"}`},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable, `{"status":"unavailable"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAPI()
			a.DB = fakePinger{err: tt.pingErr}

			// httptest runs the handler in memory: no port, no network.
			rec := httptest.NewRecorder()
			a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
				t.Errorf("body = %s, want %s", got, tt.wantBody)
			}
		})
	}
}

func TestHealthRejectsOtherMethods(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestAPI().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
