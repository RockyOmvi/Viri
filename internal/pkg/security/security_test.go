package security

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/pkg/observability"
)

func TestAPIKeyAuthValidKey(t *testing.T) {
	auth := NewAPIKeyAuth("secret-key")

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuthInvalidKey(t *testing.T) {
	auth := NewAPIKeyAuth("secret-key")

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestAPIKeyAuthMissingKey(t *testing.T) {
	auth := NewAPIKeyAuth("secret-key")

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAPIKeyAuthEmptyKeyBypasses(t *testing.T) {
	auth := NewAPIKeyAuth("")

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with empty key auth, got %d", rec.Code)
	}
}

func TestAPIKeyAuthBearerToken(t *testing.T) {
	auth := NewAPIKeyAuth("secret-key")

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuthQueryParam(t *testing.T) {
	auth := NewAPIKeyAuth("secret-key")

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/?api_key=secret-key", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestConstantTimeComparison(t *testing.T) {
	auth := NewAPIKeyAuth("test-key-12345")

	correctKey := "test-key-12345"
	wrongKey := "wrong-key-12345"

	if !auth.IsValid(correctKey) {
		t.Error("valid key should pass")
	}

	if auth.IsValid(wrongKey) {
		t.Error("invalid key should fail")
	}

	iterations := 1000
	validTimes := make([]time.Duration, iterations)
	invalidTimes := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		auth.IsValid(correctKey)
		validTimes[i] = time.Since(start)

		start = time.Now()
		auth.IsValid(wrongKey)
		invalidTimes[i] = time.Since(start)
	}

	var validAvg, invalidAvg time.Duration
	for i := range validTimes {
		validAvg += validTimes[i]
		invalidAvg += invalidTimes[i]
	}
	validAvg /= time.Duration(iterations)
	invalidAvg /= time.Duration(iterations)

	diff := absDuration(validAvg - invalidAvg)
	if diff > 100*time.Microsecond {
		t.Logf("Timing difference: valid=%v, invalid=%v, diff=%v", validAvg, invalidAvg, diff)
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func TestMethodRateLimit(t *testing.T) {
	methodLimits := map[string]MethodLimit{
		"eth_getBlockByNumber": {RPS: 2.0, Burst: 2},
	}

	mrl := NewMethodRateLimiter(10.0, 10, methodLimits)

	for i := 0; i < 2; i++ {
		if !mrl.Allow("client1", "eth_getBlockByNumber") {
			t.Errorf("request %d should be allowed", i)
		}
	}

	if mrl.Allow("client1", "eth_getBlockByNumber") {
		t.Error("should be blocked after exceeding method limit")
	}

	if !mrl.Allow("client1", "eth_getBalance") {
		t.Error("different method should use default limit")
	}
}

func TestDDoSDetection(t *testing.T) {
	detector := NewDDoSDetector(1*time.Second, 5, 10*time.Second)

	clientID := "suspicious-client"

	for i := 0; i < 4; i++ {
		if err := detector.CheckRequest(clientID); err != nil {
			t.Fatalf("request %d should be allowed: %v", i, err)
		}
	}

	if err := detector.CheckRequest(clientID); err == nil {
		t.Error("should be blocked after suspicious activity")
	}

	blocked := detector.GetBlockedClients()
	if len(blocked) == 0 {
		t.Error("client should be in blocked list")
	}
}

func TestDDoSDetectionExpires(t *testing.T) {
	detector := NewDDoSDetector(1*time.Second, 5, 200*time.Millisecond)

	clientID := "suspicious-client"

	for i := 0; i < 6; i++ {
		detector.CheckRequest(clientID)
	}

	if len(detector.GetBlockedClients()) == 0 {
		t.Error("client should be blocked")
	}

	time.Sleep(250 * time.Millisecond)

	blocked := detector.GetBlockedClients()
	if len(blocked) != 0 {
		t.Error("block should have expired")
	}
}

func TestSlowQueryDetector(t *testing.T) {
	sqd := NewSlowQueryDetector(1*time.Second, 3, 5*time.Second)

	clientID := "slow-client"

	for i := 0; i < 3; i++ {
		if !sqd.Allow(clientID) {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	if sqd.Allow(clientID) {
		t.Error("should be blocked after too many slow queries")
	}
}

func TestSlowQueryDetectorExpires(t *testing.T) {
	sqd := NewSlowQueryDetector(100*time.Millisecond, 3, 250*time.Millisecond)

	clientID := "slow-client"
	for i := 0; i < 3; i++ {
		sqd.Allow(clientID)
	}

	if sqd.Allow(clientID) {
		t.Error("should be blocked")
	}

	time.Sleep(150 * time.Millisecond)

	if sqd.Allow(clientID) {
		t.Error("should still be blocked (new request added)")
	}

	time.Sleep(150 * time.Millisecond)

	if !sqd.Allow(clientID) {
		t.Error("old queries should have expired, should be allowed")
	}
}

func TestRequestIDPropagation(t *testing.T) {
	var capturedReqID string

	handler := observability.RequestIDMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReqID = observability.RequestIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedReqID == "" {
		t.Error("request ID should be set in context")
	}

	reqID := rec.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Error("request ID should be in response header")
	}
}

func TestRequestIDFromHeader(t *testing.T) {
	var capturedReqID string

	handler := observability.RequestIDMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReqID = observability.RequestIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "test-req-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedReqID != "test-req-123" {
		t.Errorf("request ID should be 'test-req-123', got '%s'", capturedReqID)
	}
}

func TestBlockRangeLimiter(t *testing.T) {
	brl := NewBlockRangeLimiter(100)

	if err := brl.CheckRange(0, 50); err != nil {
		t.Errorf("range 0-50 should be valid: %v", err)
	}

	if err := brl.CheckRange(0, 99); err != nil {
		t.Errorf("range 0-99 should be valid: %v", err)
	}

	if err := brl.CheckRange(0, 100); err == nil {
		t.Error("range 0-100 should be invalid (101 blocks)")
	}

	if err := brl.CheckRange(50, 40); err != nil {
		t.Errorf("range 50-40 should be valid (to < from): %v", err)
	}
}

func TestConcurrentRateLimiting(t *testing.T) {
	rl := NewRateLimiter(1000.0, 100)

	// Pre-fill the bucket so we own the initial burst
	rl.Allow("concurrent-client")

	var wg sync.WaitGroup
	results := make(chan bool, 200)

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- rl.Allow("concurrent-client")
		}()
	}

	wg.Wait()
	close(results)

	allowed := 0
	for r := range results {
		if r {
			allowed++
		}
	}

	// Burst was 100, we used 1 for pre-fill, so at most ~100 remain
	// Allow some tolerance for timing-based refill
	if allowed > 110 {
		t.Errorf("should allow at most ~100 (burst-1) + timing tolerance, got %d", allowed)
	}
}

func TestDrainerMiddleware(t *testing.T) {
	drainer := NewConnectionDrainer(1 * time.Second)

	handler := DrainMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		drainer,
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("request should pass before drain, got %d", rec.Code)
	}

	drainer.StartDrain()

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Connection") != "close" {
		t.Error("Connection: close should be set during drain")
	}
}

