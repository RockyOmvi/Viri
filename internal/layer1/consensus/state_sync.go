package consensus

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
)

type BlockRequester func(fromHeight, toHeight uint64) error
type BlockApplier func(blockData []byte) error
type HeightGetter func() uint64

type StateSyncer struct {
	mu            sync.RWMutex
	syncing       bool
	currentHeight uint64
	targetHeight  uint64
	pendingBlocks map[uint64][]byte
	requested     map[uint64]bool
	blockRequester  BlockRequester
	blockApplier    BlockApplier
	heightGetter    HeightGetter
	logger          *logging.Logger
	syncTimeout     time.Duration
	syncStartTime   time.Time
	maxBatchSize    uint64
}

func NewStateSyncer(blockRequester BlockRequester, blockApplier BlockApplier, heightGetter HeightGetter, log *logging.Logger) *StateSyncer {
	return &StateSyncer{
		pendingBlocks:  make(map[uint64][]byte),
		requested:      make(map[uint64]bool),
		blockRequester: blockRequester,
		blockApplier:   blockApplier,
		heightGetter:   heightGetter,
		logger:         log,
		syncTimeout:    60 * time.Second,
		maxBatchSize:   100,
	}
}

func (ss *StateSyncer) StartSync(targetHeight uint64) {
	ss.mu.Lock()

	localHeight := ss.heightGetter()
	if targetHeight <= localHeight {
		ss.mu.Unlock()
		return
	}

	if ss.syncing {
		if targetHeight > ss.targetHeight {
			ss.targetHeight = targetHeight
		}
		ss.requested = make(map[uint64]bool)
		ss.pendingBlocks = make(map[uint64][]byte)
		ss.syncStartTime = time.Now()
		ss.mu.Unlock()
		return
	}

	ss.syncing = true
	ss.currentHeight = localHeight
	ss.targetHeight = targetHeight
	ss.pendingBlocks = make(map[uint64][]byte)
	ss.requested = make(map[uint64]bool)
	ss.syncStartTime = time.Now()

	ss.logInfo(fmt.Sprintf("State sync started from=%d to=%d", ss.currentHeight, ss.targetHeight))

	// Mark first batch as requested while holding the lock
	batchEnd := localHeight + ss.maxBatchSize - 1
	if batchEnd > targetHeight {
		batchEnd = targetHeight
	}
	for h := localHeight; h <= batchEnd; h++ {
		ss.requested[h] = true
	}
	requester := ss.blockRequester
	ss.mu.Unlock()

	// Issue the first batch synchronously (before requestLoop goroutine starts,
	// eliminating the race window where the goroutine is delayed by the scheduler)
	if err := requester(localHeight, batchEnd); err != nil {
		ss.logger.WithField("error", err.Error()).Warn("initial block request batch failed")
	}

	go ss.requestLoop()
}

func (ss *StateSyncer) RequestBlocks(fromHeight, toHeight uint64) error {
	ss.mu.Lock()

	if !ss.syncing {
		ss.mu.Unlock()
		return fmt.Errorf("not syncing")
	}

	batchStart := fromHeight
	for h := batchStart; h <= toHeight && h-batchStart < ss.maxBatchSize; h++ {
		if ss.requested[h] {
			continue
		}
		ss.requested[h] = true
	}

	batchEnd := batchStart + ss.maxBatchSize - 1
	if batchEnd > toHeight {
		batchEnd = toHeight
	}
	ss.mu.Unlock()

	if batchEnd < batchStart {
		return nil
	}

	return ss.blockRequester(batchStart, batchEnd)
}

func (ss *StateSyncer) ApplyBlock(blockData []byte) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if !ss.syncing {
		return fmt.Errorf("not syncing")
	}

	if len(blockData) < 8 {
		return fmt.Errorf("invalid block data")
	}

	blockHeight := binary.BigEndian.Uint64(blockData[:8])

	if blockHeight < ss.currentHeight {
		return nil
	}

	if blockHeight > ss.targetHeight {
		ss.pendingBlocks[blockHeight] = blockData
		return nil
	}

	ss.pendingBlocks[blockHeight] = blockData

	for ss.currentHeight <= ss.targetHeight {
		block, exists := ss.pendingBlocks[ss.currentHeight]
		if !exists {
			break
		}

		delete(ss.pendingBlocks, ss.currentHeight)

		if err := ss.blockApplier(block); err != nil {
			ss.logError(fmt.Sprintf("Failed to apply block height=%d error=%v", ss.currentHeight, err))
			delete(ss.requested, ss.currentHeight)
			ss.resyncFromHeightLocked(ss.currentHeight)
			return err
		}

		ss.logInfo(fmt.Sprintf("Applied block during sync height=%d", ss.currentHeight))
		ss.currentHeight++
	}

	if ss.currentHeight > ss.targetHeight {
		ss.finishSync()
	}

	return nil
}

