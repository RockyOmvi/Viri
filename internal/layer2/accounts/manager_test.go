package accounts

import "testing"

func TestCreateAccount(t *testing.T) {
	am := NewAccountManager()
	addr := []byte("addr1")

	account, err := am.CreateAccount(addr, AccountTypeNormal, 100)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if account.Balance != 100 {
		t.Fatalf("balance mismatch")
	}

	if _, err := am.CreateAccount(addr, AccountTypeNormal, 100); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestGetAccountClone(t *testing.T) {
	am := NewAccountManager()
	addr := []byte("addr")
	account, _ := am.CreateAccount(addr, AccountTypeNormal, 100)
	_ = am.SetStorage(addr, "k", []byte("v"))
	account.Signers = [][]byte{[]byte("s1")}

	loaded, ok := am.GetAccount(addr)
	if !ok {
		t.Fatalf("expected account")
	}
	loaded.Balance = 0
	loaded.Storage["k"] = []byte("x")

	check, _ := am.GetAccount(addr)
	if check.Balance != account.Balance {
		t.Fatalf("clone not isolated")
	}
	if string(check.Storage["k"]) != "v" {
		t.Fatalf("storage clone mismatch")
	}
}

func TestTransfer(t *testing.T) {
	am := NewAccountManager()
	from := []byte("from")
	to := []byte("to")
	_, _ = am.CreateAccount(from, AccountTypeNormal, 100)

	if err := am.Transfer(from, to, 200); err == nil {
		t.Fatalf("expected insufficient balance")
	}

	if err := am.Transfer([]byte("missing"), to, 1); err == nil {
		t.Fatalf("expected missing sender error")
	}

	if err := am.Transfer(from, to, 50); err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	fromAcc, _ := am.GetAccount(from)
	toAcc, _ := am.GetAccount(to)
	if fromAcc.Balance != 50 || toAcc.Balance != 50 {
		t.Fatalf("balance mismatch")
	}
}

func TestStorage(t *testing.T) {
	am := NewAccountManager()
	addr := []byte("addr")

	if err := am.SetStorage(addr, "k", []byte("v")); err == nil {
		t.Fatalf("expected missing account error")
	}

	_, _ = am.CreateAccount(addr, AccountTypeNormal, 0)
	if err := am.SetStorage(addr, "k", []byte("v")); err != nil {
		t.Fatalf("set storage failed: %v", err)
	}

	val, err := am.GetStorage(addr, "k")
	if err != nil {
		t.Fatalf("get storage failed: %v", err)
	}
	if string(val) != "v" {
		t.Fatalf("storage mismatch")
	}

	if _, err := am.GetStorage(addr, "missing"); err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestIncrementNonce(t *testing.T) {
	am := NewAccountManager()
	addr := []byte("addr")

	if err := am.IncrementNonce(addr); err == nil {
		t.Fatalf("expected missing account error")
	}

	acc, _ := am.CreateAccount(addr, AccountTypeNormal, 0)
	if acc.Nonce != 0 {
		t.Fatalf("expected nonce 0")
	}

	if err := am.IncrementNonce(addr); err != nil {
		t.Fatalf("increment failed: %v", err)
	}

	acc2, _ := am.GetAccount(addr)
	if acc2.Nonce != 1 {
		t.Fatalf("expected nonce 1")
	}
}

func TestAccountCount(t *testing.T) {
	am := NewAccountManager()
	if am.AccountCount() != 0 {
		t.Fatalf("expected 0 accounts")
	}
	_, _ = am.CreateAccount([]byte("a"), AccountTypeNormal, 0)
	_, _ = am.CreateAccount([]byte("b"), AccountTypeNormal, 0)
	if am.AccountCount() != 2 {
		t.Fatalf("expected 2 accounts")
	}
}
