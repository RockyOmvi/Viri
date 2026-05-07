package vm

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	WasmMagic = 0x6D736100
	WasmVersion = 1

	opCodeGetLocal  byte = 0x20
	opCodeSetLocal  byte = 0x21
	opCodeI32Const  byte = 0x41
	opCodeI64Const  byte = 0x42
	opCodeI32Add    byte = 0x6A
	opCodeI32Sub    byte = 0x6B
	opCodeI32Mul    byte = 0x6C
	opCodeI64Add    byte = 0x7C
	opCodeI64Sub    byte = 0x7D
	opCodeI64Mul    byte = 0x7E
	opCodeI32Eq     byte = 0x48
	opCodeI32Ne     byte = 0x49
	opCodeI32LtS    byte = 0x4A
	opCodeI32GtS    byte = 0x4C
	opCodeI32LeS    byte = 0x4E
	opCodeI32GeS    byte = 0x50
	opCodeBrIf      byte = 0x0D
	opCodeBr        byte = 0x0C
	opCodeEnd       byte = 0x0B
	opCodeIf        byte = 0x04
	opCodeElse      byte = 0x05
	opCodeLoop      byte = 0x03
	opCodeBlock     byte = 0x02
	opCodeCall      byte = 0x10
	opCodeReturn    byte = 0x0F
	opCodeDrop      byte = 0x1A
	opCodeSelect    byte = 0x1B
	opCodeI32Store  byte = 0x36
	opCodeI32Load   byte = 0x28
	opCodeI64Store  byte = 0x37
	opCodeI64Load   byte = 0x29
)

type WasmVM struct {
	memory     []byte
	stack      []int64
	locals     []int64
	functions  map[string][]byte
	imports    map[string]ImportFunc
	gasUsed    uint64
	gasLimit   uint64
	returnData []byte
}

type ImportFunc func(vm *WasmVM, args []int64) ([]int64, error)

type ExecutionResult struct {
	ReturnData []byte
	GasUsed    uint64
	Err        error
}

func NewWasmVM(gasLimit uint64) *WasmVM {
	return &WasmVM{
		memory:    make([]byte, 64*1024),
		stack:     make([]int64, 0, 256),
		locals:    make([]int64, 16),
		functions: make(map[string][]byte),
		imports:   make(map[string]ImportFunc),
		gasLimit:  gasLimit,
	}
}

func (vm *WasmVM) RegisterImport(name string, fn ImportFunc) {
	vm.imports[name] = fn
}

func (vm *WasmVM) Memory() []byte {
	return vm.memory
}

func (vm *WasmVM) SetMemory(offset uint32, data []byte) error {
	if uint64(offset)+uint64(len(data)) > uint64(len(vm.memory)) {
		return fmt.Errorf("memory write out of bounds")
	}
	copy(vm.memory[offset:], data)
	return nil
}

func (vm *WasmVM) GetMemory(offset uint32, length uint32) ([]byte, error) {
	if uint64(offset)+uint64(length) > uint64(len(vm.memory)) {
		return nil, fmt.Errorf("memory read out of bounds")
	}
	return vm.memory[offset : offset+length], nil
}

func (vm *WasmVM) Execute(code []byte, args []int64) *ExecutionResult {
	if len(code) < 8 {
		return &ExecutionResult{Err: fmt.Errorf("invalid wasm binary")}
	}

	magic := binary.LittleEndian.Uint32(code[0:4])
	if magic != WasmMagic {
		return &ExecutionResult{Err: fmt.Errorf("invalid wasm magic number")}
	}

	version := binary.LittleEndian.Uint32(code[4:8])
	if version != WasmVersion {
		return &ExecutionResult{Err: fmt.Errorf("unsupported wasm version: %d", version)}
	}

	for i := range args {
		if i < len(vm.locals) {
			vm.locals[i] = args[i]
		}
	}

	vm.gasUsed = 0
	vm.stack = vm.stack[:0]

	err := vm.executeBody(code[8:])
	if err != nil {
		return &ExecutionResult{GasUsed: vm.gasUsed, Err: err}
	}

	var returnData []byte
	if len(vm.stack) > 0 {
		val := vm.stack[len(vm.stack)-1]
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, uint64(val))
		returnData = buf
	}

	return &ExecutionResult{
		ReturnData: returnData,
		GasUsed:    vm.gasUsed,
	}
}

