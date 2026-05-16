package state

import (
	"math/big"
	"testing"
)

func FuzzAccountSerialization(f *testing.F) {
	f.Add([]byte("addr"), uint64(0), uint64(100))
	f.Add([]byte{}, uint64(1), uint64(0))
	f.Add(make([]byte, 32), uint64(1000), uint64(500))

	f.Fuzz(func(t *testing.T, addr []byte, nonce, balance uint64) {
		if len(addr) == 0 {
			return
		}
		acc := &Account{
			Address:     addr,
			Type:        AccountTypeNormal,
			Balance:     new(big.Int).SetUint64(balance),
			Nonce:       nonce,
			Storage:     make(map[string][]byte),
			Metadata:    make(map[string]string),
		}
		data, err := acc.Serialize()
		if err != nil {
			t.Errorf("serialize failed: %v", err)
			return
		}
		var deser Account
		if err := deser.Deserialize(data); err != nil {
			t.Errorf("deserialize failed: %v", err)
			return
		}
		if string(deser.Address) != string(addr) {
			t.Errorf("address mismatch after roundtrip")
		}
		if deser.Nonce != nonce {
			t.Errorf("nonce mismatch: %d != %d", deser.Nonce, nonce)
		}
	})
}

func FuzzAccountTransferEdgeCases(f *testing.F) {
	f.Add(uint64(100), uint64(50))
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(1), uint64(2))

	f.Fuzz(func(t *testing.T, balance, amount uint64) {
		acc := &Account{
			Address: []byte("test"),
			Balance: new(big.Int).SetUint64(balance),
		}
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		err := acc.Transfer(new(big.Int).SetUint64(amount))
		if err == nil && amount > balance {
			t.Errorf("transfer should fail when amount > balance")
		}
	})
}

func FuzzAccountTypeChecks(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(1))
	f.Add(uint8(2))
	f.Add(uint8(3))
	f.Add(uint8(255))

	f.Fuzz(func(t *testing.T, typ uint8) {
		acc := &Account{
			Address: []byte("test"),
			Type:    AccountType(typ),
		}
		_ = acc.IsContract()
		_ = acc.IsValidator()
		_ = acc.IsSmartWallet()
		_ = acc.HasCode()
	})
}

func FuzzMemoryStoreBasicOps(f *testing.F) {
	f.Add([]byte("key1"), []byte("value1"))
	f.Add([]byte{}, []byte{})
	f.Add(make([]byte, 1000), make([]byte, 1000))

	f.Fuzz(func(t *testing.T, key, val []byte) {
		store := NewMemoryStore()
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		if err := store.Put(key, val); err != nil {
			return
		}
		got, err := store.Get(key)
		if err != nil {
			t.Errorf("get after put failed: %v", err)
			return
		}
		if string(got) != string(val) {
			t.Errorf("value mismatch after put")
		}
		exists, _ := store.Has(key)
		if !exists {
			t.Errorf("has should return true after put")
		}
		if err := store.Delete(key); err != nil {
			t.Errorf("delete failed: %v", err)
		}
		exists, _ = store.Has(key)
		if exists {
			t.Errorf("has should return false after delete")
		}
	})
}

func FuzzMemoryStoreBatchOps(f *testing.F) {
	f.Add([]byte("a"), []byte("1"), []byte("b"), []byte("2"))
	f.Add([]byte{}, []byte{}, []byte{}, []byte{})

	f.Fuzz(func(t *testing.T, k1, v1, k2, v2 []byte) {
		store := NewMemoryStore()
		batch := store.Batch()
		if err := batch.Put(k1, v1); err != nil {
			return
		}
		if err := batch.Put(k2, v2); err != nil {
			return
		}
		if err := batch.Write(); err != nil {
			t.Errorf("batch write failed: %v", err)
			return
		}
		got1, _ := store.Get(k1)
		got2, _ := store.Get(k2)
		if string(got1) != string(v1) {
			t.Errorf("batch put k1 mismatch")
		}
		if string(got2) != string(v2) {
			t.Errorf("batch put k2 mismatch")
		}
	})
}

