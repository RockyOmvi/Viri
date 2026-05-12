package execution

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer2/contracts"
	"github.com/viri-chain/viri/internal/layer2/privacy"
	"github.com/viri-chain/viri/internal/layer2/vm"
	"github.com/viri-chain/viri/internal/layer2/zk"
)

type TxType uint8

const (
	TxTransfer TxType = iota
	TxContractDeploy
	TxContractCall
)

type ExecutionResult struct {
	GasUsed    uint64
	GasRefund  uint64
	Status     uint8
	Output     []byte
	Logs       []*ledger.Log
	Err        error
}

type ExecutionState struct {
	Accounts     map[string]*AccountState
	BlockHeight  uint64
	BlockGasUsed uint64
}

type AccountState struct {
	Address       []byte
	Balance       *big.Int
	Nonce         uint64
	Code          []byte
	Storage       map[string][]byte
	TokenBalances map[string]*big.Int // token address hex -> balance (for fee-in-token)
}

// GetTokenBalance returns the balance for a specific token.
// If the token is nil/zero, returns the native Balance.
func (a *AccountState) GetTokenBalance(token []byte) *big.Int {
	if len(token) == 0 {
		return a.Balance
	}
	if a.TokenBalances == nil {
		return new(big.Int)
	}
	b, ok := a.TokenBalances[string(token)]
	if !ok || b == nil {
		return new(big.Int)
	}
	return b
}

// DeductTokenBalance subtracts amount from the specified token balance.
func (a *AccountState) DeductTokenBalance(token []byte, amount *big.Int) {
	if len(token) == 0 {
		a.Balance.Sub(a.Balance, amount)
		return
	}
	if a.TokenBalances == nil {
		a.TokenBalances = make(map[string]*big.Int)
	}
	current := a.TokenBalances[string(token)]
	if current == nil {
		current = new(big.Int)
	}
	current.Sub(current, amount)
	a.TokenBalances[string(token)] = current
}

// AddTokenBalance adds amount to the specified token balance.
func (a *AccountState) AddTokenBalance(token []byte, amount *big.Int) {
	if len(token) == 0 {
		a.Balance.Add(a.Balance, amount)
		return
	}
	if a.TokenBalances == nil {
		a.TokenBalances = make(map[string]*big.Int)
	}
	current := a.TokenBalances[string(token)]
	if current == nil {
		current = new(big.Int)
	}
	current.Add(current, amount)
	a.TokenBalances[string(token)] = current
}

// Precompile addresses for built-in operations
var (
	addrShieldedDeposit  = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF1}
	addrShieldedWithdraw = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF2}
	addrZKVerify        = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF3}
)

type VMType uint8

const (
	VMTypeDefault VMType = iota
	VMTypeEVM
	VMTypeWASM
)

type ExecutionEngine struct {
	baseGas        uint64
	transferGas    uint64
	deployGas      uint64
	callGas        uint64
	storageGas     uint64
	logGas         uint64
	shieldedPool   *privacy.ShieldedPool
	zkVerifier     *zk.Verifier
	gnarkVerifier  *zk.GnarkVerifier
	gnarkCircuit   *zk.Circuit
	contractMgr    *contracts.ContractManager
	vmType         VMType
	parallel       bool
}

func NewExecutionEngine() *ExecutionEngine {
	return &ExecutionEngine{
		baseGas:      21000,
		transferGas:  5000,
		deployGas:    32000,
		callGas:      10000,
		storageGas:   20000,
		logGas:       375,
	}
}

func (e *ExecutionEngine) SetShieldedPool(sp *privacy.ShieldedPool) {
	e.shieldedPool = sp
}

func (e *ExecutionEngine) SetZKVerifier(zv *zk.Verifier) {
	e.zkVerifier = zv
}

// SetGnarkVerifier configures the constraint-based ZK verifier with its circuit.
func (e *ExecutionEngine) SetGnarkVerifier(gv *zk.GnarkVerifier, circuit *zk.Circuit) {
	e.gnarkVerifier = gv
	e.gnarkCircuit = circuit
}

// SetVMType configures which VM backend to use for contract execution.
func (e *ExecutionEngine) SetVMType(t VMType) {
	e.vmType = t
}

// SetParallel enables or disables parallel transaction execution.
func (e *ExecutionEngine) SetParallel(enabled bool) {
	e.parallel = enabled
}

func (e *ExecutionEngine) SetContractManager(cm *contracts.ContractManager) {
	e.contractMgr = cm
}

