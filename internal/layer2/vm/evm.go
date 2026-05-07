package vm

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

var two256 = new(big.Int).Lsh(big.NewInt(1), 256)

func wrap256(x *big.Int) *big.Int {
	return x.Mod(x, two256)
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

func (op EVMOpCode) String() string {
	names := map[EVMOpCode]string{
		EVMSTOP: "STOP", EVMADD: "ADD", EVMMUL: "MUL", EVMSUB: "SUB",
		EVMDIV: "DIV", EVMSDIV: "SDIV", EVMMOD: "MOD", EVMSMOD: "SMOD",
		EVMSIGNEXTEND: "SIGNEXTEND",
		EVMLT: "LT", EVMGT: "GT", EVMSLT: "SLT", EVMSGT: "SGT",
		EVMEQ: "EQ", EVMISZERO: "ISZERO",
		EVMAND: "AND", EVMOR: "OR", EVMXOR: "XOR", EVMNOT: "NOT",
		EVMBYTE: "BYTE", EVMSHL: "SHL", EVMSHR: "SHR", EVMSAR: "SAR",
		EVMSHA3: "SHA3",
		EVMADDRESS: "ADDRESS", EVMBALANCE: "BALANCE", EVMORIGIN: "ORIGIN",
		EVMCALLER: "CALLER", EVMCALLVALUE: "CALLVALUE",
		EVMCALLDATALOAD: "CALLDATALOAD", EVMCALLDATASIZE: "CALLDATASIZE", EVMCALLDATACOPY: "CALLDATACOPY",
		EVMCODESIZE: "CODESIZE", EVMCODECOPY: "CODECOPY",
		EVMPOP: "POP", EVMMLOAD: "MLOAD", EVMMSTORE: "MSTORE", EVMMSTORE8: "MSTORE8",
		EVMSLOAD: "SLOAD", EVMSSTORE: "SSTORE",
		EVMJUMP: "JUMP", EVMJUMPI: "JUMPI", EVMPC: "PC", EVMMSIZE: "MSIZE", EVMGAS: "GAS", EVMJUMPDEST: "JUMPDEST",
		EVMLOG0: "LOG0", EVMLOG1: "LOG1", EVMLOG2: "LOG2", EVMLOG3: "LOG3", EVMLOG4: "LOG4",
		EVMCREATE: "CREATE", EVMCALL: "CALL", EVMCALLCODE: "CALLCODE", EVMRETURN: "RETURN",
		EVMSELFDESTRUCT: "SELFDESTRUCT",
		EVMCREATE2: "CREATE2", EVMREVERT: "REVERT", EVMINVALID: "INVALID",
		EVMRETURNDATASIZE: "RETURNDATASIZE", EVMRETURNDATACOPY: "RETURNDATACOPY",
		EVMSTATICCALL: "STATICCALL", EVMDELEGATECALL: "DELEGATECALL",
	}
	if name, ok := names[op]; ok {
		return name
	}
	return fmt.Sprintf("OP_0x%x", byte(op))
}

const (
	EVMSTOP         EVMOpCode = 0x00
	EVMADD          EVMOpCode = 0x01
	EVMMUL          EVMOpCode = 0x02
	EVMSUB          EVMOpCode = 0x03
	EVMDIV          EVMOpCode = 0x04
	EVMSDIV         EVMOpCode = 0x05
	EVMMOD          EVMOpCode = 0x06
	EVMSMOD         EVMOpCode = 0x07
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
)

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
}

