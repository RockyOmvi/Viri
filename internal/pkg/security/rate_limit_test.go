package security

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(10.0, 5)

	// Should allow up to burst size
	for i := 0; i < 5; i++ {
		if !rl.Allow("client1") {
			t.Errorf("Request %d should be allowed within burst", i)
		}
	}

	// Next request should be rejected
	if rl.Allow("client1") {
		t.Error("Request should be rejected after burst limit")
	}

	// Different client should have separate limits
	if !rl.Allow("client2") {
		t.Error("Different client should have separate limit")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(2.0, 2)

	// Use up burst
	rl.Allow("client1")
	rl.Allow("client1")

	// Should not allow more
	if rl.Allow("client1") {
		t.Error("Should be at limit")
	}

	// Wait for token refill
	time.Sleep(600 * time.Millisecond)

	// Should now allow one more request
	if !rl.Allow("client1") {
		t.Error("Should allow after refill")
	}
}

func TestConnectionLimiterAcquireRelease(t *testing.T) {
	cl := NewConnectionLimiter(5, 2)

	// Test global limit
	for i := 0; i < 5; i++ {
		clientID := fmt.Sprintf("global_client%d", i)
		if !cl.AcquireConnection(clientID) {
			t.Errorf("Should be able to acquire connection %d", i)
		}
	}

	// Should reject exceeding global limit
	if cl.AcquireConnection("client2") {
		t.Error("Should reject connection exceeding global limit")
	}

	// Test client limit
	cl2 := NewConnectionLimiter(10, 3)
	for i := 0; i < 3; i++ {
		if !cl2.AcquireConnection("client1") {
			t.Errorf("Should be able to acquire client connection %d", i)
		}
	}

	// Should reject exceeding per-client limit
	if cl2.AcquireConnection("client1") {
		t.Error("Should reject connection exceeding per-client limit")
	}

	// Release and verify
	cl2.ReleaseConnection("client1")
	if !cl2.AcquireConnection("client1") {
		t.Error("Should be able to acquire after release")
	}
}

func TestConnectionLimiterStats(t *testing.T) {
	cl := NewConnectionLimiter(10, 5)

	current, max := cl.GetStats()
	if current != 0 || max != 10 {
		t.Errorf("Expected (0, 10), got (%d, %d)", current, max)
	}

	cl.AcquireConnection("client1")
	cl.AcquireConnection("client2")

	current, max = cl.GetStats()
	if current != 2 {
		t.Errorf("Expected current=2, got %d", current)
	}
}

func TestDDoSDetectorCheckRequest(t *testing.T) {
	dd := NewDDoSDetector(1*time.Second, 5, 10*time.Second)

	clientID := "attacker"

	// Allow up to the threshold
	for i := 0; i < 4; i++ {
		err := dd.CheckRequest(clientID)
		if err != nil {
			t.Errorf("Request %d should be allowed: %v", i, err)
		}
	}

	// Next request should trigger block
	err := dd.CheckRequest(clientID)
	if err == nil {
		t.Error("Request should be blocked after exceeding threshold")
	}

	// Verify client is blocked
	blocked := dd.GetBlockedClients()
	if len(blocked) == 0 {
		t.Error("Client should be in blocked list")
	}

	// Wait for block to expire
	time.Sleep(11 * time.Second)

	// Should be unblocked now
	err = dd.CheckRequest(clientID)
	if err != nil {
		t.Errorf("Should be unblocked after duration: %v", err)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(2.0, 2)

	getClientID := func(r *http.Request) string {
		return r.Header.Get("X-Client-ID")
	}

	handler := RateLimitMiddleware(rl, getClientID)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}),
	)

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Client-ID", "client1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got status %d", i, w.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Client-ID", "client1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}
}

func TestDDoSProtectionMiddleware(t *testing.T) {
	dd := NewDDoSDetector(1*time.Second, 3, 5*time.Second)

	getClientID := func(r *http.Request) string {
		return r.Header.Get("X-Client-ID")
	}

	handler := DDoSProtectionMiddleware(dd, getClientID)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}),
	)

	// Allow requests up to threshold
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Client-ID", "attacker")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got status %d", i, w.Code)
		}
	}

	// Next request should trigger block
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Client-ID", "attacker")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestConnectionLimitMiddleware(t *testing.T) {
	cl := NewConnectionLimiter(2, 2)

	getClientID := func(r *http.Request) string {
		return r.Header.Get("X-Client-ID")
	}

	handler := ConnectionLimitMiddleware(cl, getClientID)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}),
	)

	// Two concurrent requests should succeed
	done := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Client-ID", "client1")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Request %d should be allowed, got status %d", i, w.Code)
			}
			done <- true
		}(i)
	}

	// Third request should be rejected due to limit, send it immediately
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Client-ID", "client3")
	w := httptest.NewRecorder()
	
	// Small delay to ensure the first two have acquired their connections
	time.Sleep(10 * time.Millisecond)
	handler.ServeHTTP(w, req)

	// Wait for requests to complete
	<-done
	<-done

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503, got %d", w.Code)
	}
}

func BenchmarkRateLimiterAllow(b *testing.B) {
	rl := NewRateLimiter(1000.0, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow("client1")
	}
}

func BenchmarkConnectionLimiterAcquire(b *testing.B) {
	cl := NewConnectionLimiter(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if cl.AcquireConnection("client1") {
			cl.ReleaseConnection("client1")
		}
	}
}

func BenchmarkDDoSDetectorCheck(b *testing.B) {
	dd := NewDDoSDetector(1*time.Second, 1000, 10*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dd.CheckRequest("client1")
	}
}
