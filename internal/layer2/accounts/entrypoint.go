package accounts

import (
	"encoding/binary"
	"fmt"
	"math/big"

	sececdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer2/vm"
	"golang.org/x/crypto/sha3"
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
	PaymasterData  []byte // data for paymaster validation
	Signature      []byte
}

// UserOpHash computes the unique hash for a UserOperation.
func UserOpHash(op *UserOperation, entryPointAddr []byte, chainID uint64) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(op.Sender)
	binary.Write(h, binary.BigEndian, op.Nonce)
	h.Write(op.InitCode)
	h.Write(op.CallData)
	binary.Write(h, binary.BigEndian, op.GasLimit)
	binary.Write(h, binary.BigEndian, op.MaxFee)
	binary.Write(h, binary.BigEndian, op.MaxPriorityFee)
	h.Write(op.Paymaster)
	h.Write(op.PaymasterData)
	h.Write(entryPointAddr)
	binary.Write(h, binary.BigEndian, chainID)
	return h.Sum(nil)
}

// EntryPoint handles UserOperation validation and execution.
type EntryPoint struct {
	manager     *AccountManager
	chainID     uint64
	address     []byte
	codeStore   map[string][]byte // codeHash -> raw code bytes
	walletNonce uint64            // counter for deterministic wallet addresses
}

// Address returns the entry point contract address.
func (ep *EntryPoint) Address() []byte { return ep.address }

// NewEntryPoint creates a new account abstraction entry point.
func NewEntryPoint(manager *AccountManager, chainID uint64, address []byte) *EntryPoint {
	if address == nil {
		address = []byte("EntryPoint")
	}
	return &EntryPoint{
		manager:   manager,
		chainID:   chainID,
		address:   address,
		codeStore: make(map[string][]byte),
	}
}