func (vm *WasmVM) consumeGas(amount uint64) error {
	vm.gasUsed += amount
	if vm.gasUsed > vm.gasLimit {
		return fmt.Errorf("out of gas: used=%d limit=%d", vm.gasUsed, vm.gasLimit)
	}
	return nil
}

func (vm *WasmVM) executeBody(code []byte) error {
	pos := 0

	for pos < len(code) {
		opcode := code[pos]
		pos++

		if err := vm.consumeGas(1); err != nil {
			return err
		}

		switch opcode {
		case opCodeEnd, opCodeReturn:
			return nil

		case opCodeI32Const, opCodeI64Const:
			val, n, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n
			vm.stack = append(vm.stack, int64(val))

		case opCodeGetLocal:
			idx, n, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n
			if int(idx) < len(vm.locals) {
				vm.stack = append(vm.stack, vm.locals[idx])
			}

		case opCodeSetLocal:
			idx, n, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n
			if len(vm.stack) > 0 {
				val := vm.stack[len(vm.stack)-1]
				vm.stack = vm.stack[:len(vm.stack)-1]
				if int(idx) < len(vm.locals) {
					vm.locals[idx] = val
				}
			}

		case opCodeI32Add, opCodeI64Add:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on add")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			vm.stack = append(vm.stack, a+b)

		case opCodeI32Sub, opCodeI64Sub:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on sub")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			vm.stack = append(vm.stack, a-b)

		case opCodeI32Mul, opCodeI64Mul:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on mul")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			vm.stack = append(vm.stack, a*b)

		case opCodeI32Eq:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on eq")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			if a == b {
				vm.stack = append(vm.stack, 1)
			} else {
				vm.stack = append(vm.stack, 0)
			}

		case opCodeI32Ne:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on ne")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			if a != b {
				vm.stack = append(vm.stack, 1)
			} else {
				vm.stack = append(vm.stack, 0)
			}

		case opCodeI32LtS:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on lt")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			if int32(a) < int32(b) {
				vm.stack = append(vm.stack, 1)
			} else {
				vm.stack = append(vm.stack, 0)
			}

		case opCodeI32GtS:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on gt")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			if int32(a) > int32(b) {
				vm.stack = append(vm.stack, 1)
			} else {
				vm.stack = append(vm.stack, 0)
			}

		case opCodeI32LeS:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on le")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			if int32(a) <= int32(b) {
				vm.stack = append(vm.stack, 1)
			} else {
				vm.stack = append(vm.stack, 0)
			}

		case opCodeI32GeS:
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on ge")
			}
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			if int32(a) >= int32(b) {
				vm.stack = append(vm.stack, 1)
			} else {
				vm.stack = append(vm.stack, 0)
			}

		case opCodeDrop:
			if len(vm.stack) > 0 {
				vm.stack = vm.stack[:len(vm.stack)-1]
			}

		case opCodeSelect:
			if len(vm.stack) < 3 {
				return fmt.Errorf("stack underflow on select")
			}
			cond := vm.stack[len(vm.stack)-1]
			vm.stack = vm.stack[:len(vm.stack)-1]
			b := vm.stack[len(vm.stack)-1]
			a := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			if cond != 0 {
				vm.stack = append(vm.stack, a)
			} else {
				vm.stack = append(vm.stack, b)
			}

		case opCodeI32Store:
			offset, n, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n
			_, n2, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n2
			if len(vm.stack) < 2 {
				return fmt.Errorf("stack underflow on store")
			}
			val := vm.stack[len(vm.stack)-1]
			addr := vm.stack[len(vm.stack)-2]
			vm.stack = vm.stack[:len(vm.stack)-2]
			targetAddr := uint32(addr) + uint32(offset)
			if targetAddr+4 > uint32(len(vm.memory)) {
				return fmt.Errorf("store out of bounds")
			}
			binary.LittleEndian.PutUint32(vm.memory[targetAddr:], uint32(val))

		case opCodeI32Load:
			offset, n, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n
			_, n2, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n2
			if len(vm.stack) < 1 {
				return fmt.Errorf("stack underflow on load")
			}
			addr := vm.stack[len(vm.stack)-1]
			vm.stack = vm.stack[:len(vm.stack)-1]
			targetAddr := uint32(addr) + uint32(offset)
			if targetAddr+4 > uint32(len(vm.memory)) {
				return fmt.Errorf("load out of bounds")
			}
			val := int64(binary.LittleEndian.Uint32(vm.memory[targetAddr:]))
			vm.stack = append(vm.stack, val)

		case opCodeIf:
			pos++
			if len(vm.stack) < 1 {
				return fmt.Errorf("stack underflow on if")
			}
			cond := vm.stack[len(vm.stack)-1]
			vm.stack = vm.stack[:len(vm.stack)-1]
			if cond == 0 {
				depth := 1
				for depth > 0 && pos < len(code) {
					switch code[pos] {
					case opCodeIf, opCodeBlock, opCodeLoop:
						depth++
					case opCodeElse:
						if depth == 1 {
							pos++
							continue
						}
					case opCodeEnd:
						depth--
					}
					pos++
				}
			}

		case opCodeElse:
			depth := 1
			for depth > 0 && pos < len(code) {
				switch code[pos] {
				case opCodeIf, opCodeBlock, opCodeLoop:
					depth++
				case opCodeEnd:
					depth--
				}
				pos++
			}

		case opCodeLoop, opCodeBlock:
			pos++

		case opCodeBr:
			pos++
			vm.stack = vm.stack[:0]
			return nil

		case opCodeBrIf:
			pos++
			if len(vm.stack) > 0 {
				cond := vm.stack[len(vm.stack)-1]
				vm.stack = vm.stack[:len(vm.stack)-1]
				if cond != 0 {
					vm.stack = vm.stack[:0]
					return nil
				}
			}

		case opCodeCall:
			idx, n, err := readLeb128(code, pos)
			if err != nil {
				return err
			}
			pos += n
			importName := fmt.Sprintf("func_%d", idx)
			if fn, exists := vm.imports[importName]; exists {
				numArgs := len(vm.stack)
				if numArgs > 0 {
					args := make([]int64, numArgs)
					for i := range args {
						args[numArgs-1-i] = vm.stack[len(vm.stack)-1-i]
					}
					vm.stack = vm.stack[:0]
					results, err := fn(vm, args)
					if err != nil {
						return err
					}
					vm.stack = append(vm.stack, results...)
				}
			}

		default:
			if err := vm.consumeGas(10); err != nil {
				return err
			}
		}
	}

	return nil
}

