package e2e

import (
	"math/big"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
	"github.com/viri-chain/viri/internal/layer2/accounts"
	"github.com/viri-chain/viri/internal/layer2/contracts"
	"github.com/viri-chain/viri/internal/layer2/execution"
	"github.com/viri-chain/viri/internal/layer2/gas"
	"github.com/viri-chain/viri/internal/layer2/mev"
	"github.com/viri-chain/viri/internal/layer2/privacy"
	"github.com/viri-chain/viri/internal/layer2/rollups"
	"github.com/viri-chain/viri/internal/layer2/zk"
)

func TestE2E_GenesisAndState(t *testing.T) {
	store := state.NewMemoryStore()
	genesis := ledger.DefaultGenesis()
	genesis.ChainID = 1337
	blockchain, err := ledger.NewPersistentBlockchain(genesis, store)
	if err != nil {
		t.Fatalf("blockchain init: %v", err)
	}
	if blockchain.Height() != 0 {
		t.Errorf("expected height 0, got %d", blockchain.Height())
	}
	genesisBlock, err := blockchain.GetBlock(0)
	if err != nil {
		t.Fatalf("get genesis block: %v", err)
	}
	if genesisBlock.Header.Height != 0 {
		t.Errorf("genesis height should be 0")
	}
	sm, err := state.NewStateManager(store)
	if err != nil {
		t.Fatalf("state manager: %v", err)
	}
	if err := sm.Initialize(new(big.Int).SetUint64(genesis.InitialSupply)); err != nil {
		t.Fatalf("state init: %v", err)
	}
	snap := sm.Snapshot()
	if snap.BlockHeight != 0 {
		t.Errorf("expected snapshot height 0, got %d", snap.BlockHeight)
	}
	t.Logf("Genesis OK: chain=%d height=%d accounts=%d supply=%d",
		genesis.ChainID, blockchain.Height(), snap.NumAccounts, genesis.InitialSupply)
}

func TestE2E_StateManagement(t *testing.T) {
	store := state.NewMemoryStore()
	sm, _ := state.NewStateManager(store)
	sm.Initialize(big.NewInt(1000000))

	addr := crypto.SHA256([]byte("alice"))[:20]
	_, _ = sm.CreateAccount(addr, state.AccountTypeNormal, big.NewInt(50000))
	bal, err := sm.GetBalance(addr)
	if err != nil || bal.Cmp(big.NewInt(50000)) != 0 {
		t.Fatalf("balance mismatch: %v %s", err, bal)
	}
	nonce, _ := sm.GetNonce(addr)
	if nonce != 0 {
		t.Errorf("expected nonce 0, got %d", nonce)
	}
	t.Logf("State Mgmt OK: addr=%x balance=%s nonce=%d", addr, bal, nonce)
}

func signTx(tx *ledger.Transaction, key *crypto.PrivateKey) {
	tx.From = key.PubKey().Bytes()
	payload := tx.SigningPayload()
	sig, err := key.Sign(payload)
	if err != nil {
		panic("sign failed: " + err.Error())
	}
	tx.Signature = &ledger.TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
	}
	tx.Hash = tx.ComputeHash()
}

func TestE2E_TransactionTransfer(t *testing.T) {
	engine := execution.NewExecutionEngine()
	key, _ := crypto.GenerateKey()
	senderAddr := key.PubKey().Address()

	accountMap := map[string]*execution.AccountState{
		string(senderAddr): {
			Address: senderAddr, Balance: big.NewInt(1000000), Nonce: 0,
			Storage: make(map[string][]byte),
		},
	}
	getAcct := func(a []byte) (*execution.AccountState, error) {
		if ac, ok := accountMap[string(a)]; ok {
			return ac, nil
		}
		return &execution.AccountState{Address: a, Balance: big.NewInt(0), Nonce: 0, Storage: make(map[string][]byte)}, nil
	}
	setAcct := func(a []byte, ac *execution.AccountState) error {
		accountMap[string(a)] = ac
		return nil
	}

	recipient := make([]byte, 20)
	recipient[19] = 0xCC
	tx := &ledger.Transaction{
		Nonce: 0, To: recipient,
		Value: 50000, GasLimit: 100000, GasPrice: 1,
	}
	signTx(tx, key)

	result, err := engine.ExecuteTransaction(tx, 1, getAcct, setAcct)
	if err != nil {
		t.Fatalf("transfer exec: %v", err)
	}
	if result.Status != 1 {
		t.Fatalf("transfer failed: %v", result.Err)
	}
	sender := accountMap[string(senderAddr)]
	rec := accountMap[string(recipient)]
	if rec.Balance.Cmp(big.NewInt(50000)) != 0 {
		t.Fatalf("recipient balance wrong: %s", rec.Balance)
	}
	t.Logf("Transfer OK: sender=%s recipient=%s gas=%d", sender.Balance, rec.Balance, result.GasUsed)
}

