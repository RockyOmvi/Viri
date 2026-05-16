package vm

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

var two256 = new(big.Int).Lsh(big.NewInt(1), 256)

func wrap256(x *big.Int) *big.Int {
	return new(big.Int).Mod(x, two256)
}

func safeSetBig(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(v)
}

func toSigned256(x *big.Int) *big.Int {
	b := make([]byte, 32)
	x.FillBytes(b)
	if b[0]&0x80 != 0 {
		return new(big.Int).Sub(x, two256)
	}
	return new(big.Int).Set(x)
}

func toUnsigned256(x *big.Int) *big.Int {
	if x.Sign() < 0 {
		return new(big.Int).Add(x, two256)
	}
	return new(big.Int).Set(x)
}

type EVMOpCode byte

const (
	EVMSTOP         EVMOpCode = 0x00
	EVMADD          EVMOpCode = 0x01
	EVMMUL          EVMOpCode = 0x02
	EVMSUB          EVMOpCode = 0x03
	EVMDIV          EVMOpCode = 0x04
	EVMSDIV         EVMOpCode = 0x05
	EVMMOD          EVMOpCode = 0x06
	EVMSMOD         EVMOpCode = 0x07
	EVMADDMOD       EVMOpCode = 0x08
	EVMMULMOD       EVMOpCode = 0x09
	EVMEXP          EVMOpCode = 0x0A
	EVMSIGNEXTEND   EVMOpCode = 0x0B
	EVMLT           EVMOpCode = 0x10
	EVMGT           EVMOpCode = 0x11
	EVMSLT          EVMOpCode = 0x12
	EVMSGT          EVMOpCode = 0x13
	EVMEQ           EVMOpCode = 0x14
	EVMISZERO       EVMOpCode = 0x15
	EVMAND          EVMOpCode = 0x16
	EVMOR           EVMOpCode = 0x17
	EVMXOR          EVMOpCode = 0x18
	EVMNOT          EVMOpCode = 0x19
	EVMBYTE         EVMOpCode = 0x1A
	EVMSHL          EVMOpCode = 0x1B
	EVMSHR          EVMOpCode = 0x1C
	EVMSAR          EVMOpCode = 0x1D
	EVMSHA3         EVMOpCode = 0x20
	EVMGASPRICE     EVMOpCode = 0x3A
	EVMADDRESS      EVMOpCode = 0x30
	EVMBALANCE      EVMOpCode = 0x31
	EVMORIGIN       EVMOpCode = 0x32
	EVMCALLER       EVMOpCode = 0x33
	EVMCALLVALUE    EVMOpCode = 0x34
	EVMCALLDATALOAD EVMOpCode = 0x35
	EVMCALLDATASIZE EVMOpCode = 0x36
	EVMCALLDATACOPY EVMOpCode = 0x37
	EVMCODESIZE     EVMOpCode = 0x38
	EVMCODECOPY     EVMOpCode = 0x39
	EVMPOP          EVMOpCode = 0x50
	EVMMLOAD        EVMOpCode = 0x51
	EVMMSTORE       EVMOpCode = 0x52
	EVMMSTORE8      EVMOpCode = 0x53
	EVMSLOAD        EVMOpCode = 0x54
	EVMSSTORE       EVMOpCode = 0x55
	EVMJUMP         EVMOpCode = 0x56
	EVMJUMPI        EVMOpCode = 0x57
	EVMPC           EVMOpCode = 0x58
	EVMMSIZE        EVMOpCode = 0x59
	EVMGAS          EVMOpCode = 0x5A
	EVMJUMPDEST     EVMOpCode = 0x5B
	EVMPUSH1  EVMOpCode = 0x60
	EVMPUSH32 EVMOpCode = 0x7F
	EVMDUP1   EVMOpCode = 0x80
	EVMSWAP1  EVMOpCode = 0x90
	EVMLOG0         EVMOpCode = 0xA0
	EVMLOG1         EVMOpCode = 0xA1
	EVMLOG2         EVMOpCode = 0xA2
	EVMLOG3         EVMOpCode = 0xA3
	EVMLOG4         EVMOpCode = 0xA4
	EVMCREATE       EVMOpCode = 0xF0
	EVMCALL         EVMOpCode = 0xF1
	EVMCALLCODE     EVMOpCode = 0xF2
	EVMRETURN       EVMOpCode = 0xF3
	EVMDELEGATECALL EVMOpCode = 0xF4
	EVMCREATE2      EVMOpCode = 0xF5
	EVMRETURNDATASIZE EVMOpCode = 0x3D
	EVMRETURNDATACOPY EVMOpCode = 0x3E
	EVMSTATICCALL   EVMOpCode = 0xFA
	EVMREVERT       EVMOpCode = 0xFD
	EVMINVALID      EVMOpCode = 0xFE
	EVMSELFDESTRUCT EVMOpCode = 0xFF

	EVMCOINBASE     EVMOpCode = 0x41
	EVMTIMESTAMP    EVMOpCode = 0x42
	EVMNUMBER       EVMOpCode = 0x43
	EVMPREVRANDAO   EVMOpCode = 0x44
	EVMGASLIMIT     EVMOpCode = 0x45
	EVMCHAINID      EVMOpCode = 0x46
	EVMSELFBALANCE  EVMOpCode = 0x47
	EVMBASEFEE      EVMOpCode = 0x48
	EVMBLOCKHASH    EVMOpCode = 0x40
	EXTCODESIZE     EVMOpCode = 0x3B
	EXTCODECOPY     EVMOpCode = 0x3C
	EXTCODEHASH     EVMOpCode = 0x3F

	EVMPUSH0        EVMOpCode = 0x5F
	EVMTLOAD        EVMOpCode = 0x5C
	EVMTSTORE       EVMOpCode = 0x5D
	EVMMCOPY        EVMOpCode = 0x5E

	EVMDUP16  EVMOpCode = 0x8F
	EVMSWAP16 EVMOpCode = 0x9F
)

