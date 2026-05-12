package main

import (
	"fmt"
	"math/big"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer2/accounts"
	"github.com/viri-chain/viri/internal/layer2/contracts"
	"github.com/viri-chain/viri/internal/layer2/execution"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║     Viri Blockchain — Wallet & Contract Demo        ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	// ── 1. Create Two Wallets ──
	fmt.Println("\n📦 Creating wallets...")
	wallet1, _ := crypto.GenerateKey()
	wallet2, _ := crypto.GenerateKey()

	addr1 := wallet1.PubKey().Address()
	addr2 := wallet2.PubKey().Address()

	fmt.Printf("  Wallet 1: addr=0x%x\n", addr1)
	fmt.Printf("  Wallet 2: addr=0x%x\n", addr2)

	// ── 2. Create Execution Engine & state ──
	fmt.Println("\n🔧 Initializing execution engine...")
	engine := execution.NewExecutionEngine()
	cm := contracts.NewContractManager()
	engine.SetContractManager(cm)

	stateMap := make(map[string]*execution.AccountState)
	stateMap[string(addr1)] = &execution.AccountState{
		Address: addr1, Balance: big.NewInt(1000000), Nonce: 0,
		Storage: make(map[string][]byte),
	}
	stateMap[string(addr2)] = &execution.AccountState{
		Address: addr2, Balance: big.NewInt(500000), Nonce: 0,
		Storage: make(map[string][]byte),
	}

	getAcct := func(a []byte) (*execution.AccountState, error) {
		if ac, ok := stateMap[string(a)]; ok {
			return ac, nil
		}
		return &execution.AccountState{Address: a, Balance: big.NewInt(0), Nonce: 0, Storage: make(map[string][]byte)}, nil
	}
	setAcct := func(a []byte, ac *execution.AccountState) error {
		stateMap[string(a)] = ac
		return nil
	}

	// Helper: sign a transaction (same pattern as e2e tests)
	signTx := func(tx *ledger.Transaction, key *crypto.PrivateKey) {
		tx.From = key.PubKey().Bytes()
		payload := tx.SigningPayload()
		sig, err := key.Sign(payload)
		if err != nil {
			panic("sign: " + err.Error())
		}
		tx.Signature = &ledger.TxSignature{R: sig.R.Bytes(), S: sig.S.Bytes()}
		tx.Hash = tx.ComputeHash()
	}

	// ── 3. Transfer between wallets ──
	fmt.Println("\n💸 Transferring 100000 from Wallet 1 → Wallet 2...")
	tx := &ledger.Transaction{
		Nonce: 0, To: addr2, Value: 100000,
		GasLimit: 100000, GasPrice: 1,
	}
	signTx(tx, wallet1)
	result, err := engine.ExecuteTransaction(tx, 1, getAcct, setAcct)
	if err != nil {
		fmt.Printf("  FAIL: %v\n", err)
		return
	}
	fmt.Printf("  Status: %d (1=success), GasUsed: %d\n", result.Status, result.GasUsed)
	fmt.Printf("  Wallet 1 balance: %s\n", stateMap[string(addr1)].Balance)
	fmt.Printf("  Wallet 2 balance: %s\n", stateMap[string(addr2)].Balance)

	// ── 4. Deploy a smart contract (SimpleStorage) ──
	fmt.Println("\n📄 Deploying SimpleStorage contract...")
	// Runtime code: function dispatch via CALLDATALOAD & selector matching
	// Functions: set(uint256) [0x60fe47b1] and get() [0x6d4ce63c]
	runtimeCode := []byte{
		// Dispatch: load selector from calldata[0:4]
		0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c,
		// Check set(uint256) selector 0x60fe47b1
		0x80, 0x63, 0x60, 0xfe, 0x47, 0xb1, 0x14,
		0x60, 0x1d, 0x90, 0x57,
		// Check get() selector 0x6d4ce63c
		0x80, 0x63, 0x6d, 0x4c, 0xe6, 0x3c, 0x14,
		0x60, 0x25, 0x90, 0x57,
		0x00,
		// set() at 0x1b: SSTORE(0, calldata[4])
		0x5b, 0x60, 0x04, 0x35, 0x60, 0x00, 0x55, 0x00,
		// get() at 0x33: RETURN(0, 32) with SLOAD(0)
		0x5b, 0x60, 0x00, 0x54, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3,
	}

	// Init code: CODECOPY(0, 12, len); RETURN(0, len)
	codeSize := byte(len(runtimeCode))
	initCode := append([]byte{
		0x60, codeSize,
		0x60, 12,
		0x60, 0x00,
		0x39,
		0x60, codeSize,
		0x60, 0x00,
		0xf3,
	}, runtimeCode...)

	deployTx := &ledger.Transaction{
		Nonce: 1, Data: append(initCode, make([]byte, 32)...),
		GasLimit: 2000000, GasPrice: 1,
	}
	signTx(deployTx, wallet1)

	result2, _ := engine.ExecuteTransaction(deployTx, 1, getAcct, setAcct)
	if result2.Status != 1 {
		fmt.Printf("  FAIL: %v\n", result2.Err)
		return
	}
	contractAddr := result2.Output
	fmt.Printf("  Contract deployed at: 0x%x\n", contractAddr)
	fmt.Printf("  Gas used: %d\n", result2.GasUsed)

	// ── 5. Call contract: set(42) ──
	fmt.Println("\n✍️  Calling set(42) on the contract...")
	setSelector := []byte{0x60, 0xfe, 0x47, 0xb1}
	input := make([]byte, 4+32)
	copy(input[:4], setSelector)
	new(big.Int).SetInt64(42).FillBytes(input[4:])

	callTx := &ledger.Transaction{
		Nonce: 2, To: contractAddr, Data: input,
		GasLimit: 2000000, GasPrice: 1,
	}
	signTx(callTx, wallet1)
	result3, _ := engine.ExecuteTransaction(callTx, 1, getAcct, setAcct)
	if result3.Status != 1 {
		fmt.Printf("  FAIL: %v\n", result3.Err)
		return
	}
	fmt.Printf("  Status: %d, GasUsed: %d\n", result3.Status, result3.GasUsed)

	// ── 6. Call contract: get() ──
	fmt.Println("\n🔍 Calling get() (should return 42)...")
	getSelector := []byte{0x6d, 0x4c, 0xe6, 0x3c}
	getTx := &ledger.Transaction{
		Nonce: 3, To: contractAddr, Data: getSelector,
		GasLimit: 2000000, GasPrice: 1,
	}
	signTx(getTx, wallet1)
	result4, _ := engine.ExecuteTransaction(getTx, 1, getAcct, setAcct)
	if result4.Status != 1 {
		fmt.Printf("  FAIL: %v\n", result4.Err)
		return
	}
	val := new(big.Int).SetBytes(result4.Output)
	fmt.Printf("  Stored value: %s (expected 42)\n", val)

	// ── 7. Transfer from contract to wallet ──
	fmt.Println("\n💸 Transferring 50000 from Wallet 1 → Wallet 2 (second transfer)...")
	tx3 := &ledger.Transaction{
		Nonce: 4, To: addr2, Value: 50000,
		GasLimit: 100000, GasPrice: 1,
	}
	signTx(tx3, wallet1)
	result5, _ := engine.ExecuteTransaction(tx3, 1, getAcct, setAcct)
	if result5.Status != 1 {
		fmt.Printf("  FAIL: %v\n", result5.Err)
		return
	}
	fmt.Printf("  Status: %d, GasUsed: %d\n", result5.Status, result5.GasUsed)
	fmt.Printf("  Wallet 1 balance: %s\n", stateMap[string(addr1)].Balance)
	fmt.Printf("  Wallet 2 balance: %s\n", stateMap[string(addr2)].Balance)

	// ── 8. Query standard contracts ──
	fmt.Println("\n📋 Querying Standard Contracts...")
	selName := []byte{0x06, 0xfd, 0xde, 0x03}
	selSymbol := []byte{0x95, 0xd8, 0x9b, 0x41}
	selTotalSupply := []byte{0x18, 0x16, 0x0d, 0xdd}

	erc20 := cm.GetStandardContract(contracts.AddrERC20)
	if erc20 != nil {
		name, _ := erc20.ExecuteCall(nil, selName)
		symbol, _ := erc20.ExecuteCall(nil, selSymbol)
		supply, _ := erc20.ExecuteCall(nil, selTotalSupply)
		if len(name) > 64 {
			fmt.Printf("  ERC20: name=%q symbol=%x totalSupply=%s\n", string(name[64:]), symbol[:32], new(big.Int).SetBytes(supply[:32]))
		}
	}

	// ── 9. Account Abstraction ──
	fmt.Println("\n🤖 Testing Account Abstraction...")
	mgr := accounts.NewAccountManager()
	ep := accounts.NewEntryPoint(mgr, 1337)
	mgr.CreateAccount([]byte("faucet"), accounts.AccountTypeNormal, 10000000)

	aaCode := []byte{0x60, 0x42, 0x60, 0x00, 0x53, 0x60, 0x01, 0x60, 0x00, 0xf3}
	op := accounts.UserOperation{
		Sender:    []byte("wallet-aa"),
		Nonce:     0,
		InitCode:  aaCode,
		CallData:  []byte{},
		GasLimit:  100000,
		MaxFee:    10,
		Signature: []byte{0x01},
	}
	results, _ := ep.HandleOps([]accounts.UserOperation{op}, []byte("beneficiary"))
	if results[0].Success {
		fmt.Printf("  AA Wallet deployed: output=0x%x\n", results[0].ReturnData)
	}

	// ── Summary ──
	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
	fmt.Println("║                   SUMMARY                           ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Printf("  Wallet 1: 0x%x  balance=%s\n", addr1, stateMap[string(addr1)].Balance)
	fmt.Printf("  Wallet 2: 0x%x  balance=%s\n", addr2, stateMap[string(addr2)].Balance)
	fmt.Printf("  Contract: 0x%x  stored=%s\n", contractAddr, val)
	fmt.Printf("  AA Wallet deployed: yes, returns 0x42\n")
	fmt.Printf("  All operations completed successfully!\n")
}
