package queue

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listener carries job events from Postgres to this process's Bus.
//
// Whichever process records an event (a worker here, or one in another
// process, or on another machine) announces it with NOTIFY. Every process that
// serves event streams runs one Listener, which hears all announcements and
// passes the events on to its local subscribers. That is what lets the API
// and the workers be separate processes.
type Listener struct {
	pool   *pgxpool.Pool
	store  *Store
	bus    *Bus
	logger *slog.Logger

	ready     chan struct{}
	readyOnce sync.Once

	// backendPID is the Postgres process serving the current connection.
	// Tests use it to cut that connection.
	backendPID atomic.Uint32
}

func NewListener(pool *pgxpool.Pool, store *Store, bus *Bus, logger *slog.Logger) *Listener {
	return &Listener{pool: pool, store: store, bus: bus, logger: logger, ready: make(chan struct{})}
}

// Ready is closed once the listener is first connected and listening. Events
// recorded before that are not heard, so wait for it before serving streams.
func (l *Listener) Ready() <-chan struct{} {
	return l.ready
}

// Run listens until ctx is cancelled, reconnecting if the connection drops.
func (l *Listener) Run(ctx context.Context) {
	for {
		err := l.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		l.logger.Error("event listener lost its connection; reconnecting", "error", err)

		// Notifications sent while disconnected are gone for good, and there
		// is no telling which jobs they were for. So every stream is ended;
		// the clients reconnect and replay what they missed from the table.
		l.bus.DisconnectAll()

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// listen holds one connection and delivers notifications until it fails.
func (l *Listener) listen(ctx context.Context) error {
	pooled, err := l.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring a connection: %w", err)
	}
	// LISTEN belongs to a connection, and waiting for notifications occupies
	// it completely. Hijack takes this one out of the pool for good, so it
	// is never handed to a query, and the pool opens a replacement.
	conn := pooled.Hijack()
	// Close must work even when ctx is the reason we are leaving.
	defer conn.Close(context.WithoutCancel(ctx))

	if _, err := conn.Exec(ctx, "LISTEN "+eventsChannel); err != nil {
		return fmt.Errorf("starting to listen: %w", err)
	}
	l.backendPID.Store(conn.PgConn().PID())
	l.readyOnce.Do(func() { close(l.ready) })

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		l.deliver(ctx, notification.Payload)
	}
}

// deliver turns one notification ("<job id> <seq>") into an event on the bus.
func (l *Listener) deliver(ctx context.Context, payload string) {
	jobID, seqText, ok := strings.Cut(payload, " ")
	seq, err := strconv.Atoi(seqText)
	if !ok || err != nil {
		l.logger.Warn("ignoring malformed event notification", "payload", payload)
		return
	}

	// Every process hears every notification. Most are for jobs nobody here
	// is watching, and those cost nothing more than this check.
	if !l.bus.HasSubscribers(jobID) {
		return
	}

	event, err := l.store.Event(ctx, jobID, seq)
	if err != nil {
		if ctx.Err() == nil {
			l.logger.Error("loading announced event", "job_id", jobID, "seq", seq, "error", err)
		}
		// The subscribers would otherwise wait for an event that is never
		// coming. End their streams so they replay it from the table.
		l.bus.Disconnect(jobID)
		return
	}
	l.bus.Publish(event)
}
