package ledger

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type Blockchain struct {
	mu       sync.RWMutex
	blocks   []*Block
	index    map[string]*Block
	indexOrder []string
	indexCap int
	genesis  *GenesisConfig
}

func NewBlockchain(genesis *GenesisConfig) (*Blockchain, error) {
	bc := &Blockchain{
		blocks:  make([]*Block, 0),
		index:   make(map[string]*Block),
		indexOrder: make([]string, 0),
		indexCap: 1000,
		genesis: genesis,
	}

	genesisBlock := CreateGenesisBlock(genesis)
	bc.blocks = append(bc.blocks, genesisBlock)
	bc.cacheBlock(genesisBlock)

	return bc, nil
}

func CreateGenesisBlock(config *GenesisConfig) *Block {
	header := &Header{
		Version:   Version1,
		Height:    0,
		PrevHash:  make([]byte, 32),
		TxsHash:   crypto.SHA256([]byte("genesis")),
		StateRoot: crypto.SHA256([]byte("empty-state")),
		Timestamp: time.Unix(0, 0),
		Proposer:  []byte("genesis"),
		Nonce:     2048,
	}

	return &Block{
		Header:       header,
		Transactions: make([]*Transaction, 0),
	}
}

func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(bc.blocks) == 0 {
		return ErrGenesisExists
	}

	latestBlock := bc.blocks[len(bc.blocks)-1]

	if block.Header.Height != latestBlock.Header.Height+1 {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidHeight, latestBlock.Header.Height+1, block.Header.Height)
	}

	if !crypto.EqualHash(block.Header.PrevHash, latestBlock.Hash()) {
		return ErrInvalidPrevHash
	}

	if !block.Verify() {
		return ErrInvalidBlock
	}

	bc.blocks = append(bc.blocks, block)
	bc.cacheBlock(block)

	return nil
}

func (bc *Blockchain) AddBlockByHash(height uint64, prevHash []byte, blockHash []byte, proposer []byte) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(bc.blocks) == 0 {
		return ErrGenesisExists
	}

	latestBlock := bc.blocks[len(bc.blocks)-1]

	if height != latestBlock.Header.Height+1 {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidHeight, latestBlock.Header.Height+1, height)
	}

	if !crypto.EqualHash(prevHash, latestBlock.Hash()) {
		return ErrInvalidPrevHash
	}

	block := &Block{
		Header: &Header{
			Version:   Version1,
			Height:    height,
			PrevHash:  prevHash,
			TxsHash:   crypto.SHA256([]byte{}),
			StateRoot: crypto.SHA256([]byte{}),
			Proposer:  proposer,
		},
		Transactions:  make([]*Transaction, 0),
		ConsensusHash: blockHash,
	}

	hashKey := hex.EncodeToString(blockHash)
	bc.blocks = append(bc.blocks, block)
	bc.index[hashKey] = block
	bc.indexOrder = append(bc.indexOrder, hashKey)

	return nil
}

func (bc *Blockchain) GetBlock(height uint64) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if height >= uint64(len(bc.blocks)) {
		return nil, fmt.Errorf("block at height %d not found", height)
	}

	return bc.blocks[height], nil
}

func (bc *Blockchain) GetBlockByHash(hash string) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	block, exists := bc.index[hash]
	if !exists {
		return nil, fmt.Errorf("block with hash %s not found", hash)
	}

	bc.touchIndex(hash)

	return block, nil
}

func (bc *Blockchain) LatestBlock() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if len(bc.blocks) == 0 {
		return nil
	}

	return bc.blocks[len(bc.blocks)-1]
}

func (bc *Blockchain) Height() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if len(bc.blocks) == 0 {
		return 0
	}

	return bc.blocks[len(bc.blocks)-1].Header.Height
}

func (bc *Blockchain) Validate() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for i := 1; i < len(bc.blocks); i++ {
		currentBlock := bc.blocks[i]
		prevBlock := bc.blocks[i-1]

		if currentBlock.Header.Height != prevBlock.Header.Height+1 {
			return false
		}

		if !crypto.EqualHash(currentBlock.Header.PrevHash, prevBlock.Hash()) {
			return false
		}

		if !currentBlock.Verify() {
			return false
		}
	}

	return true
}

func (bc *Blockchain) cacheBlock(block *Block) {
	hash := hex.EncodeToString(block.Hash())

	if _, exists := bc.index[hash]; exists {
		bc.touchIndex(hash)
		bc.index[hash] = block
		return
	}

	bc.index[hash] = block
	bc.indexOrder = append(bc.indexOrder, hash)

	if bc.indexCap > 0 && len(bc.indexOrder) > bc.indexCap {
		evict := bc.indexOrder[0]
		bc.indexOrder = bc.indexOrder[1:]
		delete(bc.index, evict)
	}
}

func (bc *Blockchain) touchIndex(hash string) {
	if len(bc.indexOrder) == 0 {
		return
	}

	idx := -1
	for i, h := range bc.indexOrder {
		if h == hash {
			idx = i
			break
		}
	}

	if idx == -1 || idx == len(bc.indexOrder)-1 {
		return
	}

	bc.indexOrder = append(bc.indexOrder[:idx], bc.indexOrder[idx+1:]...)
	bc.indexOrder = append(bc.indexOrder, hash)
}
