package contracts

import (
	"math/big"
	"testing"

	"github.com/viri-chain/viri/internal/layer2/vm"
)

// Helper: runs a raw EVM bytecode and returns the output
func runCode(code []byte) ([]byte, uint64, error) {
	state := &mockState{storage: make(map[string][]byte)}
	ctx := &vm.EVMContext{
		Caller:   make([]byte, 20),
		Address:  make([]byte, 20),
		Value:    big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Data:     nil,
	}
	exec := vm.NewEVMExecutor(ctx, state)
	return exec.Execute(code)
}

type mockState struct {
	storage map[string][]byte
}

func (m *mockState) GetNonce(addr []byte) uint64          { return 0 }
func (m *mockState) GetBalance(addr []byte) *big.Int     { return big.NewInt(0) }
func (m *mockState) GetCode(addr []byte) []byte           { return nil }
func (m *mockState) GetStorage(addr, key []byte) []byte   { return m.storage[string(key)] }
func (m *mockState) SetStorage(addr, key, val []byte)     { m.storage[string(key)] = val }
func (m *mockState) Transfer(from, to []byte, amt *big.Int) {}
func (m *mockState) AddLog(addr []byte, topics [][]byte, data []byte) {}

func (m *mockState) Snapshot() int { return 0 }

func (m *mockState) RevertToSnapshot(int) {}

func (m *mockState) CreateAccount(addr []byte)            {}

func runAndCheck(t *testing.T, code []byte, expected []byte) {
	t.Helper()
	output, gas, err := runCode(code)
	if err != nil {
		t.Fatalf("execution error: %v (gas: %d)", err, gas)
	}
	if len(output) != len(expected) {
		t.Fatalf("output length mismatch: got %d, want %d\n  got:  %x\n  want: %x", len(output), len(expected), output, expected)
	}
	for i := range output {
		if output[i] != expected[i] {
			t.Fatalf("output mismatch at byte %d: got %02x, want %02x\n  got:  %x\n  want: %x", i, output[i], expected[i], output, expected)
		}
	}
}

func TestOpcodeEQ(t *testing.T) {
	// PUSH1 0x2A (42), PUSH1 0x2A (42), EQ, PUSH1 0x00, MSTORE, PUSH1 0x20, PUSH1 0x00, RETURN
	code := []byte{
		0x60, 0x2A, // PUSH1 42
		0x60, 0x2A, // PUSH1 42
		0x14,       // EQ
		0x60, 0x00, // PUSH1 0
		0x52,       // MSTORE
		0x60, 0x20, // PUSH1 32
		0x60, 0x00, // PUSH1 0
		0xF3,       // RETURN
	}
	expected := make([]byte, 32)
	expected[31] = 1 // result = 1 (42 == 42)
	runAndCheck(t, code, expected)
}

func TestOpcodeEQNotEqual(t *testing.T) {
	code := []byte{
		0x60, 0x2A, 0x60, 0x01, 0x14, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32) // result = 0 (42 != 1)
	runAndCheck(t, code, expected)
}

