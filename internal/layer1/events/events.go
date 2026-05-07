package events

import (
	"sync"
	"time"
)

type EventType string

const (
	EventBlockAdded      EventType = "block.added"
	EventBlockRemoved    EventType = "block.removed"
	EventTxAdded         EventType = "tx.added"
	EventTxConfirmed     EventType = "tx.confirmed"
	EventTxRejected      EventType = "tx.rejected"
	EventPeerConnected   EventType = "peer.connected"
	EventPeerDisconnected EventType = "peer.disconnected"
	EventChainReorg      EventType = "chain.reorg"
	EventValidatorAdded  EventType = "validator.added"
	EventValidatorSlashed EventType = "validator.slashed"
	EventFinality        EventType = "finality.reached"
)

type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      interface{}
	Source    string
}

type EventHandler func(event Event)

type EventBus struct {
	mu         sync.RWMutex
	handlers   map[EventType][]EventHandler
	allHandlers []EventHandler
	bufferSize int
}

func NewEventBus(bufferSize int) *EventBus {
	return &EventBus{
		handlers:   make(map[EventType][]EventHandler),
		bufferSize: bufferSize,
	}
}

func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

func (eb *EventBus) SubscribeAll(handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.allHandlers = append(eb.allHandlers, handler)
}

func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if handlers, exists := eb.handlers[event.Type]; exists {
		for _, handler := range handlers {
			go handler(event)
		}
	}

	for _, handler := range eb.allHandlers {
		go handler(event)
	}
}

func (eb *EventBus) PublishSync(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if handlers, exists := eb.handlers[event.Type]; exists {
		for _, handler := range handlers {
			handler(event)
		}
	}

	for _, handler := range eb.allHandlers {
		handler(event)
	}
}

func (eb *EventBus) Unsubscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	handlers := eb.handlers[eventType]
	for i, h := range handlers {
		if &h == &handler {
			eb.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

func (eb *EventBus) HandlerCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.handlers[eventType])
}

func (eb *EventBus) Clear() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers = make(map[EventType][]EventHandler)
	eb.allHandlers = nil
}

func NewBlockEvent(block interface{}, source string) Event {
	return Event{
		Type:   EventBlockAdded,
		Data:   block,
		Source: source,
	}
}

func NewTxEvent(tx interface{}, source string) Event {
	return Event{
		Type:   EventTxAdded,
		Data:   tx,
		Source: source,
	}
}

func NewPeerEvent(peerID string, connected bool) Event {
	eventType := EventPeerDisconnected
	if connected {
		eventType = EventPeerConnected
	}

	return Event{
		Type:   eventType,
		Data:   peerID,
		Source: "network",
	}
}
