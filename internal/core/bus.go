package core

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/zoea-nova/internal/constants"
)

const dropLogEvery = 100

type subscriber struct {
	mu      sync.RWMutex
	ch      chan Event
	dropped atomic.Uint64
	closed  bool
}

func (s *subscriber) send(event Event, timeout time.Duration) (bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return false, false
	}

	if timeout <= 0 {
		select {
		case s.ch <- event:
			return true, false
		default:
			return false, false
		}
	}

	timer := time.NewTimer(timeout)
	select {
	case s.ch <- event:
		if !timer.Stop() {
			<-timer.C
		}
		return true, false
	case <-timer.C:
		return false, true
	}
}

func (s *subscriber) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.ch)
	s.mu.Unlock()
}

// EventBus distributes events to subscribers.
type EventBus struct {
	mu          sync.RWMutex
	subscribers []*subscriber
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

	sub := &subscriber{ch: make(chan Event, b.bufferSize)}
	b.subscribers = append(b.subscribers, sub)
	return sub.ch
}

// Unsubscribe removes a subscriber channel.
func (b *EventBus) Unsubscribe(ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscribers {
		if sub.ch == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			sub.close()
			return
		}
	}
}

// Publish sends an event to all subscribers.
// Non-blocking: drops events if a subscriber's buffer is full.
func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	subscribers := append([]*subscriber(nil), b.subscribers...)
	b.mu.RUnlock()

	for i, sub := range subscribers {
		if delivered, _ := sub.send(event, 0); !delivered {
			dropped := sub.dropped.Add(1)
			if dropped%dropLogEvery == 0 {
				log.Warn().
					Str("event_type", string(event.Type)).
					Uint64("dropped", dropped).
					Int("subscriber_index", i).
					Msg("event bus subscriber dropped events")
			}
		}
	}
}

// PublishBlocking sends an event to all subscribers, waiting up to timeout per subscriber.
// Returns true if all subscribers received the event.
func (b *EventBus) PublishBlocking(event Event, timeout time.Duration) bool {
	if timeout <= 0 {
		b.mu.RLock()
		subscribers := append([]*subscriber(nil), b.subscribers...)
		b.mu.RUnlock()

		allDelivered := true
		for i, sub := range subscribers {
			if delivered, _ := sub.send(event, 0); !delivered {
				allDelivered = false
				dropped := sub.dropped.Add(1)
				if dropped%dropLogEvery == 0 {
					log.Warn().
						Str("event_type", string(event.Type)).
						Uint64("dropped", dropped).
						Int("subscriber_index", i).
						Msg("event bus subscriber dropped events")
				}
			}
		}

		return allDelivered
	}

	b.mu.RLock()
	subscribers := append([]*subscriber(nil), b.subscribers...)
	b.mu.RUnlock()

	allDelivered := true
	for i, sub := range subscribers {
		delivered, timedOut := sub.send(event, timeout)
		if delivered {
			continue
		}
		allDelivered = false
		dropped := sub.dropped.Add(1)
		if dropped%dropLogEvery == 0 {
			msg := "event bus subscriber dropped events"
			if timedOut {
				msg = "event bus subscriber timed out"
			}
			log.Warn().
				Str("event_type", string(event.Type)).
				Uint64("dropped", dropped).
				Int("subscriber_index", i).
				Msg(msg)
		}
	}

	return allDelivered
}

// Close closes all subscriber channels.
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscribers {
		ch.close()
	}
	b.subscribers = nil
}