func TestE2E_ContractDeployAndCall(t *testing.T) {
	engine := execution.NewExecutionEngine()
	key, _ := crypto.GenerateKey()
	senderAddr := key.PubKey().Address()

	accountMap := map[string]*execution.AccountState{
		string(senderAddr): {
			Address: senderAddr, Balance: big.NewInt(10000000), Nonce: 0,
			Storage: make(map[string][]byte),
		},
	}
	getAcct := func(a []byte) (*execution.AccountState, error) {
		if ac, ok := accountMap[string(a)]; ok {
			return ac, nil
		}
		return nil, nil
	}
	setAcct := func(a []byte, ac *execution.AccountState) error {
		accountMap[string(a)] = ac
		return nil
	}

	// Runtime code: MSTORE8(0, 0x42); RETURN(0, 1) -> returns 0x42
	runtimeCode := []byte{0x60, 0x42, 0x60, 0x00, 0x53, 0x60, 0x01, 0x60, 0x00, 0xf3}
	// Init code: CODECOPY(0, len(prefix), len(runtimeCode)); RETURN(0, len(runtimeCode))
	codeSize := byte(len(runtimeCode))
	prefix := []byte{
		0x60, codeSize,      // PUSH1 size
		0x60, 12,            // PUSH1 codeOffset = len(prefix)
		0x60, 0x00,          // PUSH1 destOffset = 0
		0x39,                // CODECOPY
		0x60, codeSize,      // PUSH1 size
		0x60, 0x00,          // PUSH1 offset = 0
		0xf3,                // RETURN
	}
	initCode := append(prefix, runtimeCode...)
	deployTx := &ledger.Transaction{
		Nonce: 0, Data: initCode,
		GasLimit: 1000000, GasPrice: 1,
	}
	signTx(deployTx, key)

	result, _ := engine.ExecuteTransaction(deployTx, 1, getAcct, setAcct)
	if result.Status != 1 {
		t.Fatalf("deploy failed: %v", result.Err)
	}
	contractAddr := result.Output
	t.Logf("Deploy OK: contract=%x gas=%d", contractAddr, result.GasUsed)

	callTx := &ledger.Transaction{
		Nonce: 1, To: contractAddr,
		Data: []byte{0x00}, GasLimit: 1000000, GasPrice: 1,
	}
	signTx(callTx, key)

	result2, _ := engine.ExecuteTransaction(callTx, 1, getAcct, setAcct)
	if result2.Status != 1 {
		t.Fatalf("call failed: %v", result2.Err)
	}
	if len(result2.Output) == 0 || result2.Output[0] != 0x42 {
		t.Fatalf("expected 0x42, got %x", result2.Output)
	}
	t.Logf("Call OK: output=%x gas=%d", result2.Output, result2.GasUsed)
}

func TestE2E_StandardContracts(t *testing.T) {
	cm := contracts.NewContractManager()

	selName := []byte{0x06, 0xfd, 0xde, 0x03}
	selSymbol := []byte{0x95, 0xd8, 0x9b, 0x41}
	selTotalSupply := []byte{0x18, 0x16, 0x0d, 0xdd}
	selBalanceOf := []byte{0x70, 0xa0, 0x82, 0x31}

	erc20 := cm.GetStandardContract(contracts.AddrERC20)
	if erc20 == nil {
		t.Fatal("ERC20 not registered")
	}

	name, _ := erc20.ExecuteCall(nil, selName)
	t.Logf("ERC20 name: %x", name)

	symbol, _ := erc20.ExecuteCall(nil, selSymbol)
	t.Logf("ERC20 symbol: %x", symbol)

	supply, _ := erc20.ExecuteCall(nil, selTotalSupply)
	sup := new(big.Int).SetBytes(supply)
	t.Logf("ERC20 totalSupply: %s", sup)

	balInput := append(selBalanceOf, make([]byte, 32)...)
	bal, _ := erc20.ExecuteCall(nil, balInput)
	t.Logf("ERC20 balanceOf(0): %x", bal)

	erc721 := cm.GetStandardContract(contracts.AddrERC721)
	if erc721 == nil {
		t.Fatal("ERC721 not registered")
	}

	name721, _ := erc721.ExecuteCall(nil, selName)
	t.Logf("ERC721 name: %x", name721)

	symbol721, _ := erc721.ExecuteCall(nil, selSymbol)
	t.Logf("ERC721 symbol: %x", symbol721)

	t.Logf("Standard Contracts OK")
}

