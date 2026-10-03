// Command server runs the voice translation backend: HTTP API and, from
// checkpoint 1.2, the job workers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"voicetranslation/server/internal/api"
	"voicetranslation/server/internal/config"
	"voicetranslation/server/migrations"
)

const shutdownTimeout = 30 * time.Second

// main only reports the error and sets the exit code. The work is in run so
// that its deferred cleanups execute: os.Exit skips defers.
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()

	// ctx is cancelled on Ctrl-C or SIGTERM. Everything long-running takes
	// it, so one signal tells the whole process to wind down.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configuring database pool: %w", err)
	}
	defer pool.Close()

	// pgxpool connects lazily, so ping to fail at startup, not on the first
	// request, when the database is down.
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connecting to database (try `make db-up`): %w", err)
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: api.NewHandler(logger, pool),
		// Limits how long a client may take to send its headers. There is
		// deliberately no WriteTimeout: it would cut off SSE streams, which
		// stay open for as long as a job runs.
		ReadHeaderTimeout: 10 * time.Second,
	}

	// ListenAndServe blocks, so it runs in a goroutine and reports back on a
	// channel. The buffer of 1 lets the goroutine finish even if nobody is
	// left to receive.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		// Returned before any shutdown was requested, e.g. the port is taken.
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	// A second Ctrl-C now kills the process the default way instead of
	// being swallowed while we wait for requests to finish.
	stop()

	// ctx is already cancelled, so the grace period needs a fresh context.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown stops accepting connections and waits for in-flight requests.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down http server: %w", err)
	}
	// ListenAndServe always returns ErrServerClosed after Shutdown.
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	logger.Info("server stopped cleanly")
	return nil
}