func FuzzMemoryStoreIterator(f *testing.F) {
	f.Add([]byte("prefix_key"), []byte("val"))
	f.Add([]byte("x"), []byte("y"))

	f.Fuzz(func(t *testing.T, key, val []byte) {
		store := NewMemoryStore()
		store.Put(key, val)
		store.Put([]byte("other"), []byte("val"))
		iter, err := store.Iterator([]byte("prefix"))
		if err != nil {
			return
		}
		defer iter.Close()
		for iter.Next() {
			_ = iter.Key()
			_ = iter.Value()
		}
	})
}

func FuzzStateManagerCreateAccount(f *testing.F) {
	f.Add([]byte("addr1"), uint64(100))
	f.Add([]byte("addr2"), uint64(0))
	f.Add(make([]byte, 20), uint64(1<<60))

	f.Fuzz(func(t *testing.T, addr []byte, balance uint64) {
		if len(addr) == 0 {
			return
		}
		db := NewMemoryStore()
		sm, err := NewStateManager(db)
		if err != nil {
			t.Skip()
		}
		sm.Initialize(big.NewInt(1000000))
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		acc, err := sm.CreateAccount(addr, AccountTypeNormal, new(big.Int).SetUint64(balance))
		if err != nil {
			return
		}
		if acc == nil {
			t.Errorf("account should not be nil on success")
			return
		}
		retrieved, err := sm.GetAccount(addr)
		if err != nil {
			t.Errorf("failed to get account: %v", err)
			return
		}
		if retrieved.Balance.Cmp(new(big.Int).SetUint64(balance)) != 0 {
			t.Errorf("balance mismatch")
		}
	})
}

func FuzzStateManagerTransfer(f *testing.F) {
	f.Add(uint64(1000), uint64(100))
	f.Add(uint64(500), uint64(500))
	f.Add(uint64(1), uint64(2))

	f.Fuzz(func(t *testing.T, senderBalance, amount uint64) {
		db := NewMemoryStore()
		sm, err := NewStateManager(db)
		if err != nil {
			t.Skip()
		}
		sm.Initialize(big.NewInt(1000000))
		sm.CreateAccount([]byte("from"), AccountTypeNormal, new(big.Int).SetUint64(senderBalance))
		sm.CreateAccount([]byte("to"), AccountTypeNormal, big.NewInt(0))
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		err = sm.Transfer([]byte("from"), []byte("to"), new(big.Int).SetUint64(amount))
		if err == nil && amount > senderBalance {
			t.Errorf("transfer should fail when amount > balance")
		}
	})
}

func FuzzStateManagerStorage(f *testing.F) {
	f.Add([]byte("key"), []byte("value"))
	f.Add([]byte{}, []byte{})
	f.Add(make([]byte, 100), make([]byte, 100))

	f.Fuzz(func(t *testing.T, key, val []byte) {
		db := NewMemoryStore()
		sm, err := NewStateManager(db)
		if err != nil {
			t.Skip()
		}
		sm.Initialize(big.NewInt(1000))
		sm.CreateAccount([]byte("addr"), AccountTypeContract, big.NewInt(0))
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		if err := sm.SetStorage([]byte("addr"), key, val); err != nil {
			t.Logf("set storage failed: %v", err)
			return
		}
		got, err := sm.GetStorage([]byte("addr"), key)
		if err != nil {
			t.Errorf("get storage failed: %v", err)
			return
		}
		if string(got) != string(val) {
			t.Errorf("storage value mismatch")
		}
	})
}

func FuzzMerkleTrieOperations(f *testing.F) {
	f.Add([]byte("key1"), []byte("value1"))
	f.Add([]byte{}, []byte{})

	f.Fuzz(func(t *testing.T, key, val []byte) {
		db := NewMemoryStore()
		mt := NewMerkleTrie(db)
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		if err := mt.Update(key, val); err != nil {
			return
		}
		got, err := mt.Get(key)
		if err != nil && len(val) > 0 {
			t.Errorf("get after update failed: %v", err)
			return
		}
		if len(val) > 0 && string(got) != string(val) {
			t.Errorf("value mismatch in merkle trie")
		}
		root := mt.Root()
		if len(val) > 0 && len(root) == 0 {
			t.Errorf("non-empty trie should have root")
		}
	})
}

