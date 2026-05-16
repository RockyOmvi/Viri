package node

import (
	"sync"
	"time"
)

// LightMode configures a light client with memory-optimized settings.
type LightMode struct {
	mu                sync.RWMutex
	enabled           bool
	MaxPeers          int
	MaxMempoolSize    int
	BadgerCacheSize   int64 // in MB
	PruneAfterEpochs  uint64
	DisableHistoricRPC bool
	StateCacheSize    int
}

// DefaultLightMode returns a light client config optimized for 1GB RAM.
func DefaultLightMode() *LightMode {
	return &LightMode{
		MaxPeers:          4,
		MaxMempoolSize:    1000,
		BadgerCacheSize:   256,
		PruneAfterEpochs:  1000,
		DisableHistoricRPC: true,
		StateCacheSize:    10_000,
	}
}

// Enable turns on light mode.
func (l *LightMode) Enable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = true
}

// IsEnabled returns whether light mode is active.
func (l *LightMode) IsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// StateDeleter is the interface for deleting state data by epoch.
type StateDeleter interface {
	DeleteBefore(epoch uint64) (uint64, error)
}

// StatePruner manages state pruning for low-memory environments.
type StatePruner struct {
	mu             sync.Mutex
	keepEpochs     uint64
	currentEpoch   uint64
	prunedBlocks   uint64
	lastPruneTime  time.Time
	stateDeleter   StateDeleter
}

// NewStatePruner creates a pruner that keeps the last N epochs of state.
func NewStatePruner(sd StateDeleter, keepEpochs uint64) *StatePruner {
	return &StatePruner{
		keepEpochs:   keepEpochs,
		stateDeleter: sd,
	}
}

// Prune removes old state data beyond the configured retention.
func (sp *StatePruner) Prune(currentEpoch uint64) (uint64, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if currentEpoch <= sp.keepEpochs {
		return 0, nil
	}

	pruneBefore := currentEpoch - sp.keepEpochs
	count, err := sp.stateDeleter.DeleteBefore(pruneBefore)
	if err != nil {
		return 0, err
	}

	sp.prunedBlocks += count
	sp.lastPruneTime = time.Now()
	sp.currentEpoch = currentEpoch
	return count, nil
}

// Stats returns pruning statistics.
func (sp *StatePruner) Stats() (uint64, time.Time) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.prunedBlocks, sp.lastPruneTime
}
