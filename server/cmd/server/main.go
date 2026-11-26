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
	"github.com/PelumiWeb/translator-app-go/server/internal/billing"
	"github.com/PelumiWeb/translator-app-go/server/internal/config"
	"github.com/PelumiWeb/translator-app-go/server/internal/es256"
	"github.com/PelumiWeb/translator-app-go/server/internal/models"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/azure"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/fake"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/fish"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/google"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/groq"
	"github.com/PelumiWeb/translator-app-go/server/internal/push"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
	"github.com/PelumiWeb/translator-app-go/server/internal/storage"
	"github.com/PelumiWeb/translator-app-go/server/internal/waitlist"
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
	var wl waitlistCommand
	flag.IntVar(&wl.invite, "waitlist-invite", 0, "instead of serving: mark the next `n` people waiting with an iPhone invited, and print them as CSV for TestFlight")
	flag.StringVar(&wl.remove, "waitlist-remove", "", "instead of serving: take this `email` off the waitlist")
	flag.BoolVar(&wl.count, "waitlist-count", false, "instead of serving: print how many are waiting")
	flag.BoolVar(&wl.list, "waitlist-list", false, "instead of serving: print everyone on the waitlist, invited or not, without inviting anyone")
	var sc safetyCommand
	flag.BoolVar(&sc.listReports, "reports-list", false, "instead of serving: print the open reports, oldest first")
	flag.StringVar(&sc.closeReport, "report-close", "", "instead of serving: close the report with this `id`, deleting its copy of the message")
	flag.StringVar(&sc.note, "note", "", "with -report-close: what was done about it")
	flag.StringVar(&sc.suspend, "account-suspend", "", "instead of serving: suspend the account with this `ID`, signing it out everywhere")
	flag.StringVar(&sc.deleteUser, "account-delete", "", "instead of serving: delete the account with this `ID` and everything it said (with -yes)")
	flag.BoolVar(&sc.yes, "yes", false, "with -account-delete: really delete it; without, only say what would go")
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
	if wl.set() {
		return wl.run(ctx, waitlist.NewStore(pool), os.Stdout)
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
	reports, err := storage.NewLocal(cfg.ReportsDir)
	if err != nil {
		return err
	}
	defer reports.Close()

	if sc.set() {
		queueStore, accountStore := queue.NewStore(pool), accounts.NewStore(pool)
		roomStore := rooms.NewStore(pool, queueStore, rooms.Limits{})
		return sc.run(ctx, safetyStores{
			accounts: accountStore,
			rooms:    roomStore,
			reports:  reports,
			api: &api.API{
				Logger: logger, Accounts: accountStore, Rooms: roomStore, Voices: voices, Messages: messages,
			},
		}, os.Stdout)
	}

	catalog, err := models.Load(cfg.ModelsDir)
	if err != nil {
		return err
	}
	logger.Info("models loaded", "dir", cfg.ModelsDir, "count", len(catalog.List()))

	transcriber, transcriberName, err := newTranscriber(cfg)
	if err != nil {
		return err
	}
	logger.Info("transcription", "engine", transcriberName)

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
	logger.Info("daily limits", "voice_minutes", cfg.DailyVoiceMinutes, "text_messages", cfg.DailyTexts)
	logger.Info("billing", "required", cfg.RequireSubscription,
		"trial_voice_minutes", cfg.TrialVoiceMinutes, "trial_text_messages", cfg.TrialTexts)

	store := queue.NewStore(pool)
	accountStore := accounts.NewStore(pool)
	roomStore := rooms.NewStore(pool, store, rooms.Limits{
		VoiceMS: cfg.DailyVoiceMinutes * 60_000,
		Texts:   cfg.DailyTexts,
	})

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
		pushes, err := newPushServices(cfg)
		if err != nil {
			return err
		}
		logger.Info("push notifications", "apns", pushes.APNs != nil, "fcm", pushes.FCM != nil)
		deliverer := &rooms.Deliverer{
			Store:      roomStore,
			Translator: translator,
			Speaker:    synthesizer,
			Voices:     accountStore,
			Files:      voices,
			Messages:   messages,
			Phones:     accountStore,
			ReportsTo:  cfg.ReportsNotifyHandle,
		}
		// Left nil, not an empty Services, when neither is set up: the
		// deliverer checks for nil to skip the work of finding phones.
		if pushes.Enabled() {
			deliverer.Pusher = pushes
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
	// The same for test accounts. A mistake in them stops the server, rather
	// than leaving the owner to find out on a phone that sign-in fails.
	var testAccounts api.CodeChecker
	if parsed, err := auth.ParseTestAccounts(cfg.TestAccounts); err != nil {
		return fmt.Errorf("reading TEST_ACCOUNTS: %w", err)
	} else if parsed != nil {
		testAccounts = parsed
		logger.Info("test accounts", "count", parsed.Len())
	}
	appleTokens, err := newAppleTokens(cfg)
	if err != nil {
		return err
	}
	logger.Info("sign in with apple revocation", "on", appleTokens != nil)
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
		TestAccounts:   testAccounts,
		Voices:         voices,
		Rooms:          roomStore,
		Changes:        hub,
		Messages:       messages,
		Reports:        reports,
		Waitlist:       waitlist.NewStore(pool),
		// Purchases are taken and recorded even while sending does not need
		// one, so a subscription bought before BILLING=on is already known.
		Billing: &billing.Service{
			Verifier: billing.Verifier{Roots: billing.AppleRoots(), BundleID: cfg.AppleAudience},
			Store:    billing.NewStore(pool),
		},
		RequireSubscription: cfg.RequireSubscription,
		AppleTokens:         appleTokens,
		TrialLimits:         rooms.Limits{VoiceMS: cfg.TrialVoiceMinutes * 60_000, Texts: cfg.TrialTexts},
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

// newTranscriber picks the service that transcribes the recordings phones
// send. Unlike the voice and translation engines there is no "none": the
// phones expect an answer, so without a key the fake stands in, which keeps
// local development free of keys.
func newTranscriber(cfg config.Config) (provider.Provider, string, error) {
	engine := cfg.TranscribeEngine
	if engine == "" {
		engine = "fake"
		if cfg.GroqAPIKey != "" {
			engine = "groq"
		}
	}
	switch engine {
	case "groq":
		if cfg.GroqAPIKey == "" {
			return nil, "", errors.New("TRANSCRIBE_ENGINE=groq needs GROQ_API_KEY (put it in server/.env)")
		}
		return groq.Transcriber{APIKey: cfg.GroqAPIKey, Model: cfg.GroqModel}, engine, nil
	case "fake":
		return fake.Provider{
			Text:         "this is a fake transcript from the fake provider",
			WordDelay:    300 * time.Millisecond,
			FailAttempts: cfg.FakeFailAttempts,
		}, engine, nil
	default:
		return nil, "", fmt.Errorf("unknown TRANSCRIBE_ENGINE %q: want groq or fake", cfg.TranscribeEngine)
	}
}

// newTranslator picks the service that translates room messages. A nil
// interface value means there is none: messages then reach only listeners of
// the language they were said in. A Google key on its own chooses Google.
func newTranslator(cfg config.Config) (provider.Translator, string, error) {
	engine := cfg.TranslateEngine
	if engine == "" {
		// A key on its own chooses its engine. With both keys there is no
		// telling which is meant, so TRANSLATE_ENGINE has to say.
		switch {
		case cfg.GoogleTranslateKey != "" && cfg.AzureTranslatorKey != "":
			return nil, "", errors.New("GOOGLE_TRANSLATE_KEY and AZURE_TRANSLATOR_KEY are both set: choose one with TRANSLATE_ENGINE=google or TRANSLATE_ENGINE=azure")
		case cfg.GoogleTranslateKey != "":
			engine = "google"
		case cfg.AzureTranslatorKey != "":
			engine = "azure"
		}
	}
	switch engine {
	case "":
		return nil, "none", nil
	case "google":
		if cfg.GoogleTranslateKey == "" {
			return nil, "", errors.New("TRANSLATE_ENGINE=google needs GOOGLE_TRANSLATE_KEY (put it in server/.env)")
		}
		return google.Translator{APIKey: cfg.GoogleTranslateKey}, engine, nil
	case "azure":
		if cfg.AzureTranslatorKey == "" {
			return nil, "", errors.New("TRANSLATE_ENGINE=azure needs AZURE_TRANSLATOR_KEY (put it in server/.env)")
		}
		return azure.Translator{Key: cfg.AzureTranslatorKey, Region: cfg.AzureTranslatorRegion}, engine, nil
	case "fake":
		return fake.Translator{}, engine, nil
	default:
		return nil, "", fmt.Errorf("unknown TRANSLATE_ENGINE %q: want google, azure, fake, or nothing for none", cfg.TranslateEngine)
	}
}

// newAppleTokens reads the Sign in with Apple key, when one is named. Left a
// nil interface, not a nil *auth.AppleTokens, when there is none: the API
// checks for nil to skip revoking. A key named but unreadable stops the
// server, rather than leaving revocation quietly off.
func newAppleTokens(cfg config.Config) (api.AppleTokenService, error) {
	if cfg.AppleSignInKeyFile == "" {
		return nil, nil
	}
	if cfg.AppleSignInKeyID == "" {
		return nil, errors.New("APPLE_SIGNIN_KEY_FILE needs APPLE_SIGNIN_KEY_ID, the key's ID from the developer account")
	}
	p8, err := os.ReadFile(cfg.AppleSignInKeyFile)
	if err != nil {
		return nil, fmt.Errorf("reading APPLE_SIGNIN_KEY_FILE: %w", err)
	}
	key, err := es256.ParseP8(p8)
	if err != nil {
		return nil, fmt.Errorf("APPLE_SIGNIN_KEY_FILE: %w", err)
	}
	return auth.AppleTokens{TeamID: cfg.APNsTeamID, KeyID: cfg.AppleSignInKeyID, ClientID: cfg.AppleAudience, Key: key}, nil
}

// newPushServices reads the push services' key files, for those that are set
// up. A file named but unreadable stops the server, rather than leaving
// notifications quietly off.
func newPushServices(cfg config.Config) (push.Services, error) {
	var services push.Services
	if cfg.APNsKeyFile != "" {
		if cfg.APNsKeyID == "" {
			return push.Services{}, errors.New("APNS_KEY_FILE needs APNS_KEY_ID, the key's ID from the developer account")
		}
		p8, err := os.ReadFile(cfg.APNsKeyFile)
		if err != nil {
			return push.Services{}, fmt.Errorf("reading APNS_KEY_FILE: %w", err)
		}
		if services.APNs, err = push.NewAPNs(p8, cfg.APNsKeyID, cfg.APNsTeamID, cfg.AppleAudience); err != nil {
			return push.Services{}, err
		}
	}
	if cfg.FCMServiceAccountFile != "" {
		account, err := os.ReadFile(cfg.FCMServiceAccountFile)
		if err != nil {
			return push.Services{}, fmt.Errorf("reading FCM_SERVICE_ACCOUNT_FILE: %w", err)
		}
		if services.FCM, err = push.NewFCM(account); err != nil {
			return push.Services{}, err
		}
	}
	return services, nil
}