// newExecutor creates a VM executor matching the configured VM type.
func (e *ExecutionEngine) newExecutor(ctx *vm.EVMContext, state *evmStateAdapter) vm.Executor {
	switch e.vmType {
	case VMTypeWASM:
		adapter := vm.NewWasmAdapter(ctx.GasLimit)
		return adapter
	default:
		return vm.NewEVMExecutor(ctx, state)
	}
}

func isPrecompileAddress(addr []byte) bool {
	if len(addr) != 20 {
		return false
	}
	switch {
	case bytes.Equal(addr, addrShieldedDeposit):
		return true
	case bytes.Equal(addr, addrShieldedWithdraw):
		return true
	case bytes.Equal(addr, addrZKVerify):
		return true
	}
	return false
}

func (e *ExecutionEngine) ExecuteBlock(txs []*ledger.Transaction, blockHeight uint64, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) ([]*ExecutionResult, uint64, error) {
	if e.parallel && len(txs) > 1 {
		return e.ExecuteBlockParallel(txs, blockHeight, getAccount, setAccount)
	}

	results := make([]*ExecutionResult, 0, len(txs))
	totalGasUsed := uint64(0)

	for _, tx := range txs {
		result, err := e.ExecuteTransaction(tx, blockHeight, getAccount, setAccount)
		if err != nil {
			return results, totalGasUsed, err
		}

		results = append(results, result)
		totalGasUsed += result.GasUsed
	}

	return results, totalGasUsed, nil
}

func (e *ExecutionEngine) ExecuteTransaction(tx *ledger.Transaction, blockHeight uint64, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) (*ExecutionResult, error) {
	if !tx.Verify() {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("invalid transaction signature"),
		}, nil
	}

	senderAddr := tx.SenderAddress()

	sender, err := getAccount(senderAddr)
	if err != nil {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("sender account not found"),
		}, nil
	}

	txType := ClassifyTransaction(tx)
	gasCost := e.calculateGasCost(txType, tx)

	if tx.GasLimit < gasCost {
		return &ExecutionResult{
			GasUsed: 0,
			Status:  0,
			Err:     fmt.Errorf("insufficient gas limit"),
		}, nil
	}

	feeToken := tx.FeeToken()
	feeAmount := new(big.Int).SetUint64(gasCost * tx.GasPrice)

	// Check balance: value in native, fee in fee token
	if len(feeToken) == 0 {
		// Native fee: balance must cover value + fee
		totalNeeded := new(big.Int).SetUint64(tx.Value)
		totalNeeded.Add(totalNeeded, feeAmount)
		if sender.Balance.Cmp(totalNeeded) < 0 {
			return &ExecutionResult{
				GasUsed: 0, Status: 0,
				Err: fmt.Errorf("insufficient native balance: have %s, need %s", sender.Balance, totalNeeded),
			}, nil
		}
	} else {
		// Token fee: native balance must cover value, token balance must cover fee
		if new(big.Int).SetUint64(tx.Value).Cmp(sender.Balance) > 0 {
			return &ExecutionResult{
				GasUsed: 0, Status: 0,
				Err: fmt.Errorf("insufficient native balance for value transfer"),
			}, nil
		}
		tokenBal := sender.GetTokenBalance(feeToken)
		if tokenBal.Cmp(feeAmount) < 0 {
			return &ExecutionResult{
				GasUsed: 0, Status: 0,
				Err: fmt.Errorf("insufficient token balance for fee: have %s, need %s", tokenBal, feeAmount),
			}, nil
		}
	}

	if tx.Nonce != sender.Nonce {
		return &ExecutionResult{
			GasUsed: 0,
			Status:  0,
			Err:     fmt.Errorf("invalid nonce: expected %d, got %d", sender.Nonce, tx.Nonce),
		}, nil
	}

	sender.Nonce = tx.Nonce + 1

	// Deduct value in native coin
	if tx.Value > 0 {
		sender.Balance.Sub(sender.Balance, new(big.Int).SetUint64(tx.Value))
	}

	// Deduct fee in appropriate currency
	sender.DeductTokenBalance(feeToken, feeAmount)

	if err := setAccount(senderAddr, sender); err != nil {
		return &ExecutionResult{
			GasUsed: 0,
			Status:  0,
			Err:     fmt.Errorf("failed to update sender account"),
		}, nil
	}

	var result *ExecutionResult
	switch txType {
	case TxTransfer:
		result = e.executeTransfer(tx, getAccount, setAccount)
	case TxContractDeploy:
		result = e.executeDeploy(tx, getAccount, setAccount)
	case TxContractCall:
		result = e.executeCall(tx, getAccount, setAccount)
	default:
		result = &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("unknown transaction type"),
		}
	}

	if result.Err != nil {
		if result.GasUsed == 0 {
			result.GasUsed = gasCost
		}
		return result, nil
	}

	result.Status = 1
	if result.GasUsed == 0 {
		result.GasUsed = gasCost
	}
	feeToCharge := result.GasUsed
	result.GasRefund = (tx.GasLimit - feeToCharge) * tx.GasPrice

	if result.GasRefund > 0 {
		refundAddr := tx.SenderAddress()
		refundAccount, err := getAccount(refundAddr)
		if err == nil {
			refundAccount.AddTokenBalance(feeToken, new(big.Int).SetUint64(result.GasRefund))
			if err := setAccount(refundAddr, refundAccount); err != nil {
				fmt.Printf("[WARN] Failed to persist gas refund for %x: %v\n", refundAddr, err)
			}
		}
	}

	result.Logs = append(result.Logs, &ledger.Log{
		Address: tx.To,
		Topics:  [][]byte{tx.SenderAddress()},
		Data:    []byte{byte(result.Status)},
	})

	return result, nil
}

