package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// MessageType for categorizing different types of messages
type MessageType int

const (
	MsgTypeGeneral MessageType = iota
	MsgTypeProposal
	MsgTypeVote
	MsgTypeBlockRequest
	MsgTypeConsensus
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mu              sync.RWMutex
	requestsPerSec  float64
	burst           int
	clientBuckets   map[string]*TokenBucket
	cleanupInterval time.Duration
	lastCleanup     time.Time
	peerData        map[peer.ID]*PeerRateData
	globalMsgs      map[string]time.Time
	globalMsgsMu    sync.Mutex
}

// TokenBucket represents a token bucket for a single client
type TokenBucket struct {
	tokens    float64
	capacity  float64
	refillRate float64
	lastRefill time.Time
}

// PeerRateData holds rate limiting and reputation data for a peer
type PeerRateData struct {
	generalBucket   *TokenBucket
	proposalBucket  *TokenBucket
	voteBucket     *TokenBucket
	blockReqBucket *TokenBucket
	reputation     int
	lastReputation time.Time
	banned         bool
	banEnd         time.Time
	mu             sync.Mutex
}

func newPeerRateData() *PeerRateData {
	return &PeerRateData{
		generalBucket:   &TokenBucket{tokens: 500, capacity: 500, refillRate: 500, lastRefill: time.Now()},
		proposalBucket:  &TokenBucket{tokens: 50, capacity: 50, refillRate: 50, lastRefill: time.Now()},
		voteBucket:      &TokenBucket{tokens: 200, capacity: 200, refillRate: 200, lastRefill: time.Now()},
		blockReqBucket:  &TokenBucket{tokens: 30, capacity: 30, refillRate: 30, lastRefill: time.Now()},
		reputation:      100,
		lastReputation:  time.Now(),
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSec float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		requestsPerSec:  requestsPerSec,
		burst:           burst,
		clientBuckets:   make(map[string]*TokenBucket),
		peerData:        make(map[peer.ID]*PeerRateData),
		cleanupInterval: 5 * time.Minute,
		lastCleanup:     time.Now(),
		globalMsgs:      make(map[string]time.Time),
	}
	return rl
}

