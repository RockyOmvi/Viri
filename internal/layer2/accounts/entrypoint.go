package accounts

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/viri-chain/viri/internal/layer2/vm"
)

// UserOperation is the EIP-4337-like account abstraction entry point.
// Instead of executing a raw transaction, users submit a UserOperation
// describing what their smart wallet should do. The EntryPoint validates
// the operation and executes it via the wallet's code.
type UserOperation struct {
	Sender         []byte // smart wallet address
	Nonce          uint64
	InitCode       []byte // deployment code (empty if already deployed)
	CallData       []byte // calldata to execute on the sender wallet
	GasLimit       uint64
	MaxFee         uint64 // max fee per gas (wei equivalent)
	MaxPriorityFee uint64 // max priority fee per gas
	Paymaster      []byte // paymaster address (nil = self-pay)
	Signature      []byte
}

// UserOpHash computes the unique hash for a UserOperation.
func UserOpHash(op *UserOperation, chainID uint64) []byte {
	h := sha256.New()
	h.Write(op.Sender)
	binary.Write(h, binary.BigEndian, op.Nonce)
	h.Write(op.InitCode)
	h.Write(op.CallData)
	binary.Write(h, binary.BigEndian, op.GasLimit)
	binary.Write(h, binary.BigEndian, op.MaxFee)
	binary.Write(h, binary.BigEndian, op.MaxPriorityFee)
	h.Write(op.Paymaster)
	binary.Write(h, binary.BigEndian, chainID)
	return h.Sum(nil)
}

// EntryPoint handles UserOperation validation and execution.
type EntryPoint struct {
	manager   *AccountManager
	chainID   uint64
	codeStore map[string][]byte // codeHash -> raw code bytes
}

// NewEntryPoint creates a new account abstraction entry point.
// Accepts optional VM executor for future extension.
func NewEntryPoint(manager *AccountManager, chainID uint64, _ ...vm.Executor) *EntryPoint {
	return &EntryPoint{
		manager:   manager,
		chainID:   chainID,
		codeStore: make(map[string][]byte),
	}
}

// HandleOps processes a batch of UserOperations.
// Each operation is validated, then executed. The beneficiary
// receives accumulated fees.
func (ep *EntryPoint) HandleOps(ops []UserOperation, beneficiary []byte) ([]OpResult, error) {
	results := make([]OpResult, 0, len(ops))

	for _, op := range ops {
		result, err := ep.handleOp(&op)
		if err != nil {
			senderPrefix := op.Sender
			if len(senderPrefix) > 4 {
				senderPrefix = senderPrefix[:4]
			}
			return results, fmt.Errorf("op[%x]: %w", senderPrefix, err)
		}
		results = append(results, result)

		// Transfer collected fee to beneficiary
		if result.FeeCollected > 0 {
			_ = ep.manager.Transfer(op.Sender, beneficiary, result.FeeCollected)
		}
	}

	return results, nil
}

// OpResult contains the outcome of a single UserOperation.
type OpResult struct {
	Sender       []byte
	Success      bool
	GasUsed      uint64
	FeeCollected uint64
	ReturnData   []byte
	Logs         []string
}

func (ep *EntryPoint) handleOp(op *UserOperation) (OpResult, error) {
	if err := ep.validateOp(op); err != nil {
		return OpResult{Sender: op.Sender, Success: false}, err
	}

	// Deploy wallet if InitCode is present
	if len(op.InitCode) > 0 {
		if err := ep.deployWallet(op); err != nil {
			return OpResult{Sender: op.Sender, Success: false}, fmt.Errorf("deploy: %w", err)
		}
	}

	// Execute the operation
	return ep.executeOp(op)
}

func (ep *EntryPoint) validateOp(op *UserOperation) error {
	if len(op.Sender) == 0 {
		return fmt.Errorf("empty sender")
	}

	// Get or create account
	acc, exists := ep.manager.GetAccount(op.Sender)
	if !exists {
		if len(op.InitCode) == 0 {
			return fmt.Errorf("account not found and no init code")
		}
		return nil // will be deployed
	}

	// Nonce check
	if op.Nonce != acc.Nonce {
		return fmt.Errorf("invalid nonce: expected %d, got %d", acc.Nonce, op.Nonce)
	}

	// Signature validation via wallet's registered signers
	if len(acc.Signers) > 0 {
		opHash := UserOpHash(op, ep.chainID)
		if !ep.verifyOpSignature(opHash, op.Signature, acc.Signers, acc.Threshold) {
			return fmt.Errorf("invalid signature")
		}
	}

	// Fee check
	if len(op.Paymaster) == 0 {
		// Self-pay: ensure sender has enough balance
		totalFee := op.GasLimit * op.MaxFee
		if acc.Balance < totalFee {
			return fmt.Errorf("insufficient balance for fee: have %d, need %d", acc.Balance, totalFee)
		}
	}

	return nil
}

func (ep *EntryPoint) verifyOpSignature(hash, sig []byte, signers [][]byte, threshold uint8) bool {
	if threshold == 0 {
		threshold = 1
	}
	if len(signers) < int(threshold) {
		return false
	}
	if len(sig) == 0 {
		return false
	}
	// TODO: Execute wallet code via EVM validateUserOp for real signature verification.
	// The stub below accepts any non-empty sig as a placeholder.
	return true
}

