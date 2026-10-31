// Command server runs the voice translation backend: the HTTP API, the job
// workers, or both (the default). See the -role flag.
package main

import (
	"cmp"
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

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/api"
	"github.com/PelumiWeb/translator-app-go/server/internal/auth"
	"github.com/PelumiWeb/translator-app-go/server/internal/config"
	"github.com/PelumiWeb/translator-app-go/server/internal/models"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/fake"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/fish"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/google"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
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
	voices, err := storage.NewLocal(cfg.VoicesDir)
	if err != nil {
		return err
	}
	defer voices.Close()
	messages, err := storage.NewLocal(cfg.MessagesDir)
	if err != nil {
		return err
	}
	defer messages.Close()

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
	// The voice engine. A nil interface value means there is none: the API
	// then refuses requests to speak, and the phones speak in their own
	// voices. A Fish Audio key on its own is enough to choose Fish Audio.
	engine := cfg.VoiceEngine
	if engine == "" && cfg.FishAudioAPIKey != "" {
		engine = "fish"
	}
	var synthesizer provider.Synthesizer
	switch engine {
	case "":
	case "fish":
		if cfg.FishAudioAPIKey == "" {
			return errors.New("VOICE_ENGINE=fish needs FISH_AUDIO_API_KEY (put it in server/.env)")
		}
		synthesizer = fish.Synthesizer{
			APIKey:    cfg.FishAudioAPIKey,
			Model:     cfg.FishAudioModel,
			VoiceText: provider.OnboardingPassage,
		}
	case "fake":
		synthesizer = fake.Synthesizer{
			Delay:        1500 * time.Millisecond,
			FailAttempts: cfg.FakeFailAttempts,
		}
	default:
		return fmt.Errorf("unknown VOICE_ENGINE %q: want fish, fake, or nothing for none", cfg.VoiceEngine)
	}
	logger.Info("voice engine", "engine", cmp.Or(engine, "none"))

	translator, translatorName, err := newTranslator(cfg)
	if err != nil {
		return err
	}
	logger.Info("translation", "engine", translatorName)

	store := queue.NewStore(pool)
	accountStore := accounts.NewStore(pool)
	roomStore := rooms.NewStore(pool, store)

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
		deliverer := &rooms.Deliverer{
			Store:      roomStore,
			Translator: translator,
			Speaker:    synthesizer,
			Voices:     accountStore,
			Files:      voices,
			Messages:   messages,
		}
		workers := queue.NewPool(store, transcriber, synthesizer, deliverer, audio, logger, queue.Config{
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
	// Changes to rooms take the same road, on a channel of their own.
	hub := rooms.NewHub(pool, logger)
	background.Go(func() { hub.Run(ctx) })
	for _, ready := range []<-chan struct{}{listener.Ready(), hub.Ready()} {
		select {
		case <-ready: // do not serve streams before events can be heard
		case <-ctx.Done():
			return nil
		}
	}

	// Left as a nil interface, not a nil *auth.Verifier, when there is no
	// client ID: the API checks for nil to refuse Google sign-ins.
	var google api.TokenVerifier
	if cfg.GoogleClientID != "" {
		google = auth.NewGoogleVerifier(cfg.GoogleClientID)
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
		CanSpeak:       synthesizer != nil,
		Accounts:       accountStore,
		Apple:          auth.NewAppleVerifier(cfg.AppleAudience),
		Google:         google,
		Voices:         voices,
		Rooms:          roomStore,
		Changes:        hub,
		Messages:       messages,
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

// newTranslator picks the service that translates room messages. A nil
// interface value means there is none: messages then reach only listeners of
// the language they were said in. A Google key on its own chooses Google.
func newTranslator(cfg config.Config) (provider.Translator, string, error) {
	engine := cfg.TranslateEngine
	if engine == "" && cfg.GoogleTranslateKey != "" {
		engine = "google"
	}
	switch engine {
	case "":
		return nil, "none", nil
	case "google":
		if cfg.GoogleTranslateKey == "" {
			return nil, "", errors.New("TRANSLATE_ENGINE=google needs GOOGLE_TRANSLATE_KEY (put it in server/.env)")
		}
		return google.Translator{APIKey: cfg.GoogleTranslateKey}, engine, nil
	case "fake":
		return fake.Translator{}, engine, nil
	default:
		return nil, "", fmt.Errorf("unknown TRANSLATE_ENGINE %q: want google, fake, or nothing for none", cfg.TranslateEngine)
	}
}