func TestE2E_GnarkZKProof(t *testing.T) {
	engine := execution.NewExecutionEngine()

	circuit := zk.NewCircuit("e2e_test_add", 2, 1, zk.FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)
	gv := zk.NewGnarkVerifier()
	engine.SetGnarkVerifier(gv, circuit)

	key, _ := crypto.GenerateKey()
	senderAddr := key.PubKey().Address()

	accountMap := map[string]*execution.AccountState{
		string(senderAddr): {
			Address: senderAddr, Balance: big.NewInt(1000000), Nonce: 0,
			Storage: make(map[string][]byte),
		},
	}
	getAcct := func(a []byte) (*execution.AccountState, error) {
		if ac, ok := accountMap[string(a)]; ok {
			return ac, nil
		}
		return nil, nil
	}
	setAcct := func(a []byte, ac *execution.AccountState) error {
		accountMap[string(a)] = ac
		return nil
	}

	gp := zk.NewGnarkProver()
	witness := &zk.Witness{
		Public: []*big.Int{big.NewInt(3), big.NewInt(5)},
		Secret: []*big.Int{big.NewInt(8)},
	}
	validProof, err := gp.Prove(circuit, witness)
	if err != nil {
		t.Fatalf("prove failed: %v", err)
	}

	data, err := zk.SerializeProofForTx(validProof, []*big.Int{big.NewInt(3), big.NewInt(5)})
	if err != nil {
		t.Fatalf("serialize proof: %v", err)
	}

	zkAddr := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xF3}
	tx := &ledger.Transaction{
		Nonce: 0, To: zkAddr, Data: data,
		GasLimit: 100000, GasPrice: 1,
	}
	signTx(tx, key)

	result, err := engine.ExecuteTransaction(tx, 1, getAcct, setAcct)
	if err != nil {
		t.Fatalf("ZK verify exec: %v", err)
	}
	if result.Status != 1 {
		t.Fatalf("ZK verify failed: %v", result.Err)
	}
	t.Logf("Gnark ZK Proof OK: 3+5=8 verified via real Groth16")
}

func TestE2E_AccountAbstraction(t *testing.T) {
	mgr := accounts.NewAccountManager()
	ep := accounts.NewEntryPoint(mgr, 1337)

	mgr.CreateAccount([]byte("faucet-aa"), accounts.AccountTypeNormal, 10000000)

	initCode := []byte{
		0x60, 0x42, 0x60, 0x00, 0x53,
		0x60, 0x01, 0x60, 0x00, 0xf3,
	}
	sender := []byte("e2e-wallet-01")
	op := accounts.UserOperation{
		Sender: sender, Nonce: 0, InitCode: initCode,
		CallData: []byte{}, GasLimit: 100000, MaxFee: 10,
		Signature: []byte{0x01},
	}
	results, err := ep.HandleOps([]accounts.UserOperation{op}, []byte("beneficiary-aa"))
	if err != nil {
		t.Fatalf("AA deploy+exec: %v", err)
	}
	if !results[0].Success {
		t.Fatalf("AA deploy failed: %v", results[0].ReturnData)
	}
	if len(results[0].ReturnData) == 0 || results[0].ReturnData[0] != 0x42 {
		t.Fatalf("expected 0x42, got %x", results[0].ReturnData)
	}
	t.Logf("AA Deploy OK: gas=%d fee=%d output=[0x42]", results[0].GasUsed, results[0].FeeCollected)

	mgr.Transfer([]byte("faucet-aa"), sender, 1000000)
	op2 := accounts.UserOperation{
		Sender: sender, Nonce: 1,
		CallData: []byte{}, GasLimit: 100000, MaxFee: 10,
		Signature: []byte{0x01},
	}
	results2, err := ep.HandleOps([]accounts.UserOperation{op2}, []byte("beneficiary-aa"))
	if err != nil {
		t.Fatalf("AA reuse: %v", err)
	}
	if !results2[0].Success {
		t.Fatalf("AA reuse failed")
	}
	t.Logf("AA Reuse OK: output=%x", results2[0].ReturnData)
}

