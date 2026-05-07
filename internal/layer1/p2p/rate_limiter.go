package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type RateLimiterConfig struct {
	MaxMessagesPerSecond int
	MaxBytesPerSecond    int
	BurstSize            int
	WindowSize           time.Duration
	BlockDuration        time.Duration
}

func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		MaxMessagesPerSecond: 500,
		MaxBytesPerSecond:    50 * 1024 * 1024,
		BurstSize:            100,
		WindowSize:           time.Second,
		BlockDuration:        5 * time.Second,
	}
}

type RateWindow struct {
	Count     int
	Bytes     int
	Timestamp time.Time
}

type RateLimiter struct {
	mu       sync.RWMutex
	config   *RateLimiterConfig
	windows  map[peer.ID]*RateWindow
	blocked  map[peer.ID]time.Time
}

func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	if config.BlockDuration == 0 {
		config.BlockDuration = 30 * time.Second
	}

	return &RateLimiter{
		config:  config,
		windows: make(map[peer.ID]*RateWindow),
		blocked: make(map[peer.ID]time.Time),
	}
}

func (rl *RateLimiter) Allow(peerID peer.ID, msgSize int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if blockedUntil, exists := rl.blocked[peerID]; exists {
		if time.Now().Before(blockedUntil) {
			return false
		}
		delete(rl.blocked, peerID)
	}

	window := rl.getWindow(peerID)

	if window.Count >= rl.config.MaxMessagesPerSecond {
		rl.blockPeer(peerID)
		return false
	}

	if window.Bytes+msgSize >= rl.config.MaxBytesPerSecond {
		rl.blockPeer(peerID)
		return false
	}

	window.Count++
	window.Bytes += msgSize

	return true
}

func (rl *RateLimiter) IsBlocked(peerID peer.ID) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	blockedUntil, exists := rl.blocked[peerID]
	if !exists {
		return false
	}

	return time.Now().Before(blockedUntil)
}

func (rl *RateLimiter) Unblock(peerID peer.ID) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.blocked, peerID)
}

func (rl *RateLimiter) Reset(peerID peer.ID) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.windows, peerID)
	delete(rl.blocked, peerID)
}

func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.windows = make(map[peer.ID]*RateWindow)
	rl.blocked = make(map[peer.ID]time.Time)
}

func (rl *RateLimiter) GetStats() map[string]RateLimiterStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	stats := make(map[string]RateLimiterStats)
	for peerID, window := range rl.windows {
		stats[peerID.String()] = RateLimiterStats{
			Messages: window.Count,
			Bytes:    window.Bytes,
			Blocked:  rl.isBlockedLocked(peerID),
		}
	}

	return stats
}

func (rl *RateLimiter) getWindow(peerID peer.ID) *RateWindow {
	window, exists := rl.windows[peerID]
	if !exists || time.Since(window.Timestamp) > rl.config.WindowSize {
		window = &RateWindow{
			Timestamp: time.Now(),
		}
		rl.windows[peerID] = window
	}
	return window
}

func (rl *RateLimiter) blockPeer(peerID peer.ID) {
	rl.blocked[peerID] = time.Now().Add(rl.config.BlockDuration)
}

func (rl *RateLimiter) isBlockedLocked(peerID peer.ID) bool {
	blockedUntil, exists := rl.blocked[peerID]
	if !exists {
		return false
	}
	return time.Now().Before(blockedUntil)
}

type RateLimiterStats struct {
	Messages int
	Bytes    int
	Blocked  bool
}
