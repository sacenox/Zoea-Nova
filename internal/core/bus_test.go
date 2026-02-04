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
		Type:      EventAgentCreated,
		AgentID:   "test-id",
		AgentName: "test-agent",
		Timestamp: time.Now(),
	}
	bus.Publish(event)

	// Receive event
	select {
	case received := <-ch:
		if received.Type != EventAgentCreated {
			t.Errorf("expected type=%s, got %s", EventAgentCreated, received.Type)
		}
		if received.AgentID != "test-id" {
			t.Errorf("expected agent_id=test-id, got %s", received.AgentID)
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
		Type:      EventAgentMessage,
		AgentID:   "agent-1",
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

	// Fill buffer
	bus.Publish(Event{Type: EventAgentCreated})

	// This should not block (event dropped)
	done := make(chan bool)
	go func() {
		bus.Publish(Event{Type: EventAgentDeleted})
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
