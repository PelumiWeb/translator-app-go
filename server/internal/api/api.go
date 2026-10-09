// Package api holds the HTTP handlers.
package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/models"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
)

// The interfaces below list exactly what the handlers use from the rest of
// the server. Go interfaces are satisfied implicitly, so the package that
// uses a dependency declares the interface, as small as it can. The real
// types (*pgxpool.Pool, *queue.Store, *queue.Bus, *storage.Local) fit
// without knowing these exist, and tests pass small fakes instead.

type Pinger interface {
	Ping(ctx context.Context) error
}

type JobStore interface {
	EnqueueOnce(ctx context.Context, sourceLang, audioKey, idempotencyKey string) (id string, created bool, err error)
	Status(ctx context.Context, jobID string) (string, error)
	Events(ctx context.Context, jobID string, afterSeq int) ([]queue.Event, error)
}

type EventSubscriber interface {
	Subscribe(jobID string) (events <-chan queue.Event, unsubscribe func())
}

type AudioStore interface {
	Put(key string, r io.Reader) error
	Delete(key string) error
}

type ModelCatalog interface {
	List() []models.Model
	Get(id string) (models.Model, error)
	Open(id string) (file io.ReadSeekCloser, modified time.Time, err error)
}

// API carries the handlers' dependencies. The handlers are methods on it,
// which is how they reach those dependencies without any global state.
type API struct {
	Logger *slog.Logger
	DB     Pinger
	Jobs   JobStore
	Events EventSubscriber
	Audio  AudioStore
	Models ModelCatalog

	// Wake nudges an idle worker after a job is enqueued.
	Wake func()
	// Stopping is closed when the server begins to shut down. Event streams
	// end then; otherwise they would keep the server from ever stopping.
	Stopping <-chan struct{}

	MaxUploadBytes int64
	Heartbeat      time.Duration // gap between keep-alive comments on a stream
}

// Handler builds the routes. Since Go 1.22 the standard ServeMux matches on
// method and path, so no router library is needed.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("POST /v1/jobs", a.handleCreateJob)
	mux.HandleFunc("GET /v1/jobs/{id}/events", a.handleJobEvents)
	mux.HandleFunc("GET /v1/models/manifest", a.handleModelManifest)
	mux.HandleFunc("GET /v1/models/{id}", a.handleModelDownload)
	return mux
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := a.DB.Ping(r.Context()); err != nil {
		a.Logger.Error("health check: database unreachable", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	// Headers must be set before WriteHeader; after it they are ignored.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// An encode error here means the client went away; nothing useful to do.
	_ = json.NewEncoder(w).Encode(body)
}

// errorBody is the one error shape every endpoint returns.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}
