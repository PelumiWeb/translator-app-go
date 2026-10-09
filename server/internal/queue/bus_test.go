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

func TestBusHasSubscribers(t *testing.T) {
	bus := NewBus()
	if bus.HasSubscribers("job") {
		t.Error("a new bus reports subscribers")
	}

	_, unsubscribe := bus.Subscribe("job")
	if !bus.HasSubscribers("job") || bus.HasSubscribers("other") {
		t.Error("HasSubscribers does not match the one subscription to job")
	}

	unsubscribe()
	if bus.HasSubscribers("job") {
		t.Error("still reports subscribers after the last one left")
	}
}

func TestBusDisconnectEndsOnlyThatJobsSubscriptions(t *testing.T) {
	bus := NewBus()
	first, unsubFirst := bus.Subscribe("job-a")
	second, unsubSecond := bus.Subscribe("job-a")
	other, unsubOther := bus.Subscribe("job-b")
	defer unsubOther()

	bus.Disconnect("job-a")

	for name, ch := range map[string]<-chan Event{"first": first, "second": second} {
		if _, ok := <-ch; ok {
			t.Errorf("the %s subscription to job-a is still open", name)
		}
	}
	// Unsubscribing after a disconnect must not close the channel twice.
	unsubFirst()
	unsubSecond()

	bus.Publish(Event{JobID: "job-b", Seq: 1})
	if got, ok := <-other; !ok || got.Seq != 1 {
		t.Error("the subscription to job-b was disturbed")
	}
}

func TestBusDisconnectAll(t *testing.T) {
	bus := NewBus()
	a, _ := bus.Subscribe("job-a")
	b, _ := bus.Subscribe("job-b")

	bus.DisconnectAll()

	if _, ok := <-a; ok {
		t.Error("job-a subscription still open")
	}
	if _, ok := <-b; ok {
		t.Error("job-b subscription still open")
	}
	if len(bus.subs) != 0 {
		t.Errorf("bus still tracks %d jobs", len(bus.subs))
	}
}
