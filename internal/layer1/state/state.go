package state

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type StateManager struct {
	mu           sync.RWMutex
	db           KVStore
	accountState *AccountState
	totalSupply  *big.Int
	blockHeight  uint64
	stateRoot    []byte
}

type StateSnapshot struct {
	BlockHeight uint64
	StateRoot   []byte
	TotalSupply *big.Int
	NumAccounts int
}

func NewStateManager(db KVStore) (*StateManager, error) {
	sm := &StateManager{
		db:           db,
		accountState: NewAccountState(db),
		totalSupply:  big.NewInt(0),
	}

	if err := sm.loadState(); err != nil {
		if err.Error() == "state not initialized" {
			return sm, nil
		}
		return nil, err
	}

	return sm, nil
}

func (sm *StateManager) Initialize(totalSupply *big.Int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.totalSupply = new(big.Int).Set(totalSupply)
	sm.blockHeight = 0
	sm.stateRoot = crypto.SHA256([]byte("empty-state"))

	return sm.saveState()
}

func (sm *StateManager) GetAccount(address []byte) (*Account, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.accountState.GetAccount(address)
}

func (sm *StateManager) SetAccount(account *Account) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.accountState.SetAccount(account)
}

func (sm *StateManager) CreateAccount(address []byte, accountType AccountType, initialBalance *big.Int) (*Account, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	exists, _ := sm.accountState.HasAccount(address)
	if exists {
		return nil, fmt.Errorf("account already exists")
	}

	account := NewAccount(address, accountType)
	account.Balance = new(big.Int).Set(initialBalance)

	if err := sm.accountState.SetAccount(account); err != nil {
		return nil, err
	}

	return account, nil
}

func (sm *StateManager) GetBalance(address []byte) (*big.Int, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.accountState.GetBalance(address)
}

func (sm *StateManager) GetNonce(address []byte) (uint64, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.accountState.GetNonce(address)
}

func (sm *StateManager) IncrementNonce(address []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	account, err := sm.accountState.GetAccount(address)
	if err != nil {
		return err
	}

	account.IncrementNonce()
	return sm.accountState.SetAccount(account)
}

func (sm *StateManager) Transfer(from, to []byte, amount *big.Int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.accountState.Transfer(from, to, amount)
}

func (sm *StateManager) GetCode(address []byte) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.accountState.GetCode(address)
}

func (sm *StateManager) SetCode(address []byte, code []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	account, err := sm.accountState.GetAccount(address)
	if err != nil {
		return err
	}

	account.Code = code
	account.CodeHash = crypto.SHA256(code)
	account.Type = AccountTypeContract

	return sm.accountState.SetAccount(account)
}

func (sm *StateManager) GetStorage(address []byte, key []byte) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	account, err := sm.accountState.GetAccount(address)
	if err != nil {
		return nil, nil
	}
	if account.Storage == nil {
		return nil, nil
	}
	return account.Storage[string(key)], nil
}

func (sm *StateManager) SetStorage(address []byte, key []byte, value []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	account, err := sm.accountState.GetAccount(address)
	if err != nil {
		return err
	}
	if account.Storage == nil {
		account.Storage = make(map[string][]byte)
	}
	account.Storage[string(key)] = value
	return sm.accountState.SetAccount(account)
}

func (sm *StateManager) Commit(blockHeight uint64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.blockHeight = blockHeight

	// Compute real state root from account data
	sm.stateRoot = sm.computeStateRoot()

	return sm.saveState()
}

// computeStateRoot builds a Merkle root from all account state.
func (sm *StateManager) computeStateRoot() []byte {
	accounts, err := sm.accountState.AllAccounts()
	if err != nil || len(accounts) == 0 {
		return crypto.SHA256([]byte("empty-state"))
	}

	// Build leaf hashes from each serialized account
	leaves := make([][]byte, 0, len(accounts))
	for _, acc := range accounts {
		data, err := acc.Serialize()
		if err != nil {
			continue
		}
		leaves = append(leaves, data)
	}

	if len(leaves) == 0 {
		return crypto.SHA256([]byte("empty-state"))
	}

	tree, err := crypto.NewMerkleTree(leaves)
	if err != nil || tree.RootHash == nil {
		return crypto.SHA256([]byte("empty-state"))
	}

	return tree.RootHash
}

func (sm *StateManager) Snapshot() *StateSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	accounts, _ := sm.accountState.AllAccounts()

	return &StateSnapshot{
		BlockHeight: sm.blockHeight,
		StateRoot:   sm.stateRoot,
		TotalSupply: new(big.Int).Set(sm.totalSupply),
		NumAccounts: len(accounts),
	}
}

func (sm *StateManager) BlockHeight() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.blockHeight
}

func (sm *StateManager) StateRoot() []byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.stateRoot
}

func (sm *StateManager) TotalSupply() *big.Int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return new(big.Int).Set(sm.totalSupply)
}

func (sm *StateManager) AllAccounts() ([]*Account, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.accountState.AllAccounts()
}

// DeleteBefore prunes state data before the given epoch for light client mode.
// For the current in-memory state model, this resets the state to force re-sync
// from full nodes. Returns the number of accounts pruned.
func (sm *StateManager) DeleteBefore(epoch uint64) (uint64, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	accounts, err := sm.accountState.AllAccounts()
	if err != nil {
		return 0, err
	}
	count := uint64(len(accounts))

	// Reset state — light client will re-fetch from full nodes
	sm.accountState = NewAccountState(sm.db)
	sm.blockHeight = 0
	sm.stateRoot = crypto.SHA256([]byte("empty-state"))

	return count, sm.saveState()
}

func (sm *StateManager) Close() error {
	return sm.db.Close()
}

func (sm *StateManager) loadState() error {
	data, err := sm.db.Get([]byte("__state__"))
	if err != nil {
		return fmt.Errorf("state not initialized")
	}

	var stateData struct {
		BlockHeight uint64  `json:"block_height"`
		StateRoot   []byte  `json:"state_root"`
		TotalSupply *big.Int `json:"total_supply"`
	}

	if err := json.Unmarshal(data, &stateData); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	sm.blockHeight = stateData.BlockHeight
	sm.stateRoot = stateData.StateRoot
	sm.totalSupply = stateData.TotalSupply

	return nil
}

func (sm *StateManager) saveState() error {
	stateData := struct {
		BlockHeight uint64  `json:"block_height"`
		StateRoot   []byte  `json:"state_root"`
		TotalSupply *big.Int `json:"total_supply"`
	}{
		BlockHeight: sm.blockHeight,
		StateRoot:   sm.stateRoot,
		TotalSupply: sm.totalSupply,
	}

	data, err := json.Marshal(stateData)
	if err != nil {
		return err
	}

	return sm.db.Put([]byte("__state__"), data)
}
