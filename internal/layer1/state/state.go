package state

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type StateManager struct {
	mu            sync.RWMutex
	db            KVStore
	accountState  *AccountState
	totalSupply   *big.Int
	maxSupply     *big.Int // S-03: cap on total supply
	blockHeight   uint64
	stateRoot     []byte
	mpt           *MerklePatriciaTrie
	dirtyAccounts map[string]bool // tracks accounts modified since last Commit
}

type StateSnapshot struct {
	BlockHeight uint64
	StateRoot   []byte
	TotalSupply *big.Int
	NumAccounts int
}

func NewStateManager(db KVStore) (*StateManager, error) {
	sm := &StateManager{
		db:            db,
		accountState:  NewAccountState(db),
		totalSupply:   big.NewInt(0),
		mpt:           NewMPT(db),
		dirtyAccounts: make(map[string]bool),
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

	// S-01: Guard against re-initialization if state already exists.
	if sm.blockHeight > 0 || sm.totalSupply.Sign() > 0 {
		return nil // already initialized
	}

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
	sm.dirtyAccounts[string(account.Address)] = true
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

	sm.dirtyAccounts[string(address)] = true
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
	sm.dirtyAccounts[string(from)] = true
	sm.dirtyAccounts[string(to)] = true
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

	sm.dirtyAccounts[string(address)] = true
	return sm.accountState.SetAccount(account)
}

func (sm *StateManager) GetStorage(address []byte, key []byte) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	account, err := sm.accountState.GetAccount(address)
	if err != nil {
		return nil, err
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
	sm.dirtyAccounts[string(address)] = true
	return sm.accountState.SetAccount(account)
}

func (sm *StateManager) Commit(blockHeight uint64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.blockHeight = blockHeight

	// Incrementally update state root — only re-hash dirty accounts into the MPT.
	root, err := sm.computeStateRootIncremental()
	if err != nil {
		return fmt.Errorf("failed to compute state root: %w", err)
	}
	sm.stateRoot = root

	// Save the MPT root for this epoch
	if err := sm.mpt.SaveEpochRoot(blockHeight, root); err != nil {
		return fmt.Errorf("failed to save MPT epoch root: %w", err)
	}

	// Prune unreachable nodes from old epochs on commit (keep last 10 epochs)
	const keepWindow = 10
	if blockHeight > keepWindow {
		if _, err := sm.mpt.GarbageCollect(blockHeight - keepWindow); err != nil {
			return fmt.Errorf("failed to garbage collect old MPT nodes: %w", err)
		}
	}

	// Clear dirty set after successful commit
	sm.dirtyAccounts = make(map[string]bool)

	return sm.saveState()
}

// computeStateRootIncremental updates the MPT with only the accounts that
// changed since the last Commit, making this O(k) where k is the number of
// modified accounts rather than O(n) over all accounts.
func (sm *StateManager) computeStateRootIncremental() ([]byte, error) {
	for addrStr := range sm.dirtyAccounts {
		acc, err := sm.accountState.GetAccount([]byte(addrStr))
		if err != nil {
			// Account was deleted — remove from trie
			_ = sm.mpt.Delete([]byte(addrStr))
			continue
		}
		data, err := acc.Serialize()
		if err != nil {
			continue
		}
		if err := sm.mpt.Update([]byte(addrStr), data); err != nil {
			return nil, fmt.Errorf("mpt update for account %x failed: %w", addrStr, err)
		}
	}

	return sm.mpt.Root(), nil
}

// computeStateRoot is the legacy O(n) state root computation.
// Retained for migration and verification purposes.
func (sm *StateManager) computeStateRoot() ([]byte, error) {
	accounts, err := sm.accountState.AllAccounts()
	if err != nil || len(accounts) == 0 {
		return crypto.SHA256([]byte("empty-state")), nil
	}

	leaves := make([][]byte, 0, len(accounts))
	for _, acc := range accounts {
		data, err := acc.Serialize()
		if err != nil {
			continue
		}
		leaves = append(leaves, data)
	}

	if len(leaves) == 0 {
		return crypto.SHA256([]byte("empty-state")), nil
	}

	tree, err := crypto.NewMerkleTree(leaves)
	if err != nil || tree.RootHash == nil {
		return crypto.SHA256([]byte("empty-state")), nil
	}

	return tree.RootHash, nil
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

// Prove generates a Merkle proof for the given key against the current state trie.
func (sm *StateManager) Prove(key []byte) ([][]byte, error) {
	return nil, fmt.Errorf("merkle proofs not yet supported for MPT state trie")
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
	sm.mpt = NewMPT(sm.db)
	sm.dirtyAccounts = make(map[string]bool)
	sm.blockHeight = 0
	sm.stateRoot = crypto.SHA256([]byte("empty-state"))

	return count, sm.saveState()
}

func (sm *StateManager) Close() error {
	return sm.db.Close()
}

func (sm *StateManager) IsInitialized() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.blockHeight > 0 || sm.totalSupply.Sign() > 0
}

// S-03: MintTokens now enforces MaxSupply if set.
func (sm *StateManager) MintTokens(address []byte, amount *big.Int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// S-03: Validate against MaxSupply
	if sm.maxSupply != nil && sm.maxSupply.Sign() > 0 {
		newSupply := new(big.Int).Add(sm.totalSupply, amount)
		if newSupply.Cmp(sm.maxSupply) > 0 {
			return fmt.Errorf("minting %s would exceed max supply %s (current: %s)", amount, sm.maxSupply, sm.totalSupply)
		}
	}

	acct, err := sm.accountState.GetAccount(address)
	if err != nil {
		acct = NewAccount(address, AccountTypeNormal)
	}
	acct.Balance = new(big.Int).Add(acct.Balance, amount)
	sm.totalSupply = new(big.Int).Add(sm.totalSupply, amount)
	sm.dirtyAccounts[string(address)] = true
	return sm.accountState.SetAccount(acct)
}

// S-02: BurnTokens now requires an address and actually deducts from account balance.
func (sm *StateManager) BurnTokens(address []byte, amount *big.Int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	acct, err := sm.accountState.GetAccount(address)
	if err != nil {
		return fmt.Errorf("account not found for burn: %w", err)
	}
	if acct.Balance.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient balance for burn: have %s, burn %s", acct.Balance, amount)
	}

	acct.Balance = new(big.Int).Sub(acct.Balance, amount)
	sm.totalSupply = new(big.Int).Sub(sm.totalSupply, amount)
	sm.dirtyAccounts[string(address)] = true
	return sm.accountState.SetAccount(acct)
}

// SetMaxSupply sets the maximum supply cap (S-03).
func (sm *StateManager) SetMaxSupply(max *big.Int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.maxSupply = new(big.Int).Set(max)
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

func (sm *StateManager) GarbageCollect(beforeEpoch uint64) (uint64, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.mpt.GarbageCollect(beforeEpoch)
}