func (ss *StateSyncer) IsSyncing() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.syncing
}

func (ss *StateSyncer) SyncProgress() (uint64, uint64) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	if !ss.syncing {
		return 0, 0
	}
	return ss.currentHeight, ss.targetHeight
}

func (ss *StateSyncer) ReceiveBlock(blockData []byte) error {
	return ss.ApplyBlock(blockData)
}

func (ss *StateSyncer) Stop() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.syncing = false
	ss.pendingBlocks = make(map[uint64][]byte)
	ss.requested = make(map[uint64]bool)
}

func (ss *StateSyncer) requestLoop() {
	ss.requestNextBatch()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		ss.mu.RLock()
		if !ss.syncing {
			ss.mu.RUnlock()
			return
		}

		if time.Since(ss.syncStartTime) > ss.syncTimeout {
			ss.logWarn("Sync timeout, restarting")
			ss.mu.RUnlock()
			ss.mu.Lock()
			ss.resyncFromHeightLocked(ss.currentHeight)
			ss.mu.Unlock()
			continue
		}

		target := ss.targetHeight
		current := ss.currentHeight
		ss.mu.RUnlock()

		if current > target {
			return
		}

		ss.requestNextBatch()

		select {
		case <-ticker.C:
		case <-time.After(2 * time.Second):
		}
	}
}

func (ss *StateSyncer) requestNextBatch() {
	ss.mu.RLock()
	if !ss.syncing {
		ss.mu.RUnlock()
		return
	}
	target := ss.targetHeight
	current := ss.currentHeight
	ss.mu.RUnlock()

	if current > target {
		return
	}

	batchEnd := current + ss.maxBatchSize - 1
	if batchEnd > target {
		batchEnd = target
	}
	if err := ss.RequestBlocks(current, batchEnd); err != nil {
		ss.logger.WithField("error", err.Error()).Warn("state sync RequestBlocks failed")
	}
}

func (ss *StateSyncer) resyncFromHeight(height uint64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.resyncFromHeightLocked(height)
}

func (ss *StateSyncer) resyncFromHeightLocked(height uint64) {
	ss.currentHeight = height
	ss.requested = make(map[uint64]bool)
	ss.pendingBlocks = make(map[uint64][]byte)
	ss.syncStartTime = time.Now()
}

func (ss *StateSyncer) finishSync() {
	ss.syncing = false
	ss.pendingBlocks = make(map[uint64][]byte)
	ss.requested = make(map[uint64]bool)
	ss.logInfo(fmt.Sprintf("State sync completed height=%d duration=%v", ss.currentHeight-1, time.Since(ss.syncStartTime)))
}

func (ss *StateSyncer) logInfo(msg string) {
	if ss.logger != nil {
		ss.logger.Info(msg)
	}
}

func (ss *StateSyncer) logWarn(msg string) {
	if ss.logger != nil {
		ss.logger.Warn(msg)
	}
}

func (ss *StateSyncer) logError(msg string) {
	if ss.logger != nil {
		ss.logger.Error(msg)
	}
}

func ExtractBlockHeight(blockData []byte) uint64 {
	if len(blockData) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(blockData[:8])
}

func SerializeBlockHeight(height uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, height)
	return buf
}

func DeserializeBlockHeight(data []byte) (uint64, error) {
	if len(data) < 8 {
		return 0, fmt.Errorf("insufficient data")
	}
	return binary.BigEndian.Uint64(data[:8]), nil
}

func SerializeBlockRequest(fromHeight, toHeight uint64) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[:8], fromHeight)
	binary.BigEndian.PutUint64(buf[8:16], toHeight)
	return buf
}

func DeserializeBlockRequest(data []byte) (uint64, uint64, error) {
	if len(data) < 16 {
		return 0, 0, fmt.Errorf("insufficient data")
	}
	fromHeight := binary.BigEndian.Uint64(data[:8])
	toHeight := binary.BigEndian.Uint64(data[8:16])
	return fromHeight, toHeight, nil
}

func SerializeBlockResponse(block *ledger.Block) ([]byte, error) {
	return ledger.SerializeBlock(block)
}

func DeserializeBlockResponse(data []byte) (*ledger.Block, error) {
	return ledger.DeserializeBlock(data)
}
