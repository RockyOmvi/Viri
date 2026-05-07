package ledger

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/viri-chain/viri/internal/layer1/state"
)

type TxIndex struct {
	mu sync.RWMutex
	db state.KVStore
}

type TxIndexEntry struct {
	Hash    string `json:"hash"`
	Height  uint64 `json:"height"`
	Index   int    `json:"index"`
	From    string `json:"from"`
	To      string `json:"to"`
	Value   uint64 `json:"value"`
	GasUsed uint64 `json:"gas_used"`
}

func NewTxIndex(db state.KVStore) *TxIndex {
	return &TxIndex{db: db}
}

func (ti *TxIndex) IndexTransaction(tx *Transaction, height uint64, index int) error {
	entry := TxIndexEntry{
		Hash:   fmt.Sprintf("0x%x", tx.Hash),
		Height: height,
		Index:  index,
		From:   fmt.Sprintf("0x%x", tx.From),
		To:     fmt.Sprintf("0x%x", tx.To),
		Value:  tx.Value,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal tx index entry: %w", err)
	}

	return ti.db.Put(txIndexKey(tx.Hash), data)
}

func (ti *TxIndex) IndexTransactions(txs []*Transaction, height uint64) error {
	for i, tx := range txs {
		if err := ti.IndexTransaction(tx, height, i); err != nil {
			return fmt.Errorf("failed to index tx at index %d: %w", i, err)
		}
	}
	return nil
}

func (ti *TxIndex) GetTransaction(hash []byte) (*TxIndexEntry, error) {
	data, err := ti.db.Get(txIndexKey(hash))
	if err != nil {
		return nil, fmt.Errorf("transaction not found")
	}

	var entry TxIndexEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tx index entry: %w", err)
	}

	return &entry, nil
}

func (ti *TxIndex) GetTransactionsByAddress(addr []byte, limit int) ([]TxIndexEntry, error) {
	addrStr := fmt.Sprintf("0x%x", addr)

	iterStore, ok := ti.db.(state.IterableKVStore)
	if !ok {
		return nil, fmt.Errorf("database does not support iteration")
	}

	it, err := iterStore.Iterator(txIndexPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer it.Close()

	var results []TxIndexEntry
	for it.Next() && len(results) < limit {
		var entry TxIndexEntry
		if err := json.Unmarshal(it.Value(), &entry); err != nil {
			continue
		}
		if entry.From == addrStr || entry.To == addrStr {
			results = append(results, entry)
		}
	}

	return results, nil
}

func (ti *TxIndex) BuildFromChain(bc *PersistentBlockchain, fromHeight, toHeight uint64) error {
	for h := fromHeight; h <= toHeight; h++ {
		block, err := bc.GetBlock(h)
		if err != nil {
			continue
		}
		if err := ti.IndexTransactions(block.Transactions, h); err != nil {
			return fmt.Errorf("failed to build index for block %d: %w", h, err)
		}
	}
	return nil
}

func txIndexKey(hash []byte) []byte {
	return append([]byte{0x04}, hash...)
}

func txIndexPrefix() []byte {
	return []byte{0x04}
}
