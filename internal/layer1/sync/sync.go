package sync

import (
	"fmt"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
)

type SyncMode uint8

const (
	FullSync SyncMode = iota
	FastSync
	SnapSync
)

type SyncConfig struct {
	Mode          SyncMode
	PivotInterval uint64
	BatchSize     int
	Timeout       time.Duration
	MaxRetries    int
}

func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		Mode:          FastSync,
		PivotInterval: 64,
		BatchSize:     128,
		Timeout:       10 * time.Second,
		MaxRetries:    3,
	}
}

type SyncState uint8

const (
	SyncIdle SyncState = iota
	SyncingHeaders
	SyncingState
	SyncingBlocks
	SyncComplete
)

type SyncProgress struct {
	State          SyncState
	StartingHeight uint64
	CurrentHeight  uint64
	HighestHeight  uint64
	PivotBlock     uint64
	Phase          string
	ETA            time.Duration
}

type Syncer struct {
	mu       sync.Mutex
	config   *SyncConfig
	state    SyncState
	progress SyncProgress
	logger   *logging.Logger
	done     chan struct{}
}

func NewSyncer(config *SyncConfig, log *logging.Logger) *Syncer {
	if config == nil {
		config = DefaultSyncConfig()
	}
	return &Syncer{
		config: config,
		logger: log,
		done:   make(chan struct{}),
	}
}

func (s *Syncer) Start(initialHeight uint64, fetcher SyncFetcher) error {
	s.mu.Lock()
	if s.state != SyncIdle {
		s.mu.Unlock()
		return fmt.Errorf("sync already in progress")
	}
	s.state = SyncingHeaders
	s.progress.StartingHeight = initialHeight
	s.mu.Unlock()

	go s.run(initialHeight, fetcher)
	return nil
}

func (s *Syncer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func (s *Syncer) Progress() SyncProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func (s *Syncer) IsSyncing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state != SyncIdle && s.state != SyncComplete
}

func (s *Syncer) IsComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == SyncComplete
}

func (s *Syncer) run(initialHeight uint64, fetcher SyncFetcher) {
	defer func() {
		s.mu.Lock()
		s.state = SyncComplete
		s.mu.Unlock()
		s.logger.Info("Sync completed")
	}()

	remoteHeight, err := fetcher.GetRemoteHeight()
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to get remote height")
		return
	}

	s.mu.Lock()
	s.progress.HighestHeight = remoteHeight
	s.mu.Unlock()

	if remoteHeight <= initialHeight {
		s.logger.Info("Already in sync")
		return
	}

	s.logger.WithField("local", initialHeight).
		WithField("remote", remoteHeight).
		WithField("blocks_behind", remoteHeight-initialHeight).
		Info("Starting sync")

	switch s.config.Mode {
	case FullSync:
		s.syncFull(initialHeight, remoteHeight, fetcher)
	case FastSync:
		s.syncFast(initialHeight, remoteHeight, fetcher)
	case SnapSync:
		s.syncSnap(initialHeight, remoteHeight, fetcher)
	}
}

func (s *Syncer) syncFull(from, to uint64, fetcher SyncFetcher) {
	s.mu.Lock()
	s.state = SyncingBlocks
	s.progress.Phase = "downloading_blocks"
	s.mu.Unlock()

	start := time.Now()
	batchSize := uint64(s.config.BatchSize)

	for height := from + 1; height <= to; height += batchSize {
		select {
		case <-s.done:
			s.logger.Info("Sync cancelled")
			return
		default:
		}

		end := height + batchSize - 1
		if end > to {
			end = to
		}

		blocks, err := fetcher.GetBlocks(height, end)
		if err != nil {
			s.logger.WithField("error", err.Error()).
				WithField("from", height).
				WithField("to", end).
				Warn("Failed to fetch block batch, retrying...")
			height -= batchSize
			continue
		}

		for _, block := range blocks {
			if err := fetcher.ApplyBlock(block); err != nil {
				s.logger.WithField("error", err.Error()).
					WithField("height", block.Header.Height).
					Error("Failed to apply block")
				return
			}

			s.mu.Lock()
			s.progress.CurrentHeight = block.Header.Height
			elapsed := time.Since(start)
			remaining := to - block.Header.Height
			speed := float64(block.Header.Height-from) / elapsed.Seconds()
			if speed > 0 {
				s.progress.ETA = time.Duration(float64(remaining)/speed) * time.Second
			}
			s.mu.Unlock()
		}

		s.logger.WithField("height", end).
			WithField("progress", fmt.Sprintf("%.1f%%", float64(end-from)/float64(to-from)*100)).
			WithField("eta", s.progress.ETA).
			Debug("Block sync progress")
	}
}

