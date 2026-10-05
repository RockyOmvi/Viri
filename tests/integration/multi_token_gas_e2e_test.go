package integration

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer2/contracts"
	"github.com/viri-chain/viri/internal/layer2/execution"
	"github.com/viri-chain/viri/internal/layer2/gas"
)

// TestMultiTokenGasEndToEndExecution verifies that a user with 0 native VIRI
// can pay transaction fees entirely in an ERC-20 token (e.g., USDC or AddrERC20).
func TestMultiTokenGasEndToEndExecution(t *testing.T) {
	engine := execution.NewExecutionEngine()

	// 1. Setup Contract Manager with standard ERC-20 token
	cm := contracts.NewContractManager()
	engine.SetContractManager(cm)

	// 2. Setup Fee Conversion Oracle
	// 1 ERC-20 Token = 2.0 Native VIRI units
	feeOracle := gas.NewFeeConversionOracle(5 * time.Minute)
	tokenAddr := contracts.AddrERC20
	feeOracle.SetRate(tokenAddr, 2.0)
	engine.SetFeeOracle(feeOracle)

	// 3. Create Sender Wallet (Zero native VIRI, funded with ERC-20 tokens)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	senderAddr := senderKey.PubKey().Address()

	recipientKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate recipient key: %v", err)
	}
	recipientAddr := recipientKey.PubKey().Address()

	// In-memory state store for execution
	accountsState := make(map[string]*execution.AccountState)

	// Sender starts with 0 Native balance, but 50,000 ERC-20 tokens
	initialTokenBal := big.NewInt(50_000)
	accountsState[string(senderAddr)] = &execution.AccountState{
		Address:       senderAddr,
		Balance:       big.NewInt(0), // 0 NATIVE VIRI!
		TokenBalances: map[string]*big.Int{string(tokenAddr): new(big.Int).Set(initialTokenBal)},
		Nonce:         0,
		Storage:       make(map[string][]byte),
	}

	accountsState[string(recipientAddr)] = &execution.AccountState{
		Address:       recipientAddr,
		Balance:       big.NewInt(0),
		TokenBalances: make(map[string]*big.Int),
		Nonce:         0,
		Storage:       make(map[string][]byte),
	}

	getAccount := func(addr []byte) (*execution.AccountState, error) {
		if acc, ok := accountsState[string(addr)]; ok {
			return acc, nil
		}
		return &execution.AccountState{
			Address:       addr,
			Balance:       big.NewInt(0),
			TokenBalances: make(map[string]*big.Int),
			Nonce:         0,
			Storage:       make(map[string][]byte),
		}, nil
	}

	setAccount := func(addr []byte, acc *execution.AccountState) error {
		accountsState[string(addr)] = acc
		return nil
	}

	// 4. Build Transaction with FeeCurrency set to tokenAddr
	// GasLimit: 30,000, GasPrice: 1
	// Base transfer cost = 26,000 gas.
	// Native fee charged = 26,000 * 1 = 26,000 Native units.
	// Since 1 Token = 2.0 Native units, token fee charged = 26,000 / 2 = 13,000 Tokens!
	// Gas refund: (30,000 - 26,000) = 4,000 gas = 2,000 Tokens refunded!
	// Net tokens deducted: 13,000 Tokens!
	tx := &ledger.Transaction{
		Nonce:       0,
		From:        senderKey.PubKey().Bytes(),
		To:          recipientAddr,
		Value:       0, // 0 native transfer
		GasLimit:    30000,
		GasPrice:    1,
		FeeCurrency: tokenAddr,
		ChainID:     1,
	}

	payload := tx.SigningPayload()
	if !bytes.Contains(payload, tokenAddr) {
		t.Fatalf("Signing payload must contain fee token address")
	}

	sig, err := senderKey.Sign(payload)
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}
	tx.Signature = &ledger.TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
		V: 0,
	}

	// 5. Execute Transaction
	res, err := engine.ExecuteTransaction(tx, 1, getAccount, setAccount)
	if err != nil {
		t.Fatalf("Execution error: %v", err)
	}
	if res.Status != 1 {
		t.Fatalf("Expected status = 1 (success), got %d, err: %v", res.Status, res.Err)
	}

	// 6. Verify Balances after execution
	updatedSender := accountsState[string(senderAddr)]

	// Native balance must still be exactly 0 (no native coin was spent or needed)
	if updatedSender.Balance.Sign() != 0 {
		t.Fatalf("Expected sender native balance to remain 0, got %s", updatedSender.Balance)
	}

	// Token balance must be reduced by the exact converted fee (13,000 tokens for 26,000 gas)
	finalTokenBal := updatedSender.GetTokenBalance(tokenAddr)
	expectedNetFeeInToken := uint64(26000 / 2) // 13,000 tokens
	expectedRemaining := new(big.Int).Sub(initialTokenBal, new(big.Int).SetUint64(expectedNetFeeInToken))

	if finalTokenBal.Cmp(expectedRemaining) != 0 {
		t.Fatalf("Expected remaining token balance = %s, got %s (net fee deducted: %s)",
			expectedRemaining, finalTokenBal, new(big.Int).Sub(initialTokenBal, finalTokenBal))
	}

	// Nonce must be incremented to 1
	if updatedSender.Nonce != 1 {
		t.Fatalf("Expected sender nonce = 1, got %d", updatedSender.Nonce)
	}
}

