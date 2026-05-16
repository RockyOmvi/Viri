package accounts

import (
	"math/big"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestEntryPoint_New(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1, nil)
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
	ep := NewEntryPoint(mgr, 1, nil)

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
	ep := NewEntryPoint(mgr, 1, nil)

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
	ep := NewEntryPoint(mgr, 1, nil)

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
	ep := NewEntryPoint(mgr, 1, nil)

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
	ep := NewEntryPoint(mgr, 1, nil)

	acc := &Account{
		Address: []byte("test-nonce-04"),
		Nonce:   5,
		Balance: new(big.Int),
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
	ep := NewEntryPoint(mgr, 1, nil)

	sender := []byte("test-fees-05")
	beneficiary := []byte("beneficiary-05")
	mgr.CreateAccount(sender, AccountTypeSmartWallet, 1000000)

	code := []byte{
		0x60, 0x42, 0x60, 0x00, 0x53,
		0x60, 0x01, 0x60, 0x00, 0xf3,
	}
	codeHash := crypto.Keccak256(code)
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
	if results[0].FeeCollected == 0 {
		t.Fatal("expected non-zero fee collected")
	}
	if results[0].GasUsed == 0 {
		t.Fatal("expected non-zero gas used")
	}
	if results[0].GasUsed > op.GasLimit {
		t.Fatalf("gas used %d exceeds gas limit %d", results[0].GasUsed, op.GasLimit)
	}

	benef, exists := mgr.GetAccount(beneficiary)
	if !exists || benef.Balance.Sign() == 0 {
		t.Fatalf("expected beneficiary to receive fees, got balance %s", benef.Balance.String())
	}
}

func TestEntryPoint_SignatureFailure(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1, nil)

	sender := []byte("test-sig-fail")
	mgr.CreateAccount(sender, AccountTypeSmartWallet, 1000000)

	// Set signers so signature validation runs
	acc, _ := mgr.GetAccount(sender)
	acc.Signers = [][]byte{[]byte("some-signer")}
	acc.Threshold = 1
	mgr.SetAccountDirect(sender, acc)

	op := UserOperation{
		Sender:    sender,
		Nonce:     0,
		CallData:  []byte{},
		GasLimit:  50000,
		MaxFee:    10,
		Signature: []byte{0x01, 0x02, 0x03},
	}
	_, err := ep.HandleOps([]UserOperation{op}, []byte("beneficiary"))
	if err == nil {
		t.Fatal("expected signature validation failure")
	}
}

func TestEntryPoint_DeployWalletOverwrite(t *testing.T) {
	mgr := NewAccountManager()
	ep := NewEntryPoint(mgr, 1, nil)

	sender := []byte("test-overwrite")
	mgr.CreateAccount(sender, AccountTypeNormal, 100)

	initCode := []byte{
		0x60, 0x42, 0x60, 0x00, 0x53,
		0x60, 0x01, 0x60, 0x00, 0xf3,
	}
	op := UserOperation{
		Sender:    sender,
		Nonce:     0,
		InitCode:  initCode,
		CallData:  []byte{},
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}
	_, err := ep.HandleOps([]UserOperation{op}, []byte("beneficiary"))
	if err == nil {
		t.Fatal("expected error deploying to existing account")
	}
}

func TestEntryPoint_UserOpHash(t *testing.T) {
	op := &UserOperation{
		Sender:         []byte("sender"),
		Nonce:          1,
		InitCode:       []byte("init"),
		CallData:       []byte("calldata"),
		GasLimit:       100000,
		MaxFee:         10,
		MaxPriorityFee: 2,
		Paymaster:      []byte("paymaster"),
		PaymasterData:  []byte("pmdata"),
		Signature:      []byte("sig"),
	}
	h1 := UserOpHash(op, []byte("ep"), 1)
	h2 := UserOpHash(op, []byte("ep"), 1)
	if len(h1) == 0 {
		t.Fatal("expected non-empty hash")
	}
	if string(h1) != string(h2) {
		t.Fatal("hash should be deterministic")
	}
	h3 := UserOpHash(op, []byte("ep2"), 1)
	if string(h1) == string(h3) {
		t.Fatal("hash should differ with different entry point")
	}
	h4 := UserOpHash(op, []byte("ep"), 2)
	if string(h1) == string(h4) {
		t.Fatal("hash should differ with different chain ID")
	}
}
