package ledger

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/state"
)

type PersistentBlockchain struct {
	mu         sync.RWMutex
	genesis    *GenesisConfig
	db         state.KVStore
	txPool     *TxPool
	txIndex    *TxIndex
	economics  *Economics
	height     uint64
	tipHash    []byte
	cache      map[uint64]*Block
	cacheOrder []uint64
	cacheCap   int
}

func NewPersistentBlockchain(genesis *GenesisConfig, db state.KVStore) (*PersistentBlockchain, error) {
	bc := &PersistentBlockchain{
		genesis:    genesis,
		db:         db,
		economics:  NewEconomics(nil),
		txPool:     NewTxPool(nil, nil),
		txIndex:    NewTxIndex(db),
		cache:      make(map[uint64]*Block),
		cacheOrder: make([]uint64, 0),
		cacheCap:   1000,
	}

	if err := bc.loadFromDB(); err != nil {
		if err.Error() == "chain not initialized" {
			if err := bc.initializeGenesis(); err != nil {
				return nil, fmt.Errorf("failed to initialize genesis: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load chain: %w", err)
		}
	}

	return bc, nil
}

func (bc *PersistentBlockchain) initializeGenesis() error {
	genesisBlock := CreateGenesisBlock(bc.genesis)
	genesisHash := genesisBlock.Hash()

	blockData, err := SerializeBlock(genesisBlock)
	if err != nil {
		return err
	}

	batch := bc.db.Batch()
	batch.Put(blockKey(0), blockData)
	batch.Put(hashIndexKey(genesisHash), uint64ToBytes(0))
	batch.Put(heightKey(), uint64ToBytes(0))
	batch.Put(tipHashKey(), genesisHash)

	if err := batch.Write(); err != nil {
		return err
	}

	bc.height = 0
	bc.tipHash = genesisHash

	return nil
}

func (bc *PersistentBlockchain) AddBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if block.Header.Height != bc.height+1 {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidHeight, bc.height+1, block.Header.Height)
	}

	if !crypto.EqualHash(block.Header.PrevHash, bc.tipHash) {
		return ErrInvalidPrevHash
	}

	if !block.Verify() {
		return ErrInvalidBlock
	}

	_, err := bc.economics.ProcessBlock(block.Transactions, block.Header.Height)
	if err != nil {
		return fmt.Errorf("economics processing failed: %w", err)
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

	bc.txPool.RemoveConfirmed(block.Transactions)

	if err := bc.txIndex.IndexTransactions(block.Transactions, block.Header.Height); err != nil {
		return fmt.Errorf("failed to index transactions: %w", err)
	}

	return nil
}

func (bc *PersistentBlockchain) AddBlockByHash(height uint64, prevHash []byte, blockHash []byte, proposer []byte) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if height != bc.height+1 {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidHeight, bc.height+1, height)
	}

	if !crypto.EqualHash(prevHash, bc.tipHash) {
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

	_, err := bc.economics.ProcessBlock(block.Transactions, block.Header.Height)
	if err != nil {
		return fmt.Errorf("economics processing failed: %w", err)
	}

	blockData, err := SerializeBlock(block)
	if err != nil {
		return err
	}

	batch := bc.db.Batch()
	batch.Put(blockKey(height), blockData)
	batch.Put(hashIndexKey(blockHash), uint64ToBytes(height))
	batch.Put(heightKey(), uint64ToBytes(height))
	batch.Put(tipHashKey(), blockHash)

	if err := batch.Write(); err != nil {
		return err
	}

	bc.height = height
	bc.tipHash = blockHash
	bc.cacheBlock(block)

	return nil
}

func (bc *PersistentBlockchain) GetBlock(height uint64) (*Block, error) {
	bc.mu.RLock()
	if block, ok := bc.cache[height]; ok {
		bc.mu.RUnlock()
		bc.mu.Lock()
		bc.touchCache(height)
		bc.mu.Unlock()
		return block, nil
	}
	bc.mu.RUnlock()

	data, err := bc.db.Get(blockKey(height))
	if err != nil {
		return nil, fmt.Errorf("block at height %d not found", height)
	}

	block, err := DeserializeBlock(data)
	if err != nil {
		return nil, err
	}

	bc.mu.Lock()
	bc.cacheBlock(block)
	bc.mu.Unlock()
	return block, nil
}

func (bc *PersistentBlockchain) GetBlockByHash(hash string) (*Block, error) {
	hashStr := hash
	if len(hashStr) >= 2 && hashStr[:2] == "0x" {
		hashStr = hashStr[2:]
	}

	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hash format: %w", err)
	}

	data, err := bc.db.Get(hashIndexKey(hashBytes))
	if err != nil {
		return nil, fmt.Errorf("block with hash %s not found", hash)
	}
	height := bytesToUint64(data)

	block, err := bc.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (bc *PersistentBlockchain) GetBlocks(from, to uint64) ([]*Block, error) {
	bc.mu.RLock()
	currentHeight := bc.height
	bc.mu.RUnlock()

	if from > currentHeight {
		return nil, fmt.Errorf("from height %d exceeds current height %d", from, currentHeight)
	}

	if to > currentHeight {
		to = currentHeight
	}

	var blocks []*Block
	var errs []error
	for h := from; h <= to; h++ {
		block, err := bc.GetBlock(h)
		if err != nil {
			errs = append(errs, fmt.Errorf("height %d: %w", h, err))
			continue
		}
		blocks = append(blocks, block)
	}
	if len(errs) > 0 {
		return blocks, fmt.Errorf("block retrieval errors: %v", errs)
	}

	return blocks, nil
}

