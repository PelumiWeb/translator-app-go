package queue

import "sync"

// subscriberBuffer is how many undelivered events a subscriber may fall
// behind by before it is cut off.
const subscriberBuffer = 64

// Bus passes job events from the workers to the SSE handlers inside one
// process. It holds nothing: the job_events table is the record, and the bus
// only tells listeners that something new was written.
type Bus struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{} // job id -> set of subscribers
}

func NewBus() *Bus {
	return &Bus{subs: make(map[string]map[chan Event]struct{})}
}

// Subscribe returns a channel of future events for one job, and a function
// that ends the subscription. The channel is closed when the subscription
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
