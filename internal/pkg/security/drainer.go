package security

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ConnectionDrainer struct {
	activeConnections sync.WaitGroup
	draining          atomic.Bool
	timeout           time.Duration
	mu                sync.Mutex
	connMap           map[string]bool
}

func NewConnectionDrainer(timeout time.Duration) *ConnectionDrainer {
	return &ConnectionDrainer{
		timeout: timeout,
		connMap: make(map[string]bool),
	}
}

func (d *ConnectionDrainer) Track(connID string) {
	d.mu.Lock()
	d.connMap[connID] = true
	d.mu.Unlock()
	d.activeConnections.Add(1)
}

func (d *ConnectionDrainer) Release(connID string) {
	d.mu.Lock()
	delete(d.connMap, connID)
	d.mu.Unlock()
	d.activeConnections.Done()
}

func (d *ConnectionDrainer) StartDrain() {
	d.draining.Store(true)
}

func (d *ConnectionDrainer) IsDraining() bool {
	return d.draining.Load()
}

func (d *ConnectionDrainer) Wait() error {
	done := make(chan struct{})
	go func() {
		d.activeConnections.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(d.timeout):
		return context.DeadlineExceeded
	}
}

func DrainMiddleware(next http.Handler, drainer *ConnectionDrainer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if drainer.IsDraining() {
			w.Header().Set("Connection", "close")
		}

		connID := r.RemoteAddr + ":" + r.URL.Path
		drainer.Track(connID)
		defer drainer.Release(connID)

		next.ServeHTTP(w, r)
	})
}