// Allow checks if a request from the given client is allowed
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Cleanup old buckets periodically
	if time.Since(rl.lastCleanup) > rl.cleanupInterval {
		rl.cleanupOldBuckets()
		rl.lastCleanup = time.Now()
	}

	bucket, exists := rl.clientBuckets[clientID]
	if !exists {
		bucket = &TokenBucket{
			tokens:     float64(rl.burst),
			capacity:   float64(rl.burst),
			refillRate: rl.requestsPerSec,
			lastRefill: time.Now(),
		}
		rl.clientBuckets[clientID] = bucket
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens = min(bucket.capacity, bucket.tokens+elapsed*bucket.refillRate)
	bucket.lastRefill = now

	// Check if we have enough tokens
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// AllowPeer checks if a peer is allowed to send a message of a specific type
func (rl *RateLimiter) AllowPeer(peerID peer.ID, msgSize int, msgType MessageType, msgData []byte) (bool, string) {
	rl.mu.Lock()
	peerData, exists := rl.peerData[peerID]
	if !exists {
		peerData = newPeerRateData()
		rl.peerData[peerID] = peerData
	}
	rl.mu.Unlock()

	peerData.mu.Lock()
	defer peerData.mu.Unlock()

	now := time.Now()

	// Refill general bucket
	elapsed := now.Sub(peerData.generalBucket.lastRefill).Seconds()
	peerData.generalBucket.tokens = min(peerData.generalBucket.capacity, peerData.generalBucket.tokens+elapsed*peerData.generalBucket.refillRate)
	peerData.generalBucket.lastRefill = now

	if peerData.generalBucket.tokens < 1 {
		return false, "general rate limit exceeded"
	}
	peerData.generalBucket.tokens--

	// Check message type specific buckets
	switch msgType {
	case MsgTypeProposal:
		elapsed = now.Sub(peerData.proposalBucket.lastRefill).Seconds()
		peerData.proposalBucket.tokens = min(peerData.proposalBucket.capacity, peerData.proposalBucket.tokens+elapsed*peerData.proposalBucket.refillRate)
		peerData.proposalBucket.lastRefill = now
		if peerData.proposalBucket.tokens < 1 {
			return false, "proposal rate limit exceeded"
		}
		peerData.proposalBucket.tokens--
	case MsgTypeVote:
		elapsed = now.Sub(peerData.voteBucket.lastRefill).Seconds()
		peerData.voteBucket.tokens = min(peerData.voteBucket.capacity, peerData.voteBucket.tokens+elapsed*peerData.voteBucket.refillRate)
		peerData.voteBucket.lastRefill = now
		if peerData.voteBucket.tokens < 1 {
			return false, "vote rate limit exceeded"
		}
		peerData.voteBucket.tokens--
	case MsgTypeBlockRequest:
		elapsed = now.Sub(peerData.blockReqBucket.lastRefill).Seconds()
		peerData.blockReqBucket.tokens = min(peerData.blockReqBucket.capacity, peerData.blockReqBucket.tokens+elapsed*peerData.blockReqBucket.refillRate)
		peerData.blockReqBucket.lastRefill = now
		if peerData.blockReqBucket.tokens < 1 {
			return false, "block request rate limit exceeded"
		}
		peerData.blockReqBucket.tokens--
	}

	// Check for duplicate messages globally (not per-peer)
	if len(msgData) > 0 {
		hash := sha256.Sum256(msgData)
		hashStr := hex.EncodeToString(hash[:16]) // Use first 16 bytes for efficiency

		rl.globalMsgsMu.Lock()
		// Clean old entries periodically
		if len(rl.globalMsgs) > 1000 {
			for h, t := range rl.globalMsgs {
				if now.Sub(t) > 3*time.Second {
					delete(rl.globalMsgs, h)
				}
			}
		}

		if t, ok := rl.globalMsgs[hashStr]; ok && now.Sub(t) < 3*time.Second {
			rl.globalMsgsMu.Unlock()
			return true, "" // Allow duplicate silently — gossipsub naturally relays
		}
		rl.globalMsgs[hashStr] = now
		rl.globalMsgsMu.Unlock()
	}

	return true, ""
}

// IsBlocked checks if a peer is blocked
func (rl *RateLimiter) IsBlocked(peerID peer.ID) bool {
	rl.mu.RLock()
	peerData, exists := rl.peerData[peerID]
	rl.mu.RUnlock()

	if !exists {
		return false
	}

	peerData.mu.Lock()
	defer peerData.mu.Unlock()

	now := time.Now()

	// Update reputation over time
	elapsed := now.Sub(peerData.lastReputation).Seconds()
	peerData.reputation += int(elapsed * 0.1)
	if peerData.reputation > 100 {
		peerData.reputation = 100
	}
	peerData.lastReputation = now

	// Check if peer is banned
	if peerData.banned && now.Before(peerData.banEnd) {
		return true
	}

	// Auto-unban if reputation recovered
	if peerData.reputation >= 20 {
		peerData.banned = false
	}

	return peerData.banned
}

// ReportValidMsg reports a valid message from a peer (increases reputation)
func (rl *RateLimiter) ReportValidMsg(peerID peer.ID) {
	rl.mu.RLock()
	peerData, exists := rl.peerData[peerID]
	rl.mu.RUnlock()

	if !exists {
		return
	}

	peerData.mu.Lock()
	defer peerData.mu.Unlock()

	peerData.reputation++
	if peerData.reputation > 100 {
		peerData.reputation = 100
	}
}

// ReportInvalidMsg reports an invalid message from a peer (decreases reputation)
func (rl *RateLimiter) ReportInvalidMsg(peerID peer.ID) {
	rl.mu.RLock()
	peerData, exists := rl.peerData[peerID]
	rl.mu.RUnlock()

	if !exists {
		return
	}

	peerData.mu.Lock()
	defer peerData.mu.Unlock()

	peerData.reputation -= 5
	if peerData.reputation < 0 {
		peerData.reputation = 0
	}

	// Ban if reputation too low
	if peerData.reputation < 20 {
		peerData.banned = true
		peerData.banEnd = time.Now().Add(1 * time.Hour)
	}
}

// ReportTimeout reports a timeout from a peer
func (rl *RateLimiter) ReportTimeout(peerID peer.ID) {
	rl.mu.RLock()
	peerData, exists := rl.peerData[peerID]
	rl.mu.RUnlock()

	if !exists {
		return
	}

	peerData.mu.Lock()
	defer peerData.mu.Unlock()

	peerData.reputation -= 2
	if peerData.reputation < 0 {
		peerData.reputation = 0
	}
}

// cleanupOldBuckets removes buckets that haven't been used recently
func (rl *RateLimiter) cleanupOldBuckets() {
	maxAge := 1 * time.Hour
	now := time.Now()

	for clientID, bucket := range rl.clientBuckets {
		if now.Sub(bucket.lastRefill) > maxAge {
			delete(rl.clientBuckets, clientID)
		}
	}
}

// GetClientStats returns statistics for a specific client
func (rl *RateLimiter) GetClientStats(clientID string) (tokens float64, capacity float64, exists bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	bucket, exists := rl.clientBuckets[clientID]
	if !exists {
		return 0, 0, false
	}

	return bucket.tokens, bucket.capacity, true
}

// ConnectionLimiter limits concurrent connections
type ConnectionLimiter struct {
	mu              sync.RWMutex
	maxConnections  int
	maxPerClient    int
	currentConns    int
	clientConns     map[string]int
	clientLimitTime map[string]time.Time
}

// NewConnectionLimiter creates a new connection limiter
func NewConnectionLimiter(maxConnections int, maxPerClient int) *ConnectionLimiter {
	return &ConnectionLimiter{
		maxConnections: maxConnections,
		maxPerClient:   maxPerClient,
		clientConns:    make(map[string]int),
		clientLimitTime: make(map[string]time.Time),
	}
}

// AcquireConnection attempts to acquire a connection slot
func (cl *ConnectionLimiter) AcquireConnection(clientID string) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// Check if client has exceeded max connections
	if cl.clientConns[clientID] >= cl.maxPerClient {
		return false
	}

	// Check if global limit is reached
	if cl.currentConns >= cl.maxConnections {
		return false
	}

	cl.currentConns++
	cl.clientConns[clientID]++

	return true
}

