package security

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

type MethodRateLimiter struct {
	mu            sync.RWMutex
	defaultLimiter *RateLimiter
	methodLimiters map[string]*RateLimiter
}

func NewMethodRateLimiter(defaultRPS float64, defaultBurst int, methodLimits map[string]MethodLimit) *MethodRateLimiter {
	mrl := &MethodRateLimiter{
		defaultLimiter: NewRateLimiter(defaultRPS, defaultBurst),
		methodLimiters: make(map[string]*RateLimiter),
	}

	for method, limit := range methodLimits {
		mrl.methodLimiters[method] = NewRateLimiter(limit.RPS, limit.Burst)
	}

	return mrl
}

type MethodLimit struct {
	RPS   float64
	Burst int
}

func (mrl *MethodRateLimiter) Allow(clientID, method string) bool {
	mrl.mu.RLock()
	limiter, exists := mrl.methodLimiters[method]
	mrl.mu.RUnlock()

	if exists {
		if !limiter.Allow(clientID) {
			recordRateLimitHit(method)
			return false
		}
		return true
	}

	if !mrl.defaultLimiter.Allow(clientID) {
		recordRateLimitHit(method)
		return false
	}
	return true
}

func (mrl *MethodRateLimiter) Middleware(getClientID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)

			method := extractRPCMethod(r)
			if method != "" && !mrl.Allow(clientID, method) {
				recordThrottled("method")
				w.Header().Set("Retry-After", "30")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "method rate limit exceeded",
					"method":  method,
					"retry_after_seconds": 30,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractRPCMethod(r *http.Request) string {
	if r.Method != http.MethodPost {
		return ""
	}

	if r.URL.Path != "/" && r.URL.Path != "/rpc" {
		return ""
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	defer r.Body.Close()

	r.Body = io.NopCloser(bytes.NewReader(body))

	type rpcRequest struct {
		Method string `json:"method"`
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	return req.Method
}

type BlockRangeLimiter struct {
	maxRange uint64
}

func NewBlockRangeLimiter(maxRange uint64) *BlockRangeLimiter {
	return &BlockRangeLimiter{maxRange: maxRange}
}

func (brl *BlockRangeLimiter) CheckRange(from, to uint64) error {
	if to < from {
		return nil
	}
	if (to - from + 1) > brl.maxRange {
		return &RangeExceededError{Max: brl.maxRange, Requested: to - from + 1}
	}
	return nil
}

type RangeExceededError struct {
	Max       uint64
	Requested uint64
}

func (e *RangeExceededError) Error() string {
	return "block range exceeds maximum"
}

type SlowQueryDetector struct {
	mu            sync.Mutex
	clientQueries map[string][]time.Time
	window        time.Duration
	maxInWindow   int
	blockDuration time.Duration
	blocked       map[string]time.Time
}

func NewSlowQueryDetector(window time.Duration, maxInWindow int, blockDuration time.Duration) *SlowQueryDetector {
	return &SlowQueryDetector{
		clientQueries: make(map[string][]time.Time),
		window:        window,
		maxInWindow:   maxInWindow,
		blockDuration: blockDuration,
		blocked:       make(map[string]time.Time),
	}
}

func (sqd *SlowQueryDetector) Allow(clientID string) bool {
	sqd.mu.Lock()
	defer sqd.mu.Unlock()

	now := time.Now()

	if blockedUntil, exists := sqd.blocked[clientID]; exists {
		if now.Before(blockedUntil) {
			return false
		}
		delete(sqd.blocked, clientID)
	}

	times := sqd.clientQueries[clientID]
	windowStart := now.Add(-sqd.window)
	filtered := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(windowStart) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= sqd.maxInWindow {
		sqd.blocked[clientID] = now.Add(sqd.blockDuration)
		return false
	}

	filtered = append(filtered, now)
	sqd.clientQueries[clientID] = filtered

	return true
}

func (sqd *SlowQueryDetector) Middleware(getClientID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)
			method := extractRPCMethod(r)

			if method == "eth_getLogs" || method == "eth_getBlockByNumber" || method == "eth_getBlockRange" {
				if !sqd.Allow(clientID) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": "too many expensive queries",
					})
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