func (e *ExecutionEngine) executeTransfer(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	if len(tx.To) == 0 {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("transfer requires recipient"),
		}
	}

	recipient, err := getAccount(tx.To)
	if err != nil || recipient == nil {
		recipient = &AccountState{
			Address: tx.To,
			Balance: new(big.Int),
			Nonce:   0,
			Storage: make(map[string][]byte),
		}
	}

	if recipient.Balance == nil {
		recipient.Balance = new(big.Int)
	}

	recipient.Balance.Add(recipient.Balance, new(big.Int).SetUint64(tx.Value))

	if err := setAccount(tx.To, recipient); err != nil {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("failed to update recipient account"),
		}
	}

	return &ExecutionResult{
		Status: 1,
	}
}

func (e *ExecutionEngine) executeDeploy(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	if len(tx.Data) == 0 {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("deployment requires contract code"),
		}
	}

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, tx.Nonce)
	contractAddr := crypto.Keccak256(append(tx.SenderAddress(), nonceBytes...))[:20]

	contract := &AccountState{
		Address: contractAddr,
		Balance: new(big.Int).SetUint64(tx.Value),
		Nonce:   0,
		Code:    nil,
		Storage: make(map[string][]byte),
	}

	if err := setAccount(contractAddr, contract); err != nil {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("failed to create contract account"),
		}
	}

	stateAdapter := &evmStateAdapter{
		getAccount: getAccount,
		setAccount: setAccount,
	}

	constructorArgs := []byte{}
	if len(tx.Data) > 32 {
		constructorArgs = tx.Data[len(tx.Data)-32:]
	}

	ctx := &vm.EVMContext{
		Caller:   tx.SenderAddress(),
		Address:  contractAddr,
		Value:    new(big.Int).SetUint64(tx.Value),
		GasLimit: tx.GasLimit,
		GasPrice: new(big.Int).SetUint64(tx.GasPrice),
		Data:     constructorArgs,
	}

	initCode := tx.Data
	if len(tx.Data) > 32 {
		initCode = tx.Data[:len(tx.Data)-32]
	}

	executor := e.newExecutor(ctx, stateAdapter)
	runtimeCode, gasUsed, err := executor.Execute(initCode)

	if err != nil {
		return &ExecutionResult{
			Status:  0,
			GasUsed: gasUsed,
			Err:     fmt.Errorf("contract init failed: %v", err),
		}
	}

	if len(runtimeCode) == 0 {
		runtimeCode = initCode
	}

	contract.Code = runtimeCode

	deployedContract, _ := getAccount(contractAddr)
	if deployedContract != nil && deployedContract.Storage != nil {
		contract.Storage = deployedContract.Storage
	}

	if err := setAccount(contractAddr, contract); err != nil {
		return &ExecutionResult{
			Status:  0,
			Err:     fmt.Errorf("failed to save contract: %v", err),
		}
	}

	return &ExecutionResult{
		Status:  1,
		Output:  contractAddr,
		GasUsed: gasUsed,
	}
}