// HandleOps processes a batch of UserOperations.
// Each operation is validated, then executed. The beneficiary
// receives accumulated fees.
func (ep *EntryPoint) HandleOps(ops []UserOperation, beneficiary []byte) ([]OpResult, error) {
	results := make([]OpResult, 0, len(ops))

	for i := range ops {
		op := &ops[i]
		result, err := ep.handleOp(op, beneficiary)
		if err != nil {
			senderPrefix := op.Sender
			if len(senderPrefix) > 4 {
				senderPrefix = senderPrefix[:4]
			}
			return results, fmt.Errorf("op[%x]: %w", senderPrefix, err)
		}
		results = append(results, result)
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

func (ep *EntryPoint) handleOp(op *UserOperation, beneficiary []byte) (OpResult, error) {
	if err := ep.validateOp(op); err != nil {
		return OpResult{Sender: op.Sender, Success: false}, err
	}

	if len(op.InitCode) > 0 {
		if err := ep.deployWallet(op); err != nil {
			return OpResult{Sender: op.Sender, Success: false}, fmt.Errorf("deploy: %w", err)
		}
	}

	return ep.executeOp(op, beneficiary)
}

func (ep *EntryPoint) validateOp(op *UserOperation) error {
	if len(op.Sender) == 0 {
		return fmt.Errorf("empty sender")
	}

	acc, exists := ep.manager.GetAccount(op.Sender)
	if !exists {
		if len(op.InitCode) == 0 {
			return fmt.Errorf("account not found and no init code")
		}
		return nil
	}

	if op.Nonce != acc.Nonce {
		return fmt.Errorf("invalid nonce: expected %d, got %d", acc.Nonce, op.Nonce)
	}

	opHash := UserOpHash(op, ep.address, ep.chainID)
	if len(acc.Signers) > 0 {
		if !ep.verifyOpSignature(opHash, op.Signature, acc.Signers, acc.Threshold) {
			return fmt.Errorf("invalid signature")
		}
	}

	if len(op.Paymaster) == 0 {
		totalFee := op.GasLimit * op.MaxFee
		if acc.Balance.Cmp(new(big.Int).SetUint64(totalFee)) < 0 {
			return fmt.Errorf("insufficient balance for fee: have %s, need %d", acc.Balance.String(), totalFee)
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
	recoveredAddr, err := recoverSigner(hash, sig)
	if err != nil {
		return false
	}
	validCount := uint8(0)
	for _, signer := range signers {
		if string(recoveredAddr) == string(signer) {
			validCount++
			if validCount >= threshold {
				return true
			}
		}
	}
	return false
}

// recoverSigner recovers the signer address from a hash and 65-byte signature.
// The signature format is [R (32)][S (32)][V (1)] where V = 27 + recoveryID.
func recoverSigner(hash, sig []byte) ([]byte, error) {
	if len(sig) != 65 {
		return nil, fmt.Errorf("invalid signature length: %d", len(sig))
	}
	v := sig[64]
	var recID byte
	if v >= 27 {
		recID = v - 27
	} else {
		recID = v
	}
	if recID > 3 {
		return nil, fmt.Errorf("invalid recovery ID: %d", recID)
	}
	compactSig := make([]byte, 65)
	compactSig[0] = 27 + recID
	copy(compactSig[1:33], sig[:32])
	copy(compactSig[33:65], sig[32:64])
	sigHash := crypto.Keccak256(hash)
	pubKey, _, err := sececdsa.RecoverCompact(compactSig, sigHash)
	if err != nil {
		return nil, err
	}
	// Serialize and re-parse to get a crypto.PublicKey for .Address()
	raw := pubKey.SerializeUncompressed()
	parsed, err := crypto.PubKeyFromBytes(raw)
	if err != nil {
		return nil, err
	}
	return parsed.Address(), nil
}

func (ep *EntryPoint) deployWallet(op *UserOperation) error {
	if len(op.InitCode) == 0 {
		return nil
	}

	// Check for existing account to prevent overwrite
	if ep.manager.HasAccount(op.Sender) {
		return fmt.Errorf("account already exists at sender address")
	}

	codeHash := crypto.Keccak256(op.InitCode)
	ep.codeStore[string(codeHash)] = append([]byte(nil), op.InitCode...)

	acc := &Account{
		Address:   op.Sender,
		Type:      AccountTypeSmartWallet,
		Balance:   new(big.Int),
		Nonce:     0,
		CodeHash:  codeHash,
		Storage:   make(map[string][]byte),
		Signers:   [][]byte{},
		Threshold: 1,
	}

	return ep.manager.SetAccountDirect(op.Sender, acc)
}

func (ep *EntryPoint) executeOp(op *UserOperation, beneficiary []byte) (OpResult, error) {
	acc, exists := ep.manager.GetAccount(op.Sender)
	if !exists {
		return OpResult{Sender: op.Sender, Success: false}, fmt.Errorf("account not found")
	}

	adapter := &entryPointStateAdapter{
		manager:   ep.manager,
		codeStore: ep.codeStore,
		logs:      nil,
	}
	returnData, actualGas, execErr := ep.executeWalletCode(acc, op, adapter)

	gasUsed := actualGas
	if gasUsed > op.GasLimit {
		gasUsed = op.GasLimit
	}
	fee := gasUsed*op.MaxFee + gasUsed*op.MaxPriorityFee

	var feeCollected uint64
	if execErr == nil {
		if len(op.Paymaster) == 0 {
			feeCollected = fee
			if err := ep.manager.Transfer(op.Sender, beneficiary, fee); err != nil {
				feeCollected = 0
			}
		} else {
			feeCollected = fee
			if err := ep.manager.Transfer(op.Paymaster, beneficiary, fee); err != nil {
				feeCollected = 0
			}
		}
	}

	acc.Nonce++
	ep.manager.SetAccountDirect(op.Sender, acc)

	return OpResult{
		Sender:       op.Sender,
		Success:      execErr == nil,
		GasUsed:      gasUsed,
		FeeCollected: feeCollected,
		ReturnData:   returnData,
		Logs:         adapter.logs,
	}, nil
}

func (ep *EntryPoint) executeWalletCode(acc *Account, op *UserOperation, adapter *entryPointStateAdapter) ([]byte, uint64, error) {
	if len(acc.CodeHash) == 0 {
		return nil, 0, nil
	}
	code, ok := ep.codeStore[string(acc.CodeHash)]
	if !ok || len(code) == 0 {
		return nil, 0, nil
	}

	ctx := &vm.EVMContext{
		Caller:   acc.Address,
		Address:  acc.Address,
		Value:    big.NewInt(0),
		GasLimit: op.GasLimit,
		GasPrice: big.NewInt(0),
		Data:     op.CallData,
	}
	exec := vm.NewEVMExecutor(ctx, adapter)
	result, gasUsed, err := exec.Execute(code)
	return result, gasUsed, err
}

// entryPointStateAdapter bridges AccountManager + codeStore to vm.EVMState.
type entryPointStateAdapter struct {
	manager   *AccountManager
	codeStore map[string][]byte
	logs      []string
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
	if !exists || acct.Balance == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(acct.Balance)
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
	if !amount.IsUint64() || amount.Sign() == 0 {
		return
	}
	_ = a.manager.Transfer(from, to, amount.Uint64())
}

func (a *entryPointStateAdapter) AddLog(addr []byte, topics [][]byte, data []byte) {
	a.logs = append(a.logs, fmt.Sprintf("LOG: addr=%x topics=%x data=%x", addr, topics, data))
}

func (a *entryPointStateAdapter) Snapshot() int { return len(a.logs) }

func (a *entryPointStateAdapter) RevertToSnapshot(id int) {
	if id >= 0 && id < len(a.logs) {
		a.logs = a.logs[:id]
	}
}

func (a *entryPointStateAdapter) CreateAccount(addr []byte) {
	_, _ = a.manager.CreateAccount(addr, AccountTypeSmartWallet, 0)
}

// NewAccountWithSigners creates a smart wallet with threshold signers.
func (ep *EntryPoint) NewAccountWithSigners(address []byte, signers [][]byte, threshold uint8) error {
	acc := &Account{
		Address:   address,
		Type:      AccountTypeSmartWallet,
		Balance:   new(big.Int),
		Nonce:     0,
		Threshold: threshold,
		Signers:   signers,
		Storage:   make(map[string][]byte),
	}
	return ep.manager.SetAccountDirect(address, acc)
}