func readLeb128(data []byte, pos int) (uint64, int, error) {
	if pos >= len(data) {
		return 0, 0, fmt.Errorf("unexpected end of data")
	}

	var result uint64
	var shift uint
	bytesRead := 0

	for pos+bytesRead < len(data) {
		b := data[pos+bytesRead]
		bytesRead++

		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, bytesRead, nil
		}
		shift += 7

		if shift >= 64 {
			return 0, 0, fmt.Errorf("leb128 overflow")
		}
	}

	return 0, 0, fmt.Errorf("unexpected end of leb128")
}

func (vm *WasmVM) PushI32(val int32) {
	vm.stack = append(vm.stack, int64(val))
}

func (vm *WasmVM) PushI64(val int64) {
	vm.stack = append(vm.stack, val)
}

func (vm *WasmVM) PopI32() (int32, error) {
	if len(vm.stack) == 0 {
		return 0, fmt.Errorf("stack empty")
	}
	val := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return int32(val), nil
}

func (vm *WasmVM) PopI64() (int64, error) {
	if len(vm.stack) == 0 {
		return 0, fmt.Errorf("stack empty")
	}
	val := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return val, nil
}

func (vm *WasmVM) GasUsed() uint64 {
	return vm.gasUsed
}

func (vm *WasmVM) Reset() {
	vm.stack = vm.stack[:0]
	vm.gasUsed = 0
	vm.returnData = nil
}

func U32(val uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, val)
	return buf
}

func U64(val uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, val)
	return buf
}

func F32(val float32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, math.Float32bits(val))
	return buf
}

func F64(val float64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, math.Float64bits(val))
	return buf
}
