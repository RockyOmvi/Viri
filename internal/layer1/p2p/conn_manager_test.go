package p2p

import (
	"testing"
	"time"
)

func TestConnManagerStats(t *testing.T) {
	cm := NewConnManager(nil)

	stats := cm.Stats()

	if stats.ActivePeers != 0 {
		t.Errorf("Expected 0 active peers, got %d", stats.ActivePeers)
	}

	if stats.TotalPeers != 0 {
		t.Errorf("Expected 0 total peers, got %d", stats.TotalPeers)
	}
}

func TestConnManagerNeedsConnections(t *testing.T) {
	cm := NewConnManager(&ConnManagerConfig{
		MinConnections: 5,
		MaxConnections: 100,
	})

	if !cm.NeedsConnections() {
		t.Error("Should need connections initially")
	}
}

func TestConnManagerIsAtCapacity(t *testing.T) {
	cm := NewConnManager(&ConnManagerConfig{
		MinConnections: 5,
		MaxConnections: 2,
	})

	if cm.IsAtCapacity() {
		t.Error("Should not be at capacity initially")
	}
}

func TestDefaultConnManagerConfig(t *testing.T) {
	config := DefaultConnManagerConfig()

	if config.MaxConnections != 100 {
		t.Errorf("Expected MaxConnections 100, got %d", config.MaxConnections)
	}

	if config.MinConnections != 5 {
		t.Errorf("Expected MinConnections 5, got %d", config.MinConnections)
	}

	if config.GracePeriod != 30*time.Second {
		t.Errorf("Expected GracePeriod 30s, got %v", config.GracePeriod)
	}

	if config.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected HealthCheckInterval 30s, got %v", config.HealthCheckInterval)
	}
}
