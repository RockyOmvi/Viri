package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestConnectionDrainer_TrackAndRelease(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	drainer.Track("conn-1")
	drainer.Track("conn-2")

	drainer.Release("conn-1")
	drainer.Release("conn-2")
}

func TestConnectionDrainer_StartDrain(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	if drainer.IsDraining() {
		t.Error("drainer should not be draining initially")
	}

	drainer.StartDrain()

	if !drainer.IsDraining() {
		t.Error("drainer should be draining after StartDrain")
	}
}

func TestConnectionDrainer_WaitReturnsAfterAllReleased(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		connID := string(rune('A' + i))
		drainer.Track(connID)

		go func(id string) {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			drainer.Release(id)
		}(connID)
	}

	err := drainer.Wait()
	if err != nil {
		t.Errorf("Wait should return nil after all connections released, got %v", err)
	}

	wg.Wait()
}

func TestConnectionDrainer_WaitTimesOut(t *testing.T) {
	drainer := NewConnectionDrainer(100 * time.Millisecond)

	drainer.Track("conn-1")
	drainer.Track("conn-2")

	err := drainer.Wait()
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}

	drainer.Release("conn-1")
	drainer.Release("conn-2")
}

func TestConnectionDrainer_IsDrainingAfterStartDrain(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	drainer.StartDrain()

	if !drainer.IsDraining() {
		t.Error("IsDraining should return true after StartDrain")
	}
}

func TestConnectionDrainer_MultipleTrackSameConnection(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	drainer.Track("conn-1")
	drainer.Track("conn-1")

	drainer.Release("conn-1")
	drainer.Release("conn-1")

	if drainer.IsDraining() {
		t.Error("drainer should not be draining")
	}
}

func TestConnectionDrainer_TrackAfterStartDrain(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	drainer.StartDrain()

	drainer.Track("conn-1")
	drainer.Release("conn-1")

	if !drainer.IsDraining() {
		t.Error("IsDraining should still be true")
	}
}

func TestConnectionDrainer_WaitWithoutDraining(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	drainer.Track("conn-1")
	drainer.Release("conn-1")

	err := drainer.Wait()
	if err != nil {
		t.Errorf("Wait should return nil when not draining, got %v", err)
	}
}

func TestConnectionDrainer_ConcurrentAccess(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			connID := string(rune('A' + id))
			drainer.Track(connID)
			time.Sleep(10 * time.Millisecond)
			drainer.Release(connID)
		}(i)
	}

	wg.Wait()
}

func TestConnectionDrainer_ZeroTimeout(t *testing.T) {
	drainer := NewConnectionDrainer(0)

	drainer.Track("conn-1")

	err := drainer.Wait()
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded for zero timeout, got %v", err)
	}

	drainer.Release("conn-1")
}

func TestDrainMiddleware(t *testing.T) {
	drainer := NewConnectionDrainer(5 * time.Second)

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(200)
	})

	middleware := DrainMiddleware(next, drainer)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should have been called")
	}

	if w.Header().Get("Connection") != "" {
		t.Error("Connection header should not be set when not draining")
	}

	drainer.StartDrain()

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "127.0.0.1:12346"
	w2 := httptest.NewRecorder()

	middleware.ServeHTTP(w2, req2)

	if w2.Header().Get("Connection") != "close" {
		t.Error("Connection header should be 'close' when draining")
	}
}