func (ep *EntryPoint) deployWallet(op *UserOperation) error {
	if len(op.InitCode) == 0 {
		return nil
	}

	codeHash := sha256.Sum256(op.InitCode)
	ch := codeHash[:]

	// Store the code for later retrieval by executeWalletCode
	ep.codeStore[string(ch)] = append([]byte(nil), op.InitCode...)

	acc := &Account{
		Address:   op.Sender,
		Type:      AccountTypeSmartWallet,
		Balance:   0,
		Nonce:     0,
		CodeHash:  ch,
		Storage:   make(map[string][]byte),
		Signers:   [][]byte{ch},
		Threshold: 1,
	}

	return ep.manager.SetAccountDirect(op.Sender, acc)
}

func (ep *EntryPoint) executeOp(op *UserOperation) (OpResult, error) {
	acc, exists := ep.manager.GetAccount(op.Sender)
	if !exists {
		return OpResult{Sender: op.Sender, Success: false}, fmt.Errorf("account not found")
	}

	gasUsed := op.GasLimit
	fee := gasUsed * op.MaxFee

	// If paymaster, fee goes to paymaster instead
	if len(op.Paymaster) > 0 {
		ep.manager.Transfer(op.Sender, op.Paymaster, fee)
	}

	// Execute the callData against the wallet's code
	// (In production, this routes through the EVM/WASM executor)
	returnData := ep.executeWalletCode(acc, op.CallData)

	// Collect fee
	var feeCollected uint64
	if len(op.Paymaster) == 0 {
		feeCollected = fee
	}

	// Increment nonce
	acc.Nonce++
	ep.manager.SetAccountDirect(op.Sender, acc)

	return OpResult{
		Sender:       op.Sender,
		Success:      true,
		GasUsed:      gasUsed,
		FeeCollected: feeCollected,
		ReturnData:   returnData,
	}, nil
}

func (ep *EntryPoint) executeWalletCode(acc *Account, callData []byte) []byte {
	if len(acc.CodeHash) == 0 {
		return nil
	}
	code, ok := ep.codeStore[string(acc.CodeHash)]
	if !ok || len(code) == 0 {
		return nil
	}

	adapter := &entryPointStateAdapter{
		manager:   ep.manager,
		codeStore: ep.codeStore,
	}
	ctx := &vm.EVMContext{
		Caller:   acc.Address,
		Address:  acc.Address,
		Value:    big.NewInt(0),
		GasLimit: 5000000,
		GasPrice: big.NewInt(0),
		Data:     callData,
	}
	exec := vm.NewEVMExecutor(ctx, adapter)
	result, _, err := exec.Execute(code)
	if err != nil {
		return nil
	}
	return result
}

// entryPointStateAdapter bridges AccountManager + codeStore to vm.EVMState.
type entryPointStateAdapter struct {
	manager   *AccountManager
	codeStore map[string][]byte
}

func (a *entryPointStateAdapter) GetNonce(addr []byte) uint64 {
	acct, exists := a.manager.GetAccount(addr)
	if !exists {
		return 0
	}
	return acct.Nonce
}

func (a *entryPointStateAdapter) GetBalance(addr []byte) *big.Int {
	acct, exists := a.manager.GetAccount(addr)
	if !exists {
		return big.NewInt(0)
	}
	return new(big.Int).SetUint64(acct.Balance)
}

func (a *entryPointStateAdapter) GetCode(addr []byte) []byte {
	acct, exists := a.manager.GetAccount(addr)
	if !exists || len(acct.CodeHash) == 0 {
		return nil
	}
	return a.codeStore[string(acct.CodeHash)]
}

func (a *entryPointStateAdapter) GetStorage(addr []byte, key []byte) []byte {
	acct, exists := a.manager.GetAccount(addr)
	if !exists || acct.Storage == nil {
		return nil
	}
	return acct.Storage[string(key)]
}

func (a *entryPointStateAdapter) SetStorage(addr []byte, key []byte, value []byte) {
	_ = a.manager.SetStorage(addr, string(key), value)
}

func (a *entryPointStateAdapter) Transfer(from, to []byte, amount *big.Int) {
	if !amount.IsUint64() {
		return
	}
	_ = a.manager.Transfer(from, to, amount.Uint64())
}

func (a *entryPointStateAdapter) CreateAccount(addr []byte) {
	_, _ = a.manager.CreateAccount(addr, AccountTypeSmartWallet, 0)
}

// NewAccountWithSigners creates a smart wallet with threshold signers.
func (ep *EntryPoint) NewAccountWithSigners(address []byte, signers [][]byte, threshold uint8) error {
	acc := &Account{
		Address:   address,
		Type:      AccountTypeSmartWallet,
		Balance:   0,
		Nonce:     0,
		Threshold: threshold,
		Signers:   signers,
		Storage:   make(map[string][]byte),
	}
	return ep.manager.SetAccountDirect(address, acc)
}
