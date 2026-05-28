package contracts

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"
)

// ERC20Selector represents a 4-byte function selector for ERC20 methods.
type ERC20Selector [4]byte

var (
	selTotalSupply   = ERC20Selector{0x18, 0x16, 0x0d, 0xdd}
	selBalanceOf     = ERC20Selector{0x70, 0xa0, 0x82, 0x31}
	selTransfer      = ERC20Selector{0xa9, 0x05, 0x9c, 0xbb}
	selTransferFrom  = ERC20Selector{0x23, 0xb8, 0x72, 0xdd}
	selApprove       = ERC20Selector{0x09, 0x5e, 0xa7, 0xb3}
	selAllowance     = ERC20Selector{0xdd, 0x62, 0xed, 0x3e}
	selName          = ERC20Selector{0x06, 0xfd, 0xde, 0x03}
	selSymbol        = ERC20Selector{0x95, 0xd8, 0x9b, 0x41}
	selDecimals      = ERC20Selector{0x31, 0x3c, 0xe5, 0x67}
)

// ERC20Token is a native Go ERC20 token contract.
type ERC20Token struct {
	mu         sync.RWMutex
	name       string
	symbol     string
	decimals   uint8
	totalSupply *big.Int
	balances   map[string]*big.Int
	allowances map[string]*big.Int // from+"|"+spender -> amount
}

// NewERC20Token creates a new ERC20 token with the given parameters and mints initial supply to the deployer.
func NewERC20Token(name, symbol string, decimals uint8, initialSupply *big.Int, deployer []byte) *ERC20Token {
	t := &ERC20Token{
		name:       name,
		symbol:     symbol,
		decimals:   decimals,
		totalSupply: new(big.Int).Set(initialSupply),
		balances:   make(map[string]*big.Int),
		allowances: make(map[string]*big.Int),
	}
	addr := string(deployer)
	t.balances[addr] = new(big.Int).Set(initialSupply)
	return t
}

// ExecuteCall processes an ERC20 ABI-encoded call and returns ABI-encoded output.
func (t *ERC20Token) ExecuteCall(caller, input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("input too short for selector")
	}
	var sel ERC20Selector
	copy(sel[:], input[:4])
	args := input[4:]

	switch sel {
	case selTotalSupply:
		return t.handleTotalSupply()
	case selBalanceOf:
		return t.handleBalanceOf(args)
	case selTransfer:
		return t.handleTransfer(caller, args)
	case selTransferFrom:
		return t.handleTransferFrom(caller, args)
	case selApprove:
		return t.handleApprove(caller, args)
	case selAllowance:
		return t.handleAllowance(args)
	case selName:
		return t.handleName()
	case selSymbol:
		return t.handleSymbol()
	case selDecimals:
		return t.handleDecimals()
	default:
		return nil, fmt.Errorf("unknown ERC20 selector: %x", sel)
	}
}

func padTo32(v []byte) []byte {
	b := make([]byte, 32)
	copy(b[32-len(v):], v)
	return b
}

func readAddress(data []byte) []byte {
	if len(data) < 32 {
		return nil
	}
	return data[12:32]
}

func readUint256(data []byte) *big.Int {
	if len(data) < 32 {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(data[:32])
}

func readBool(data []byte) bool {
	if len(data) < 32 {
		return false
	}
	return new(big.Int).SetBytes(data[:32]).Sign() != 0
}

func u64ToBytes(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func (t *ERC20Token) handleTotalSupply() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return padTo32(t.totalSupply.Bytes()), nil
}

func (t *ERC20Token) handleBalanceOf(args []byte) ([]byte, error) {
	owner := readAddress(args)
	if owner == nil {
		return nil, fmt.Errorf("invalid balanceOf args")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	bal := t.balances[string(owner)]
	if bal == nil {
		bal = big.NewInt(0)
	}
	return padTo32(bal.Bytes()), nil
}

func (t *ERC20Token) handleTransfer(caller, args []byte) ([]byte, error) {
	if len(args) < 64 {
		return nil, fmt.Errorf("invalid transfer args")
	}
	to := readAddress(args)
	amount := readUint256(args[32:])

	t.mu.Lock()
	defer t.mu.Unlock()
	fromBal := t.balances[string(caller)]
	if fromBal == nil || fromBal.Cmp(amount) < 0 {
		return padTo32(big.NewInt(0).Bytes()), nil
	}
	fromBal.Sub(fromBal, amount)
	toAddr := string(to)
	if t.balances[toAddr] == nil {
		t.balances[toAddr] = big.NewInt(0)
	}
	t.balances[toAddr].Add(t.balances[toAddr], amount)
	return padTo32(big.NewInt(1).Bytes()), nil
}

func (t *ERC20Token) handleTransferFrom(caller, args []byte) ([]byte, error) {
	if len(args) < 96 {
		return nil, fmt.Errorf("invalid transferFrom args")
	}
	from := readAddress(args)
	to := readAddress(args[32:])
	amount := readUint256(args[64:])

	allowanceKey := string(from) + "|" + string(caller)

	t.mu.Lock()
	defer t.mu.Unlock()
	fromBal := t.balances[string(from)]
	allowance := t.allowances[allowanceKey]
	if fromBal == nil || fromBal.Cmp(amount) < 0 {
		return padTo32(big.NewInt(0).Bytes()), nil
	}
	if allowance == nil || allowance.Cmp(amount) < 0 {
		return padTo32(big.NewInt(0).Bytes()), nil
	}
	fromBal.Sub(fromBal, amount)
	allowance.Sub(allowance, amount)
	toAddr := string(to)
	if t.balances[toAddr] == nil {
		t.balances[toAddr] = big.NewInt(0)
	}
	t.balances[toAddr].Add(t.balances[toAddr], amount)
	return padTo32(big.NewInt(1).Bytes()), nil
}

func (t *ERC20Token) handleApprove(caller, args []byte) ([]byte, error) {
	if len(args) < 64 {
		return nil, fmt.Errorf("invalid approve args")
	}
	spender := readAddress(args)
	amount := readUint256(args[32:])

	t.mu.Lock()
	defer t.mu.Unlock()
	key := string(caller) + "|" + string(spender)
	t.allowances[key] = new(big.Int).Set(amount)
	return padTo32(big.NewInt(1).Bytes()), nil
}

func (t *ERC20Token) handleAllowance(args []byte) ([]byte, error) {
	if len(args) < 64 {
		return nil, fmt.Errorf("invalid allowance args")
	}
	owner := readAddress(args)
	spender := readAddress(args[32:])

	t.mu.RLock()
	defer t.mu.RUnlock()
	allowance := t.allowances[string(owner)+"|"+string(spender)]
	if allowance == nil {
		allowance = big.NewInt(0)
	}
	return padTo32(allowance.Bytes()), nil
}

func (t *ERC20Token) handleName() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return encodeString(t.name), nil
}

func (t *ERC20Token) handleSymbol() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return encodeString(t.symbol), nil
}

func (t *ERC20Token) handleDecimals() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return padTo32([]byte{t.decimals}), nil
}

func encodeString(s string) []byte {
	data := []byte(s)
	// ABI encoding: offset(32) + length(32) + data(padded to 32)
	offset := big.NewInt(32).Bytes()
	offsetPart := padTo32(offset)
	lenPart := padTo32(big.NewInt(int64(len(data))).Bytes())
	dataPart := make([]byte, 32*((len(data)+31)/32))
	copy(dataPart, data)
	return append(append(offsetPart, lenPart...), dataPart...)
}
