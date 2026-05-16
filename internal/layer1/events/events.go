package events

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const maxConcurrentHandlers = 100

type EventType string

const (
	EventBlockAdded       EventType = "block.added"
	EventBlockRemoved     EventType = "block.removed"
	EventTxAdded          EventType = "tx.added"
	EventTxConfirmed      EventType = "tx.confirmed"
	EventTxRejected       EventType = "tx.rejected"
	EventPeerConnected    EventType = "peer.connected"
	EventPeerDisconnected EventType = "peer.disconnected"
	EventChainReorg       EventType = "chain.reorg"
	EventValidatorAdded   EventType = "validator.added"
	EventValidatorSlashed EventType = "validator.slashed"
	EventFinality         EventType = "finality.reached"
)

type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      interface{}
	Source    string
}

type EventHandler func(event Event)

type handlerEntry struct {
	id  uint64
	fn  EventHandler
}

type EventBus struct {
	mu          sync.RWMutex
	handlers    map[EventType][]handlerEntry
	allHandlers []handlerEntry
	nextID      uint64
	wg          sync.WaitGroup
	sem         chan struct{}
	shutdown    bool
}

func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]handlerEntry),
		sem:      make(chan struct{}, maxConcurrentHandlers),
	}
}

func (eb *EventBus) init() {
	eb.mu.Lock()
	if eb.handlers == nil {
		eb.handlers = make(map[EventType][]handlerEntry)
	}
	if eb.sem == nil {
		eb.sem = make(chan struct{}, maxConcurrentHandlers)
	}
	eb.mu.Unlock()
}

func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) uint64 {
	eb.init()
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := atomic.AddUint64(&eb.nextID, 1)
	eb.handlers[eventType] = append(eb.handlers[eventType], handlerEntry{id: id, fn: handler})
	return id
}

func (eb *EventBus) SubscribeAll(handler EventHandler) uint64 {
	eb.init()
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := atomic.AddUint64(&eb.nextID, 1)
	eb.allHandlers = append(eb.allHandlers, handlerEntry{id: id, fn: handler})
	return id
}

func (eb *EventBus) Publish(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	eb.mu.RLock()
	if eb.shutdown {
		eb.mu.RUnlock()
		return
	}
	typed := make([]EventHandler, len(eb.handlers[event.Type]))
	for i, h := range eb.handlers[event.Type] {
		typed[i] = h.fn
	}
	all := make([]EventHandler, len(eb.allHandlers))
	for i, h := range eb.allHandlers {
		all[i] = h.fn
	}
	totalHandlers := len(typed) + len(all)
	eb.wg.Add(totalHandlers)
	eb.mu.RUnlock()

	dispatch := func(h EventHandler) {
		eb.sem <- struct{}{}
		go func() {
			defer func() {
				<-eb.sem
				eb.wg.Done()
			}()
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "events: panic in handler: %v\n", r)
				}
			}()
			h(event)
		}()
	}

	for _, handler := range typed {
		dispatch(handler)
	}
	for _, handler := range all {
		dispatch(handler)
	}
}

func (eb *EventBus) PublishSync(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	eb.mu.RLock()
	if eb.shutdown {
		eb.mu.RUnlock()
		return
	}
	typed := make([]EventHandler, len(eb.handlers[event.Type]))
	for i, h := range eb.handlers[event.Type] {
		typed[i] = h.fn
	}
	all := make([]EventHandler, len(eb.allHandlers))
	for i, h := range eb.allHandlers {
		all[i] = h.fn
	}
	eb.mu.RUnlock()

	for _, handler := range typed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "events: panic in handler: %v\n", r)
				}
			}()
			handler(event)
		}()
	}
	for _, handler := range all {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "events: panic in handler: %v\n", r)
				}
			}()
			handler(event)
		}()
	}
}

func (eb *EventBus) Unsubscribe(eventType EventType, id uint64) {
	eb.init()
	eb.mu.Lock()
	defer eb.mu.Unlock()

	handlers := eb.handlers[eventType]
	for i, h := range handlers {
		if h.id == id {
			eb.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

func (eb *EventBus) UnsubscribeAll(id uint64) {
	eb.init()
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for i, h := range eb.allHandlers {
		if h.id == id {
			eb.allHandlers = append(eb.allHandlers[:i], eb.allHandlers[i+1:]...)
			break
		}
	}
}

func (eb *EventBus) HandlerCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.handlers == nil {
		return 0
	}
	return len(eb.handlers[eventType])
}

func (eb *EventBus) Clear() {
	eb.init()
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers = make(map[EventType][]handlerEntry)
	eb.allHandlers = nil
}

func (eb *EventBus) Shutdown() {
	eb.mu.Lock()
	eb.shutdown = true
	eb.mu.Unlock()
	eb.wg.Wait()
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
