// Command server runs the voice translation backend: the HTTP API, the job
// workers, or both (the default). See the -role flag.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
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

// role says which parts of the server this process runs. Running the API and
// the workers as separate processes lets each be scaled and restarted on its
// own; "all" keeps development to a single command.
type role string

const (
	roleAll    role = "all"
	roleAPI    role = "api"
	roleWorker role = "worker"
)

func (r role) runsAPI() bool     { return r == roleAll || r == roleAPI }
func (r role) runsWorkers() bool { return r == roleAll || r == roleWorker }

func run(logger *slog.Logger) error {
	roleFlag := flag.String("role", string(roleAll), "what to run: all, api or worker")
	flag.Parse()
	role := role(*roleFlag)
	if !role.runsAPI() && !role.runsWorkers() {
		return fmt.Errorf("unknown -role %q: want all, api or worker", *roleFlag)
	}
	logger = logger.With("role", string(role))

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
	// Stands in for a voice-cloning engine: it answers with beeps. A real
	// one is added in milestone 9.4.
	synthesizer := fake.Synthesizer{
		Delay:        1500 * time.Millisecond,
		FailAttempts: cfg.FakeFailAttempts,
	}

	store := queue.NewStore(pool)

	// Everything started below runs until ctx is cancelled. On any way out
	// of this function, cancel it and wait for all of it to finish.
	var background sync.WaitGroup
	defer func() {
		stop()
		background.Wait()
	}()

	// wake nudges an idle worker when a job is enqueued. With no workers in
	// this process it does nothing, and the other process's workers find the
	// job on their next poll, at most a second later.
	wake := func() {}

	if role.runsWorkers() {
		workers := queue.NewPool(store, transcriber, synthesizer, audio, logger, queue.Config{
			Workers:       cfg.Workers,
			PollInterval:  time.Second,
			Lease:         cfg.JobLease,
			ShutdownGrace: shutdownTimeout,
			SweepInterval: cfg.SweepInterval,
		})
		wake = workers.Wake
		background.Go(func() {
			logger.Info("workers started", "count", cfg.Workers)
			workers.Run(ctx)
		})
	}

	if !role.runsAPI() {
		<-ctx.Done()
		logger.Info("shutdown signal received")
		background.Wait()
		logger.Info("worker stopped cleanly")
		return nil
	}

	// Events travel from whoever records them to this process through
	// Postgres: the store announces each one with NOTIFY, the listener hears
	// it and hands it to the bus, and the bus feeds the open SSE streams.
	// The same path is used when the workers are in this process.
	bus := queue.NewBus()
	listener := queue.NewListener(pool, store, bus, logger)
	background.Go(func() { listener.Run(ctx) })
	select {
	case <-listener.Ready(): // do not serve streams before events can be heard
	case <-ctx.Done():
		return nil
	}

	handlers := &api.API{
		Logger:         logger,
		DB:             pool,
		Jobs:           store,
		Events:         bus,
		Audio:          audio,
		Models:         catalog,
		Wake:           wake,
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

	background.Wait()

	logger.Info("server stopped cleanly")
	return nil
}
