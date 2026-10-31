package rooms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// subscriberBuffer is how many changes a phone may fall behind by before
// its stream is ended.
const subscriberBuffer = 32

// Change says that something in a room changed: a new message, or a message
// that moved on (translated, spoken, failed). It carries no content. The
// phone fetches what it shows, so a change it missed while away costs
// nothing: it fetches everything again when it reconnects.
type Change struct {
	RoomID    string `json:"room_id"`
	MessageID string `json:"message_id,omitempty"`
}

// Hub passes changes to rooms on to the phones of their members, in this
// process. Whichever process makes a change announces it through Postgres
// (NOTIFY), so the API and the workers can run apart, as with job events.
type Hub struct {
	pool   *pgxpool.Pool
	logger *slog.Logger

	mu   sync.Mutex
	subs map[string]map[chan Change]struct{} // user ID -> set of subscribers

	ready     chan struct{}
	readyOnce sync.Once
}

func NewHub(pool *pgxpool.Pool, logger *slog.Logger) *Hub {
	return &Hub{
		pool:   pool,
		logger: logger,
		subs:   make(map[string]map[chan Change]struct{}),
		ready:  make(chan struct{}),
	}
}

// Ready is closed once the hub first listens. Changes before then are not heard.
func (h *Hub) Ready() <-chan struct{} {
	return h.ready
}

// Subscribe returns a channel of changes to userID's rooms, and a function
// that ends the subscription. The channel is closed when the subscription
// ends, whichever side ends it.
func (h *Hub) Subscribe(userID string) (changes <-chan Change, unsubscribe func()) {
	ch := make(chan Change, subscriberBuffer)
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = make(map[chan Change]struct{})
	}
	h.subs[userID][ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.remove(userID, ch)
	}
}

// Run listens until ctx is cancelled, reconnecting if the connection drops.
func (h *Hub) Run(ctx context.Context) {
	for {
		err := h.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		h.logger.Error("room listener lost its connection; reconnecting", "error", err)
		// Changes announced meanwhile are lost. Ending every stream makes the
		// phones reconnect and fetch again, which covers them.
		h.disconnectAll()

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (h *Hub) listen(ctx context.Context) error {
	pooled, err := h.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring a connection: %w", err)
	}
	// As in queue.Listener: waiting for notifications occupies a connection
	// for good, so it is taken out of the pool.
	conn := pooled.Hijack()
	defer conn.Close(context.WithoutCancel(ctx))

	if _, err := conn.Exec(ctx, "LISTEN "+changesChannel); err != nil {
		return fmt.Errorf("starting to listen: %w", err)
	}
	h.readyOnce.Do(func() { close(h.ready) })

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var announced struct {
			Change
			Users []string `json:"users"`
		}
		if err := json.Unmarshal([]byte(notification.Payload), &announced); err != nil {
			h.logger.Warn("ignoring malformed room notification", "error", err)
			continue
		}
		h.publish(announced.Users, announced.Change)
	}
}

// publish hands a change to every subscriber of each user. It never blocks:
// a subscriber whose buffer is full is cut off and will fetch afresh.
func (h *Hub) publish(users []string, change Change) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, user := range users {
		for ch := range h.subs[user] {
			select {
			case ch <- change:
			default:
				h.remove(user, ch)
			}
		}
	}
}

func (h *Hub) disconnectAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for user, subscribers := range h.subs {
		for ch := range subscribers {
			h.remove(user, ch)
		}
	}
}

// remove must be called with h.mu held.
func (h *Hub) remove(user string, ch chan Change) {
	if _, ok := h.subs[user][ch]; !ok {
		return // already removed; closing a channel twice would panic
	}
	delete(h.subs[user], ch)
	if len(h.subs[user]) == 0 {
		delete(h.subs, user)
	}
	close(ch)
}
