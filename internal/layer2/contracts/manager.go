package contracts

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/viri-chain/viri/internal/layer2/vm"
)

// StandardContract defines a native Go contract that can be called directly.
type StandardContract interface {
	ExecuteCall(caller, input []byte) ([]byte, error)
}

// Well-known precompile addresses for standard contracts.
var (
	AddrERC20  = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xE0}
	AddrERC721 = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xE1}
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
	mu         sync.RWMutex
	contracts  map[string]*Contract
	vm         *vm.WasmVM
	standards  map[string]StandardContract
}

func NewContractManager() *ContractManager {
	cm := &ContractManager{
		contracts: make(map[string]*Contract),
		vm:        vm.NewWasmVM(1000000),
		standards: make(map[string]StandardContract),
	}
	cm.registerDefaultContracts()
	return cm
}

// registerDefaultContracts registers the built-in standard contracts.
func (cm *ContractManager) registerDefaultContracts() {
	cm.standards[string(AddrERC20)] = NewERC20Token("Standard Token", "STD", 18, new(big.Int), []byte{})
	cm.standards[string(AddrERC721)] = NewERC721Token("Standard NFT", "SNFT", "https://viri-chain.io/nft/")
}

// RegisterStandardContract adds a custom standard contract at the given address.
func (cm *ContractManager) RegisterStandardContract(addr []byte, sc StandardContract) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.standards[string(addr)] = sc
}

// GetStandardContract returns a standard contract by address, or nil.
func (cm *ContractManager) GetStandardContract(addr []byte) StandardContract {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.standards[string(addr)]
}

// IsStandardContract returns true if the address is a registered standard contract.
func (cm *ContractManager) IsStandardContract(addr []byte) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.standards[string(addr)]
	return ok
}

// DeployStandardERC20 deploys a new ERC20 token at a derived address.
func (cm *ContractManager) DeployStandardERC20(owner []byte, name, symbol string, decimals uint8, initialSupply uint64) *ERC20Token {
	token := NewERC20Token(name, symbol, decimals, new(big.Int).SetUint64(initialSupply), owner)
	addr := make([]byte, 20)
	copy(addr, owner[:20])
	addr[0] ^= 0xE0
	cm.RegisterStandardContract(addr, token)
	return token
}

// DeployStandardERC721 deploys a new ERC721 NFT collection at a derived address.
func (cm *ContractManager) DeployStandardERC721(owner []byte, name, symbol, baseURI string) *ERC721Token {
	token := NewERC721Token(name, symbol, baseURI)
	addr := make([]byte, 20)
	copy(addr, owner[:20])
	addr[0] ^= 0xE1
	cm.RegisterStandardContract(addr, token)
	return token
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