func TestOpcodeISZERO(t *testing.T) {
	// PUSH1 0x00, ISZERO, MSTORE, RETURN (should return 1)
	code := []byte{
		0x60, 0x00, 0x15, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 1
	runAndCheck(t, code, expected)
}

func TestOpcodeISZERONonzero(t *testing.T) {
	code := []byte{
		0x60, 0x01, 0x15, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32) // ISZERO(1) = 0
	runAndCheck(t, code, expected)
}

func TestOpcodeLT(t *testing.T) {
	// PUSH1 20 (right), PUSH1 10 (left), LT -> 1 (10 < 20)
	code := []byte{
		0x60, 0x14, 0x60, 0x0A, 0x10, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 1
	runAndCheck(t, code, expected)
}

func TestOpcodeLTFalse(t *testing.T) {
	code := []byte{
		0x60, 0x0A, 0x60, 0x14, 0x10, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32) // 20 < 10 = false
	runAndCheck(t, code, expected)
}

func TestOpcodeGT(t *testing.T) {
	code := []byte{
		0x60, 0x0A, 0x60, 0x14, 0x11, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 1 // 20 > 10 = true
	runAndCheck(t, code, expected)
}

func TestOpcodeAND(t *testing.T) {
	code := []byte{
		0x60, 0x0F, 0x60, 0x03, 0x16, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 0x03 // 0x0F & 0x03 = 0x03
	runAndCheck(t, code, expected)
}

func TestOpcodeOR(t *testing.T) {
	code := []byte{
		0x60, 0x0F, 0x60, 0x03, 0x17, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 0x0F // 0x0F | 0x03 = 0x0F
	runAndCheck(t, code, expected)
}

func TestOpcodeXOR(t *testing.T) {
	code := []byte{
		0x60, 0xFF, 0x60, 0x0F, 0x18, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 0xF0 // 0xFF ^ 0x0F = 0xF0
	runAndCheck(t, code, expected)
}

func TestOpcodeNOT(t *testing.T) {
	// NOT on 32-byte value: PUSH32 0, NOT -> all 1s
	code := []byte{
		0x7F, // PUSH32
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x19,       // NOT
		0x60, 0x00, 0x52, // MSTORE
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN
	}
	expected := make([]byte, 32)
	for i := range expected {
		expected[i] = 0xFF
	}
	runAndCheck(t, code, expected)
}

func TestOpcodeSHL(t *testing.T) {
	code := []byte{
		0x60, 0x01, // PUSH1 1 (value)
		0x60, 0x01, // PUSH1 1 (shift)
		0x1B,       // SHL
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 0x02 // 1 << 1 = 2
	runAndCheck(t, code, expected)
}

func TestOpcodeSHLBy8(t *testing.T) {
	code := []byte{
		0x60, 0x01, // PUSH1 1 (value)
		0x60, 0x08, // PUSH1 8 (shift)
		0x1B,       // SHL
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[30] = 0x01 // 1 << 8 = 256 = 0x0100
	runAndCheck(t, code, expected)
}

func TestOpcodeSHR(t *testing.T) {
	code := []byte{
		0x60, 0x02, // PUSH1 2 (value)
		0x60, 0x01, // PUSH1 1 (shift)
		0x1C,       // SHR
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 0x01 // 2 >> 1 = 1
	runAndCheck(t, code, expected)
}

func TestOpcodeSHRBy8(t *testing.T) {
	code := []byte{
		0x61, 0x01, 0x00, // PUSH2 256 (value)
		0x60, 0x08,       // PUSH1 8 (shift)
		0x1C,             // SHR
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	expected[31] = 0x01 // 256 >> 8 = 1
	runAndCheck(t, code, expected)
}

func TestOpcodeSAR(t *testing.T) {
	code := []byte{
		0x7F,       // PUSH32 -2 (value)
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE,
		0x60, 0x01, // PUSH1 1 (shift)
		0x1D,       // SAR
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	for i := range expected {
		expected[i] = 0xFF
	}
	expected[31] = 0xFF // SAR(-2, 1) = -1 = 0xFF...FF
	runAndCheck(t, code, expected)
}

func TestOpcodeCALLDATALOAD(t *testing.T) {
	ctx := &vm.EVMContext{
		Caller:   make([]byte, 20),
		Address:  make([]byte, 20),
		Value:    big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Data:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2A},
	}
	state := &mockState{storage: make(map[string][]byte)}
	exec := vm.NewEVMExecutor(ctx, state)
	// CALLDATALOAD at offset 0, MSTORE, RETURN
	code := []byte{
		0x60, 0x00, // offset 0
		0x35,       // CALLDATALOAD
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CALLDATALOAD failed: %v", err)
	}
	if len(output) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(output))
	}
	if output[31] != 0x2A {
		t.Fatalf("expected last byte 0x2A, got 0x%02x (output: %x)", output[31], output)
	}
}

func TestOpcodeCALLDATASIZE(t *testing.T) {
	ctx := &vm.EVMContext{
		Caller:   make([]byte, 20),
		Address:  make([]byte, 20),
		Value:    big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Data:     []byte{0x01, 0x02, 0x03},
	}
	state := &mockState{storage: make(map[string][]byte)}
	exec := vm.NewEVMExecutor(ctx, state)
	code := []byte{
		0x36,       // CALLDATASIZE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CALLDATASIZE failed: %v", err)
	}
	if len(output) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(output))
	}
	if output[31] != 3 {
		t.Fatalf("expected size 3, got %d (output: %x)", output[31], output)
	}
}

func TestOpcodeChain(t *testing.T) {
	// PUSH1 20, PUSH1 10, LT, PUSH1 5, PUSH1 1, GT, AND, ISZERO, NOT, MSTORE, RETURN
	// (10 < 20) = 1, (1 > 5) = 0, 1 & 0 = 0, ISZERO(0) = 1, NOT(1) = 0xFF...FE
	code := []byte{
		0x60, 0x14, 0x60, 0x0A, 0x10, // LT: 10 < 20 = 1
		0x60, 0x05, 0x60, 0x01, 0x11, // GT: 1 > 5 = 0
		0x16,       // AND: 1 & 0 = 0
		0x15,       // ISZERO: 0 -> 1
		0x19,       // NOT: ~1 = 0xFF...FE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	expected := make([]byte, 32)
	for i := range expected {
		expected[i] = 0xFF
	}
	expected[31] = 0xFE // NOT(1) in 256-bit = 0xFF...FE
	runAndCheck(t, code, expected)
}
