package watcher

import (
	"sync"
)

// EventBus is a high-speed, typed pub/sub system for internal application communication.
// It's designed to be even faster than filesystem events for in-app messaging.
type EventBus struct {
	mu       sync.RWMutex
	channels map[string][]chan interface{}
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		channels: make(map[string][]chan interface{}),
	}
}

// Subscribe creates a new channel for a specific topic.
// The caller should read from the returned channel and close it when done.
func (eb *EventBus) Subscribe(topic string) <-chan interface{} {
	ch := make(chan interface{}, 100) // Buffered to avoid blocking publishers

	eb.mu.Lock()
	eb.channels[topic] = append(eb.channels[topic], ch)
	eb.mu.Unlock()

	return ch
}

// Unsubscribe removes a channel from a topic.
func (eb *EventBus) Unsubscribe(topic string, ch <-chan interface{}) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subs := eb.channels[topic]
	for i, sub := range subs {
		// Compare addresses
		if sub == ch {
			eb.channels[topic] = append(subs[:i], subs[i+1:]...)
			close(sub)
			return
		}
	}
}

// Publish sends data to all subscribers of a topic.
// Non-blocking: if a subscriber's channel is full, the message is dropped for that subscriber.
func (eb *EventBus) Publish(topic string, data interface{}) {
	eb.mu.RLock()
	subs := eb.channels[topic]
	eb.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- data:
		default:
			// Channel full, skip to avoid blocking
		}
	}
}

// PublishSync sends data and blocks until all subscribers have received it.
func (eb *EventBus) PublishSync(topic string, data interface{}) {
	eb.mu.RLock()
	subs := eb.channels[topic]
	eb.mu.RUnlock()

	var wg sync.WaitGroup
	for _, ch := range subs {
		wg.Add(1)
		go func(c chan interface{}) {
			defer wg.Done()
			c <- data
		}(ch)
	}
	wg.Wait()
}

// Topics for common events
const (
	TopicFileChange   = "file:change"
	TopicTreeReload   = "tree:reload"
	TopicCacheInvalid = "cache:invalidate"
	TopicConfigChange = "config:change"
	TopicToolExecuted = "tool:executed"
)
