package queue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

// AudioStore is what the workers need from audio storage.
type AudioStore interface {
	Open(key string) (io.ReadCloser, error)
	Delete(key string) error
}

type Config struct {
	Workers       int
	PollInterval  time.Duration // how often an idle worker checks for jobs
	Lease         time.Duration // how long a claim is valid
	ShutdownGrace time.Duration // how long running jobs get to finish on shutdown

	// Backoff says how long to wait before retrying after the given attempt.
	// Nil means ExponentialBackoff(2s, 1m). Tests pass a short fixed delay.
	Backoff func(attempt int) time.Duration
}

// Pool runs a fixed number of workers that claim and process jobs.
type Pool struct {
	store    *Store
	provider provider.Provider
	audio    AudioStore
	logger   *slog.Logger
	cfg      Config
	wake     chan struct{}
}

func NewPool(store *Store, p provider.Provider, audio AudioStore, logger *slog.Logger, cfg Config) *Pool {
	if cfg.Backoff == nil {
		cfg.Backoff = ExponentialBackoff(2*time.Second, time.Minute)
	}
	return &Pool{
		store:    store,
		provider: p,
		audio:    audio,
		logger:   logger,
		cfg:      cfg,
		// Buffer of 1: a wake-up sent while every worker is busy is kept
		// for the next one that goes idle instead of being lost.
		wake: make(chan struct{}, 1),
	}
}

// Wake tells an idle worker to look for a job now instead of at its next
// poll. It never blocks.
func (p *Pool) Wake() {
	select {
	case p.wake <- struct{}{}:
	default: // a wake-up is already pending; one is enough
	}
}

// Run starts the workers and blocks until ctx is cancelled and they have all
// stopped.
//
// Shutdown has two stages. Cancelling ctx stops workers from claiming new
// jobs, but jobs already running keep going. If they are still running
// after ShutdownGrace, their own context is cancelled and they are released
// back to the queue.
func (p *Pool) Run(ctx context.Context) {
	// jobCtx carries ctx's values but ignores its cancellation, so running
	// jobs outlive the stop signal until cancelJobs is called.
	jobCtx, cancelJobs := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelJobs()

	var wg sync.WaitGroup
	for id := range p.cfg.Workers {
		wg.Go(func() { p.work(ctx, jobCtx, id) })
	}

	// wg.Wait cannot be used in a select, so a goroutine turns "all workers
	// finished" into a channel that can.
	idle := make(chan struct{})
	go func() {
		wg.Wait()
		close(idle)
	}()

	<-ctx.Done()
	select {
	case <-idle:
	case <-time.After(p.cfg.ShutdownGrace):
		p.logger.Warn("shutdown grace period over, cancelling running jobs")
		cancelJobs()
		<-idle
	}
}

// work is one worker's loop: claim a job, process it, repeat.
func (p *Pool) work(ctx, jobCtx context.Context, id int) {
	logger := p.logger.With("worker", id)

	for {
		if ctx.Err() != nil {
			return
		}

		// Claim uses jobCtx on purpose. Cancelling a claim halfway could
		// leave a job marked as processing with no worker on it.
		job, err := p.store.Claim(jobCtx, p.cfg.Lease)
		if err == nil {
			p.process(jobCtx, logger.With("job_id", job.ID), job)
			continue // there may be more; look again straight away
		}
		if !errors.Is(err, ErrNoJobs) {
			logger.Error("claiming job", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-p.wake:
		case <-time.After(p.cfg.PollInterval):
		}
	}
}

func (p *Pool) process(ctx context.Context, logger *slog.Logger, job Job) {
	logger.Info("job started", "attempt", job.Attempt)
	result, err := p.transcribe(ctx, logger, job)

	// The final status must be written even when ctx has just been
	// cancelled by shutdown, so it gets a context of its own.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	switch {
	case err == nil:
		if err := p.store.Complete(finishCtx, job.ID, result.Text, result.Language); err != nil {
			logger.Error("completing job", "error", err)
			return
		}
		logger.Info("job done")

	case ctx.Err() != nil:
		// The job did not fail; the server is stopping. Hand it back so it
		// runs again after restart, and keep its audio.
		if err := p.store.Release(finishCtx, job.ID); err != nil {
			logger.Error("releasing job", "error", err)
			return
		}
		logger.Info("job released for shutdown")
		return

	case !provider.IsPermanent(err) && job.Attempt < job.MaxAttempts:
		// Probably temporary, and there are attempts left: try again later.
		// The audio is kept for the next attempt.
		delay := p.cfg.Backoff(job.Attempt)
		if err := p.store.Retry(finishCtx, job.ID, err.Error(), delay); err != nil {
			logger.Error("scheduling retry", "error", err)
			return
		}
		logger.Warn("job will be retried", "error", err, "attempt", job.Attempt, "retry_in", delay)
		return

	default:
		if err := p.store.Fail(finishCtx, job.ID, err.Error()); err != nil {
			logger.Error("failing job", "error", err)
			return
		}
		logger.Warn("job failed", "error", err, "attempt", job.Attempt)
	}

	// Reached only for done and failed jobs: the audio is no longer needed.
	if err := p.audio.Delete(job.AudioKey); err != nil {
		logger.Warn("deleting audio", "error", err)
	}
}

func (p *Pool) transcribe(ctx context.Context, logger *slog.Logger, job Job) (provider.Result, error) {
	audio, err := p.audio.Open(job.AudioKey)
	if err != nil {
		// Audio that is missing now will be missing on every retry.
		return provider.Result{}, provider.Permanent(err)
	}
	defer audio.Close()

	onPartial := func(text string) {
		// A lost partial is not worth failing the job for.
		if _, err := p.store.AppendEvent(ctx, job.ID, EventPartial, map[string]string{"text": text}); err != nil {
			logger.Warn("recording partial", "error", err)
		}
	}
	opts := provider.Options{SourceLang: job.SourceLang, Attempt: job.Attempt}
	return p.provider.Transcribe(ctx, audio, opts, onPartial)
}