func (op EVMOpCode) String() string {
	names := map[EVMOpCode]string{
		EVMSTOP: "STOP", EVMADD: "ADD", EVMMUL: "MUL", EVMSUB: "SUB",
		EVMDIV: "DIV", EVMSDIV: "SDIV", EVMMOD: "MOD", EVMSMOD: "SMOD",
		EVMADDMOD: "ADDMOD", EVMMULMOD: "MULMOD", EVMEXP: "EXP",
		EVMSIGNEXTEND: "SIGNEXTEND",
		EVMLT: "LT", EVMGT: "GT", EVMSLT: "SLT", EVMSGT: "SGT",
		EVMEQ: "EQ", EVMISZERO: "ISZERO",
		EVMAND: "AND", EVMOR: "OR", EVMXOR: "XOR", EVMNOT: "NOT",
		EVMBYTE: "BYTE", EVMSHL: "SHL", EVMSHR: "SHR", EVMSAR: "SAR",
		EVMSHA3: "SHA3",
		EVMADDRESS: "ADDRESS", EVMBALANCE: "BALANCE", EVMORIGIN: "ORIGIN",
		EVMCALLER: "CALLER", EVMCALLVALUE: "CALLVALUE",
		EVMCALLDATALOAD: "CALLDATALOAD", EVMCALLDATASIZE: "CALLDATASIZE", EVMCALLDATACOPY: "CALLDATACOPY",
		EVMCODESIZE: "CODESIZE", EVMCODECOPY: "CODECOPY", EVMGASPRICE: "GASPRICE",
		EVMPOP: "POP", EVMMLOAD: "MLOAD", EVMMSTORE: "MSTORE", EVMMSTORE8: "MSTORE8",
		EVMSLOAD: "SLOAD", EVMSSTORE: "SSTORE",
		EVMJUMP: "JUMP", EVMJUMPI: "JUMPI", EVMPC: "PC", EVMMSIZE: "MSIZE", EVMGAS: "GAS", EVMJUMPDEST: "JUMPDEST",
		EVMLOG0: "LOG0", EVMLOG1: "LOG1", EVMLOG2: "LOG2", EVMLOG3: "LOG3", EVMLOG4: "LOG4",
		EVMCREATE: "CREATE", EVMCALL: "CALL", EVMCALLCODE: "CALLCODE", EVMRETURN: "RETURN",
		EVMSELFDESTRUCT: "SELFDESTRUCT",
		EVMCREATE2: "CREATE2", EVMREVERT: "REVERT", EVMINVALID: "INVALID",
		EVMRETURNDATASIZE: "RETURNDATASIZE", EVMRETURNDATACOPY: "RETURNDATACOPY",
		EVMSTATICCALL: "STATICCALL", EVMDELEGATECALL: "DELEGATECALL",
		EVMCOINBASE: "COINBASE", EVMTIMESTAMP: "TIMESTAMP", EVMNUMBER: "NUMBER",
		EVMPREVRANDAO: "PREVRANDAO", EVMGASLIMIT: "GASLIMIT", EVMCHAINID: "CHAINID",
		EVMSELFBALANCE: "SELFBALANCE", EVMBASEFEE: "BASEFEE", EVMBLOCKHASH: "BLOCKHASH",
		EVMPUSH0: "PUSH0", EVMTLOAD: "TLOAD", EVMTSTORE: "TSTORE", EVMMCOPY: "MCOPY",
		EXTCODESIZE: "EXTCODESIZE", EXTCODECOPY: "EXTCODECOPY", EXTCODEHASH: "EXTCODEHASH",
	}
	if name, ok := names[op]; ok {
		return name
	}
	return fmt.Sprintf("OP_0x%x", byte(op))
}

type EVMContext struct {
	Caller    []byte
	Address   []byte
	Value     *big.Int
	GasLimit  uint64
	GasPrice  *big.Int
	Data      []byte
	BlockNum  uint64
	Timestamp uint64
	ChainID   *big.Int
	Coinbase  []byte
	BlockGasLimit uint64
	BaseFee   *big.Int
	PrevRandao []byte
	GetBlockHash func(uint64) []byte
}

type EVMState interface {
	GetBalance(addr []byte) *big.Int
	GetNonce(addr []byte) uint64
	GetCode(addr []byte) []byte
	GetStorage(addr []byte, key []byte) []byte
	SetStorage(addr []byte, key []byte, value []byte)
	Transfer(from, to []byte, amount *big.Int)
	CreateAccount(addr []byte)
	AddLog(addr []byte, topics [][]byte, data []byte)
	Snapshot() int
	RevertToSnapshot(int)
}

type TraceStep struct {
	OpName string   `json:"op"`
	PC     uint64   `json:"pc"`
	Gas    uint64   `json:"gas"`
	Stack  []string `json:"stack"`
	Memory string   `json:"memory,omitempty"`
	Depth  int      `json:"depth"`
	Error  string   `json:"error,omitempty"`
}

type EVMExecutor struct {
	stack   []*big.Int
	memory  []byte
	pc      uint64
	gasUsed uint64
	ctx     *EVMContext
	state   EVMState

	returndata  []byte
	code        []byte
	staticCall  bool
	jumpDests   map[uint64]bool

	traceCallback func(TraceStep)
}

func NewEVMExecutor(ctx *EVMContext, state EVMState) *EVMExecutor {
	return &EVMExecutor{
		stack:  make([]*big.Int, 0, 256),
		memory: make([]byte, 0, 1024),
		ctx:    ctx,
		state:  state,
	}
}

func (evm *EVMExecutor) SetTraceCallback(cb func(TraceStep)) {
	evm.traceCallback = cb
}

func (evm *EVMExecutor) emitTrace(op EVMOpCode) {
	if evm.traceCallback == nil {
		return
	}
	stack := make([]string, len(evm.stack))
	for i, v := range evm.stack {
		stack[i] = fmt.Sprintf("0x%x", v)
	}
	memStr := ""
	if len(evm.memory) > 0 && len(evm.memory) <= 128 {
		memStr = fmt.Sprintf("0x%x", evm.memory)
	} else if len(evm.memory) > 128 {
		memStr = fmt.Sprintf("0x%x", evm.memory[:128])
	}
	evm.traceCallback(TraceStep{
		OpName: op.String(),
		PC:     evm.pc - 1,
		Gas:    evm.gasUsed,
		Stack:  stack,
		Memory: memStr,
	})
}

const (
	gasFastest   uint64 = 3
	gasFast      uint64 = 5
	gasMid       uint64 = 8
	gasSlow      uint64 = 10
	gasExt       uint64 = 20
	gasSha3      uint64 = 30
	gasSha3Word  uint64 = 6
	gasSload     uint64 = 100
	gasSstoreSet uint64 = 20000
	gasSstoreReset uint64 = 5000
	gasBalance   uint64 = 700
	gasCreate    uint64 = 32000
	gasCodeDeposit uint64 = 200
	gasCall      uint64 = 700
	gasCallValue uint64 = 9000
	gasCallStipend uint64 = 2300
	gasSelfdestruct uint64 = 5000
	gasExp       uint64 = 10
	gasExpByte   uint64 = 50
	gasMemory    uint64 = 3
	gasLog       uint64 = 375
	gasLogTopic  uint64 = 375
	gasLogData   uint64 = 8
	gasCopy      uint64 = 3
	gMaxCodeSize uint64 = 24576
)

