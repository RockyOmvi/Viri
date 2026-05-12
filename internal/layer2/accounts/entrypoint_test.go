package accounts

import (
	"crypto/sha256"
	"testing"
)

func TestEntryPoint_New(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1)
	if ep == nil {
		t.Fatal("expected non-nil entry point")
	}
	if ep.manager != mgr {
		t.Fatal("manager not set")
	}
	if ep.chainID != 1 {
		t.Fatal("chainID not set")
	}
}

func TestEntryPoint_DeployAndExecuteWallet(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1)

	initCode := []byte{
		0x60, 0x42,
		0x60, 0x00,
		0x53,
		0x60, 0x01,
		0x60, 0x00,
		0xf3,
	}

	sender := []byte("test-wallet-addr-01")
	op := UserOperation{
		Sender:    sender,
		Nonce:     0,
		InitCode:  initCode,
		CallData:  []byte{},
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}

	results, err := ep.HandleOps([]UserOperation{op}, []byte("beneficiary"))
	if err != nil {
		t.Fatalf("HandleOps failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatal("expected success")
	}

	acc, exists := mgr.GetAccount(sender)
	if !exists {
		t.Fatal("account should exist after deploy")
	}
	if len(acc.CodeHash) == 0 {
		t.Fatal("code hash should be set")
	}
	if len(ep.codeStore[string(acc.CodeHash)]) == 0 {
		t.Fatal("code should be stored in code store")
	}
	if len(results[0].ReturnData) == 0 || results[0].ReturnData[0] != 0x42 {
		t.Fatalf("expected return data 0x42, got %x", results[0].ReturnData)
	}
}

func TestEntryPoint_WalletWithCallData(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1)

	initCode := []byte{
		0x60, 0x42,
		0x60, 0x00,
		0x53,
		0x60, 0x01,
		0x60, 0x00,
		0xf3,
	}

	sender := []byte("test-wallet-data-02")
	op := UserOperation{
		Sender:    sender,
		Nonce:     0,
		InitCode:  initCode,
		CallData:  []byte{0xde, 0xad, 0xbe, 0xef},
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}

	results, err := ep.HandleOps([]UserOperation{op}, []byte("beneficiary"))
	if err != nil {
		t.Fatalf("HandleOps failed: %v", err)
	}
	if !results[0].Success {
		t.Fatal("expected success")
	}
}

func TestEntryPoint_DeployAndReuseWallet(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1)

	initCode := []byte{
		0x60, 0x42, 0x60, 0x00, 0x53,
		0x60, 0x01, 0x60, 0x00, 0xf3,
	}
	sender := []byte("test-wallet-reuse-03")

	op1 := UserOperation{
		Sender:    sender,
		Nonce:     0,
		InitCode:  initCode,
		CallData:  []byte{},
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}
	results1, err := ep.HandleOps([]UserOperation{op1}, []byte("beneficiary"))
	if err != nil {
		t.Fatalf("first HandleOps failed: %v", err)
	}
	if !results1[0].Success {
		t.Fatal("first op should succeed")
	}

	// Fund the wallet for the second operation
	mgr.CreateAccount([]byte("faucet"), AccountTypeNormal, 10000000)
	mgr.Transfer([]byte("faucet"), sender, 1000000)

	op2 := UserOperation{
		Sender:    sender,
		Nonce:     1,
		CallData:  []byte{},
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}
	results2, err := ep.HandleOps([]UserOperation{op2}, []byte("beneficiary"))
	if err != nil {
		t.Fatalf("second HandleOps failed: %v", err)
	}
	if !results2[0].Success {
		t.Fatal("second op should succeed")
	}
	if len(results2[0].ReturnData) == 0 || results2[0].ReturnData[0] != 0x42 {
		t.Fatalf("expected return data 0x42 on reuse, got %x", results2[0].ReturnData)
	}
}

func TestEntryPoint_MissingSender(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1)

	op := UserOperation{
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}
	_, err := ep.HandleOps([]UserOperation{op}, []byte("beneficiary"))
	if err == nil {
		t.Fatal("expected error for missing sender")
	}
}

func TestEntryPoint_InvalidNonce(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1)

	acc := &Account{
		Address: []byte("test-nonce-04"),
		Nonce:   5,
		Storage: make(map[string][]byte),
	}
	_ = mgr.SetAccountDirect([]byte("test-nonce-04"), acc)

	op := UserOperation{
		Sender:    []byte("test-nonce-04"),
		Nonce:     0,
		CallData:  []byte{},
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}
	_, err := ep.HandleOps([]UserOperation{op}, []byte("beneficiary"))
	if err == nil {
		t.Fatal("expected error for invalid nonce")
	}
}

func TestEntryPoint_BeneficiaryFees(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1)

	sender := []byte("test-fees-05")
	beneficiary := []byte("beneficiary-05")
	mgr.CreateAccount(sender, AccountTypeSmartWallet, 1000000)

	code := []byte{
		0x60, 0x42, 0x60, 0x00, 0x53,
		0x60, 0x01, 0x60, 0x00, 0xf3,
	}
	h := sha256.Sum256(code)
	codeHash := h[:]
	ep.codeStore[string(codeHash)] = code

	acc, _ := mgr.GetAccount(sender)
	acc.CodeHash = codeHash
	mgr.SetAccountDirect(sender, acc)

	op := UserOperation{
		Sender:    sender,
		Nonce:     0,
		CallData:  []byte{},
		GasLimit:  50000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}
	results, err := ep.HandleOps([]UserOperation{op}, beneficiary)
	if err != nil {
		t.Fatalf("HandleOps failed: %v", err)
	}
	if !results[0].Success {
		t.Fatal("expected success")
	}
	if results[0].FeeCollected != 500000 {
		t.Fatalf("expected fee 500000, got %d", results[0].FeeCollected)
	}

	benef, exists := mgr.GetAccount(beneficiary)
	if !exists || benef.Balance != 500000 {
		t.Fatalf("expected beneficiary balance 500000, got %d", benef.Balance)
	}
}