func (bc *PersistentBlockchain) LatestBlock() (*Block, error) {
	bc.mu.RLock()
	currentHeight := bc.height
	bc.mu.RUnlock()
	return bc.GetBlock(currentHeight)
}

func (bc *PersistentBlockchain) Height() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.height
}

func (bc *PersistentBlockchain) TipHash() []byte {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.tipHash
}

func (bc *PersistentBlockchain) TxPool() *TxPool {
	return bc.txPool
}

func (bc *PersistentBlockchain) TxIndex() *TxIndex {
	return bc.txIndex
}

func (bc *PersistentBlockchain) GetTransaction(hash []byte) (*TxIndexEntry, error) {
	return bc.txIndex.GetTransaction(hash)
}

func (bc *PersistentBlockchain) SaveReceipts(receipts []*Receipt) error {
	batch := bc.db.Batch()
	for _, receipt := range receipts {
		data, err := SerializeReceipt(receipt)
		if err != nil {
			return err
		}
		batch.Put(receiptKey(receipt.TxHash), data)
	}
	return batch.Write()
}

func (bc *PersistentBlockchain) GetReceipt(txHash []byte) (*Receipt, error) {
	data, err := bc.db.Get(receiptKey(txHash))
	if err != nil {
		return nil, fmt.Errorf("receipt not found: %w", err)
	}
	return DeserializeReceipt(data)
}

func (bc *PersistentBlockchain) ExportStateSnapshot() (*StateSnapshot, error) {
	bc.mu.RLock()
	height := bc.height
	tipHash := bc.tipHash
	bc.mu.RUnlock()

	snapshot := &StateSnapshot{
		BlockHeight: height,
		RootHash:    fmt.Sprintf("0x%x", tipHash),
		Accounts:    make(map[string]*AccountEntry),
		Timestamp:   time.Now().UTC(),
	}

	return snapshot, nil
}

func (bc *PersistentBlockchain) ImportStateSnapshot(snapshot *StateSnapshot) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.height = snapshot.BlockHeight

	hashStr := snapshot.RootHash
	if len(hashStr) >= 2 && hashStr[:2] == "0x" {
		hashStr = hashStr[2:]
	}

	tipHash, err := hex.DecodeString(hashStr)
	if err != nil {
		return fmt.Errorf("invalid root hash: %w", err)
	}

	bc.tipHash = tipHash

	return nil
}

type StateSnapshot struct {
	BlockHeight uint64                  `json:"block_height"`
	RootHash    string                  `json:"root_hash"`
	Accounts    map[string]*AccountEntry `json:"accounts"`
	Timestamp   time.Time               `json:"timestamp"`
}

type AccountEntry struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
	Nonce   uint64 `json:"nonce"`
	Code    string `json:"code"`
	Root    string `json:"root"`
}

func (bc *PersistentBlockchain) Economics() *Economics {
	return bc.economics
}

func (bc *PersistentBlockchain) Validate() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for h := uint64(1); h <= bc.height; h++ {
		block, err := bc.GetBlock(h)
		if err != nil {
			return false
		}

		if !block.Verify() {
			return false
		}

		prevBlock, err := bc.GetBlock(h - 1)
		if err != nil {
			return false
		}

		if !crypto.EqualHash(block.Header.PrevHash, prevBlock.Hash()) {
			return false
		}
	}

	return true
}

func (bc *PersistentBlockchain) Close() error {
	return bc.db.Close()
}

func (bc *PersistentBlockchain) loadFromDB() error {
	heightData, err := bc.db.Get(heightKey())
	if err != nil {
		return fmt.Errorf("chain not initialized")
	}

	bc.height = bytesToUint64(heightData)

	tipHash, err := bc.db.Get(tipHashKey())
	if err != nil {
		return err
	}

	bc.tipHash = tipHash

	return nil
}

func (bc *PersistentBlockchain) cacheBlock(block *Block) {
	if block == nil {
		return
	}
	height := block.Header.Height
	if _, exists := bc.cache[height]; exists {
		bc.cache[height] = block
		bc.touchCache(height)
		return
	}

	bc.cache[height] = block
	bc.cacheOrder = append(bc.cacheOrder, height)
	if bc.cacheCap > 0 && len(bc.cacheOrder) > bc.cacheCap {
		evict := bc.cacheOrder[0]
		bc.cacheOrder = bc.cacheOrder[1:]
		delete(bc.cache, evict)
	}
}

func (bc *PersistentBlockchain) touchCache(height uint64) {
	if len(bc.cacheOrder) == 0 {
		return
	}

	idx := -1
	for i, h := range bc.cacheOrder {
		if h == height {
			idx = i
			break
		}
	}

	if idx == -1 || idx == len(bc.cacheOrder)-1 {
		return
	}

	bc.cacheOrder = append(bc.cacheOrder[:idx], bc.cacheOrder[idx+1:]...)
	bc.cacheOrder = append(bc.cacheOrder, height)
}

func blockKey(height uint64) []byte {
	return append([]byte{0x01}, uint64ToBytes(height)...)
}

func hashIndexKey(hash []byte) []byte {
	return append([]byte{0x02}, hash...)
}

func heightKey() []byte {
	return []byte{0x03, 0x00}
}

func tipHashKey() []byte {
	return []byte{0x03, 0x01}
}

func receiptKey(hash []byte) []byte {
	return append([]byte{0x05}, hash...)
}

func bytesToUint64(b []byte) uint64 {
	var n uint64
	for i := 0; i < 8; i++ {
		n = (n << 8) | uint64(b[i])
	}
	return n
}
