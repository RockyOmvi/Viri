package execution

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
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
	Address []byte
	Balance *big.Int
	Nonce   uint64
	Code    []byte
	Storage map[string][]byte
}

// Precompile addresses for built-in operations
var (
	addrShieldedDeposit  = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF1}
	addrShieldedWithdraw = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF2}
	addrZKVerify        = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF3}
)

type ExecutionEngine struct {
	baseGas      uint64
	transferGas  uint64
	deployGas    uint64
	callGas      uint64
	storageGas   uint64
	logGas       uint64
	shieldedPool *privacy.ShieldedPool
	zkVerifier   *zk.Verifier
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

	requiredBalance := new(big.Int).SetUint64(tx.Value)
	requiredBalance.Add(requiredBalance, new(big.Int).SetUint64(gasCost*tx.GasPrice))

	if sender.Balance.Cmp(requiredBalance) < 0 {
		return &ExecutionResult{
			GasUsed: 0,
			Status:  0,
			Err:     fmt.Errorf("insufficient balance"),
		}, nil
	}

	if tx.Nonce != sender.Nonce {
		return &ExecutionResult{
			GasUsed: 0,
			Status:  0,
			Err:     fmt.Errorf("invalid nonce: expected %d, got %d", sender.Nonce, tx.Nonce),
		}, nil
	}

	sender.Nonce = tx.Nonce + 1
	sender.Balance.Sub(sender.Balance, new(big.Int).SetUint64(tx.Value))
	sender.Balance.Sub(sender.Balance, new(big.Int).SetUint64(gasCost*tx.GasPrice))

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
		result.GasUsed = gasCost
		return result, nil
	}

	result.GasUsed = gasCost
	result.GasRefund = (tx.GasLimit - gasCost) * tx.GasPrice
	result.Status = 1

	if result.GasRefund > 0 {
		refundAddr := tx.SenderAddress()
		refundAccount, err := getAccount(refundAddr)
		if err == nil {
			refundAccount.Balance.Add(refundAccount.Balance, new(big.Int).SetUint64(result.GasRefund))
			_ = setAccount(refundAddr, refundAccount)
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

	// Derive unique contract address from hash(sender || nonce)
	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, tx.Nonce)
	contractAddr := crypto.SHA256(append(tx.SenderAddress(), nonceBytes...))[:20]

	contract := &AccountState{
		Address: contractAddr,
		Balance: new(big.Int).SetUint64(tx.Value),
		Nonce:   0,
		Code:    tx.Data,
		Storage: make(map[string][]byte),
	}

	if err := setAccount(contractAddr, contract); err != nil {
		return &ExecutionResult{
			Status: 0,
			Err:    fmt.Errorf("failed to deploy contract"),
		}
	}

	// Try to execute constructor if we wanted to, but for now we just deploy the code as is.
	// Actually, an EVM deployment executes init code and returns the runtime code.
	// We'll run the code and save its output as the contract code.
	stateAdapter := &evmStateAdapter{
		getAccount: getAccount,
		setAccount: setAccount,
	}
	ctx := &vm.EVMContext{
			Caller:   tx.SenderAddress(),
			Address:  contractAddr,
			Value:    new(big.Int).SetUint64(tx.Value),
			GasLimit: tx.GasLimit,
			GasPrice: new(big.Int).SetUint64(tx.GasPrice),
			Data:     tx.Data, // constructor args via calldata
		}
	executor := vm.NewEVMExecutor(ctx, stateAdapter)
	output, gasUsed, err := executor.Execute(tx.Data)

	if err != nil {
		return &ExecutionResult{
			Status:  0,
			GasUsed: gasUsed,
			Err:     fmt.Errorf("contract init failed: %v", err),
		}
	}

	// Re-read the contract account to preserve storage set during init code execution
	deployedContract, _ := getAccount(contractAddr)
	if deployedContract == nil {
		deployedContract = contract
	}
	deployedContract.Code = output
	setAccount(contractAddr, deployedContract)

	return &ExecutionResult{
		Status:  1,
		Output:  contractAddr,
		GasUsed: gasUsed,
	}
}

func (e *ExecutionEngine) executeCall(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
	// Check for precompile addresses
	if e.shieldedPool != nil || e.zkVerifier != nil {
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

	executor := vm.NewEVMExecutor(ctx, stateAdapter)
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
	_ = setAccount(tx.To, recipient)
	return &ExecutionResult{Status: 1}
}

func (e *ExecutionEngine) precompileZKVerify(tx *ledger.Transaction, getAccount func([]byte) (*AccountState, error), setAccount func([]byte, *AccountState) error) *ExecutionResult {
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

type evmStateAdapter struct {
	getAccount func([]byte) (*AccountState, error)
	setAccount func([]byte, *AccountState) error
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
