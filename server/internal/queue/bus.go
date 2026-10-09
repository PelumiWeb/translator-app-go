package queue

import "sync"

// subscriberBuffer is how many undelivered events a subscriber may fall
// behind by before it is cut off.
const subscriberBuffer = 64

// Bus hands job events to the SSE handlers in this process. It holds nothing:
// the job_events table is the record, and the bus only passes on what the
// Listener heard was written. One Bus serves all the streams of one process.
type Bus struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{} // job id -> set of subscribers
}

func NewBus() *Bus {
	return &Bus{subs: make(map[string]map[chan Event]struct{})}
}

// Subscribe returns a channel of events for one job, and a function that ends
// the subscription.
//
// The first events may be ones recorded just before the call, still on their
// way from the database. A subscriber that also reads the job's history must
// skip what it already has, by seq. The channel is closed when the subscription
// ends, whichever side ends it. Calling unsubscribe more than once is fine.
func (b *Bus) Subscribe(jobID string) (events <-chan Event, unsubscribe func()) {
	ch := make(chan Event, subscriberBuffer)

	b.mu.Lock()
	if b.subs[jobID] == nil {
		b.subs[jobID] = make(map[chan Event]struct{})
	}
	b.subs[jobID][ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.remove(jobID, ch)
	}
}

// Publish delivers an event to every subscriber of its job. It never blocks,
// so a stalled client cannot hold up a worker.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs[e.JobID] {
		select {
		case ch <- e:
		default:
			// The subscriber's buffer is full. Dropping this one event
			// would leave a silent gap, so the subscription is ended
			// instead. The client reconnects and replays from the table.
			b.remove(e.JobID, ch)
		}
	}
}

// HasSubscribers reports whether anyone in this process is following the job.
func (b *Bus) HasSubscribers(jobID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs[jobID]) > 0
}

// Disconnect ends every subscription to one job. Its streams close, and the
// clients reconnect and replay from the table. Used when an event for the job
// could not be delivered: ending the stream is honest, skipping an event is not.
func (b *Bus) Disconnect(jobID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[jobID] {
		b.remove(jobID, ch)
	}
}

// DisconnectAll ends every subscription, for when events may have been missed
// for any job.
func (b *Bus) DisconnectAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for jobID, subscribers := range b.subs {
		for ch := range subscribers {
			b.remove(jobID, ch)
		}
	}
}

// remove must be called with b.mu held.
func (b *Bus) remove(jobID string, ch chan Event) {
	if _, ok := b.subs[jobID][ch]; !ok {
		return // already removed; closing a channel twice would panic
	}
	delete(b.subs[jobID], ch)
	if len(b.subs[jobID]) == 0 {
		delete(b.subs, jobID)
	}
	close(ch)
}
