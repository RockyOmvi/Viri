package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const mempoolFilename = "mempool.json"

type MempoolPersister struct {
	mu       sync.Mutex
	dataDir  string
}

func NewMempoolPersister(dataDir string) *MempoolPersister {
	return &MempoolPersister{dataDir: dataDir}
}

func (mp *MempoolPersister) Save(pool *TxPool) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	txs := pool.GetPending()
	if len(txs) == 0 {
		return nil
	}

	data, err := json.Marshal(txs)
	if err != nil {
		return fmt.Errorf("failed to marshal mempool: %w", err)
	}

	path := filepath.Join(mp.dataDir, mempoolFilename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write mempool: %w", err)
	}

	return nil
}

func (mp *MempoolPersister) Load() ([]*Transaction, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	path := filepath.Join(mp.dataDir, mempoolFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read mempool: %w", err)
	}

	var txs []*Transaction
	if err := json.Unmarshal(data, &txs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mempool: %w", err)
	}

	os.Remove(path)

	return txs, nil
}

func (mp *MempoolPersister) Clear() error {
	path := filepath.Join(mp.dataDir, mempoolFilename)
	return os.Remove(path)
}