// ReleaseConnection releases a connection slot
func (cl *ConnectionLimiter) ReleaseConnection(clientID string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.currentConns > 0 {
		cl.currentConns--
	}

	if cl.clientConns[clientID] > 0 {
		cl.clientConns[clientID]--
		if cl.clientConns[clientID] == 0 {
			delete(cl.clientConns, clientID)
		}
	}
}

// GetStats returns current connection statistics
func (cl *ConnectionLimiter) GetStats() (current int, max int) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	return cl.currentConns, cl.maxConnections
}

// DDoSDetector detects potential DDoS attacks based on request patterns
type DDoSDetector struct {
	mu                  sync.RWMutex
	windowSize          time.Duration
	requestsPerWindow   int
	clientRequestTimes  map[string][]time.Time
	blockedClients      map[string]time.Time
	blockDuration       time.Duration
	suspiciousThreshold float64
}

// NewDDoSDetector creates a new DDoS detector
func NewDDoSDetector(windowSize time.Duration, requestsPerWindow int, blockDuration time.Duration) *DDoSDetector {
	return &DDoSDetector{
		windowSize:          windowSize,
		requestsPerWindow:   requestsPerWindow,
		clientRequestTimes:  make(map[string][]time.Time),
		blockedClients:      make(map[string]time.Time),
		blockDuration:       blockDuration,
		suspiciousThreshold: 0.8,
	}
}

// CheckRequest checks if a request should be allowed
func (dd *DDoSDetector) CheckRequest(clientID string) error {
	dd.mu.Lock()
	defer dd.mu.Unlock()

	now := time.Now()

	// Check if client is blocked
	if blockedUntil, exists := dd.blockedClients[clientID]; exists {
		if now.Before(blockedUntil) {
			return fmt.Errorf("client is temporarily blocked due to suspicious activity")
		}
		delete(dd.blockedClients, clientID)
	}

	// Get request times for this client
	times := dd.clientRequestTimes[clientID]

	// Remove old requests outside the window
	windowStart := now.Add(-dd.windowSize)
	filteredTimes := []time.Time{}
	for _, t := range times {
		if t.After(windowStart) {
			filteredTimes = append(filteredTimes, t)
		}
	}

	// Add current request
	filteredTimes = append(filteredTimes, now)
	dd.clientRequestTimes[clientID] = filteredTimes

	// Check if requests exceed the threshold
	if len(filteredTimes) > int(float64(dd.requestsPerWindow)*dd.suspiciousThreshold) {
		// Block the client
		dd.blockedClients[clientID] = now.Add(dd.blockDuration)
		return fmt.Errorf("suspicious request pattern detected - client temporarily blocked")
	}

	return nil
}

// GetBlockedClients returns list of currently blocked clients
func (dd *DDoSDetector) GetBlockedClients() []string {
	dd.mu.RLock()
	defer dd.mu.RUnlock()

	blocked := []string{}
	now := time.Now()

	for clientID, blockedUntil := range dd.blockedClients {
		if now.Before(blockedUntil) {
			blocked = append(blocked, clientID)
		}
	}

	return blocked
}

// RateLimitMiddleware creates an HTTP middleware for rate limiting
func RateLimitMiddleware(rl *RateLimiter, getClientID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)

			if !rl.Allow(clientID) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// DDoSProtectionMiddleware creates an HTTP middleware for DDoS protection
func DDoSProtectionMiddleware(detector *DDoSDetector, getClientID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)

			if err := detector.CheckRequest(clientID); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ConnectionLimitMiddleware creates an HTTP middleware for connection limiting
func ConnectionLimitMiddleware(cl *ConnectionLimiter, getClientID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)

			if !cl.AcquireConnection(clientID) {
				http.Error(w, "Too many concurrent connections", http.StatusServiceUnavailable)
				return
			}

			defer cl.ReleaseConnection(clientID)

			next.ServeHTTP(w, r)
		})
	}
}

// Helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