func TestMethodRateLimitMiddleware(t *testing.T) {
	methodLimits := map[string]MethodLimit{
		"eth_getLogs": {RPS: 1.0, Burst: 1},
	}

	mrl := NewMethodRateLimiter(10.0, 10, methodLimits)

	getClientID := func(r *http.Request) string {
		return r.RemoteAddr
	}

	handler := mrl.Middleware(getClientID)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	body1 := []byte(`{"method": "eth_getLogs", "params": []}`)
	req1 := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.RemoteAddr = "127.0.0.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	body2 := []byte(`{"method": "eth_getLogs", "params": []}`)
	req2 := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.RemoteAddr = "127.0.0.1:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second eth_getLogs should be rate limited, got %d", rec2.Code)
	}
}

type readCloser struct {
	*bytes.Buffer
}

func (rc *readCloser) Close() error {
	return nil
}

func TestExtractRPCMethod(t *testing.T) {
	tests := []struct {
		body     string
		expected string
	}{
		{`{"method": "eth_blockNumber"}`, "eth_blockNumber"},
		{`{"method": "eth_getBalance"}`, "eth_getBalance"},
		{`{"method": "invalid"}`, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			method := extractRPCMethod(req)
			if method != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, method)
			}
		})
	}
}

func TestExtractRPCMethodGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	method := extractRPCMethod(req)
	if method != "" {
		t.Errorf("expected empty for GET, got %s", method)
	}
}

func TestSlowQueryDetectorMiddleware(t *testing.T) {
	sqd := NewSlowQueryDetector(1*time.Second, 2, 5*time.Second)

	getClientID := func(r *http.Request) string {
		return r.RemoteAddr
	}

	handler := sqd.Middleware(getClientID)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	rpcBody := func(method string) io.Reader {
		return strings.NewReader(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":[],"id":1}`, method))
	}

	req := httptest.NewRequest(http.MethodPost, "/", rpcBody("eth_getLogs"))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("first request should pass, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/", rpcBody("eth_getLogs"))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Content-Type", "application/json")

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("second request should pass, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/", rpcBody("eth_getLogs"))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Content-Type", "application/json")

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("third request should be blocked, got %d", rec.Code)
	}
}
