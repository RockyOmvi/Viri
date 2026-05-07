package state

import (
	"math/big"
	"testing"
)

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	key := []byte("test-key")
	value := []byte("test-value")

	if err := store.Put(key, value); err != nil {
		t.Fatalf("Failed to put value: %v", err)
	}

	retrieved, err := store.Get(key)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", value, retrieved)
	}

	exists, err := store.Has(key)
	if err != nil {
		t.Fatalf("Failed to check key existence: %v", err)
	}

	if !exists {
		t.Error("Key should exist")
	}

	if err := store.Delete(key); err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	exists, err = store.Has(key)
	if err != nil {
		t.Fatalf("Failed to check key existence after delete: %v", err)
	}

	if exists {
		t.Error("Key should not exist after delete")
	}
}

func TestMemoryStoreBatch(t *testing.T) {
	store := NewMemoryStore()

	batch := store.Batch()

	batch.Put([]byte("key1"), []byte("value1"))
	batch.Put([]byte("key2"), []byte("value2"))
	batch.Delete([]byte("key1"))

	if err := batch.Write(); err != nil {
		t.Fatalf("Failed to write batch: %v", err)
	}

	exists, _ := store.Has([]byte("key1"))
	if exists {
		t.Error("key1 should have been deleted")
	}

	value, err := store.Get([]byte("key2"))
	if err != nil {
		t.Fatalf("Failed to get key2: %v", err)
	}

	if string(value) != "value2" {
		t.Errorf("Expected value2, got %s", value)
	}
}

func TestMemoryStoreIterator(t *testing.T) {
	store := NewMemoryStore()

	prefix := []byte("test:")
	store.Put(append(prefix, []byte("a")...), []byte("1"))
	store.Put(append(prefix, []byte("b")...), []byte("2"))
	store.Put(append(prefix, []byte("c")...), []byte("3"))
	store.Put([]byte("other"), []byte("4"))

	iter, err := store.Iterator(prefix)
	if err != nil {
		t.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	count := 0
	for iter.Next() {
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 items, got %d", count)
	}
}

func TestAccountCreation(t *testing.T) {
	addr := []byte("test-address")
	account := NewAccount(addr, AccountTypeNormal)

	if len(account.Address) == 0 {
		t.Error("Account address is empty")
	}

	if account.Type != AccountTypeNormal {
		t.Errorf("Expected normal account type, got %v", account.Type)
	}

	if account.Balance.Sign() != 0 {
		t.Errorf("Expected zero balance, got %s", account.Balance.String())
	}

	if account.Nonce != 0 {
		t.Errorf("Expected zero nonce, got %d", account.Nonce)
	}
}

func TestAccountTransfer(t *testing.T) {
	account := NewAccount([]byte("addr"), AccountTypeNormal)
	account.Balance = big.NewInt(1000)

	if err := account.Transfer(big.NewInt(500)); err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	if account.Balance.Cmp(big.NewInt(500)) != 0 {
		t.Errorf("Expected balance 500, got %s", account.Balance.String())
	}

	if err := account.Transfer(big.NewInt(600)); err == nil {
		t.Error("Transfer should have failed for insufficient balance")
	}
}

func TestAccountDeposit(t *testing.T) {
	account := NewAccount([]byte("addr"), AccountTypeNormal)
	account.Balance = big.NewInt(100)

	account.Deposit(big.NewInt(500))

	if account.Balance.Cmp(big.NewInt(600)) != 0 {
		t.Errorf("Expected balance 600, got %s", account.Balance.String())
	}
}

func TestAccountNonce(t *testing.T) {
	account := NewAccount([]byte("addr"), AccountTypeNormal)

	for i := 1; i <= 5; i++ {
		account.IncrementNonce()
		if account.Nonce != uint64(i) {
			t.Errorf("Expected nonce %d, got %d", i, account.Nonce)
		}
	}
}

func TestAccountSerialization(t *testing.T) {
	addr := []byte("test-address")
	account := NewAccount(addr, AccountTypeContract)
	account.Balance = big.NewInt(1234)
	account.Nonce = 42
	account.Code = []byte{0x60, 0x00, 0x60, 0x00}

	data, err := account.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize account: %v", err)
	}

	var restored Account
	if err := restored.Deserialize(data); err != nil {
		t.Fatalf("Failed to deserialize account: %v", err)
	}

	if restored.Nonce != account.Nonce {
		t.Errorf("Nonce mismatch: expected %d, got %d", account.Nonce, restored.Nonce)
	}

	if restored.Balance.Cmp(account.Balance) != 0 {
		t.Errorf("Balance mismatch: expected %s, got %s", account.Balance.String(), restored.Balance.String())
	}
}

