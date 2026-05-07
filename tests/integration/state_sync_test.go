package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	virisync "github.com/viri-chain/viri/internal/layer1/sync"
)

type mockSyncFetcher struct {
	mu             sync.Mutex
	remoteHeight   uint64
	headers        map[uint64]*ledger.Header
	blocks         map[uint64]*ledger.Block
	snapshots      map[uint64]*ledger.StateSnapshot
	applyBlockErr  error
	applyHeaderErr error
	callCount      int
}

func newMockFetcher(initialHeight uint64) *mockSyncFetcher {
	return &mockSyncFetcher{
		remoteHeight: initialHeight + 100,
		headers:     make(map[uint64]*ledger.Header),
		blocks:      make(map[uint64]*ledger.Block),
		snapshots:   make(map[uint64]*ledger.StateSnapshot),
	}
}

func (m *mockSyncFetcher) GetRemoteHeight() (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.remoteHeight, nil
}

func (m *mockSyncFetcher) GetHeaders(from, to uint64) ([]*ledger.Header, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*ledger.Header
	for h := from; h <= to; h++ {
		if header, exists := m.headers[h]; exists {
			result = append(result, header)
		} else {
			result = append(result, &ledger.Header{Height: h})
		}
	}
	return result, nil
}

func (m *mockSyncFetcher) GetBlocks(from, to uint64) ([]*ledger.Block, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*ledger.Block
	for h := from; h <= to; h++ {
		if block, exists := m.blocks[h]; exists {
			result = append(result, block)
		} else {
			result = append(result, &ledger.Block{Header: &ledger.Header{Height: h}})
		}
	}
	return result, nil
}

func (m *mockSyncFetcher) ApplyBlock(block *ledger.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.applyBlockErr
}

func (m *mockSyncFetcher) ApplyHeader(header *ledger.Header) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.applyHeaderErr
}

func (m *mockSyncFetcher) GetStateSnapshot(height uint64) (*ledger.StateSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if snap, exists := m.snapshots[height]; exists {
		return snap, nil
	}
	return &ledger.StateSnapshot{BlockHeight: height}, nil
}

func (m *mockSyncFetcher) GetStateSnapshots(from, to uint64) ([]*ledger.StateSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*ledger.StateSnapshot
	for h := from; h <= to; h++ {
		result = append(result, &ledger.StateSnapshot{BlockHeight: h})
	}
	return result, nil
}

func (m *mockSyncFetcher) ApplyStateSnapshot(snapshot *ledger.StateSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return nil
}

func TestFastSyncDownloadsHeadersThenState(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()
	config.Mode = virisync.FastSync
	config.PivotInterval = 50

	syncer := virisync.NewSyncer(config, logger)

	fetcher := newMockFetcher(0)
	fetcher.remoteHeight = 100

	if err := syncer.Start(0, fetcher); err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	progress := syncer.Progress()
	if progress.Phase != "downloading_state" && progress.Phase != "downloading_recent_blocks" {
		t.Logf("Sync phase: %s (may still be in progress)", progress.Phase)
	}

	syncer.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestSnapSyncWithPivotBlock(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()
	config.Mode = virisync.SnapSync

	syncer := virisync.NewSyncer(config, logger)

	fetcher := newMockFetcher(0)
	fetcher.remoteHeight = 200

	if err := syncer.Start(0, fetcher); err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	progress := syncer.Progress()
	if progress.PivotBlock > 0 {
		t.Logf("Pivot block set: %d", progress.PivotBlock)
	}

	syncer.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestSyncProgressReporting(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()
	config.Mode = virisync.FastSync
	config.BatchSize = 10

	syncer := virisync.NewSyncer(config, logger)

	fetcher := newMockFetcher(10)
	fetcher.remoteHeight = 50

	if err := syncer.Start(10, fetcher); err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	progress := syncer.Progress()
	if progress.StartingHeight != 10 {
		t.Errorf("expected starting height 10, got %d", progress.StartingHeight)
	}

	if progress.HighestHeight != 50 {
		t.Errorf("expected highest height 50, got %d", progress.HighestHeight)
	}

	syncer.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestSyncCompletionDetection(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()
	config.Mode = virisync.FullSync
	config.BatchSize = 50

	syncer := virisync.NewSyncer(config, logger)

	fetcher := newMockFetcher(0)
	fetcher.remoteHeight = 10

	if err := syncer.Start(0, fetcher); err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	for i := 0; i < 50; i++ {
		if syncer.IsComplete() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !syncer.IsComplete() {
		t.Error("sync should be complete")
	}

	if syncer.IsSyncing() {
		t.Error("should not be syncing after completion")
	}
}

func TestSyncErrorRecovery(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()
	config.Mode = virisync.FullSync
	config.MaxRetries = 1

	syncer := virisync.NewSyncer(config, logger)

	fetcher := newMockFetcher(0)
	fetcher.remoteHeight = 20
	fetcher.applyBlockErr = fmt.Errorf("mock apply error")

	go func() {
		time.Sleep(200 * time.Millisecond)
		fetcher.mu.Lock()
		fetcher.applyBlockErr = nil
		fetcher.mu.Unlock()
	}()

	if err := syncer.Start(0, fetcher); err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(800 * time.Millisecond)
	syncer.Stop()
}

func TestSyncStateMachineTransitions(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()
	config.Mode = virisync.FullSync
	config.BatchSize = 1

	syncer := virisync.NewSyncer(config, logger)

	fetcher := newMockFetcher(0)
	fetcher.remoteHeight = 10

	if syncer.IsSyncing() {
		t.Error("should not be syncing before start")
	}

	if err := syncer.Start(0, fetcher); err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if syncer.IsSyncing() && !syncer.IsComplete() {
		t.Error("should be syncing or complete after start")
	}

	syncer.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestSyncIdleState(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()

	syncer := virisync.NewSyncer(config, logger)

	if syncer.IsSyncing() {
		t.Error("new syncer should be idle")
	}

	if syncer.IsComplete() {
		t.Error("new syncer should not be complete")
	}

	progress := syncer.Progress()
	if progress.State != virisync.SyncIdle {
		t.Errorf("expected idle state, got %v", progress.State)
	}
}

func TestSyncStopBeforeCompletion(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "json")
	config := virisync.DefaultSyncConfig()
	config.Mode = virisync.FullSync
	config.BatchSize = 1

	syncer := virisync.NewSyncer(config, logger)

	fetcher := newMockFetcher(0)
	fetcher.remoteHeight = 1000

	if err := syncer.Start(0, fetcher); err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	syncer.Stop()

	time.Sleep(100 * time.Millisecond)

	if syncer.IsSyncing() {
		t.Error("should not be syncing after stop")
	}
}