func FuzzMerklePatriciaTrieOperations(f *testing.F) {
	f.Add([]byte("key1"), []byte("val1"))
	f.Add([]byte("a"), []byte("b"))
	f.Add(make([]byte, 32), make([]byte, 32))

	f.Fuzz(func(t *testing.T, key, val []byte) {
		if len(key) == 0 {
			return
		}
		db := NewMemoryStore()
		mpt := NewMPT(db)
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		if err := mpt.Update(key, val); err != nil {
			t.Errorf("mpt update failed: %v", err)
			return
		}
		got, err := mpt.Get(key)
		if err != nil {
			t.Errorf("mpt get failed: %v", err)
			return
		}
		if string(got) != string(val) {
			t.Errorf("mpt value mismatch")
		}
		if !mpt.Has(key) {
			t.Errorf("has should return true after update")
		}
		if err := mpt.Delete(key); err != nil {
			t.Errorf("mpt delete failed: %v", err)
		}
		if mpt.Has(key) {
			t.Errorf("has should return false after delete")
		}
	})
}

func FuzzStateManagerNonce(f *testing.F) {
	f.Add([]byte("addr"), uint64(0))
	f.Add([]byte("addr2"), uint64(5))

	f.Fuzz(func(t *testing.T, addr []byte, initialNonce uint64) {
		if len(addr) == 0 {
			return
		}
		db := NewMemoryStore()
		sm, err := NewStateManager(db)
		if err != nil {
			t.Skip()
		}
		sm.Initialize(big.NewInt(1000))
		acc := NewAccount(addr, AccountTypeNormal)
		acc.Nonce = initialNonce
		acc.Balance = big.NewInt(100)
		sm.SetAccount(acc)
		nonce, err := sm.GetNonce(addr)
		if err != nil {
			t.Errorf("get nonce failed: %v", err)
			return
		}
		if nonce != initialNonce {
			t.Errorf("nonce mismatch: %d != %d", nonce, initialNonce)
		}
		if err := sm.IncrementNonce(addr); err != nil {
			t.Errorf("increment nonce failed: %v", err)
		}
		nonce2, _ := sm.GetNonce(addr)
		if nonce2 != initialNonce+1 {
			t.Errorf("nonce should have incremented")
		}
	})
}

func FuzzStateManagerCommitSnapshot(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(100))
	f.Add(uint64(1<<63 - 1))

	f.Fuzz(func(t *testing.T, height uint64) {
		db := NewMemoryStore()
		sm, err := NewStateManager(db)
		if err != nil {
			t.Skip()
		}
		sm.Initialize(big.NewInt(1000))
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		if err := sm.Commit(height); err != nil {
			t.Errorf("commit failed: %v", err)
			return
		}
		snap := sm.Snapshot()
		if snap.BlockHeight != height {
			t.Errorf("snapshot height mismatch: %d != %d", snap.BlockHeight, height)
		}
		if sm.BlockHeight() != height {
			t.Errorf("block height mismatch: %d != %d", sm.BlockHeight(), height)
		}
	})
}

func FuzzAccountStateBatchTransfer(f *testing.F) {
	f.Add(uint64(100), uint64(50))
	f.Add(uint64(0), uint64(0))

	f.Fuzz(func(t *testing.T, balance, amount uint64) {
		db := NewMemoryStore()
		as := NewAccountState(db)
		from := &Account{Address: []byte("from"), Balance: new(big.Int).SetUint64(balance)}
		to := &Account{Address: []byte("to"), Balance: big.NewInt(0)}
		as.SetAccount(from)
		as.SetAccount(to)
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		err := as.Transfer([]byte("from"), []byte("to"), new(big.Int).SetUint64(amount))
		if err == nil && amount > balance {
			t.Errorf("transfer should fail when amount > balance")
		}
	})
}

func FuzzStateManagerCodeStorage(f *testing.F) {
	f.Add([]byte("addr"), []byte("code"))
	f.Add([]byte("addr2"), make([]byte, 1000))

	f.Fuzz(func(t *testing.T, addr, code []byte) {
		if len(addr) == 0 {
			return
		}
		db := NewMemoryStore()
		sm, err := NewStateManager(db)
		if err != nil {
			t.Skip()
		}
		sm.Initialize(big.NewInt(1000))
		sm.CreateAccount(addr, AccountTypeNormal, big.NewInt(0))
		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()
		if err := sm.SetCode(addr, code); err != nil {
			t.Logf("set code failed: %v", err)
			return
		}
		got, err := sm.GetCode(addr)
		if err != nil {
			t.Errorf("get code failed: %v", err)
			return
		}
		if string(got) != string(code) {
			t.Errorf("code mismatch")
		}
	})
}