func (evm *EVMExecutor) useGas(cost uint64) error {
	evm.gasUsed += cost
	if evm.gasUsed > evm.ctx.GasLimit {
		return fmt.Errorf("out of gas")
	}
	return nil
}

func memoryGas(currentSize, newSize uint64) uint64 {
	if newSize <= currentSize {
		return 0
	}
	newWords := (newSize + 31) / 32
	oldWords := (currentSize + 31) / 32
	return (newWords - oldWords) * gasMemory
}

func (evm *EVMExecutor) expandMemory(offset, size uint64) error {
	if size == 0 {
		return nil
	}
	newSize := offset + size
	if newSize < offset {
		return fmt.Errorf("memory expansion overflow")
	}
	cost := memoryGas(uint64(len(evm.memory)), newSize)
	if err := evm.useGas(cost); err != nil {
		return err
	}
	if newSize > uint64(len(evm.memory)) {
		evm.memory = append(evm.memory, make([]byte, newSize-uint64(len(evm.memory)))...)
	}
	return nil
}

func (evm *EVMExecutor) getMemory(offset, size uint64) []byte {
	if size == 0 {
		return nil
	}
	if offset+size < offset {
		return nil
	}
	result := make([]byte, size)
	for i := uint64(0); i < size; i++ {
		idx := offset + i
		if idx < uint64(len(evm.memory)) {
			result[i] = evm.memory[idx]
		}
	}
	return result
}

func (evm *EVMExecutor) popUint64() (uint64, error) {
	if len(evm.stack) == 0 {
		return 0, fmt.Errorf("stack underflow")
	}
	val := evm.stack[len(evm.stack)-1]
	evm.stack = evm.stack[:len(evm.stack)-1]
	return val.Uint64(), nil
}

func (evm *EVMExecutor) popBig() *big.Int {
	if len(evm.stack) == 0 {
		return nil
	}
	val := evm.stack[len(evm.stack)-1]
	evm.stack = evm.stack[:len(evm.stack)-1]
	return val
}

func (evm *EVMExecutor) toAddress(val *big.Int) []byte {
	normalized := wrap256(val)
	bytes := normalized.Bytes()
	addr := make([]byte, 20)
	if len(bytes) > 20 {
		copy(addr, bytes[len(bytes)-20:])
	} else {
		copy(addr[20-len(bytes):], bytes)
	}
	return addr
}

func (evm *EVMExecutor) createAddress(caller []byte, nonce uint64) []byte {
	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	input := append(append([]byte{}, caller...), nonceBytes...)
	hash := crypto.Keccak256(input)
	return hash[12:]
}

func (evm *EVMExecutor) createAddress2(caller []byte, salt []byte, initCode []byte) []byte {
	codeHash := crypto.Keccak256(initCode)
	input := append([]byte{0xFF}, caller...)
	input = append(input, salt...)
	input = append(input, codeHash...)
	hash := crypto.Keccak256(input)
	return hash[12:]
}

