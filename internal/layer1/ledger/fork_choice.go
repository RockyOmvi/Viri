package ledger

import (
	"fmt"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type ForkChoice struct {
}

func NewForkChoice() *ForkChoice {
	return &ForkChoice{}
}

type ReorgResult struct {
	OldTip    uint64
	NewTip    uint64
	CommonAncestor uint64
	ReorgDepth uint64
}

func (fc *ForkChoice) SelectChain(current *PersistentBlockchain, candidate *Block, candidateStateRoot []byte) (*ReorgResult, error) {
	if candidate.Header.Height <= current.Height() {
		return nil, nil
	}

	weight := computeChainWeight(candidate)
	currentWeight := computeChainWeightFromHeight(current.Height())

	if weight <= currentWeight {
		return nil, nil
	}

	return &ReorgResult{
		OldTip:         current.Height(),
		NewTip:         candidate.Header.Height,
		CommonAncestor: findCommonAncestorHeight(current, candidate),
		ReorgDepth:     candidate.Header.Height - current.Height(),
	}, nil
}

func computeChainWeight(block *Block) uint64 {
	return block.Header.Height * 1000 + uint64(len(block.Transactions))
}

func computeChainWeightFromHeight(height uint64) uint64 {
	return height * 1000
}

func findCommonAncestorHeight(current *PersistentBlockchain, candidate *Block) uint64 {
	currentHeight := current.Height()
	candidateHeight := candidate.Header.Height

	minHeight := currentHeight
	if candidateHeight < minHeight {
		minHeight = candidateHeight
	}

	return minHeight
}

func (bc *PersistentBlockchain) ProcessReorg(newBlocks []*Block) error {
	if len(newBlocks) == 0 {
		return nil
	}

	bc.mu.Lock()
	defer bc.mu.Unlock()

	commonAncestor := findCommonAncestorHeight(bc, newBlocks[0])

	// Unwind old blocks: re-add their transactions to the pool
	for h := bc.height; h > commonAncestor; h-- {
		block, err := bc.GetBlock(h)
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions {
			bc.txPool.Add(tx)
		}
	}

	// Apply new blocks
	for _, block := range newBlocks {
		if block.Header.Height <= commonAncestor {
			continue
		}

		if block.Header.Height != bc.height+1 {
			return fmt.Errorf("invalid block height in reorg: expected %d, got %d", bc.height+1, block.Header.Height)
		}

		if !crypto.EqualHash(block.Header.PrevHash, bc.tipHash) {
			parentBlock, err := bc.GetBlock(block.Header.Height - 1)
			if err != nil {
				return fmt.Errorf("missing parent block for reorg: %w", err)
			}

			if !crypto.EqualHash(block.Header.PrevHash, parentBlock.Hash()) {
				return ErrInvalidPrevHash
			}
		}

		if !block.Verify() {
			return ErrInvalidBlock
		}

		// Apply economic effects for the new block
		if _, err := bc.economics.ProcessBlock(block.Transactions, block.Header.Height); err != nil {
			return fmt.Errorf("economics processing failed during reorg: %w", err)
		}

		blockData, err := SerializeBlock(block)
		if err != nil {
			return err
		}

		blockHash := block.Hash()
		batch := bc.db.Batch()
		batch.Put(blockKey(block.Header.Height), blockData)
		batch.Put(hashIndexKey(blockHash), uint64ToBytes(block.Header.Height))
		batch.Put(heightKey(), uint64ToBytes(block.Header.Height))
		batch.Put(tipHashKey(), blockHash)

		if err := batch.Write(); err != nil {
			return err
		}

		bc.height = block.Header.Height
		bc.tipHash = blockHash
		bc.cacheBlock(block)

		if err := bc.txIndex.IndexTransactions(block.Transactions, block.Header.Height); err != nil {
			return fmt.Errorf("failed to index transactions during reorg: %w", err)
		}
	}

	return nil
}

func (bc *PersistentBlockchain) HandleSideBlock(block *Block) (*ReorgResult, error) {
	if block.Header.Height <= bc.Height() {
		return nil, nil
	}

	fc := NewForkChoice()
	result, err := fc.SelectChain(bc, block, nil)
	if err != nil {
		return nil, fmt.Errorf("fork choice failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	if err := bc.ProcessReorg([]*Block{block}); err != nil {
		return nil, fmt.Errorf("reorg processing failed: %w", err)
	}

	return result, nil
}

func (bc *PersistentBlockchain) ValidateChainContinuity() error {
	// Read current height under lock, then verify blocks without holding lock
	// to avoid recursive RLock hazard in GetBlock.
	bc.mu.RLock()
	currentHeight := bc.height
	bc.mu.RUnlock()

	for h := uint64(1); h <= currentHeight; h++ {
		block, err := bc.GetBlock(h)
		if err != nil {
			return fmt.Errorf("missing block at height %d: %w", h, err)
		}

		prevBlock, err := bc.GetBlock(h - 1)
		if err != nil {
			return fmt.Errorf("missing parent block at height %d: %w", h-1, err)
		}

		if !crypto.EqualHash(block.Header.PrevHash, prevBlock.Hash()) {
			return fmt.Errorf("chain discontinuity at height %d: prev_hash mismatch", h)
		}
	}

	return nil
}
