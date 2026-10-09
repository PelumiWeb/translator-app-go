// Command server runs the voice translation backend: the HTTP API and the
// job workers, in one process.
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

	"github.com/PelumiWeb/translator-app-go/server/internal/api"
	"github.com/PelumiWeb/translator-app-go/server/internal/config"
	"github.com/PelumiWeb/translator-app-go/server/internal/models"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/fake"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
	"github.com/PelumiWeb/translator-app-go/server/internal/storage"
	"github.com/PelumiWeb/translator-app-go/server/migrations"
)

const (
	shutdownTimeout = 30 * time.Second
	// A 30 second clip of 16 kHz mono 16-bit audio is just under 1 MB.
	maxUploadBytes = 10 << 20 // 10 MB
)

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

	audio, err := storage.NewLocal(cfg.AudioDir)
	if err != nil {
		return err
	}
	defer audio.Close()

	catalog, err := models.Load(cfg.ModelsDir)
	if err != nil {
		return err
	}
	logger.Info("models loaded", "dir", cfg.ModelsDir, "count", len(catalog.List()))

	// The only provider so far. A real one is chosen in milestone 7.
	transcriber := fake.Provider{
		Text:         "this is a fake transcript from the fake provider",
		WordDelay:    300 * time.Millisecond,
		FailAttempts: cfg.FakeFailAttempts,
	}
	// The bus connects the two halves of the process: workers record events
	// through the store, and the store announces them to the SSE handlers.
	bus := queue.NewBus()
	store := queue.NewStore(pool, bus)

	workers := queue.NewPool(store, transcriber, audio, logger, queue.Config{
		Workers:       cfg.Workers,
		PollInterval:  time.Second,
		Lease:         cfg.JobLease,
		ShutdownGrace: shutdownTimeout,
		SweepInterval: cfg.SweepInterval,
	})

	// Closed when the workers have stopped, so run can wait for them.
	workersDone := make(chan struct{})
	go func() {
		defer close(workersDone)
		logger.Info("workers started", "count", cfg.Workers)
		workers.Run(ctx)
	}()

	handlers := &api.API{
		Logger:         logger,
		DB:             pool,
		Jobs:           store,
		Events:         bus,
		Audio:          audio,
		Models:         catalog,
		Wake:           workers.Wake,
		Stopping:       ctx.Done(),
		MaxUploadBytes: maxUploadBytes,
		Heartbeat:      15 * time.Second,
	}

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handlers.Handler(),
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
		stop() // cancels ctx, which stops the workers
		<-workersDone
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
	// The workers are draining at the same time: they saw ctx cancelled too.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down http server: %w", err)
	}
	// ListenAndServe always returns ErrServerClosed after Shutdown.
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	<-workersDone

	logger.Info("server stopped cleanly")
	return nil
}