func (e *ExecutionEngine) executeCall(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	// Check for precompile addresses
	if e.shieldedPool != nil || e.zkVerifier != nil || e.gnarkVerifier != nil {
		if res := e.handlePrecompile(tx, getAccount, setAccount); res != nil {
			return res
		}
	}

	contract, err := getAccount(tx.To)
	if err != nil || len(contract.Code) == 0 {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("contract not found"),
		}
	}

	// Credit the contract with the transferred value (already deducted from sender)
	if tx.Value > 0 {
		if contract.Balance == nil {
			contract.Balance = new(big.Int)
		}
		contract.Balance.Add(contract.Balance, new(big.Int).SetUint64(tx.Value))
		if err := setAccount(tx.To, contract); err != nil {
			return &ExecutionResult{
				Status: 0,
				Err:    fmt.Errorf("failed to credit contract balance"),
			}
		}
	}

	stateAdapter := &evmStateAdapter{
		getAccount: getAccount,
		setAccount: setAccount,
	}

	ctx := &vm.EVMContext{
		Caller:   tx.SenderAddress(),
		Address:  tx.To,
		Value:    new(big.Int).SetUint64(tx.Value),
		GasLimit: tx.GasLimit,
		GasPrice: new(big.Int).SetUint64(tx.GasPrice),
		Data:     tx.Data,
	}

	executor := e.newExecutor(ctx, stateAdapter)
	output, gasUsed, err := executor.Execute(contract.Code)

	if err != nil {
		return &ExecutionResult{
			Status:  0,
			GasUsed: gasUsed,
			Err:     err,
		}
	}

	return &ExecutionResult{
		Status:  1,
		Output:  output,
		GasUsed: gasUsed,
	}
}

func (e *ExecutionEngine) handlePrecompile(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	if len(tx.To) != 20 {
		return nil
	}

	switch {
	case bytes.Equal(tx.To, addrShieldedDeposit):
		return e.precompileShieldedDeposit(tx, getAccount, setAccount)
	case bytes.Equal(tx.To, addrShieldedWithdraw):
		return e.precompileShieldedWithdraw(tx, getAccount, setAccount)
	case bytes.Equal(tx.To, addrZKVerify):
		return e.precompileZKVerify(tx, getAccount, setAccount)
	}

	// Check standard contracts (ERC20, ERC721, etc.)
	if e.contractMgr != nil {
		if sc := e.contractMgr.GetStandardContract(tx.To); sc != nil {
			output, err := sc.ExecuteCall(tx.SenderAddress(), tx.Data)
			if err != nil {
				return &ExecutionResult{Status: 0, GasUsed: 1000, Err: err}
			}
			return &ExecutionResult{Status: 1, GasUsed: 1000, Output: output}
		}
	}

	return nil
}

func (e *ExecutionEngine) precompileShieldedDeposit(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	if e.shieldedPool == nil {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("shielded pool not available")}
	}
	if len(tx.Data) < 32 {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("invalid deposit data: need randomness(32)")}
	}
	randomness := tx.Data[:32]
	note, err := e.shieldedPool.CreateNote(tx.Value, tx.From, randomness)
	if err != nil {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("shielded deposit failed: %v", err)}
	}
	return &ExecutionResult{
		Status: 1,
		Output: append(note.Commitment, note.Nullifier...),
	}
}

func (e *ExecutionEngine) precompileShieldedWithdraw(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	if e.shieldedPool == nil {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("shielded pool not available")}
	}
	if len(tx.Data) < 32 {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("invalid withdraw data: need nullifier(32)")}
	}
	nullifier := tx.Data[:32]
	if err := e.shieldedPool.SpendNote(nullifier); err != nil {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("shielded withdraw failed: %v", err)}
	}
	recipient, err := getAccount(tx.To)
	if err != nil || recipient == nil {
		recipient = &AccountState{
			Address: tx.To,
			Balance: new(big.Int),
			Nonce:   0,
			Storage: make(map[string][]byte),
		}
	}
	recipient.Balance.Add(recipient.Balance, new(big.Int).SetUint64(tx.Value))
	if err := setAccount(tx.To, recipient); err != nil {
		fmt.Printf("[WARN] Failed to persist shielded withdraw recipient %x: %v\n", tx.To, err)
	}
	return &ExecutionResult{Status: 1}
}

func (e *ExecutionEngine) precompileZKVerify(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	if e.gnarkVerifier != nil && e.gnarkCircuit != nil {
		return e.precompileGnarkVerify(tx)
	}
	if e.zkVerifier == nil {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("ZK verifier not available")}
	}
	if len(tx.Data) < 96 {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("invalid verify data: need A(32)+B(32)+C(32)")}
	}
	proof := &zk.Proof{
		A: []*big.Int{new(big.Int).SetBytes(tx.Data[:32])},
		B: []*big.Int{new(big.Int).SetBytes(tx.Data[32:64])},
		C: []*big.Int{new(big.Int).SetBytes(tx.Data[64:96])},
	}
	if err := e.zkVerifier.Verify(proof); err != nil {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("ZK proof verification failed: %v", err)}
	}
	return &ExecutionResult{Status: 1, Output: []byte{0x01}}
}

