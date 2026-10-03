// Package api holds the HTTP handlers.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Pinger is the one thing the health check needs from the database.
//
// Go interfaces are satisfied implicitly, so the package that uses a
// dependency declares the interface, as small as it can. *pgxpool.Pool has a
// matching Ping method and fits without knowing this interface exists; tests
// pass a two-line fake instead of a database.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewHandler builds the routes. Since Go 1.22 the standard ServeMux matches
// on method and path, so no router library is needed.
func NewHandler(logger *slog.Logger, db Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth(logger, db))
	return mux
}

// handleHealth returns a handler instead of being one. The outer function
// receives the dependencies; the returned closure serves requests. This is
// the usual Go alternative to a controller class with injected fields.
func handleHealth(logger *slog.Logger, db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			logger.Error("health check: database unreachable", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	// Headers must be set before WriteHeader; after it they are ignored.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// An encode error here means the client went away; nothing useful to do.
	_ = json.NewEncoder(w).Encode(body)
}