func (s *Syncer) syncFast(from, to uint64, fetcher SyncFetcher) {
	s.mu.Lock()
	s.state = SyncingHeaders
	s.progress.Phase = "downloading_headers"
	s.mu.Unlock()

	pivotBlock := to - (to % s.config.PivotInterval)
	if pivotBlock < from {
		pivotBlock = to
	}

	s.logger.WithField("pivot", pivotBlock).Info("Downloading headers to pivot block")

	headers, err := fetcher.GetHeaders(from+1, pivotBlock)
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to fetch headers")
		return
	}

	for _, header := range headers {
		select {
		case <-s.done:
			s.logger.Info("Sync cancelled")
			return
		default:
		}

		if err := fetcher.ApplyHeader(header); err != nil {
			s.logger.WithField("error", err.Error()).
				WithField("height", header.Height).
				Warn("Failed to apply header")
			continue
		}

		s.mu.Lock()
		s.progress.CurrentHeight = header.Height
		s.progress.PivotBlock = pivotBlock
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.state = SyncingState
	s.progress.Phase = "downloading_state"
	s.mu.Unlock()

	s.logger.WithField("pivot", pivotBlock).Info("Downloading state snapshot")

	snapshot, err := fetcher.GetStateSnapshot(pivotBlock)
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to fetch state snapshot")
		return
	}

	if err := fetcher.ApplyStateSnapshot(snapshot); err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to apply state snapshot")
		return
	}

	s.mu.Lock()
	s.state = SyncingBlocks
	s.progress.Phase = "downloading_recent_blocks"
	s.mu.Unlock()

	s.logger.Info("Downloading recent blocks after pivot")

	s.syncFastRecentBlocks(pivotBlock, to, fetcher)
}

func (s *Syncer) syncFastRecentBlocks(from, to uint64, fetcher SyncFetcher) {
	start := time.Now()
	batchSize := uint64(s.config.BatchSize)

	for height := from + 1; height <= to; height += batchSize {
		select {
		case <-s.done:
			return
		default:
		}

		end := height + batchSize - 1
		if end > to {
			end = to
		}

		blocks, err := fetcher.GetBlocks(height, end)
		if err != nil {
			s.logger.WithField("error", err.Error()).Warn("Failed to fetch blocks, retrying")
			height -= batchSize
			continue
		}

		for _, block := range blocks {
			if err := fetcher.ApplyBlock(block); err != nil {
				s.logger.WithField("error", err.Error()).Error("Failed to apply block")
				return
			}

			s.mu.Lock()
			s.progress.CurrentHeight = block.Header.Height
			elapsed := time.Since(start)
			remaining := to - block.Header.Height
			speed := float64(block.Header.Height-from) / elapsed.Seconds()
			if speed > 0 {
				s.progress.ETA = time.Duration(float64(remaining)/speed) * time.Second
			}
			s.mu.Unlock()
		}
	}
}

func (s *Syncer) syncSnap(from, to uint64, fetcher SyncFetcher) {
	s.mu.Lock()
	s.state = SyncingState
	s.progress.Phase = "downloading_snapshots"
	s.mu.Unlock()

	s.logger.Info("Starting snap sync")

	snapshots, err := fetcher.GetStateSnapshots(from, to)
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to fetch snapshots")
		return
	}

	for _, snapshot := range snapshots {
		select {
		case <-s.done:
			return
		default:
		}

		if err := fetcher.ApplyStateSnapshot(snapshot); err != nil {
			s.logger.WithField("error", err.Error()).Error("Failed to apply snapshot")
			return
		}

		s.mu.Lock()
		s.progress.CurrentHeight = snapshot.BlockHeight
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.state = SyncingBlocks
	s.progress.Phase = "syncing_recent"
	s.mu.Unlock()

	s.syncFastRecentBlocks(to, to, fetcher)
}

type SyncFetcher interface {
	GetRemoteHeight() (uint64, error)
	GetHeaders(from, to uint64) ([]*ledger.Header, error)
	GetBlocks(from, to uint64) ([]*ledger.Block, error)
	ApplyBlock(block *ledger.Block) error
	ApplyHeader(header *ledger.Header) error
	GetStateSnapshot(height uint64) (*ledger.StateSnapshot, error)
	GetStateSnapshots(from, to uint64) ([]*ledger.StateSnapshot, error)
	ApplyStateSnapshot(snapshot *ledger.StateSnapshot) error
}
