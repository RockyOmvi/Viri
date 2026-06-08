package security

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/pkg/jsonrpc"
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
		return limiter.Allow(clientID)
	}

	return mrl.defaultLimiter.Allow(clientID)
}

func (mrl *MethodRateLimiter) Middleware(getClientID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)

			method, reqID := parseRPCRequest(r)
			if method != "" && !mrl.Allow(clientID, method) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusTooManyRequests)
				jsonrpc.WriteError(w, reqID, jsonrpc.CodeServerError, "Method rate limit exceeded", map[string]interface{}{
					"method":              method,
					"retry_after_seconds": 30,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractRPCMethod(r *http.Request) string {
	method, _ := parseRPCRequest(r)
	return method
}

func parseRPCRequest(r *http.Request) (method string, id interface{}) {
	if r.Method != http.MethodPost {
		return "", nil
	}

	if r.URL.Path != "/" && r.URL.Path != "/rpc" {
		return "", nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", nil
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	var req struct {
		Method string      `json:"method"`
		ID     interface{} `json:"id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", jsonrpc.RequestID(body)
	}

	return req.Method, req.ID
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