// TestMultiTokenGasViaStandardERC20Contract verifies that gas can be paid from a
// standard ERC20 contract balance (contracts.ERC20Token) if not in native TokenBalances.
func TestMultiTokenGasViaStandardERC20Contract(t *testing.T) {
	engine := execution.NewExecutionEngine()
	cm := contracts.NewContractManager()
	engine.SetContractManager(cm)

	feeOracle := gas.NewFeeConversionOracle(5 * time.Minute)
	tokenAddr := contracts.AddrERC20
	feeOracle.SetRate(tokenAddr, 1.0)
	engine.SetFeeOracle(feeOracle)

	senderKey, _ := crypto.GenerateKey()
	senderAddr := senderKey.PubKey().Address()

	// Mint tokens directly in the ERC-20 contract
	tok := cm.GetERC20(tokenAddr)
	if tok == nil {
		t.Fatalf("Expected standard ERC-20 contract at %x", tokenAddr)
	}
	tok.Mint(senderAddr, big.NewInt(100_000))

	// In-memory accounts: sender has 0 native and 0 native TokenBalances
	accountsState := make(map[string]*execution.AccountState)
	accountsState[string(senderAddr)] = &execution.AccountState{
		Address:       senderAddr,
		Balance:       big.NewInt(0),
		TokenBalances: make(map[string]*big.Int), // empty!
		Nonce:         0,
		Storage:       make(map[string][]byte),
	}

	getAccount := func(addr []byte) (*execution.AccountState, error) {
		if acc, ok := accountsState[string(addr)]; ok {
			return acc, nil
		}
		return &execution.AccountState{
			Address:       addr,
			Balance:       big.NewInt(0),
			TokenBalances: make(map[string]*big.Int),
			Nonce:         0,
			Storage:       make(map[string][]byte),
		}, nil
	}
	setAccount := func(addr []byte, acc *execution.AccountState) error {
		accountsState[string(addr)] = acc
		return nil
	}

	tx := &ledger.Transaction{
		Nonce:       0,
		From:        senderKey.PubKey().Bytes(),
		To:          senderAddr,
		Value:       0,
		GasLimit:    26000,
		GasPrice:    1,
		FeeCurrency: tokenAddr,
		ChainID:     1,
	}
	payload := tx.SigningPayload()
	sig, _ := senderKey.Sign(payload)
	tx.Signature = &ledger.TxSignature{R: sig.R.Bytes(), S: sig.S.Bytes(), V: 0}

	res, err := engine.ExecuteTransaction(tx, 1, getAccount, setAccount)
	if err != nil {
		t.Fatalf("Execution error: %v", err)
	}
	if res.Status != 1 {
		t.Fatalf("Expected status = 1, got %d: %v", res.Status, res.Err)
	}

	// Verify tokens were deducted from the ERC-20 contract (26,000 gas * 1 token/gas)
	contractBal := tok.BalanceOf(senderAddr)
	expectedBal := big.NewInt(100_000 - 26000)
	if contractBal.Cmp(expectedBal) != 0 {
		t.Fatalf("Expected ERC-20 balance = %s, got %s", expectedBal, contractBal)
	}
}
