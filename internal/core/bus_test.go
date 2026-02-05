package core

import (
	"testing"
	"time"
)

func TestEventBus(t *testing.T) {
	bus := NewEventBus(10)

	// Subscribe
	ch := bus.Subscribe()

	// Publish event
	event := Event{
		Type:      EventMysisCreated,
		MysisID:   "test-id",
		MysisName: "test-mysis",
		Timestamp: time.Now(),
	}
	bus.Publish(event)

	// Receive event
	select {
	case received := <-ch:
		if received.Type != EventMysisCreated {
			t.Errorf("expected type=%s, got %s", EventMysisCreated, received.Type)
		}
		if received.MysisID != "test-id" {
			t.Errorf("expected mysis_id=test-id, got %s", received.MysisID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	// Unsubscribe
	bus.Unsubscribe(ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed")
	}
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	bus := NewEventBus(10)

	ch1 := bus.Subscribe()
	ch2 := bus.Subscribe()

	event := Event{
		Type:      EventMysisMessage,
		MysisID:   "mysis-1",
		Timestamp: time.Now(),
	}
	bus.Publish(event)

	// Both should receive
	select {
	case <-ch1:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1 timeout")
	}

	select {
	case <-ch2:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2 timeout")
	}

	bus.Close()
}

func TestEventBusNonBlocking(t *testing.T) {
	// Small buffer
	bus := NewEventBus(1)
	ch := bus.Subscribe()

	for len(ch) < cap(ch) {
		bus.Publish(Event{Type: EventMysisCreated})
	}

	// This should not block (event dropped)
	done := make(chan bool)
	go func() {
		bus.Publish(Event{Type: EventMysisDeleted})
		done <- true
	}()

	select {
	case <-done:
		// Good, didn't block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("publish blocked")
	}

	// Drain the buffer
	<-ch
	bus.Close()
}

func TestEventBusPublishBlockingDelivers(t *testing.T) {
	bus := NewEventBus(1)
	ch := bus.Subscribe()

	for len(ch) < cap(ch) {
		bus.Publish(Event{Type: EventMysisCreated})
	}

	resultCh := make(chan bool, 1)
	go func() {
		resultCh <- bus.PublishBlocking(Event{Type: EventMysisDeleted}, 100*time.Millisecond)
	}()

	select {
	case <-resultCh:
		t.Fatal("expected PublishBlocking to wait for buffer space")
	case <-time.After(20 * time.Millisecond):
	}

	<-ch

	select {
	case delivered := <-resultCh:
		if !delivered {
			t.Fatal("expected PublishBlocking to deliver")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for PublishBlocking")
	}

	deletedFound := false
	for i := 0; i < cap(ch); i++ {
		received := <-ch
		if received.Type == EventMysisDeleted {
			deletedFound = true
		}
	}
	if !deletedFound {
		t.Fatal("expected to find blocking event")
	}

	bus.Close()
}

func TestEventBusPublishBlockingTimeout(t *testing.T) {
	bus := NewEventBus(1)
	ch := bus.Subscribe()

	for len(ch) < cap(ch) {
		bus.Publish(Event{Type: EventMysisCreated})
	}

	start := time.Now()
	delivered := bus.PublishBlocking(Event{Type: EventMysisDeleted}, 20*time.Millisecond)
	if delivered {
		t.Fatal("expected PublishBlocking to timeout")
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("expected PublishBlocking to wait for timeout")
	}

	<-ch
	bus.Close()
}

func TestEventBusClose(t *testing.T) {
	bus := NewEventBus(10)

	ch1 := bus.Subscribe()
	ch2 := bus.Subscribe()

	bus.Close()

	// Both channels should be closed
	_, ok1 := <-ch1
	_, ok2 := <-ch2

	if ok1 || ok2 {
		t.Error("expected all channels to be closed")
	}
}