func (evm *EVMExecutor) Execute(code []byte) ([]byte, uint64, error) {
	evm.pc = 0
	evm.gasUsed = 0
	evm.stack = evm.stack[:0]
	evm.code = code
	evm.jumpDests = evm.computeJumpDests(code)

	for evm.pc < uint64(len(code)) {
		op := EVMOpCode(code[evm.pc])
		evm.pc++

		evm.emitTrace(op)

		switch op {
		case EVMSTOP:
			return nil, evm.gasUsed, nil

		case EVMADD:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, wrap256(new(big.Int).Add(a, b)))

		case EVMSUB:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, wrap256(new(big.Int).Sub(a, b)))

		case EVMMUL:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, wrap256(new(big.Int).Mul(a, b)))

		case EVMDIV:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Sign() == 0 {
				evm.stack = append(evm.stack, new(big.Int))
			} else {
				evm.stack = append(evm.stack, new(big.Int).Div(a, b))
			}

		case EVMSDIV:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Sign() == 0 {
				evm.stack = append(evm.stack, new(big.Int))
			} else {
				sa := toSigned256(a)
				sb := toSigned256(b)
				evm.stack = append(evm.stack, toUnsigned256(new(big.Int).Div(sa, sb)))
			}

		case EVMMOD:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Sign() == 0 {
				evm.stack = append(evm.stack, new(big.Int))
			} else {
				evm.stack = append(evm.stack, new(big.Int).Mod(a, b))
			}

		case EVMSMOD:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Sign() == 0 {
				evm.stack = append(evm.stack, new(big.Int))
			} else {
					sa := toSigned256(a)
				sb := toSigned256(b)
				evm.stack = append(evm.stack, toUnsigned256(new(big.Int).Mod(sa, sb)))
			}

		case EVMADDMOD:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			c, b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			if c.Sign() == 0 {
				evm.stack = append(evm.stack, new(big.Int))
			} else {
				sum := new(big.Int).Add(a, b)
				evm.stack = append(evm.stack, new(big.Int).Mod(sum, c))
			}

		case EVMMULMOD:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			c, b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			if c.Sign() == 0 {
				evm.stack = append(evm.stack, new(big.Int))
			} else {
				product := new(big.Int).Mul(a, b)
				evm.stack = append(evm.stack, new(big.Int).Mod(product, c))
			}

		case EVMEXP:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			exponent, base := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			expBytes := exponent.Bytes()
			expGas := gasExp + gasExpByte*uint64(len(expBytes))
			if err := evm.useGas(expGas); err != nil {
				return nil, evm.gasUsed, err
			}
			if exponent.Sign() == 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				result := new(big.Int).Exp(base, exponent, two256)
				evm.stack = append(evm.stack, wrap256(result))
			}

		case EVMSIGNEXTEND:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Sign() < 0 {
				evm.stack = append(evm.stack, a)
				break
			}
			t := b.Uint64()
			if t >= 31 {
				evm.stack = append(evm.stack, a)
				break
			}
			bytes := make([]byte, 32)
			a.FillBytes(bytes)
			signBytePos := 31 - int(t)
			signBit := (bytes[signBytePos] >> 7) & 1
			fillByte := byte(0)
			if signBit == 1 {
				fillByte = 0xFF
			}
			for i := 0; i < signBytePos; i++ {
				bytes[i] = fillByte
			}
			evm.stack = append(evm.stack, new(big.Int).SetBytes(bytes))

		case EVMLT:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Cmp(a) < 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMGT:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Cmp(a) > 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMSLT:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			sb, sa := toSigned256(b), toSigned256(a)
			if sb.Cmp(sa) < 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMSGT:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			sb, sa := toSigned256(b), toSigned256(a)
			if sb.Cmp(sa) > 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMEQ:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if a.Cmp(b) == 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMISZERO:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			a := evm.stack[len(evm.stack)-1]
			evm.stack[len(evm.stack)-1] = big.NewInt(0)
			if a.Sign() == 0 {
				evm.stack[len(evm.stack)-1] = big.NewInt(1)
			}

		case EVMAND:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, new(big.Int).And(a, b))

		case EVMOR:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, new(big.Int).Or(a, b))

		case EVMXOR:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, new(big.Int).Xor(a, b))

		case EVMNOT:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			a := evm.stack[len(evm.stack)-1]
			mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
			evm.stack[len(evm.stack)-1] = new(big.Int).Xor(a, mask)

		case EVMBYTE:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			pos, val := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if pos.Sign() >= 0 && pos.Cmp(big.NewInt(31)) <= 0 {
				shift := new(big.Int).Sub(big.NewInt(31), pos)
				shift = shift.Mul(shift, big.NewInt(8))
				byteVal := new(big.Int).Rsh(val, uint(shift.Uint64()))
				byteVal = byteVal.And(byteVal, big.NewInt(0xFF))
				evm.stack = append(evm.stack, byteVal)
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMSHL:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			shift, val := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			s := shift.Uint64()
			if s > 255 {
				evm.stack = append(evm.stack, big.NewInt(0))
			} else {
				r := new(big.Int).Lsh(val, uint(s))
				mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
				r.And(r, mask)
				evm.stack = append(evm.stack, r)
			}

		case EVMSHR:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			shift, val := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			s := shift.Uint64()
			if s > 255 {
				evm.stack = append(evm.stack, big.NewInt(0))
			} else {
				evm.stack = append(evm.stack, new(big.Int).Rsh(val, uint(s)))
			}

		case EVMSAR:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			shift, val := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			s := shift.Uint64()
			if s > 255 {
				s = 255
			}
			sval := toSigned256(val)
			r := new(big.Int).Rsh(sval, uint(s))
			evm.stack = append(evm.stack, toUnsigned256(r))

		case EVMSHA3:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if size.Sign() < 0 || offset.Sign() < 0 {
				return nil, evm.gasUsed, fmt.Errorf("negative sha3 offset or size")
			}
			off := offset.Uint64()
			sz := size.Uint64()
			if err := evm.expandMemory(off, sz); err != nil {
				return nil, evm.gasUsed, err
			}
			wordCount := (sz + 31) / 32
			sha3Gas := gasSha3 + gasSha3Word*wordCount
			if err := evm.useGas(sha3Gas); err != nil {
				return nil, evm.gasUsed, err
			}
			data := evm.getMemory(off, sz)
			hash := crypto.Keccak256(data)
			evm.stack = append(evm.stack, new(big.Int).SetBytes(hash))

		case EVMADDRESS:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.Address))

		case EVMBALANCE:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasBalance); err != nil {
				return nil, evm.gasUsed, err
			}
			addr := evm.toAddress(evm.stack[len(evm.stack)-1])
			evm.stack[len(evm.stack)-1] = evm.state.GetBalance(addr)

		case EVMORIGIN:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.Caller))

		case EVMCALLER:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.Caller))

		case EVMGASPRICE:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, safeSetBig(evm.ctx.GasPrice))

		case EVMCALLVALUE:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, safeSetBig(evm.ctx.Value))

		case EVMCALLDATALOAD:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			offset, err := evm.popUint64()
			if err != nil {
				return nil, evm.gasUsed, err
			}
			data := make([]byte, 32)
			for i := uint64(0); i < 32; i++ {
				idx := offset + i
				if idx < uint64(len(evm.ctx.Data)) {
					data[i] = evm.ctx.Data[idx]
				}
			}
			evm.stack = append(evm.stack, new(big.Int).SetBytes(data))

		case EVMCALLDATASIZE:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.ctx.Data))))

		case EVMCALLDATACOPY:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			destOffsetV, srcOffsetV, sizeV := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			if destOffsetV.Sign() < 0 || srcOffsetV.Sign() < 0 || sizeV.Sign() < 0 {
				return nil, evm.gasUsed, fmt.Errorf("negative calldatacopy offset or size")
			}
			destOffset := destOffsetV.Uint64()
			srcOffset := srcOffsetV.Uint64()
			size := sizeV.Uint64()
			if err := evm.expandMemory(destOffset, size); err != nil {
				return nil, evm.gasUsed, err
			}
			copyGas := gasFastest + gasCopy*((size+31)/32)
			if err := evm.useGas(copyGas); err != nil {
				return nil, evm.gasUsed, err
			}
			for i := uint64(0); i < size; i++ {
				idx := srcOffset + i
				if idx >= uint64(len(evm.ctx.Data)) {
					break
				}
				if destOffset+i < uint64(len(evm.memory)) {
					evm.memory[destOffset+i] = evm.ctx.Data[idx]
				}
			}

		case EVMCODESIZE:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.code))))

		case EVMCODECOPY:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			memOffsetV, codeOffsetV, sizeV := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			if memOffsetV.Sign() < 0 || codeOffsetV.Sign() < 0 || sizeV.Sign() < 0 {
				return nil, evm.gasUsed, fmt.Errorf("negative codecopy offset or size")
			}
			memOffset := memOffsetV.Uint64()
			codeOffset := codeOffsetV.Uint64()
			size := sizeV.Uint64()
			if err := evm.expandMemory(memOffset, size); err != nil {
				return nil, evm.gasUsed, err
			}
			copyGas := gasFastest + gasCopy*((size+31)/32)
			if err := evm.useGas(copyGas); err != nil {
				return nil, evm.gasUsed, err
			}
			for i := uint64(0); i < size; i++ {
				idx := codeOffset + i
				if idx >= uint64(len(evm.code)) {
					break
				}
				if memOffset+i < uint64(len(evm.memory)) {
					evm.memory[memOffset+i] = evm.code[idx]
				}
			}

		case EVMRETURNDATASIZE:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.returndata))))

		case EVMRETURNDATACOPY:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			destOffsetV, srcOffsetV, sizeV := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			if destOffsetV.Sign() < 0 || srcOffsetV.Sign() < 0 || sizeV.Sign() < 0 {
				return nil, evm.gasUsed, fmt.Errorf("negative returndatacopy offset or size")
			}
			destOffset := destOffsetV.Uint64()
			srcOffset := srcOffsetV.Uint64()
			size := sizeV.Uint64()
			if srcOffset+size < srcOffset || srcOffset+size > uint64(len(evm.returndata)) {
				return nil, evm.gasUsed, fmt.Errorf("return data copy out of bounds")
			}
			if err := evm.expandMemory(destOffset, size); err != nil {
				return nil, evm.gasUsed, err
			}
			copyGas := gasFastest + gasCopy*((size+31)/32)
			if err := evm.useGas(copyGas); err != nil {
				return nil, evm.gasUsed, err
			}
			for i := uint64(0); i < size; i++ {
				idx := srcOffset + i
				if idx < uint64(len(evm.returndata)) && destOffset+i < uint64(len(evm.memory)) {
					evm.memory[destOffset+i] = evm.returndata[idx]
				}
			}

		case EVMPOP:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = evm.stack[:len(evm.stack)-1]

		case EVMMLOAD:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, err := evm.popUint64()
			if err != nil {
				return nil, evm.gasUsed, err
			}
			if err := evm.expandMemory(offset, 32); err != nil {
				return nil, evm.gasUsed, err
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.getMemory(offset, 32)))

		case EVMMSTORE:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, err := evm.popUint64()
			if err != nil {
				return nil, evm.gasUsed, err
			}
			val := evm.popBig()
			if val == nil {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.expandMemory(offset, 32); err != nil {
				return nil, evm.gasUsed, err
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			valBytes := make([]byte, 32)
			val.FillBytes(valBytes)
			copy(evm.memory[offset:], valBytes)

		case EVMMSTORE8:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, err := evm.popUint64()
			if err != nil {
				return nil, evm.gasUsed, err
			}
			val := evm.popBig()
			if val == nil {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.expandMemory(offset, 1); err != nil {
				return nil, evm.gasUsed, err
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			valBytes := val.Bytes()
			if len(valBytes) > 0 && offset < uint64(len(evm.memory)) {
				evm.memory[offset] = valBytes[len(valBytes)-1]
			}

		case EVMSLOAD:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasSload); err != nil {
				return nil, evm.gasUsed, err
			}
			key := make([]byte, 32)
			evm.stack[len(evm.stack)-1].FillBytes(key)
			evm.stack[len(evm.stack)-1] = new(big.Int).SetBytes(evm.state.GetStorage(evm.ctx.Address, key))

		case EVMSSTORE:
			if evm.staticCall {
				return nil, evm.gasUsed, fmt.Errorf("sstore in staticcall context")
			}
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			key := make([]byte, 32)
			val := make([]byte, 32)
			evm.stack[len(evm.stack)-1].FillBytes(key)
			evm.stack[len(evm.stack)-2].FillBytes(val)
			evm.stack = evm.stack[:len(evm.stack)-2]
			current := evm.state.GetStorage(evm.ctx.Address, key)
			currentIsZero := len(current) == 0 || (len(current) == 1 && current[0] == 0)
			valIsZero := len(val) == 0 || (len(val) == 1 && val[0] == 0)
			if currentIsZero && !valIsZero {
				if err := evm.useGas(gasSstoreSet); err != nil {
					return nil, evm.gasUsed, err
				}
			} else {
				if err := evm.useGas(gasSstoreReset); err != nil {
					return nil, evm.gasUsed, err
				}
			}
			evm.state.SetStorage(evm.ctx.Address, key, val)

		case EVMJUMP:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			dest := evm.stack[len(evm.stack)-1].Uint64()
			evm.stack = evm.stack[:len(evm.stack)-1]
			if !evm.jumpDests[dest] {
				return nil, evm.gasUsed, fmt.Errorf("invalid jump destination")
			}
			evm.pc = dest

		case EVMJUMPI:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			dest, cond := evm.stack[len(evm.stack)-2].Uint64(), evm.stack[len(evm.stack)-1]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if cond.Sign() != 0 {
				if !evm.jumpDests[dest] {
					return nil, evm.gasUsed, fmt.Errorf("invalid jump destination")
				}
				evm.pc = dest
			}

		case EVMPC:
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, big.NewInt(int64(evm.pc-1)))

		case EVMMSIZE:
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.memory))))

		case EVMGAS:
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			remaining := evm.ctx.GasLimit - evm.gasUsed
			evm.stack = append(evm.stack, new(big.Int).SetUint64(remaining))

		case EVMJUMPDEST:
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}

		case EVMINVALID:
			evm.gasUsed = evm.ctx.GasLimit
			return nil, evm.gasUsed, fmt.Errorf("invalid opcode")

		case EVMRETURN:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-1].Uint64(), evm.stack[len(evm.stack)-2].Uint64()
			evm.stack = evm.stack[:len(evm.stack)-2]
			if err := evm.useGas(gasSlow); err != nil {
				return nil, evm.gasUsed, err
			}
			return evm.getMemory(offset, size), evm.gasUsed, nil

		case EVMREVERT:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-1].Uint64(), evm.stack[len(evm.stack)-2].Uint64()
			evm.stack = evm.stack[:len(evm.stack)-2]
			if err := evm.useGas(gasSlow); err != nil {
				return nil, evm.gasUsed, err
			}
			return evm.getMemory(offset, size), evm.gasUsed, fmt.Errorf("revert")

		case EVMCREATE:
			if evm.staticCall {
				return nil, evm.gasUsed, fmt.Errorf("create in staticcall context")
			}
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			value, offset, size := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			if value.Sign() > 0 && evm.state.GetBalance(evm.ctx.Address).Cmp(value) < 0 {
				evm.stack = append(evm.stack, big.NewInt(0))
				break
			}
			if err := evm.useGas(gasCreate); err != nil {
				return nil, evm.gasUsed, err
			}
			off := offset.Uint64()
			sz := size.Uint64()
			if err := evm.expandMemory(off, sz); err != nil {
				return nil, evm.gasUsed, err
			}
			initCode := evm.getMemory(off, sz)
			snap := evm.state.Snapshot()
			nonce := evm.state.GetNonce(evm.ctx.Address)
			contractAddr := evm.createAddress(evm.ctx.Address, nonce)
			evm.state.CreateAccount(contractAddr)
			evm.state.Transfer(evm.ctx.Address, contractAddr, value)

			subCtx := &EVMContext{
				Caller:   evm.ctx.Address,
				Address:  contractAddr,
				Value:    new(big.Int).Set(value),
				GasLimit: evm.ctx.GasLimit - evm.gasUsed,
				GasPrice: safeSetBig(evm.ctx.GasPrice),
				Data:     nil,
				BlockNum: evm.ctx.BlockNum,
			}
			sub := NewEVMExecutor(subCtx, evm.state)
			retdata, subGas, err := sub.Execute(initCode)
			evm.gasUsed += subGas
			if err != nil {
				evm.state.RevertToSnapshot(snap)
				evm.stack = append(evm.stack, big.NewInt(0))
			} else {
				if uint64(len(retdata)) > gMaxCodeSize {
					evm.state.RevertToSnapshot(snap)
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					deployGas := gasCodeDeposit * ((uint64(len(retdata)) + 31) / 32)
					if err := evm.useGas(deployGas); err != nil {
						evm.state.RevertToSnapshot(snap)
						return nil, evm.gasUsed, err
					}
					evm.stack = append(evm.stack, new(big.Int).SetBytes(contractAddr))
				}
			}

		case EVMCREATE2:
			if evm.staticCall {
				return nil, evm.gasUsed, fmt.Errorf("create2 in staticcall context")
			}
			if len(evm.stack) < 4 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			value, offset, size, salt := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4]
			evm.stack = evm.stack[:len(evm.stack)-4]
			if value.Sign() > 0 && evm.state.GetBalance(evm.ctx.Address).Cmp(value) < 0 {
				evm.stack = append(evm.stack, big.NewInt(0))
				break
			}
			if err := evm.useGas(gasCreate); err != nil {
				return nil, evm.gasUsed, err
			}
			off := offset.Uint64()
			sz := size.Uint64()
			if err := evm.expandMemory(off, sz); err != nil {
				return nil, evm.gasUsed, err
			}
			initCode := evm.getMemory(off, sz)
			saltBytes := make([]byte, 32)
			salt.FillBytes(saltBytes)
			contractAddr := evm.createAddress2(evm.ctx.Address, saltBytes, initCode)
			snap := evm.state.Snapshot()
			evm.state.CreateAccount(contractAddr)
			evm.state.Transfer(evm.ctx.Address, contractAddr, value)

			subCtx := &EVMContext{
				Caller:   evm.ctx.Address,
				Address:  contractAddr,
				Value:    new(big.Int).Set(value),
				GasLimit: evm.ctx.GasLimit - evm.gasUsed,
				GasPrice: safeSetBig(evm.ctx.GasPrice),
				Data:     nil,
				BlockNum: evm.ctx.BlockNum,
			}
			sub := NewEVMExecutor(subCtx, evm.state)
			retdata, subGas, err := sub.Execute(initCode)
			evm.gasUsed += subGas
			if err != nil {
				evm.state.RevertToSnapshot(snap)
				evm.stack = append(evm.stack, big.NewInt(0))
			} else {
				if uint64(len(retdata)) > gMaxCodeSize {
					evm.state.RevertToSnapshot(snap)
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					deployGas := gasCodeDeposit * ((uint64(len(retdata)) + 31) / 32)
					if err := evm.useGas(deployGas); err != nil {
						evm.state.RevertToSnapshot(snap)
						return nil, evm.gasUsed, err
					}
					evm.stack = append(evm.stack, new(big.Int).SetBytes(contractAddr))
				}
			}

		case EVMCALL, EVMCALLCODE, EVMDELEGATECALL, EVMSTATICCALL:
			var gasLimit, calleeAddr, value *big.Int
			var argOffset, argSize, retOffset, retSize *big.Int
			isDelegate := op == EVMDELEGATECALL || op == EVMSTATICCALL
			if isDelegate {
				if len(evm.stack) < 6 {
					return nil, evm.gasUsed, fmt.Errorf("stack underflow")
				}
				gasLimit, calleeAddr, argOffset, argSize, retOffset, retSize =
					evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2],
					evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4],
					evm.stack[len(evm.stack)-5], evm.stack[len(evm.stack)-6]
				evm.stack = evm.stack[:len(evm.stack)-6]
				value = big.NewInt(0)
				if op == EVMSTATICCALL && evm.staticCall {
					return nil, evm.gasUsed, fmt.Errorf("staticcall cannot modify state")
				}
				if op == EVMSTATICCALL {
					value = big.NewInt(0)
				} else {
					value = evm.ctx.Value
				}
			} else {
				if len(evm.stack) < 7 {
					return nil, evm.gasUsed, fmt.Errorf("stack underflow")
				}
				gasLimit, calleeAddr, value, argOffset, argSize, retOffset, retSize =
					evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2],
					evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4],
					evm.stack[len(evm.stack)-5], evm.stack[len(evm.stack)-6],
					evm.stack[len(evm.stack)-7]
				evm.stack = evm.stack[:len(evm.stack)-7]
			}

			target := evm.toAddress(calleeAddr)

			callGas := gasCall
			valueTransferred := value.Sign() > 0 && (op == EVMCALL || op == EVMCALLCODE)
			if valueTransferred {
				if evm.state.GetBalance(evm.ctx.Address).Cmp(value) < 0 {
					evm.stack = append(evm.stack, big.NewInt(0))
					break
				}
				callGas += gasCallValue
			}
			if err := evm.useGas(callGas); err != nil {
				return nil, evm.gasUsed, err
			}

			aOff := argOffset.Uint64()
			aSz := argSize.Uint64()
			if err := evm.expandMemory(aOff, aSz); err != nil {
				return nil, evm.gasUsed, err
			}
			rOff := retOffset.Uint64()
			rSz := retSize.Uint64()
			if err := evm.expandMemory(rOff, rSz); err != nil {
				return nil, evm.gasUsed, err
			}

			callData := evm.getMemory(aOff, aSz)

			gasAvailable := gasLimit.Uint64()
			if gasAvailable > evm.ctx.GasLimit-evm.gasUsed {
				gasAvailable = evm.ctx.GasLimit - evm.gasUsed
			}
			gasAvailable = gasAvailable - gasAvailable/64

			var snap int
			if op != EVMSTATICCALL {
				snap = evm.state.Snapshot()
			}
			if valueTransferred {
				evm.state.Transfer(evm.ctx.Address, target, value)
			}

			var subCtx *EVMContext
			switch op {
			case EVMCALL:
				subCtx = &EVMContext{
					Caller:   evm.ctx.Address,
					Address:  target,
					Value:    new(big.Int).Set(value),
					GasLimit: gasAvailable,
					GasPrice: safeSetBig(evm.ctx.GasPrice),
					Data:     callData,
					BlockNum: evm.ctx.BlockNum,
				}
			case EVMCALLCODE:
				subCtx = &EVMContext{
					Caller:   evm.ctx.Address,
					Address:  evm.ctx.Address,
					Value:    new(big.Int).Set(value),
					GasLimit: gasAvailable,
					GasPrice: safeSetBig(evm.ctx.GasPrice),
					Data:     callData,
					BlockNum: evm.ctx.BlockNum,
				}
			case EVMDELEGATECALL:
				subCtx = &EVMContext{
					Caller:   evm.ctx.Caller,
					Address:  evm.ctx.Address,
					Value:    safeSetBig(evm.ctx.Value),
					GasLimit: gasAvailable,
					GasPrice: safeSetBig(evm.ctx.GasPrice),
					Data:     callData,
					BlockNum: evm.ctx.BlockNum,
				}
			case EVMSTATICCALL:
				subCtx = &EVMContext{
					Caller:   evm.ctx.Address,
					Address:  target,
					Value:    big.NewInt(0),
					GasLimit: gasAvailable,
					GasPrice: safeSetBig(evm.ctx.GasPrice),
					Data:     callData,
					BlockNum: evm.ctx.BlockNum,
				}
			}

			code := evm.state.GetCode(target)
			if op == EVMCALLCODE || op == EVMDELEGATECALL {
				code = evm.state.GetCode(target)
			}
			if len(code) == 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
				evm.returndata = nil
			} else {
				sub := NewEVMExecutor(subCtx, evm.state)
				if op == EVMSTATICCALL {
					sub.staticCall = true
				}
				retdata, subGas, err := sub.Execute(code)
				evm.gasUsed += subGas
				evm.returndata = retdata
				if err != nil {
					if op != EVMSTATICCALL {
						evm.state.RevertToSnapshot(snap)
					}
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					evm.stack = append(evm.stack, big.NewInt(1))
					copyLen := rSz
					if uint64(len(retdata)) < copyLen {
						copyLen = uint64(len(retdata))
					}
					if rOff+copyLen > uint64(len(evm.memory)) {
						if err := evm.expandMemory(rOff, copyLen); err != nil {
							return nil, evm.gasUsed, err
						}
					}
					for i := uint64(0); i < copyLen; i++ {
						if rOff+i < uint64(len(evm.memory)) {
							evm.memory[rOff+i] = retdata[i]
						}
					}
				}
			}

		case EVMLOG0, EVMLOG1, EVMLOG2, EVMLOG3, EVMLOG4:
			if evm.staticCall {
				return nil, evm.gasUsed, fmt.Errorf("log in staticcall context")
			}
			numTopics := int(op) - int(EVMLOG0)
			needed := 2 + numTopics
			if len(evm.stack) < needed {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-2].Uint64(), evm.stack[len(evm.stack)-1].Uint64()
			stackIdx := len(evm.stack) - 2 - numTopics
			topics := make([][]byte, numTopics)
			for i := 0; i < numTopics; i++ {
				topic := make([]byte, 32)
				evm.stack[stackIdx+i].FillBytes(topic)
				topics[i] = topic
			}
			evm.stack = evm.stack[:stackIdx]
			logGas := gasLog + uint64(numTopics)*gasLogTopic + size*gasLogData
			if err := evm.useGas(logGas); err != nil {
				return nil, evm.gasUsed, err
			}
			if err := evm.expandMemory(offset, size); err != nil {
				return nil, evm.gasUsed, err
			}
			data := evm.getMemory(offset, size)
			if evm.state != nil {
				evm.state.AddLog(evm.ctx.Address, topics, data)
			}

		case EVMSELFDESTRUCT:
			if evm.staticCall {
				return nil, evm.gasUsed, fmt.Errorf("selfdestruct in staticcall context")
			}
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasSelfdestruct); err != nil {
				return nil, evm.gasUsed, err
			}
			recipient := evm.toAddress(evm.stack[len(evm.stack)-1])
			evm.stack = evm.stack[:len(evm.stack)-1]
			balance := evm.state.GetBalance(evm.ctx.Address)
			if balance.Sign() > 0 {
				evm.state.Transfer(evm.ctx.Address, recipient, balance)
			}
			evm.state.SetStorage(evm.ctx.Address, nil, nil)
			return nil, evm.gasUsed, nil

		case EVMCOINBASE:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			if evm.ctx.Coinbase == nil {
				evm.stack = append(evm.stack, big.NewInt(0))
			} else {
				evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.Coinbase))
			}

		case EVMTIMESTAMP:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, new(big.Int).SetUint64(evm.ctx.Timestamp))

		case EVMNUMBER:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, new(big.Int).SetUint64(evm.ctx.BlockNum))

		case EVMPREVRANDAO:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			if evm.ctx.PrevRandao == nil {
				evm.stack = append(evm.stack, big.NewInt(0))
			} else {
				evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.PrevRandao))
			}

		case EVMGASLIMIT:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, new(big.Int).SetUint64(evm.ctx.BlockGasLimit))

		case EVMCHAINID:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			if evm.ctx.ChainID != nil {
				evm.stack = append(evm.stack, new(big.Int).Set(evm.ctx.ChainID))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMSELFBALANCE:
			if err := evm.useGas(gasFast); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, evm.state.GetBalance(evm.ctx.Address))

		case EVMBASEFEE:
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			if evm.ctx.BaseFee != nil {
				evm.stack = append(evm.stack, new(big.Int).Set(evm.ctx.BaseFee))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMBLOCKHASH:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasMid); err != nil {
				return nil, evm.gasUsed, err
			}
			blockNum := evm.stack[len(evm.stack)-1].Uint64()
			evm.stack = evm.stack[:len(evm.stack)-1]
			if evm.ctx.GetBlockHash != nil {
				hash := evm.ctx.GetBlockHash(blockNum)
				if len(hash) == 0 {
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					evm.stack = append(evm.stack, new(big.Int).SetBytes(hash))
				}
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMPUSH0:
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.stack = append(evm.stack, big.NewInt(0))

		case EVMTLOAD:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasFastest); err != nil {
				return nil, evm.gasUsed, err
			}
			key := make([]byte, 32)
			evm.stack[len(evm.stack)-1].FillBytes(key)
			evm.stack[len(evm.stack)-1] = new(big.Int).SetBytes(evm.state.GetStorage(evm.ctx.Address, key))

		case EVMTSTORE:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			key := make([]byte, 32)
			val := make([]byte, 32)
			evm.stack[len(evm.stack)-1].FillBytes(key)
			evm.stack[len(evm.stack)-2].FillBytes(val)
			evm.stack = evm.stack[:len(evm.stack)-2]
			if err := evm.useGas(gasSstoreReset); err != nil {
				return nil, evm.gasUsed, err
			}
			evm.state.SetStorage(evm.ctx.Address, key, val)

		case EVMMCOPY:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			dstV, srcV, sizeV := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			if dstV.Sign() < 0 || srcV.Sign() < 0 || sizeV.Sign() < 0 {
				return nil, evm.gasUsed, fmt.Errorf("negative mcopy offset or size")
			}
			dst := dstV.Uint64()
			src := srcV.Uint64()
			sz := sizeV.Uint64()
			maxEnd := dst + sz
			if src+sz > maxEnd {
				maxEnd = src + sz
			}
			if err := evm.expandMemory(0, maxEnd); err != nil {
				return nil, evm.gasUsed, err
			}
			if err := evm.useGas(gasFastest + gasCopy*((sz+31)/32)); err != nil {
				return nil, evm.gasUsed, err
			}
			copyLen := sz
			if src+sz > uint64(len(evm.memory)) || dst+sz > uint64(len(evm.memory)) {
				copyLen = 0
			}
			if copyLen > 0 {
				copy(evm.memory[dst:dst+copyLen], evm.memory[src:src+copyLen])
			}

		case EXTCODESIZE:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasExt); err != nil {
				return nil, evm.gasUsed, err
			}
			addr := evm.toAddress(evm.stack[len(evm.stack)-1])
			evm.stack[len(evm.stack)-1] = big.NewInt(int64(len(evm.state.GetCode(addr))))

		case EXTCODECOPY:
			if len(evm.stack) < 4 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			addrV, memOffsetV, codeOffsetV, sizeV := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4]
			evm.stack = evm.stack[:len(evm.stack)-4]
			if addrV.Sign() < 0 || memOffsetV.Sign() < 0 || codeOffsetV.Sign() < 0 || sizeV.Sign() < 0 {
				return nil, evm.gasUsed, fmt.Errorf("negative extcodecopy offset or size")
			}
			addr := evm.toAddress(addrV)
			memOffset := memOffsetV.Uint64()
			codeOffset := codeOffsetV.Uint64()
			size := sizeV.Uint64()
			if err := evm.expandMemory(memOffset, size); err != nil {
				return nil, evm.gasUsed, err
			}
			extCode := evm.state.GetCode(addr)
			copyGas := gasExt + gasCopy*((size+31)/32)
			if err := evm.useGas(copyGas); err != nil {
				return nil, evm.gasUsed, err
			}
			for i := uint64(0); i < size; i++ {
				idx := codeOffset + i
				if idx < uint64(len(extCode)) && memOffset+i < uint64(len(evm.memory)) {
					evm.memory[memOffset+i] = extCode[idx]
				}
			}

		case EXTCODEHASH:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			if err := evm.useGas(gasExt); err != nil {
				return nil, evm.gasUsed, err
			}
			addr := evm.toAddress(evm.stack[len(evm.stack)-1])
			code := evm.state.GetCode(addr)
			if len(code) == 0 {
				evm.stack[len(evm.stack)-1] = big.NewInt(0)
			} else {
				hash := crypto.Keccak256(code)
				evm.stack[len(evm.stack)-1] = new(big.Int).SetBytes(hash)
			}

		default:
			if op >= EVMPUSH1 && op <= EVMPUSH32 {
				if err := evm.useGas(gasFastest); err != nil {
					return nil, evm.gasUsed, err
				}
				numBytes := int(op) - int(EVMPUSH1) + 1
				var val *big.Int
				if evm.pc+uint64(numBytes) <= uint64(len(code)) {
					val = new(big.Int).SetBytes(code[evm.pc : evm.pc+uint64(numBytes)])
					evm.pc += uint64(numBytes)
				} else {
					val = new(big.Int).SetBytes(code[evm.pc:])
					evm.pc = uint64(len(code))
				}
				evm.stack = append(evm.stack, val)
			} else if op >= EVMDUP1 && op < EVMDUP1+16 {
				if err := evm.useGas(gasFastest); err != nil {
					return nil, evm.gasUsed, err
				}
				n := int(op) - int(EVMDUP1) + 1
				if len(evm.stack) < n {
					return nil, evm.gasUsed, fmt.Errorf("stack underflow")
				}
				evm.stack = append(evm.stack, new(big.Int).Set(evm.stack[len(evm.stack)-n]))
			} else if op >= EVMSWAP1 && op < EVMSWAP1+16 {
				if err := evm.useGas(gasFastest); err != nil {
					return nil, evm.gasUsed, err
				}
				n := int(op) - int(EVMSWAP1) + 1
				if len(evm.stack) < n+1 {
					return nil, evm.gasUsed, fmt.Errorf("stack underflow")
				}
				idx := len(evm.stack) - 1 - n
				evm.stack[len(evm.stack)-1], evm.stack[idx] = evm.stack[idx], evm.stack[len(evm.stack)-1]
			} else {
				evm.gasUsed = evm.ctx.GasLimit
				return nil, evm.gasUsed, fmt.Errorf("invalid opcode 0x%x", byte(op))
			}
		}
	}

	return nil, evm.gasUsed, nil
}

func (evm *EVMExecutor) computeJumpDests(code []byte) map[uint64]bool {
	dests := make(map[uint64]bool)
	for i := 0; i < len(code); i++ {
		if EVMOpCode(code[i]) == EVMJUMPDEST {
			dests[uint64(i)] = true
		} else if EVMOpCode(code[i]) >= EVMPUSH1 && EVMOpCode(code[i]) <= EVMPUSH32 {
			i += int(EVMOpCode(code[i])) - int(EVMPUSH1) + 1
		} else if EVMOpCode(code[i]) == EVMPUSH0 {
		}
	}
	return dests
}
