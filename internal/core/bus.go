package core

import (
	"sync"

	"github.com/xonecas/zoea-nova/internal/constants"
)

// EventBus distributes events to subscribers.
type EventBus struct {
	mu          sync.RWMutex
	subscribers []chan Event
	bufferSize  int
}

// NewEventBus creates a new event bus.
func NewEventBus(bufferSize int) *EventBus {
	if bufferSize < constants.MinEventBusBufferSize {
		bufferSize = constants.MinEventBusBufferSize
	}
	return &EventBus{
		bufferSize: bufferSize,
	}
}

// Subscribe returns a channel that receives events.
// The caller is responsible for reading from the channel to avoid blocking.
func (b *EventBus) Subscribe() <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, b.bufferSize)
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel.
func (b *EventBus) Unsubscribe(ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscribers {
		if sub == ch {
			close(sub)
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			return
		}
	}
}

// Publish sends an event to all subscribers.
// Non-blocking: drops events if a subscriber's buffer is full.
func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Drop event if buffer is full (non-blocking)
		}
	}
}

// Close closes all subscriber channels.
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscribers {
		close(ch)
	}
	b.subscribers = nil
}