func TestE2E_ShieldedPool(t *testing.T) {
	circuit := zk.NewCircuit("shielded_e2e", 2, 1, zk.FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)
	pk := zk.GenerateProvingKey(circuit)
	vk := zk.GenerateVerifyingKey(pk, circuit)

	prover := zk.NewProver(pk, circuit)
	assignment := &zk.Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}
	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof gen: %v", err)
	}
	verifier := zk.NewVerifier(vk, circuit)
	if err := verifier.Verify(proof); err != nil {
		t.Fatalf("proof verify: %v", err)
	}

	pool := privacy.NewShieldedPool()
	note, err := pool.CreateNote(1000, []byte("sender"), []byte("randomness"))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if note.Value != 1000 {
		t.Fatalf("expected note value 1000, got %d", note.Value)
	}
	if pool.NoteCount() != 1 {
		t.Fatalf("expected 1 note, got %d", pool.NoteCount())
	}
	if pool.TotalShielded() != 1000 {
		t.Fatalf("expected total shielded 1000, got %d", pool.TotalShielded())
	}

	if err := pool.SpendNote(note.Nullifier); err != nil {
		t.Fatalf("spend note: %v", err)
	}
	if pool.HasNullifier(note.Nullifier) != true {
		t.Fatalf("expected nullifier to be registered")
	}

	t.Logf("Shielded Pool OK: value=%d nullifier=%x total=%d",
		note.Value, note.Nullifier, pool.TotalShielded())
}

func TestE2E_GasOracle(t *testing.T) {
	gc := gas.NewGasOracle(gas.DefaultGasConfig())
	baseFee := gc.GetBaseFee()
	if baseFee == 0 {
		t.Fatalf("expected non-zero base fee")
	}
	prioFee := gc.GetRecommendedPriorityFee()
	t.Logf("Gas Oracle OK: baseFee=%d priorityFee=%d", baseFee, prioFee)
}

func TestE2E_MEV(t *testing.T) {
	ms := mev.NewMEVState(mev.StandardMode)
	mode := ms.GetMode()
	if mode != mev.StandardMode {
		t.Fatalf("expected standard mode, got %v", mode)
	}
	t.Logf("MEV State OK: mode=%v", mode)
}

func TestE2E_RollupChain(t *testing.T) {
	rc := rollups.NewRollupChain("main", rollups.RollupTypeOptimistic, 100)
	if rc == nil {
		t.Fatalf("rollup chain nil")
	}
	t.Logf("Rollup Chain OK: type=%v batchSize=%d", rollups.RollupTypeOptimistic, 100)
	_ = rc
}

func TestE2E_MerklePatriciaTrie(t *testing.T) {
	sm, _ := state.NewStateManager(state.NewMemoryStore())
	sm.Initialize(big.NewInt(1000000))

	for i := 0; i < 10; i++ {
		addr := crypto.SHA256([]byte{byte(i)})[:20]
		sm.CreateAccount(addr, state.AccountTypeNormal, big.NewInt(int64(i*1000)))
	}
	snap := sm.Snapshot()
	if snap.NumAccounts < 10 {
		t.Fatalf("expected >=10 accounts, got %d", snap.NumAccounts)
	}
	t.Logf("MPT OK: accounts=%d height=%d root=%x", snap.NumAccounts, snap.BlockHeight, snap.StateRoot)
}

func TestE2E_FeeMarket(t *testing.T) {
	fm := ledger.DefaultFeeMarket()
	baseFee := fm.BaseFee()
	if baseFee == 0 {
		t.Fatalf("expected non-zero base fee")
	}
	t.Logf("Fee Market OK: baseFee=%d", baseFee)
}

func TestE2E_ParallelExecution(t *testing.T) {
	engine := execution.NewExecutionEngine()
	engine.SetParallel(true)

	type node struct {
		addr []byte
		key  *crypto.PrivateKey
	}
	nodes := make([]node, 5)
	stateMap := map[string]*execution.AccountState{}
	for i := range nodes {
		k, _ := crypto.GenerateKey()
		nodes[i] = node{addr: k.PubKey().Address(), key: k}
		stateMap[string(k.PubKey().Address())] = &execution.AccountState{
			Address: k.PubKey().Address(), Balance: big.NewInt(1000000), Nonce: 0,
			Storage: make(map[string][]byte),
		}
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

	var txs []*ledger.Transaction
	for i, src := range nodes {
		dst := nodes[(i+1)%len(nodes)]
		tx := &ledger.Transaction{
			Nonce: 0, To: dst.addr,
			Value: 1000, GasLimit: 100000, GasPrice: 1,
		}
		signTx(tx, src.key)
		txs = append(txs, tx)
	}

	results, totalGas, err := engine.ExecuteBlock(txs, 1, getAcct, setAcct)
	if err != nil {
		t.Fatalf("parallel exec: %v", err)
	}
	for i, r := range results {
		if r.Status != 1 {
			t.Fatalf("tx %d failed: %v", i, r.Err)
		}
	}
	t.Logf("Parallel Execution OK: %d txs in block totalGas=%d", len(txs), totalGas)
}


