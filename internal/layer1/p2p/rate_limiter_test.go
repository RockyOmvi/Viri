package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{
		MaxMessagesPerSecond: 5,
		MaxBytesPerSecond:    1000,
		BurstSize:            5,
		WindowSize:           1 * time.Second,
	})

	peerID := peer.ID("test-peer")

	for i := 0; i < 5; i++ {
		if !rl.Allow(peerID, 100) {
			t.Errorf("Message %d should be allowed", i+1)
		}
	}

	if rl.Allow(peerID, 100) {
		t.Error("Message beyond limit should be rejected")
	}
}

func TestRateLimiterBlockAndUnblock(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{
		MaxMessagesPerSecond: 2,
		MaxBytesPerSecond:    1000,
		BurstSize:            2,
		WindowSize:           1 * time.Second,
	})

	peerID := peer.ID("test-peer")

	rl.Allow(peerID, 100)
	rl.Allow(peerID, 100)
	rl.Allow(peerID, 100)

	if !rl.IsBlocked(peerID) {
		t.Error("Peer should be blocked after exceeding limit")
	}

	rl.Unblock(peerID)

	if rl.IsBlocked(peerID) {
		t.Error("Peer should not be blocked after unblock")
	}
}

func TestRateLimiterReset(t *testing.T) {
	rl := NewRateLimiter(nil)

	peerID := peer.ID("test-peer")
	rl.Allow(peerID, 100)

	rl.Reset(peerID)

	if !rl.Allow(peerID, 100) {
		t.Error("Peer should be allowed after reset")
	}
}

func TestRateLimiterResetAll(t *testing.T) {
	rl := NewRateLimiter(nil)

	peerID1 := peer.ID("peer1")
	peerID2 := peer.ID("peer2")

	rl.Allow(peerID1, 100)
	rl.Allow(peerID2, 100)

	rl.ResetAll()

	if !rl.Allow(peerID1, 100) {
		t.Error("Peer1 should be allowed after reset all")
	}
	if !rl.Allow(peerID2, 100) {
		t.Error("Peer2 should be allowed after reset all")
	}
}

func TestRateLimiterStats(t *testing.T) {
	rl := NewRateLimiter(nil)

	peerID := peer.ID("test-peer")
	rl.Allow(peerID, 100)
	rl.Allow(peerID, 200)

	stats := rl.GetStats()

	peerStats, exists := stats[peerID.String()]
	if !exists {
		t.Fatal("Expected stats for peer")
	}

	if peerStats.Messages != 2 {
		t.Errorf("Expected 2 messages, got %d", peerStats.Messages)
	}

	if peerStats.Bytes != 300 {
		t.Errorf("Expected 300 bytes, got %d", peerStats.Bytes)
	}
}

func TestRateLimiterByteLimit(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{
		MaxMessagesPerSecond: 100,
		MaxBytesPerSecond:    500,
		BurstSize:            100,
		WindowSize:           1 * time.Second,
	})

	peerID := peer.ID("test-peer")

	rl.Allow(peerID, 200)
	rl.Allow(peerID, 200)

	if rl.Allow(peerID, 200) {
		t.Error("Should be blocked by byte limit")
	}
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	config := DefaultRateLimiterConfig()

	if config.MaxMessagesPerSecond != 100 {
		t.Errorf("Expected MaxMessagesPerSecond 100, got %d", config.MaxMessagesPerSecond)
	}

	if config.MaxBytesPerSecond != 10*1024*1024 {
		t.Errorf("Expected MaxBytesPerSecond 10MB, got %d", config.MaxBytesPerSecond)
	}
}
