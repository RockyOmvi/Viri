package events

import (
	"sync"
	"testing"
	"time"
)

func TestEventBusPublishSubscribe(t *testing.T) {
	bus := NewEventBus(100)

	var received Event
	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe(EventBlockAdded, func(event Event) {
		received = event
		wg.Done()
	})

	bus.Publish(Event{
		Type: EventBlockAdded,
		Data: "test-block",
	})

	wg.Wait()

	if received.Type != EventBlockAdded {
		t.Errorf("Expected event type %s, got %s", EventBlockAdded, received.Type)
	}

	if received.Data != "test-block" {
		t.Errorf("Expected data test-block, got %v", received.Data)
	}
}

func TestEventBusSubscribeAll(t *testing.T) {
	bus := NewEventBus(100)

	count := 0
	var mu sync.Mutex

	bus.SubscribeAll(func(event Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Publish(Event{Type: EventBlockAdded})
	bus.Publish(Event{Type: EventTxAdded})
	bus.Publish(Event{Type: EventPeerConnected})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if count != 3 {
		t.Errorf("Expected 3 events received, got %d", count)
	}
}

func TestEventBusMultipleHandlers(t *testing.T) {
	bus := NewEventBus(100)

	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		bus.Subscribe(EventBlockAdded, func(event Event) {
			wg.Done()
		})
	}

	bus.Publish(Event{Type: EventBlockAdded})
	wg.Wait()
}

func TestEventBusHandlerCount(t *testing.T) {
	bus := NewEventBus(100)

	if bus.HandlerCount(EventBlockAdded) != 0 {
		t.Error("Expected 0 handlers initially")
	}

	bus.Subscribe(EventBlockAdded, func(event Event) {})
	bus.Subscribe(EventBlockAdded, func(event Event) {})

	if bus.HandlerCount(EventBlockAdded) != 2 {
		t.Errorf("Expected 2 handlers, got %d", bus.HandlerCount(EventBlockAdded))
	}
}

func TestEventBusClear(t *testing.T) {
	bus := NewEventBus(100)

	bus.Subscribe(EventBlockAdded, func(event Event) {})
	bus.Subscribe(EventTxAdded, func(event Event) {})

	bus.Clear()

	if bus.HandlerCount(EventBlockAdded) != 0 {
		t.Error("Expected 0 handlers after clear")
	}
}

func TestEventFactory(t *testing.T) {
	blockEvent := NewBlockEvent("block-data", "test-source")

	if blockEvent.Type != EventBlockAdded {
		t.Errorf("Expected event type %s, got %s", EventBlockAdded, blockEvent.Type)
	}

	if blockEvent.Data != "block-data" {
		t.Errorf("Expected data block-data, got %v", blockEvent.Data)
	}

	txEvent := NewTxEvent("tx-data", "test-source")
	if txEvent.Type != EventTxAdded {
		t.Errorf("Expected event type %s, got %s", EventTxAdded, txEvent.Type)
	}

	peerEvent := NewPeerEvent("peer-123", true)
	if peerEvent.Type != EventPeerConnected {
		t.Errorf("Expected event type %s, got %s", EventPeerConnected, peerEvent.Type)
	}

	peerEvent2 := NewPeerEvent("peer-123", false)
	if peerEvent2.Type != EventPeerDisconnected {
		t.Errorf("Expected event type %s, got %s", EventPeerDisconnected, peerEvent2.Type)
	}
}

func TestEventBusPublishSync(t *testing.T) {
	bus := NewEventBus(100)

	count := 0
	bus.Subscribe(EventBlockAdded, func(event Event) {
		count++
	})

	bus.PublishSync(Event{Type: EventBlockAdded})

	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}
}