type EVMState interface {
	GetBalance(addr []byte) *big.Int
	GetCode(addr []byte) []byte
	GetStorage(addr []byte, key []byte) []byte
	SetStorage(addr []byte, key []byte, value []byte)
	Transfer(from, to []byte, amount *big.Int)
	CreateAccount(addr []byte)
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

	returndata []byte
	code       []byte

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

func (evm *EVMExecutor) Execute(code []byte) ([]byte, uint64, error) {
	evm.pc = 0
	evm.gasUsed = 0
	evm.stack = evm.stack[:0]
	evm.code = code
	jumpDests := evm.computeJumpDests(code)

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
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, wrap256(new(big.Int).Add(a, b)))

		case EVMSUB:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, wrap256(new(big.Int).Sub(a, b)))

		case EVMMUL:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.stack = append(evm.stack, wrap256(new(big.Int).Mul(a, b)))

		case EVMDIV:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
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
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Sign() == 0 {
				evm.stack = append(evm.stack, new(big.Int))
			} else {
				sa := toSigned256(a)
				sb := toSigned256(b)
				evm.stack = append(evm.stack, toUnsigned256(new(big.Int).Mod(sa, sb)))
			}

		case EVMSIGNEXTEND:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
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
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Cmp(a) < 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMSGT:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if b.Cmp(a) > 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
			} else {
				evm.stack = append(evm.stack, big.NewInt(0))
			}

		case EVMEQ:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
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
			a := evm.stack[len(evm.stack)-1]
			evm.stack[len(evm.stack)-1] = big.NewInt(0)
			if a.Sign() == 0 {
				evm.stack[len(evm.stack)-1] = big.NewInt(1)
			}

		case EVMAND:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			result := new(big.Int).And(a, b)
			evm.stack = append(evm.stack, result)

		case EVMOR:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			result := new(big.Int).Or(a, b)
			evm.stack = append(evm.stack, result)

		case EVMXOR:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			b, a := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			result := new(big.Int).Xor(a, b)
			evm.stack = append(evm.stack, result)

		case EVMNOT:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			a := evm.stack[len(evm.stack)-1]
			mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
			evm.stack[len(evm.stack)-1] = new(big.Int).Xor(a, mask)

		case EVMBYTE:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
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
			shift, val := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			s := shift.Uint64()
			if s > 255 {
				evm.stack = append(evm.stack, big.NewInt(0))
			} else {
				r := new(big.Int).Rsh(val, uint(s))
				evm.stack = append(evm.stack, r)
			}

		case EVMSAR:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			shift, val := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2]
			evm.stack = evm.stack[:len(evm.stack)-2]
			s := shift.Uint64()
			if s > 255 {
				b := make([]byte, 32)
				val.FillBytes(b)
				if b[0]&0x80 != 0 {
					evm.stack = append(evm.stack, new(big.Int).Sub(two256, big.NewInt(1)))
				} else {
					evm.stack = append(evm.stack, big.NewInt(0))
				}
			} else {
				b := make([]byte, 32)
				val.FillBytes(b)
				sign := b[0] >> 7
				for i := uint64(0); i < s; i++ {
					carry := sign
					for j := 0; j < 32; j++ {
						lsb := b[j] & 1
						b[j] = (carry << 7) | (b[j] >> 1)
						carry = lsb
					}
				}
				evm.stack = append(evm.stack, new(big.Int).SetBytes(b))
			}

		case EVMSHA3:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-1].Int64(), evm.stack[len(evm.stack)-2].Int64()
			evm.stack = evm.stack[:len(evm.stack)-2]
			data := evm.getMemory(offset, size)
			hash := crypto.Keccak256(data)
			evm.stack = append(evm.stack, new(big.Int).SetBytes(hash))

		case EVMADDRESS:
			evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.Address))

		case EVMBALANCE:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			addr := evm.toAddress(evm.stack[len(evm.stack)-1])
			evm.stack[len(evm.stack)-1] = evm.state.GetBalance(addr)

		case EVMORIGIN:
			evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.Caller))

		case EVMCALLER:
			evm.stack = append(evm.stack, new(big.Int).SetBytes(evm.ctx.Caller))

		case EVMCALLVALUE:
			evm.stack = append(evm.stack, new(big.Int).Set(evm.ctx.Value))

		case EVMCALLDATALOAD:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset := evm.stack[len(evm.stack)-1].Int64()
			data := make([]byte, 32)
			for i := int64(0); i < 32; i++ {
				idx := offset + i
				if idx >= 0 && idx < int64(len(evm.ctx.Data)) {
					data[i] = evm.ctx.Data[idx]
				}
			}
			evm.stack[len(evm.stack)-1] = new(big.Int).SetBytes(data)

		case EVMCALLDATASIZE:
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.ctx.Data))))

		case EVMCALLDATACOPY:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			destOffset, srcOffset, size := evm.stack[len(evm.stack)-1].Int64(), evm.stack[len(evm.stack)-2].Int64(), evm.stack[len(evm.stack)-3].Int64()
			evm.stack = evm.stack[:len(evm.stack)-3]
			data := make([]byte, size)
			for i := int64(0); i < size; i++ {
				idx := srcOffset + i
				if idx >= 0 && idx < int64(len(evm.ctx.Data)) {
					data[i] = evm.ctx.Data[idx]
				}
			}
			evm.setMemory(destOffset, data, size)

		case EVMCODESIZE:
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.code))))

		case EVMCODECOPY:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			memOffset, codeOffset, size := evm.stack[len(evm.stack)-1].Int64(), evm.stack[len(evm.stack)-2].Int64(), evm.stack[len(evm.stack)-3].Int64()
			evm.stack = evm.stack[:len(evm.stack)-3]
			data := make([]byte, size)
			for i := int64(0); i < size; i++ {
				idx := codeOffset + i
				if idx >= 0 && idx < int64(len(evm.code)) {
					data[i] = evm.code[idx]
				}
			}
			evm.setMemory(memOffset, data, size)

		case EVMRETURNDATASIZE:
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.returndata))))

		case EVMRETURNDATACOPY:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			destOffset, srcOffset, size := evm.stack[len(evm.stack)-1].Int64(), evm.stack[len(evm.stack)-2].Int64(), evm.stack[len(evm.stack)-3].Int64()
			evm.stack = evm.stack[:len(evm.stack)-3]
			if srcOffset+size > int64(len(evm.returndata)) {
				return nil, evm.gasUsed, fmt.Errorf("return data copy out of bounds")
			}
			data := make([]byte, size)
			for i := int64(0); i < size; i++ {
				idx := srcOffset + i
				if idx >= 0 && idx < int64(len(evm.returndata)) {
					data[i] = evm.returndata[idx]
				}
			}
			evm.setMemory(destOffset, data, size)

		case EVMPOP:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			evm.stack = evm.stack[:len(evm.stack)-1]

		case EVMMLOAD:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset := evm.stack[len(evm.stack)-1].Int64()
			evm.stack[len(evm.stack)-1] = new(big.Int).SetBytes(evm.getMemory(offset, 32))

		case EVMMSTORE:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset := evm.stack[len(evm.stack)-1].Int64()
			val := make([]byte, 32)
			evm.stack[len(evm.stack)-2].FillBytes(val)
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.setMemory(offset, val, 32)

		case EVMMSTORE8:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset := evm.stack[len(evm.stack)-1].Int64()
			val := evm.stack[len(evm.stack)-2].Bytes()
			evm.stack = evm.stack[:len(evm.stack)-2]
			byteVal := byte(0)
			if len(val) > 0 {
				byteVal = val[len(val)-1]
			}
			evm.setMemory(offset, []byte{byteVal}, 1)

		case EVMSLOAD:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			key := make([]byte, 32)
			evm.stack[len(evm.stack)-1].FillBytes(key)
			evm.stack[len(evm.stack)-1] = new(big.Int).SetBytes(evm.state.GetStorage(evm.ctx.Address, key))

		case EVMSSTORE:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			key := make([]byte, 32)
			val := make([]byte, 32)
			evm.stack[len(evm.stack)-1].FillBytes(key)
			evm.stack[len(evm.stack)-2].FillBytes(val)
			evm.stack = evm.stack[:len(evm.stack)-2]
			evm.state.SetStorage(evm.ctx.Address, key, val)

		case EVMJUMP:
			if len(evm.stack) < 1 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			dest := evm.stack[len(evm.stack)-1].Uint64()
			evm.stack = evm.stack[:len(evm.stack)-1]
			if !jumpDests[dest] {
				return nil, evm.gasUsed, fmt.Errorf("invalid jump destination")
			}
			evm.pc = dest

		case EVMJUMPI:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			dest, cond := evm.stack[len(evm.stack)-2].Uint64(), evm.stack[len(evm.stack)-1]
			evm.stack = evm.stack[:len(evm.stack)-2]
			if cond.Sign() != 0 {
				if !jumpDests[dest] {
					return nil, evm.gasUsed, fmt.Errorf("invalid jump destination")
				}
				evm.pc = dest
			}

		case EVMPC:
			evm.stack = append(evm.stack, big.NewInt(int64(evm.pc-1)))

		case EVMMSIZE:
			evm.stack = append(evm.stack, big.NewInt(int64(len(evm.memory))))

		case EVMGAS:
			remaining := evm.ctx.GasLimit - evm.gasUsed
			evm.stack = append(evm.stack, big.NewInt(int64(remaining)))

		case EVMINVALID:
			return nil, evm.gasUsed, fmt.Errorf("invalid opcode")

		case EVMJUMPDEST:

		case EVMRETURN:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-1].Int64(), evm.stack[len(evm.stack)-2].Int64()
			return evm.getMemory(offset, size), evm.gasUsed, nil

		case EVMREVERT:
			if len(evm.stack) < 2 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-1].Int64(), evm.stack[len(evm.stack)-2].Int64()
			return evm.getMemory(offset, size), evm.gasUsed, fmt.Errorf("revert")

		case EVMCREATE:
			if len(evm.stack) < 3 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			value, offset, size := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3]
			evm.stack = evm.stack[:len(evm.stack)-3]
			initCode := evm.getMemory(offset.Int64(), size.Int64())
			contractAddr := evm.createAddress(evm.ctx.Address, evm.ctx.BlockNum)
			evm.state.CreateAccount(contractAddr)
			evm.state.Transfer(evm.ctx.Address, contractAddr, value)

			subCtx := &EVMContext{
				Caller:   evm.ctx.Address,
				Address:  contractAddr,
				Value:    new(big.Int).Set(value),
				GasLimit: evm.ctx.GasLimit - evm.gasUsed,
				GasPrice: new(big.Int).Set(evm.ctx.GasPrice),
				Data:     nil,
				BlockNum: evm.ctx.BlockNum,
			}
			sub := NewEVMExecutor(subCtx, evm.state)
			retdata, _, err := sub.Execute(initCode)
			if err == nil {
				contract := evm.state.GetCode(contractAddr)
				if len(retdata) > 0 {
					_ = contract
				}
			}
			evm.stack = append(evm.stack, new(big.Int).SetBytes(contractAddr))

		case EVMCALL:
			if len(evm.stack) < 7 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			gasLimit, calleeAddr, value, argOffset, argSize, retOffset, retSize := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4], evm.stack[len(evm.stack)-5], evm.stack[len(evm.stack)-6], evm.stack[len(evm.stack)-7]
			evm.stack = evm.stack[:len(evm.stack)-7]

			target := evm.toAddress(calleeAddr)
			callData := evm.getMemory(argOffset.Int64(), argSize.Int64())

			subCtx := &EVMContext{
				Caller:   evm.ctx.Address,
				Address:  target,
				Value:    new(big.Int).Set(value),
				GasLimit: gasLimit.Uint64(),
				GasPrice: new(big.Int).Set(evm.ctx.GasPrice),
				Data:     callData,
				BlockNum: evm.ctx.BlockNum,
			}

			evm.state.Transfer(evm.ctx.Address, target, value)
			code := evm.state.GetCode(target)
			if len(code) == 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
				evm.setMemory(retOffset.Int64(), make([]byte, retSize.Int64()), retSize.Int64())
			} else {
				sub := NewEVMExecutor(subCtx, evm.state)
				retdata, _, err := sub.Execute(code)
				evm.returndata = retdata
				if err != nil {
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					evm.stack = append(evm.stack, big.NewInt(1))
					evm.setMemory(retOffset.Int64(), retdata, retSize.Int64())
				}
			}

		case EVMCALLCODE:
			if len(evm.stack) < 7 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			gasLimit, calleeAddr, value, argOffset, argSize, retOffset, retSize := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4], evm.stack[len(evm.stack)-5], evm.stack[len(evm.stack)-6], evm.stack[len(evm.stack)-7]
			evm.stack = evm.stack[:len(evm.stack)-7]

			target := evm.toAddress(calleeAddr)
			callData := evm.getMemory(argOffset.Int64(), argSize.Int64())
			code := evm.state.GetCode(target)

			subCtx := &EVMContext{
				Caller:   evm.ctx.Address,
				Address:  evm.ctx.Address,
				Value:    new(big.Int).Set(value),
				GasLimit: gasLimit.Uint64(),
				GasPrice: new(big.Int).Set(evm.ctx.GasPrice),
				Data:     callData,
				BlockNum: evm.ctx.BlockNum,
			}

			if len(code) == 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
				evm.setMemory(retOffset.Int64(), make([]byte, retSize.Int64()), retSize.Int64())
			} else {
				sub := NewEVMExecutor(subCtx, evm.state)
				retdata, _, err := sub.Execute(code)
				evm.returndata = retdata
				if err != nil {
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					evm.stack = append(evm.stack, big.NewInt(1))
					evm.setMemory(retOffset.Int64(), retdata, retSize.Int64())
				}
			}

		case EVMDELEGATECALL:
			if len(evm.stack) < 6 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			gasLimit, calleeAddr, argOffset, argSize, retOffset, retSize := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4], evm.stack[len(evm.stack)-5], evm.stack[len(evm.stack)-6]
			evm.stack = evm.stack[:len(evm.stack)-6]

			target := evm.toAddress(calleeAddr)
			callData := evm.getMemory(argOffset.Int64(), argSize.Int64())
			code := evm.state.GetCode(target)

			subCtx := &EVMContext{
				Caller:   evm.ctx.Caller,
				Address:  evm.ctx.Address,
				Value:    new(big.Int).Set(evm.ctx.Value),
				GasLimit: gasLimit.Uint64(),
				GasPrice: new(big.Int).Set(evm.ctx.GasPrice),
				Data:     callData,
				BlockNum: evm.ctx.BlockNum,
			}

			if len(code) == 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
				evm.setMemory(retOffset.Int64(), make([]byte, retSize.Int64()), retSize.Int64())
			} else {
				sub := NewEVMExecutor(subCtx, evm.state)
				retdata, _, err := sub.Execute(code)
				evm.returndata = retdata
				if err != nil {
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					evm.stack = append(evm.stack, big.NewInt(1))
					evm.setMemory(retOffset.Int64(), retdata, retSize.Int64())
				}
			}

		case EVMCREATE2:
			if len(evm.stack) < 4 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			value, offset, size, salt := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4]
			evm.stack = evm.stack[:len(evm.stack)-4]
			initCode := evm.getMemory(offset.Int64(), size.Int64())
			saltBytes := make([]byte, 32)
			salt.FillBytes(saltBytes)
			contractAddr := evm.createAddress2(evm.ctx.Address, saltBytes, initCode)
			evm.state.CreateAccount(contractAddr)
			evm.state.Transfer(evm.ctx.Address, contractAddr, value)
			evm.stack = append(evm.stack, new(big.Int).SetBytes(contractAddr))

		case EVMSTATICCALL:
			if len(evm.stack) < 6 {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			gasLimit, calleeAddr, argOffset, argSize, retOffset, retSize := evm.stack[len(evm.stack)-1], evm.stack[len(evm.stack)-2], evm.stack[len(evm.stack)-3], evm.stack[len(evm.stack)-4], evm.stack[len(evm.stack)-5], evm.stack[len(evm.stack)-6]
			evm.stack = evm.stack[:len(evm.stack)-6]

			target := evm.toAddress(calleeAddr)
			callData := evm.getMemory(argOffset.Int64(), argSize.Int64())

			subCtx := &EVMContext{
				Caller:   evm.ctx.Address,
				Address:  target,
				Value:    big.NewInt(0),
				GasLimit: gasLimit.Uint64(),
				GasPrice: new(big.Int).Set(evm.ctx.GasPrice),
				Data:     callData,
				BlockNum: evm.ctx.BlockNum,
			}

			code := evm.state.GetCode(target)
			if len(code) == 0 {
				evm.stack = append(evm.stack, big.NewInt(1))
				evm.setMemory(retOffset.Int64(), make([]byte, retSize.Int64()), retSize.Int64())
			} else {
				sub := NewEVMExecutor(subCtx, evm.state)
				retdata, _, err := sub.Execute(code)
				evm.returndata = retdata
				if err != nil {
					evm.stack = append(evm.stack, big.NewInt(0))
				} else {
					evm.stack = append(evm.stack, big.NewInt(1))
					evm.setMemory(retOffset.Int64(), retdata, retSize.Int64())
				}
			}

		case EVMLOG0, EVMLOG1, EVMLOG2, EVMLOG3, EVMLOG4:
			numTopics := int(op) - int(EVMLOG0)
			needed := 2 + numTopics
			if len(evm.stack) < needed {
				return nil, evm.gasUsed, fmt.Errorf("stack underflow")
			}
			offset, size := evm.stack[len(evm.stack)-2].Int64(), evm.stack[len(evm.stack)-1].Int64()
			stackIdx := len(evm.stack) - 2 - numTopics
			topics := make([][]byte, numTopics)
			for i := 0; i < numTopics; i++ {
				topics[i] = evm.stack[stackIdx+i].Bytes()
			}
			evm.stack = evm.stack[:stackIdx]
			_ = topics
			_ = offset
			_ = size

		default:
			if op >= EVMPUSH1 && op <= EVMPUSH32 {
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
				n := int(op) - int(EVMDUP1) + 1
				if len(evm.stack) < n {
					return nil, evm.gasUsed, fmt.Errorf("stack underflow")
				}
				evm.stack = append(evm.stack, new(big.Int).Set(evm.stack[len(evm.stack)-n]))
			} else if op >= EVMSWAP1 && op < EVMSWAP1+16 {
				n := int(op) - int(EVMSWAP1) + 1
				if len(evm.stack) < n+1 {
					return nil, evm.gasUsed, fmt.Errorf("stack underflow")
				}
				idx := len(evm.stack) - 1 - n
				evm.stack[len(evm.stack)-1], evm.stack[idx] = evm.stack[idx], evm.stack[len(evm.stack)-1]
			} else {
				evm.gasUsed++
			}
		}

		evm.gasUsed++
		if evm.gasUsed > evm.ctx.GasLimit {
			return nil, evm.gasUsed, fmt.Errorf("out of gas")
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
		}
	}
	return dests
}

