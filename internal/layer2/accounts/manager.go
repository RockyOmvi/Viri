package accounts

import (
	"fmt"
	"sync"
)

type AccountType uint8

const (
	AccountTypeNormal AccountType = iota
	AccountTypeSmartWallet
	AccountTypeMultiSig
)

type Account struct {
	Address    []byte
	Type       AccountType
	Balance    uint64
	Nonce      uint64
	Threshold  uint8
	Signers    [][]byte
	CodeHash   []byte
	Storage    map[string][]byte
}

type AccountManager struct {
	mu       sync.RWMutex
	accounts map[string]*Account
}

func NewAccountManager() *AccountManager {
	return &AccountManager{
		accounts: make(map[string]*Account),
	}
}

func (am *AccountManager) CreateAccount(address []byte, accountType AccountType, balance uint64) (*Account, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := string(address)
	if _, exists := am.accounts[key]; exists {
		return nil, fmt.Errorf("account already exists")
	}

	account := &Account{
		Address: address,
		Type:    accountType,
		Balance: balance,
		Storage: make(map[string][]byte),
	}

	am.accounts[key] = account
	return account, nil
}

func (am *AccountManager) GetAccount(address []byte) (*Account, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	account, exists := am.accounts[string(address)]
	if !exists {
		return nil, false
	}

	return account.Clone(), true
}

func (am *AccountManager) Transfer(from, to []byte, amount uint64) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	fromKey := string(from)
	toKey := string(to)

	fromAcc, exists := am.accounts[fromKey]
	if !exists {
		return fmt.Errorf("sender account not found")
	}

	if fromAcc.Balance < amount {
		return fmt.Errorf("insufficient balance")
	}

	toAcc, exists := am.accounts[toKey]
	if !exists {
		toAcc = &Account{
			Address: to,
			Balance: 0,
			Storage: make(map[string][]byte),
		}
		am.accounts[toKey] = toAcc
	}

	fromAcc.Balance -= amount
	toAcc.Balance += amount

	return nil
}

func (am *AccountManager) SetStorage(address []byte, key string, value []byte) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	account, exists := am.accounts[string(address)]
	if !exists {
		return fmt.Errorf("account not found")
	}

	account.Storage[key] = value
	return nil
}

func (am *AccountManager) GetStorage(address []byte, key string) ([]byte, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	account, exists := am.accounts[string(address)]
	if !exists {
		return nil, fmt.Errorf("account not found")
	}

	val, exists := account.Storage[key]
	if !exists {
		return nil, fmt.Errorf("storage key not found")
	}

	return append([]byte(nil), val...), nil
}

func (am *AccountManager) IncrementNonce(address []byte) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	account, exists := am.accounts[string(address)]
	if !exists {
		return fmt.Errorf("account not found")
	}

	account.Nonce++
	return nil
}

func (am *AccountManager) AccountCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.accounts)
}

// SetAccountDirect writes an account directly (used by EntryPoint and Recovery).
func (am *AccountManager) SetAccountDirect(address []byte, acc *Account) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.accounts[string(address)] = acc
	return nil
}

// HasAccount checks whether an account exists.
func (am *AccountManager) HasAccount(address []byte) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	_, exists := am.accounts[string(address)]
	return exists
}

func (a *Account) Clone() *Account {
	cloned := &Account{
		Address:   append([]byte(nil), a.Address...),
		Type:      a.Type,
		Balance:   a.Balance,
		Nonce:     a.Nonce,
		Threshold: a.Threshold,
		CodeHash:  append([]byte(nil), a.CodeHash...),
		Storage:   make(map[string][]byte),
	}

	for k, v := range a.Storage {
		cloned.Storage[k] = append([]byte(nil), v...)
	}

	for _, s := range a.Signers {
		cloned.Signers = append(cloned.Signers, append([]byte(nil), s...))
	}

	return cloned
}
