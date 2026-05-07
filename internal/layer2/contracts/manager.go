package contracts

import (
	"fmt"
	"sync"

	"github.com/viri-chain/viri/internal/layer2/vm"
)

type Contract struct {
	Address    []byte
	Code       []byte
	CodeHash   []byte
	ABI        string
	Owner      []byte
	Balance    uint64
	CreatedAt  uint64
	UpdatedAt  uint64
}

type ContractManager struct {
	mu        sync.RWMutex
	contracts map[string]*Contract
	vm        *vm.WasmVM
}

func NewContractManager() *ContractManager {
	return &ContractManager{
		contracts: make(map[string]*Contract),
		vm:        vm.NewWasmVM(1000000),
	}
}

func (cm *ContractManager) Deploy(owner []byte, code []byte, blockHeight uint64) (*Contract, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	addr := make([]byte, 20)
	copy(addr, owner)

	contract := &Contract{
		Address:   addr,
		Code:      code,
		CodeHash:  vm.U64(uint64(len(code))),
		Owner:     owner,
		Balance:   0,
		CreatedAt: blockHeight,
		UpdatedAt: blockHeight,
	}

	key := string(addr)
	cm.contracts[key] = contract

	return contract, nil
}

func (cm *ContractManager) Execute(address []byte, input []byte, blockHeight uint64) ([]byte, error) {
	cm.mu.RLock()
	contract, exists := cm.contracts[string(address)]
	cm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("contract not found")
	}

	vm := vm.NewWasmVM(1000000)
	vm.SetMemory(0, input)

	result := vm.Execute(contract.Code, nil)
	if result.Err != nil {
		return nil, result.Err
	}

	cm.mu.Lock()
	contract.UpdatedAt = blockHeight
	cm.mu.Unlock()

	return result.ReturnData, nil
}

func (cm *ContractManager) GetContract(address []byte) (*Contract, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	contract, exists := cm.contracts[string(address)]
	if !exists {
		return nil, false
	}

	return contract.Clone(), true
}

func (cm *ContractManager) HasContract(address []byte) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	_, exists := cm.contracts[string(address)]
	return exists
}

func (cm *ContractManager) SetBalance(address []byte, balance uint64) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	contract, exists := cm.contracts[string(address)]
	if !exists {
		return fmt.Errorf("contract not found")
	}

	contract.Balance = balance
	return nil
}

func (cm *ContractManager) GetBalance(address []byte) (uint64, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	contract, exists := cm.contracts[string(address)]
	if !exists {
		return 0, fmt.Errorf("contract not found")
	}

	return contract.Balance, nil
}

func (c *Contract) Clone() *Contract {
	return &Contract{
		Address:   append([]byte(nil), c.Address...),
		Code:      append([]byte(nil), c.Code...),
		CodeHash:  append([]byte(nil), c.CodeHash...),
		ABI:       c.ABI,
		Owner:     append([]byte(nil), c.Owner...),
		Balance:   c.Balance,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
