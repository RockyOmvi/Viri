package state

import (
	"encoding/json"
	"fmt"
	"math/big"
)

type AccountType uint8

const (
	AccountTypeNormal AccountType = iota
	AccountTypeContract
	AccountTypeValidator
	AccountTypeSmartWallet
)

type Account struct {
	Address     []byte
	Type        AccountType
	Balance     *big.Int
	Nonce       uint64
	Code        []byte
	CodeHash    []byte
	StorageRoot []byte
	Storage     map[string][]byte
	Metadata    map[string]string
}

type AccountState struct {
	db KVStore
}

func NewAccountState(db KVStore) *AccountState {
	return &AccountState{db: db}
}

func NewAccount(address []byte, accountType AccountType) *Account {
	return &Account{
		Address:  address,
		Type:     accountType,
		Balance:  new(big.Int),
		Nonce:    0,
		Storage:  make(map[string][]byte),
		Metadata: make(map[string]string),
	}
}

func (a *Account) Key() []byte {
	return accountKey(a.Address)
}

func (a *Account) Serialize() ([]byte, error) {
	return json.Marshal(a)
}

func (a *Account) Deserialize(data []byte) error {
	return json.Unmarshal(data, a)
}

func (a *Account) Transfer(amount *big.Int) error {
	if amount.Sign() < 0 {
		return fmt.Errorf("transfer amount must be non-negative")
	}
	a.Balance = new(big.Int).Sub(a.Balance, amount)
	if a.Balance.Sign() < 0 {
		return fmt.Errorf("insufficient balance")
	}
	return nil
}

func (a *Account) Deposit(amount *big.Int) {
	a.Balance = new(big.Int).Add(a.Balance, amount)
}

func (a *Account) IncrementNonce() {
	a.Nonce++
}

func (a *Account) IsContract() bool {
	return a.Type == AccountTypeContract
}

func (a *Account) IsValidator() bool {
	return a.Type == AccountTypeValidator
}

func (a *Account) IsSmartWallet() bool {
	return a.Type == AccountTypeSmartWallet
}

func (a *Account) HasCode() bool {
	return len(a.Code) > 0
}

func (as *AccountState) GetAccount(address []byte) (*Account, error) {
	data, err := as.db.Get(accountKey(address))
	if err != nil {
		return nil, err
	}

	var account Account
	if err := account.Deserialize(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize account: %w", err)
	}

	return &account, nil
}

func (as *AccountState) SetAccount(account *Account) error {
	data, err := account.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize account: %w", err)
	}

	return as.db.Put(accountKey(account.Address), data)
}

func (as *AccountState) DeleteAccount(address []byte) error {
	return as.db.Delete(accountKey(address))
}

func (as *AccountState) HasAccount(address []byte) (bool, error) {
	return as.db.Has(accountKey(address))
}

func (as *AccountState) GetBalance(address []byte) (*big.Int, error) {
	account, err := as.GetAccount(address)
	if err != nil {
		return big.NewInt(0), err
	}
	return account.Balance, nil
}

func (as *AccountState) GetNonce(address []byte) (uint64, error) {
	account, err := as.GetAccount(address)
	if err != nil {
		return 0, err
	}
	return account.Nonce, nil
}

func (as *AccountState) GetCode(address []byte) ([]byte, error) {
	account, err := as.GetAccount(address)
	if err != nil {
		return nil, err
	}
	return account.Code, nil
}

func (as *AccountState) Transfer(from, to []byte, amount *big.Int) error {
	sender, err := as.GetAccount(from)
	if err != nil {
		return fmt.Errorf("sender account not found: %w", err)
	}

	receiver, err := as.GetAccount(to)
	if err != nil {
		return fmt.Errorf("receiver account not found: %w", err)
	}

	if err := sender.Transfer(amount); err != nil {
		return err
	}

	receiver.Deposit(amount)

	batch := as.db.Batch()
	senderData, _ := sender.Serialize()
	receiverData, _ := receiver.Serialize()

	batch.Put(accountKey(from), senderData)
	batch.Put(accountKey(to), receiverData)

	return batch.Write()
}

func (as *AccountState) AllAccounts() ([]*Account, error) {
	iterable, ok := as.db.(IterableKVStore)
	if !ok {
		return nil, fmt.Errorf("database does not support iteration")
	}

	iter, err := iterable.Iterator(accountKeyPrefix())
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var accounts []*Account
	for iter.Next() {
		var account Account
		if err := account.Deserialize(iter.Value()); err != nil {
			continue
		}
		accounts = append(accounts, &account)
	}

	return accounts, nil
}

func accountKey(address []byte) []byte {
	prefix := []byte{0x01}
	return append(prefix, address...)
}

func accountKeyPrefix() []byte {
	return []byte{0x01}
}