func TestAccountState(t *testing.T) {
	store := NewMemoryStore()
	accountState := NewAccountState(store)

	addr := []byte("test-account")
	account := NewAccount(addr, AccountTypeNormal)
	account.Balance = big.NewInt(5000)

	if err := accountState.SetAccount(account); err != nil {
		t.Fatalf("Failed to set account: %v", err)
	}

	retrieved, err := accountState.GetAccount(addr)
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}

	if retrieved.Balance.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("Expected balance 5000, got %s", retrieved.Balance.String())
	}

	exists, err := accountState.HasAccount(addr)
	if err != nil {
		t.Fatalf("Failed to check account existence: %v", err)
	}

	if !exists {
		t.Error("Account should exist")
	}

	balance, err := accountState.GetBalance(addr)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}

	if balance.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("Expected balance 5000, got %s", balance.String())
	}
}

func TestStateManager(t *testing.T) {
	store := NewMemoryStore()
	stateMgr, err := NewStateManager(store)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	if err := stateMgr.Initialize(big.NewInt(1_000_000)); err != nil {
		t.Fatalf("Failed to initialize state: %v", err)
	}

	addr := []byte("test-account")
	account, err := stateMgr.CreateAccount(addr, AccountTypeNormal, big.NewInt(5000))
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	if account.Balance.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("Expected balance 5000, got %s", account.Balance.String())
	}

	balance, err := stateMgr.GetBalance(addr)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}

	if balance.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("Expected balance 5000, got %s", balance.String())
	}

	if err := stateMgr.IncrementNonce(addr); err != nil {
		t.Fatalf("Failed to increment nonce: %v", err)
	}

	nonce, err := stateMgr.GetNonce(addr)
	if err != nil {
		t.Fatalf("Failed to get nonce: %v", err)
	}

	if nonce != 1 {
		t.Errorf("Expected nonce 1, got %d", nonce)
	}

	if err := stateMgr.Commit(1); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	snapshot := stateMgr.Snapshot()
	if snapshot.BlockHeight != 1 {
		t.Errorf("Expected block height 1, got %d", snapshot.BlockHeight)
	}

	if snapshot.NumAccounts != 1 {
		t.Errorf("Expected 1 account, got %d", snapshot.NumAccounts)
	}
}

func TestStateManagerTransfer(t *testing.T) {
	store := NewMemoryStore()
	stateMgr, _ := NewStateManager(store)
	stateMgr.Initialize(big.NewInt(1_000_000))

	from := []byte("sender")
	to := []byte("receiver")

	stateMgr.CreateAccount(from, AccountTypeNormal, big.NewInt(1000))
	stateMgr.CreateAccount(to, AccountTypeNormal, big.NewInt(0))

	if err := stateMgr.Transfer(from, to, big.NewInt(500)); err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	senderBalance, _ := stateMgr.GetBalance(from)
	receiverBalance, _ := stateMgr.GetBalance(to)

	if senderBalance.Cmp(big.NewInt(500)) != 0 {
		t.Errorf("Sender balance: expected 500, got %s", senderBalance.String())
	}

	if receiverBalance.Cmp(big.NewInt(500)) != 0 {
		t.Errorf("Receiver balance: expected 500, got %s", receiverBalance.String())
	}
}

func TestStateManagerDuplicateAccount(t *testing.T) {
	store := NewMemoryStore()
	stateMgr, _ := NewStateManager(store)
	stateMgr.Initialize(big.NewInt(1_000_000))

	addr := []byte("test-account")

	_, err := stateMgr.CreateAccount(addr, AccountTypeNormal, big.NewInt(100))
	if err != nil {
		t.Fatalf("First account creation failed: %v", err)
	}

	_, err = stateMgr.CreateAccount(addr, AccountTypeNormal, big.NewInt(200))
	if err == nil {
		t.Error("Expected error for duplicate account creation")
	}
}

func TestStateManagerSetCode(t *testing.T) {
	store := NewMemoryStore()
	stateMgr, _ := NewStateManager(store)
	stateMgr.Initialize(big.NewInt(1_000_000))

	addr := []byte("contract-account")
	stateMgr.CreateAccount(addr, AccountTypeNormal, big.NewInt(0))

	code := []byte{0x60, 0x00, 0x60, 0x00}
	if err := stateMgr.SetCode(addr, code); err != nil {
		t.Fatalf("Failed to set code: %v", err)
	}

	retrievedCode, err := stateMgr.GetCode(addr)
	if err != nil {
		t.Fatalf("Failed to get code: %v", err)
	}

	if string(retrievedCode) != string(code) {
		t.Errorf("Code mismatch: expected %x, got %x", code, retrievedCode)
	}

	account, _ := stateMgr.GetAccount(addr)
	if !account.IsContract() {
		t.Error("Account should be contract type after code set")
	}
}

func TestStateManagerClose(t *testing.T) {
	store := NewMemoryStore()
	stateMgr, _ := NewStateManager(store)
	stateMgr.Initialize(big.NewInt(1_000_000))

	if err := stateMgr.Close(); err != nil {
		t.Fatalf("Failed to close state manager: %v", err)
	}
}
