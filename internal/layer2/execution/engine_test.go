package execution

import (
	"math/big"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func TestExecuteTransfer(t *testing.T) {
	engine := NewExecutionEngine()

	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	senderPubKey := senderKey.PubKey().Bytes()
	senderAddr := senderKey.PubKey().Address()
	recipientAddr := []byte{0x02}

	accounts := make(map[string]*AccountState)
	accounts[string(senderAddr)] = &AccountState{
		Address: senderAddr,
		Balance: big.NewInt(1000000),
		Nonce:   0,
		Storage: make(map[string][]byte),
	}

	getAccount := func(addr []byte) (*AccountState, error) {
		acc, exists := accounts[string(addr)]
		if !exists {
			return nil, nil
		}
		return acc, nil
	}

	setAccount := func(addr []byte, acc *AccountState) error {
		accounts[string(addr)] = acc
		return nil
	}

	tx := &ledger.Transaction{
		Nonce:    0,
		From:     senderPubKey,
		To:       recipientAddr,
		Value:    1000,
		GasLimit: 100000,
		GasPrice: 1,
	}

	payload := tx.SigningPayload()
	sig, err := senderKey.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	tx.Signature = &ledger.TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
	}

	tx.Hash = tx.ComputeHash()

	result, err := engine.ExecuteTransaction(tx, 1, getAccount, setAccount)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != 1 {
		t.Errorf("expected success, got error: %v", result.Err)
	}

	if result.GasUsed == 0 {
		t.Error("expected gas to be used")
	}

	sender := accounts[string(senderAddr)]
	if sender.Balance.Cmp(big.NewInt(1000000-1000-int64(result.GasUsed))) < 0 {
		t.Error("sender balance should be reduced")
	}
}

func TestExecuteInvalidNonce(t *testing.T) {
	engine := NewExecutionEngine()

	senderAddr := []byte{0x01}

	accounts := make(map[string]*AccountState)
	accounts[string(senderAddr)] = &AccountState{
		Address: senderAddr,
		Balance: big.NewInt(1000000),
		Nonce:   5,
		Storage: make(map[string][]byte),
	}

	getAccount := func(addr []byte) (*AccountState, error) {
		acc, exists := accounts[string(addr)]
		if !exists {
			return nil, nil
		}
		return acc, nil
	}

	setAccount := func(addr []byte, acc *AccountState) error {
		accounts[string(addr)] = acc
		return nil
	}

	tx := &ledger.Transaction{
		Nonce:    3,
		From:     senderAddr,
		To:       []byte{0x02},
		Value:    100,
		GasLimit: 100000,
		GasPrice: 1,
	}

	result, err := engine.ExecuteTransaction(tx, 1, getAccount, setAccount)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status == 1 {
		t.Error("expected invalid nonce to fail")
	}
}

func TestExecuteInsufficientBalance(t *testing.T) {
	engine := NewExecutionEngine()

	senderAddr := []byte{0x01}

	accounts := make(map[string]*AccountState)
	accounts[string(senderAddr)] = &AccountState{
		Address: senderAddr,
		Balance: big.NewInt(100),
		Nonce:   0,
		Storage: make(map[string][]byte),
	}

	getAccount := func(addr []byte) (*AccountState, error) {
		acc, exists := accounts[string(addr)]
		if !exists {
			return nil, nil
		}
		return acc, nil
	}

	setAccount := func(addr []byte, acc *AccountState) error {
		accounts[string(addr)] = acc
		return nil
	}

	tx := &ledger.Transaction{
		Nonce:    0,
		From:     senderAddr,
		To:       []byte{0x02},
		Value:    1000000,
		GasLimit: 100000,
		GasPrice: 1,
	}

	result, err := engine.ExecuteTransaction(tx, 1, getAccount, setAccount)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status == 1 {
		t.Error("expected insufficient balance to fail")
	}
}

func TestExecuteContractDeploy(t *testing.T) {
	engine := NewExecutionEngine()

	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	senderPubKey := senderKey.PubKey().Bytes()
	senderAddr := senderKey.PubKey().Address()

	accounts := make(map[string]*AccountState)
	accounts[string(senderAddr)] = &AccountState{
		Address: senderAddr,
		Balance: big.NewInt(1000000),
		Nonce:   0,
		Storage: make(map[string][]byte),
	}

	getAccount := func(addr []byte) (*AccountState, error) {
		acc, exists := accounts[string(addr)]
		if !exists {
			return nil, nil
		}
		return acc, nil
	}

	setAccount := func(addr []byte, acc *AccountState) error {
		accounts[string(addr)] = acc
		return nil
	}

	tx := &ledger.Transaction{
		Nonce:    0,
		From:     senderPubKey,
		Data:     []byte{0x60, 0x60, 0x60, 0x40},
		GasLimit: 1000000,
		GasPrice: 1,
	}

	payload := tx.SigningPayload()
	sig, err := senderKey.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	tx.Signature = &ledger.TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
	}

	tx.Hash = tx.ComputeHash()

	result, err := engine.ExecuteTransaction(tx, 1, getAccount, setAccount)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != 1 {
		t.Errorf("expected contract deploy to succeed, got error: %v", result.Err)
	}
}

func TestClassifyTransaction(t *testing.T) {
	transfer := &ledger.Transaction{
		From:  []byte{0x01},
		To:    []byte{0x02},
		Value: 100,
	}

	if ClassifyTransaction(transfer) != TxTransfer {
		t.Error("expected transfer transaction")
	}

	deploy := &ledger.Transaction{
		From: []byte{0x01},
		Data: []byte{0x60, 0x60},
	}

	if ClassifyTransaction(deploy) != TxContractDeploy {
		t.Error("expected contract deploy transaction")
	}

	call := &ledger.Transaction{
		From: []byte{0x01},
		To:   []byte{0x02},
		Data: []byte{0x60, 0x60},
	}

	if ClassifyTransaction(call) != TxContractCall {
		t.Error("expected contract call transaction")
	}
}
