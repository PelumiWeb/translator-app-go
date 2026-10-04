package queue

import "testing"

func TestBusDeliversOnlyToSubscribersOfThatJob(t *testing.T) {
	bus := NewBus()
	a, unsubA := bus.Subscribe("job-a")
	defer unsubA()
	b, unsubB := bus.Subscribe("job-b")
	defer unsubB()

	bus.Publish(Event{JobID: "job-a", Seq: 1, Type: EventQueued})

	select {
	case got := <-a:
		if got.Seq != 1 {
			t.Errorf("seq = %d, want 1", got.Seq)
		}
	default:
		t.Error("subscriber of job-a received nothing")
	}
	select {
	case got := <-b:
		t.Errorf("subscriber of job-b received %+v", got)
	default:
	}
}

func TestBusPublishWithoutSubscribers(t *testing.T) {
	NewBus().Publish(Event{JobID: "nobody", Seq: 1}) // must not block or panic
}

func TestBusUnsubscribeClosesChannel(t *testing.T) {
	bus := NewBus()
	ch, unsubscribe := bus.Subscribe("job")

	unsubscribe()
	unsubscribe() // a second call must not panic

	// Receiving from a closed channel returns at once with ok == false.
	if _, ok := <-ch; ok {
		t.Error("channel is still open after unsubscribe")
	}
	if len(bus.subs) != 0 {
		t.Errorf("bus still tracks %d jobs, want 0", len(bus.subs))
	}
}

func TestBusCutsOffSubscriberThatFallsBehind(t *testing.T) {
	bus := NewBus()
	ch, unsubscribe := bus.Subscribe("job")
	defer unsubscribe()

	// Nobody reads ch, so the buffer fills and one more event overflows it.
	for seq := 1; seq <= subscriberBuffer+1; seq++ {
		bus.Publish(Event{JobID: "job", Seq: seq})
	}

	received := 0
	for range ch { // ends because the bus closed the channel
		received++
	}
	if received != subscriberBuffer {
		t.Errorf("received %d events before the cut-off, want %d", received, subscriberBuffer)
	}
}
