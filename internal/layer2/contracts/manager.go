package contracts

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"

	"github.com/viri-chain/viri/internal/layer1/crypto"
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
	Storage    map[string][]byte
}

type ContractManager struct {
	mu         sync.RWMutex
	contracts  map[string]*Contract
	standards  map[string]StandardContract
}

func NewContractManager() *ContractManager {
	cm := &ContractManager{
		contracts: make(map[string]*Contract),
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
	key := append([]byte("erc20"), owner...)
	addr := crypto.Keccak256(key)[:20]
	addr[0] = 0xE0
	cm.RegisterStandardContract(addr, token)
	return token
}

// DeployStandardERC721 deploys a new ERC721 NFT collection at a derived address.
func (cm *ContractManager) DeployStandardERC721(owner []byte, name, symbol, baseURI string) *ERC721Token {
	token := NewERC721Token(name, symbol, baseURI)
	key := append([]byte("erc721"), owner...)
	addr := crypto.Keccak256(key)[:20]
	addr[0] = 0xE1
	cm.RegisterStandardContract(addr, token)
	return token
}

func (cm *ContractManager) Deploy(owner []byte, code []byte, blockHeight uint64) (*Contract, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	nonce := uint64(len(cm.contracts))
	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	input := append(append([]byte{}, owner...), nonceBytes...)
	addr := crypto.Keccak256(input)[12:]

	contract := &Contract{
		Address:   addr,
		Code:      code,
		CodeHash:  crypto.Keccak256(code),
		Owner:     owner,
		Balance:   0,
		CreatedAt: blockHeight,
		UpdatedAt: blockHeight,
		Storage:   make(map[string][]byte),
	}

	key := string(addr)
	cm.contracts[key] = contract

	return contract, nil
}

func (cm *ContractManager) Execute(address, caller, input []byte, blockHeight uint64) ([]byte, error) {
	cm.mu.RLock()
	contract, exists := cm.contracts[string(address)]
	cm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("contract not found")
	}

	st := &contractManagerState{cm: cm, addr: address}
	ctx := &vm.EVMContext{
		Caller:   caller,
		Address:  address,
		Value:    big.NewInt(0),
		GasLimit: 1000000,
		GasPrice: big.NewInt(0),
		Data:     input,
	}

	e := vm.NewEVMExecutor(ctx, st)
	retdata, _, err := e.Execute(contract.Code)
	if err != nil {
		return nil, err
	}

	cm.mu.Lock()
	contract.UpdatedAt = blockHeight
	cm.mu.Unlock()

	return retdata, nil
}

type contractManagerState struct {
	cm   *ContractManager
	addr []byte
	logs []string
}

func (s *contractManagerState) GetBalance(addr []byte) *big.Int {
	s.cm.mu.RLock()
	defer s.cm.mu.RUnlock()
	c, exists := s.cm.contracts[string(addr)]
	if !exists {
		return big.NewInt(0)
	}
	return new(big.Int).SetUint64(c.Balance)
}
func (s *contractManagerState) GetNonce([]byte) uint64                                   { return 0 }
func (s *contractManagerState) GetCode(addr []byte) []byte {
	s.cm.mu.RLock()
	defer s.cm.mu.RUnlock()
	c, exists := s.cm.contracts[string(addr)]
	if !exists {
		return nil
	}
	return c.Code
}
func (s *contractManagerState) GetStorage(addr []byte, key []byte) []byte {
	s.cm.mu.RLock()
	defer s.cm.mu.RUnlock()
	c, exists := s.cm.contracts[string(addr)]
	if !exists || c.Storage == nil {
		return nil
	}
	return c.Storage[string(key)]
}
func (s *contractManagerState) SetStorage(addr []byte, key []byte, value []byte) {
	s.cm.mu.Lock()
	defer s.cm.mu.Unlock()
	c, exists := s.cm.contracts[string(addr)]
	if !exists {
		return
	}
	if c.Storage == nil {
		c.Storage = make(map[string][]byte)
	}
	if value == nil {
		delete(c.Storage, string(key))
	} else {
		c.Storage[string(key)] = append([]byte(nil), value...)
	}
}
func (s *contractManagerState) Transfer(from, to []byte, amount *big.Int) {
	if !amount.IsUint64() || amount.Sign() == 0 {
		return
	}
	s.cm.mu.Lock()
	defer s.cm.mu.Unlock()
	a := amount.Uint64()
	f, fok := s.cm.contracts[string(from)]
	t, tok := s.cm.contracts[string(to)]
	if !fok {
		return
	}
	if f.Balance < a {
		return
	}
	f.Balance -= a
	if !tok {
		t = &Contract{Address: append([]byte(nil), to...), Storage: make(map[string][]byte)}
		s.cm.contracts[string(to)] = t
	}
	t.Balance += a
}
func (s *contractManagerState) CreateAccount(addr []byte) {
	s.cm.mu.Lock()
	defer s.cm.mu.Unlock()
	if _, exists := s.cm.contracts[string(addr)]; !exists {
		s.cm.contracts[string(addr)] = &Contract{Address: append([]byte(nil), addr...), Storage: make(map[string][]byte)}
	}
}
func (s *contractManagerState) AddLog(addr []byte, topics [][]byte, data []byte) {
	s.logs = append(s.logs, fmt.Sprintf("LOG: addr=%x topics=%x data=%x", addr, topics, data))
}
func (s *contractManagerState) Snapshot() int { return len(s.logs) }
func (s *contractManagerState) RevertToSnapshot(id int) {
	if id >= 0 && id < len(s.logs) {
		s.logs = s.logs[:id]
	}
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
	cloned := &Contract{
		Address:   append([]byte(nil), c.Address...),
		Code:      append([]byte(nil), c.Code...),
		CodeHash:  append([]byte(nil), c.CodeHash...),
		ABI:       c.ABI,
		Owner:     append([]byte(nil), c.Owner...),
		Balance:   c.Balance,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Storage:   make(map[string][]byte),
	}
	for k, v := range c.Storage {
		cloned.Storage[k] = append([]byte(nil), v...)
	}
	return cloned
}