func (evm *EVMExecutor) getMemory(offset, size int64) []byte {
	result := make([]byte, size)
	for i := int64(0); i < size; i++ {
		idx := offset + i
		if idx >= 0 && idx < int64(len(evm.memory)) {
			result[i] = evm.memory[idx]
		}
	}
	return result
}

func (evm *EVMExecutor) setMemory(offset int64, data []byte, size int64) {
	newLen := offset + size
	if newLen > int64(len(evm.memory)) {
		evm.memory = append(evm.memory, make([]byte, newLen-int64(len(evm.memory)))...)
	}
	for i := int64(0); i < size && i < int64(len(data)); i++ {
		if offset+i >= 0 {
			evm.memory[offset+i] = data[i]
		}
	}
}

func (evm *EVMExecutor) toAddress(val *big.Int) []byte {
	bytes := val.Bytes()
	addr := make([]byte, 20)
	if len(bytes) > 20 {
		copy(addr, bytes[len(bytes)-20:])
	} else {
		copy(addr[20-len(bytes):], bytes)
	}
	return addr
}

func (evm *EVMExecutor) createAddress(caller []byte, nonce uint64) []byte {
	h := sha256.New()
	h.Write(caller)
	binary.Write(h, binary.BigEndian, nonce)
	hash := h.Sum(nil)
	return hash[12:]
}

func (evm *EVMExecutor) createAddress2(caller []byte, salt []byte, initCode []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0xFF})
	h.Write(caller)
	h.Write(salt)
	codeHash := sha256.Sum256(initCode)
	h.Write(codeHash[:])
	hash := h.Sum(nil)
	return hash[12:]
}