func (e *ExecutionEngine) precompileGnarkVerify(tx *ledger.Transaction) *ExecutionResult {
	if len(tx.Data) < 96 {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("invalid verify data: need A(32)+B(32)+C(32)+publicWitnesses")}
	}
	proof := &zk.Proof{
		A: []*big.Int{new(big.Int).SetBytes(tx.Data[:32])},
		B: []*big.Int{new(big.Int).SetBytes(tx.Data[32:64])},
		C: []*big.Int{new(big.Int).SetBytes(tx.Data[64:96])},
	}
	numPublic := e.gnarkCircuit.NumInputs
	publicWitness := &zk.Witness{
		Public: make([]*big.Int, numPublic),
		Secret: []*big.Int{},
	}
	for i := 0; i < numPublic; i++ {
		offset := 96 + i*32
		if offset+32 > len(tx.Data) {
			publicWitness.Public[i] = big.NewInt(0)
			continue
		}
		publicWitness.Public[i] = new(big.Int).SetBytes(tx.Data[offset : offset+32])
	}
	proof.CircuitID = []byte(e.gnarkCircuit.Name)
	proof.Public = publicWitness.Public
	if err := e.gnarkVerifier.Verify(proof, e.gnarkCircuit, publicWitness); err != nil {
		return &ExecutionResult{Status: 0, Err: fmt.Errorf("gnark proof verification failed: %v", err)}
	}
	return &ExecutionResult{Status: 1, Output: []byte{0x01}}
}

type evmStateAdapter struct {
	getAccount func([]byte) (*AccountState, error)
	setAccount func([]byte, *AccountState) error
}

func (s *evmStateAdapter) GetNonce(addr []byte) uint64 {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return 0
	}
	return acct.Nonce
}

func (s *evmStateAdapter) GetBalance(addr []byte) *big.Int {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return new(big.Int)
	}
	return acct.Balance
}

func (s *evmStateAdapter) GetCode(addr []byte) []byte {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return nil
	}
	return acct.Code
}

func (s *evmStateAdapter) GetStorage(addr []byte, key []byte) []byte {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil || acct.Storage == nil {
		return nil
	}
	return acct.Storage[string(key)]
}

func (s *evmStateAdapter) SetStorage(addr []byte, key []byte, value []byte) {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return
	}
	if acct.Storage == nil {
		acct.Storage = make(map[string][]byte)
	}
	acct.Storage[string(key)] = value
	s.setAccount(addr, acct)
}

func (s *evmStateAdapter) Transfer(from, to []byte, amount *big.Int) {
	fromAcct, err := s.getAccount(from)
	if err != nil || fromAcct == nil {
		return
	}
	toAcct, err := s.getAccount(to)
	if err != nil || toAcct == nil {
		toAcct = &AccountState{
			Address: to,
			Balance: new(big.Int),
			Nonce:   0,
			Storage: make(map[string][]byte),
		}
	}
	if fromAcct.Balance.Cmp(amount) >= 0 {
		fromAcct.Balance.Sub(fromAcct.Balance, amount)
		toAcct.Balance.Add(toAcct.Balance, amount)
		s.setAccount(from, fromAcct)
		s.setAccount(to, toAcct)
	}
}

func (s *evmStateAdapter) CreateAccount(addr []byte) {
	acct := &AccountState{
		Address: addr,
		Balance: new(big.Int),
		Nonce:   0,
		Storage: make(map[string][]byte),
	}
	s.setAccount(addr, acct)
}

func (e *ExecutionEngine) calculateGasCost(txType TxType, tx *ledger.Transaction) uint64 {
	gas := e.baseGas

	switch txType {
	case TxTransfer:
		gas += e.transferGas
	case TxContractDeploy:
		gas += e.deployGas
		gas += uint64(len(tx.Data)) * e.storageGas / 1000
	case TxContractCall:
		gas += e.callGas
	}

	if len(tx.Data) > 0 {
		gas += uint64(len(tx.Data)) / 32
	}

	return gas
}

func ClassifyTransaction(tx *ledger.Transaction) TxType {
	if len(tx.To) == 0 && len(tx.Data) > 0 {
		return TxContractDeploy
	}

	if len(tx.To) > 0 && len(tx.Data) > 0 {
		return TxContractCall
	}

	return TxTransfer
}
