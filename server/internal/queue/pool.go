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

	// SweepInterval is how often to look for jobs whose worker has died.
	// Zero means 30 seconds.
	SweepInterval time.Duration

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
	if cfg.Lease <= 0 {
		cfg.Lease = 2 * time.Minute
	}
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = 30 * time.Second
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

	// The sweeper stops as soon as shutdown begins; it has its own
	// WaitGroup so Run can wait for it without mixing it up with the workers.
	var sweeper sync.WaitGroup
	sweeper.Go(func() { p.sweep(ctx) })
	defer sweeper.Wait()

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

// sweep periodically returns jobs to the queue whose worker has gone silent.
// Every server process runs one. Several running at once is harmless: the
// database lets only one of them change a given row.
func (p *Pool) sweep(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.SweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := p.store.RequeueExpired(ctx)
			if err != nil {
				if ctx.Err() == nil { // not just the shutdown interrupting it
					p.logger.Error("sweeping expired leases", "error", err)
				}
				continue
			}
			if n > 0 {
				p.logger.Warn("recovered jobs from workers that stopped", "count", n)
				p.Wake()
			}
		}
	}
}

// errLeaseLost is the reason a job's context is cancelled when its lease
// could not be renewed because another worker now owns the job.
var errLeaseLost = errors.New("queue: lease lost to another worker")

// keepLease renews the job's lease in the background for as long as the job
// runs. Without it, any job slower than the lease would look abandoned and be
// handed to a second worker while the first is still on it.
//
// If the lease turns out to be lost anyway, it cancels the job through lost.
// The returned function stops the renewing and waits for it to end.
func (p *Pool) keepLease(ctx context.Context, lost context.CancelCauseFunc, logger *slog.Logger, job Job) (stop func()) {
	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)
		// Three chances to renew before the lease runs out, so one failed
		// database call does not cost the job.
		ticker := time.NewTicker(p.cfg.Lease / 3)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := p.store.ExtendLease(ctx, job, p.cfg.Lease)
				if errors.Is(err, ErrNotProcessing) {
					lost(errLeaseLost)
					return
				}
				if err != nil && ctx.Err() == nil {
					logger.Warn("renewing lease", "error", err)
				}
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
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

	// workCtx is cancelled like ctx (shutdown), and also if the lease is
	// lost. WithCancelCause records which, so the two can be told apart.
	workCtx, cancelWork := context.WithCancelCause(ctx)
	defer cancelWork(nil)
	stopHeartbeat := p.keepLease(workCtx, cancelWork, logger, job)

	result, err := p.transcribe(workCtx, logger, job)
	stopHeartbeat()

	if errors.Is(context.Cause(workCtx), errLeaseLost) {
		// The job belongs to another worker now. Anything written here
		// would be about their attempt, so write nothing, and leave the
		// audio for them.
		logger.Warn("job abandoned: its lease expired and another worker took it")
		return
	}

	// The final status must be written even when ctx has just been
	// cancelled by shutdown, so it gets a context of its own.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	switch {
	case err == nil:
		if err := p.store.Complete(finishCtx, job, result.Text, result.Language); err != nil {
			logger.Error("completing job", "error", err)
			return
		}
		logger.Info("job done")

	case ctx.Err() != nil:
		// The job did not fail; the server is stopping. Hand it back so it
		// runs again after restart, and keep its audio.
		if err := p.store.Release(finishCtx, job); err != nil {
			logger.Error("releasing job", "error", err)
			return
		}
		logger.Info("job released for shutdown")
		return

	case !provider.IsPermanent(err) && job.Attempt < job.MaxAttempts:
		// Probably temporary, and there are attempts left: try again later.
		// The audio is kept for the next attempt.
		delay := p.cfg.Backoff(job.Attempt)
		if err := p.store.Retry(finishCtx, job, err.Error(), delay); err != nil {
			logger.Error("scheduling retry", "error", err)
			return
		}
		logger.Warn("job will be retried", "error", err, "attempt", job.Attempt, "retry_in", delay)
		return

	default:
		if err := p.store.Fail(finishCtx, job, err.Error()); err != nil {
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
