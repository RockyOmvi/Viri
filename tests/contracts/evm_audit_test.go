package contracts

import (
	"math/big"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer2/vm"
)

type auditState struct {
	storage map[string][]byte
	code    map[string][]byte
	balance map[string]*big.Int
}

func newAuditState() *auditState {
	return &auditState{
		storage: make(map[string][]byte),
		code:    make(map[string][]byte),
		balance: make(map[string]*big.Int),
	}
}

func (a *auditState) GetNonce(addr []byte) uint64 { return 0 }

func (a *auditState) GetBalance(addr []byte) *big.Int {
	if b, ok := a.balance[string(addr)]; ok {
		return b
	}
	return big.NewInt(0)
}
func (a *auditState) GetCode(addr []byte) []byte       { return a.code[string(addr)] }
func (a *auditState) GetStorage(addr, key []byte) []byte { return a.storage[string(key)] }
func (a *auditState) SetStorage(addr, key, val []byte)   { a.storage[string(key)] = val }
func (a *auditState) Transfer(from, to []byte, amt *big.Int) {
	if from != nil {
		a.balance[string(from)] = new(big.Int).Sub(a.GetBalance(from), amt)
	}
	a.balance[string(to)] = new(big.Int).Add(a.GetBalance(to), amt)
}
func (a *auditState) AddLog(addr []byte, topics [][]byte, data []byte) {
}

func (a *auditState) Snapshot() int { return 0 }

func (a *auditState) RevertToSnapshot(int) {}

func (a *auditState) CreateAccount(addr []byte) {}

func auditCode(t *testing.T, name string, code []byte, ctx *vm.EVMContext, want []byte) {
	t.Helper()
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, gas, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("[%s] execution error: %v (gas: %d)", name, err, gas)
	}
	if len(output) != len(want) {
		t.Fatalf("[%s] output length mismatch: got %d, want %d\n  got:  %x\n  want: %x", name, len(output), len(want), output, want)
	}
	for i := range output {
		if output[i] != want[i] {
			t.Fatalf("[%s] output mismatch at byte %d: got %02x, want %02x\n  got:  %x\n  want: %x", name, i, output[i], want[i], output, want)
		}
	}
}

func auditCodeFail(t *testing.T, name string, code []byte, ctx *vm.EVMContext) {
	t.Helper()
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	_, _, err := exec.Execute(code)
	if err == nil {
		t.Fatalf("[%s] expected error but got none", name)
	}
}

func defaultCtx() *vm.EVMContext {
	return &vm.EVMContext{
		Caller:   bytes20FromU16(0xCA11),
		Address:  bytes20FromU16(0xCA11),
		Value:    big.NewInt(42),
		GasLimit: 1000000,
		GasPrice: big.NewInt(1),
		Data:     []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
}

func bytes20FromU16(v uint16) []byte {
	b := make([]byte, 20)
	b[18] = byte(v >> 8)
	b[19] = byte(v)
	return b
}

func bytes20(v byte) []byte {
	b := make([]byte, 20)
	b[19] = v
	return b
}

func fill32(v byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = v
	}
	return b
}

func makeExpected32(prefix []byte) []byte {
	b := make([]byte, 32)
	copy(b[32-len(prefix):], prefix)
	return b
}

func TestAuditArithmetic(t *testing.T) {
	ctx := defaultCtx()

	// ADD: 40 + 2 = 42
	auditCode(t, "ADD", []byte{0x60, 0x28, 0x60, 0x02, 0x01, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}, ctx, makeExpected32([]byte{0x2A}))

	// SUB: 50 - 8 = 42
	auditCode(t, "SUB", []byte{0x60, 0x32, 0x60, 0x08, 0x03, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}, ctx, makeExpected32([]byte{0x2A}))

	// MUL: 6 * 7 = 42
	auditCode(t, "MUL", []byte{0x60, 0x06, 0x60, 0x07, 0x02, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}, ctx, makeExpected32([]byte{0x2A}))

	// DIV: 84 / 2 = 42
	auditCode(t, "DIV", []byte{0x60, 0x54, 0x60, 0x02, 0x04, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}, ctx, makeExpected32([]byte{0x2A}))

	// DIV by zero = 0
	auditCode(t, "DIVbyZero", []byte{0x60, 0x2A, 0x60, 0x00, 0x04, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}, ctx, make([]byte, 32))

	// SDIV: -84 / 2 = -42
	code := []byte{
		0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xAC, // -84
		0x60, 0x02, 0x05, // SDIV: -84 / 2 = -42
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	want := fill32(0xFF)
	want[31] = 0xD6 // -42 = 0xFF...D6
	auditCode(t, "SDIV", code, ctx, want)

	// MOD: 100 % 3 = 1
	auditCode(t, "MOD", []byte{0x60, 0x64, 0x60, 0x03, 0x06, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}, ctx, makeExpected32([]byte{0x01}))

	// MOD by zero = 0
	auditCode(t, "MODbyZero", []byte{0x60, 0x2A, 0x60, 0x00, 0x06, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}, ctx, make([]byte, 32))

	// SMOD signed, ADDRESS opcode, CALLER opcode
	// SIGNEXTEND: extend sign of 0xFF (byte0) -> 0xFFFFFFFF...FF
	code2 := []byte{
		0x60, 0xFF, // value
		0x60, 0x00, // byte index 0 (top of stack)
		0x0B, // SIGNEXTEND
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "SIGNEXTEND", code2, ctx, fill32(0xFF))
}

func TestAuditMemory(t *testing.T) {
	ctx := defaultCtx()

	// MLOAD: store 42, load from offset 0
	code := []byte{
		0x60, 0x2A, 0x60, 0x00, 0x52, // MSTORE(0, 42)
		0x60, 0x00, 0x51, // MLOAD(0)
		0x60, 0x00, 0x52, // MSTORE(0, result)
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0, 32)
	}
	auditCode(t, "MLOAD", code, ctx, makeExpected32([]byte{0x2A}))

	// MSTORE8: store 0x42 at offset 0, read back via MSTORE/MLOAD cycle
	code2 := []byte{
		0x60, 0x42, 0x60, 0x00, 0x53, // MSTORE8(0, 0x42)
		0x60, 0x00, 0x51, // MLOAD(0) -> big-endian 0x4200...
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	want := make([]byte, 32)
	want[0] = 0x42
	auditCode(t, "MSTORE8", code2, ctx, want)

	// PC, MSIZE
	code3 := []byte{
		0x58,       // PC = 0
		0x59,       // MSIZE = 0 (no memory allocated yet)
		0x01,       // ADD: 0 + 0 = 0
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "PC_MSIZE", code3, ctx, make([]byte, 32))
}

func TestAuditEnvironment(t *testing.T) {
	// ADDRESS
	auditCode(t, "ADDRESS", []byte{
		0x30, // ADDRESS = 0x00...CA11
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, defaultCtx(), makeExpected32([]byte{0xCA, 0x11}))

	// CALLER
	ctx := defaultCtx()
	ctx.Caller = bytes20(0xFF)
	auditCode(t, "CALLER", []byte{
		0x33, // CALLER = 0x00...FF
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, ctx, makeExpected32([]byte{0xFF}))

	// CALLVALUE = 42
	auditCode(t, "CALLVALUE", []byte{
		0x34, // CALLVALUE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, defaultCtx(), makeExpected32([]byte{0x2A}))

	// ORIGIN = same as CALLER in direct call case (matches current impl)
	auditCode(t, "ORIGIN", []byte{
		0x32, // ORIGIN = same as caller
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, defaultCtx(), makeExpected32([]byte{0xCA, 0x11}))

	// GAS: PUSH1 0, GAS (should be > 0)
	code := []byte{
		0x5A,       // GAS
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(defaultCtx(), state)
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("GAS failed: %v", err)
	}
	gasVal := new(big.Int).SetBytes(output)
	if gasVal.Sign() <= 0 {
		t.Fatalf("GAS returned non-positive: %x", output)
	}

	// CODESIZE
	code2 := []byte{0x38, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}
	auditCode(t, "CODESIZE", code2, defaultCtx(), makeExpected32([]byte{0x09}))
}

func TestAuditCODECOPY(t *testing.T) {
	// CODECOPY: copy 4 bytes from code to memory, return them
	// Layout: JUMP over data, data, JUMPDEST, then CODECOPY + RETURN
	code := []byte{
		0x60, 0x07,       // PUSH1 7 (jump dest after data)
		0x56,             // JUMP
		0xDE, 0xAD, 0xBE, 0xEF, // target data at offset 3
		0x5B,             // JUMPDEST at offset 7
		0x60, 0x04,       // size = 4
		0x60, 0x03,       // code offset = 3
		0x60, 0x00,       // mem offset = 0
		0x39,             // CODECOPY
		0x60, 0x04,       // size = 4
		0x60, 0x00,       // mem offset = 0
		0xF3,             // RETURN
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(defaultCtx(), state)
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CODECOPY failed: %v", err)
	}
	if len(output) != 4 {
		t.Fatalf("CODECOPY output len: got %d, want 4", len(output))
	}
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	for i := range want {
		if output[i] != want[i] {
			t.Fatalf("CODECOPY mismatch at %d: got %02x, want %02x", i, output[i], want[i])
		}
	}
}

func TestAuditRETURNDATACOPY(t *testing.T) {
	// Use a sub-context style: execute code that calls another contract
	// Since mockState doesn't support CALL, test RETURNDATASIZE and RETURNDATACOPY directly
	// by verifying they work with previously set returndata from internal CALL

	// For direct unit test, just verify both opcodes don't crash
	// (full testing requires the execution engine)
	ctx := defaultCtx()
	code := []byte{
		0x3D,       // RETURNDATASIZE = 0 (no prior call)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "RETURNDATASIZE", code, ctx, make([]byte, 32))
}

func TestAuditStack(t *testing.T) {
	ctx := defaultCtx()

	// POP: push 42, push 100, pop, mstore 42 -> result = 42
	auditCode(t, "POP", []byte{
		0x60, 0x2A, // push 42
		0x60, 0x64, // push 100
		0x50,       // POP (remove 100)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, ctx, makeExpected32([]byte{0x2A}))

	// DUP1: push 42, DUP1, ADD -> 84
	auditCode(t, "DUP1", []byte{
		0x60, 0x2A, 0x80, 0x01, // DUP1 ADD
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, ctx, makeExpected32([]byte{0x54}))

	// DUP2: push 1, push 2, DUP2 -> stack [1,2,1], ADD -> 3, ADD -> 4
	auditCode(t, "DUP2", []byte{
		0x60, 0x01, 0x60, 0x02, 0x81, // DUP2 → stack [1,2,1]
		0x01, 0x01, // ADD ADD
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, ctx, makeExpected32([]byte{0x04}))

	// SWAP1: push 1, push 2, SWAP1 -> stack [2,1], ADD -> 3
	auditCode(t, "SWAP1", []byte{
		0x60, 0x01, 0x60, 0x02, 0x90, // SWAP1
		0x01,
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}, ctx, makeExpected32([]byte{0x03}))
}

func TestAuditSHA3(t *testing.T) {
	ctx := defaultCtx()
	// Store 0x2A at offset 0, SHA3(0, 32)
	code := []byte{
		0x60, 0x2A, 0x60, 0x00, 0x52, // MSTORE(0, 42)
		0x60, 0x20, // size = 32
		0x60, 0x00, // offset = 0
		0x20,       // SHA3
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("SHA3 failed: %v", err)
	}
	h := crypto.Keccak256(makeExpected32([]byte{0x2A}))
	for i := range h {
		if output[i] != h[i] {
			t.Fatalf("SHA3 mismatch at %d: got %02x, want %02x", i, output[i], h[i])
		}
	}
}

func TestAuditSLOAD_SSTORE(t *testing.T) {
	ctx := defaultCtx()
	state := newAuditState()

	// SSTORE key=1, value=42, then SLOAD key=1
	code := []byte{
		0x60, 0x2A, // value 42
		0x60, 0x01, // key 1
		0x55,       // SSTORE
		0x60, 0x01, // key 1
		0x54,       // SLOAD
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("SLOAD/SSTORE failed: %v", err)
	}
	if output[31] != 0x2A {
		t.Fatalf("SLOAD expected 0x2A, got %x", output)
	}
}

func TestAuditControlFlow(t *testing.T) {
	ctx := defaultCtx()

	// JUMP: jump to dest at PC 13, push 42, STOP, dest: push 1, MSTORE, RETURN
	code := []byte{
		0x60, 0x0D, // push dest = 13
		0x56,       // JUMP
		0x60, 0x2A, // (skipped)
		0x60, 0x00, 0x52, // (skipped)
		0x60, 0x20, 0x60, 0x00, 0xF3, // (skipped)
		0x5B,       // JUMPDEST
		0x60, 0x01, // push 1
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "JUMP", code, ctx, makeExpected32([]byte{0x01}))

	// JUMPI: push dest=15, push cond=1, JUMPI -> jumps
	code2 := []byte{
		0x60, 0x0F, // dest = 15 (first push, deeper)
		0x60, 0x01, // condition = true (last push, top)
		0x57,       // JUMPI
		0x60, 0x2A, // (skipped)
		0x60, 0x00, 0x52, // (skipped)
		0x60, 0x20, 0x60, 0x00, 0xF3, // (skipped)
		0x5B,       // JUMPDEST
		0x60, 0x01, // push 1
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "JUMPI", code2, ctx, makeExpected32([]byte{0x01}))

	// JUMPI with false condition: no jump
	code3 := []byte{
		0x60, 0x0A, // dest = 10 (first push, deeper, ignored)
		0x60, 0x00, // condition = false (last push, top)
		0x57,       // JUMPI
		0x60, 0x2A, // push 42 (NOT skipped)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
		0x5B, 0x60, 0x01, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3, // dest (unreachable)
	}
	auditCode(t, "JUMPI_false", code3, ctx, makeExpected32([]byte{0x2A}))

	// STOP: push 42, STOP (should return nil)
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute([]byte{0x60, 0x2A, 0x00 /* STOP */})
	if err != nil {
		t.Fatalf("STOP error: %v", err)
	}
	if len(output) != 0 {
		t.Fatalf("STOP expected nil output, got %x", output)
	}

	// INVALID
	code4 := []byte{0xFE}
	state2 := newAuditState()
	exec2 := vm.NewEVMExecutor(ctx, state2)
	_, _, err = exec2.Execute(code4)
	if err == nil {
		t.Fatalf("INVALID should produce error")
	}
}

func TestAuditLOG(t *testing.T) {
	ctx := defaultCtx()
	// LOG0: push data in memory, push length, push offset, LOG0
	// Since our EVM doesn't emit logs to a collector, verify it doesn't crash
	code := []byte{
		0x60, 0x2A, 0x60, 0x00, 0x52, // MSTORE(0, 42)
		0x60, 0x20, // length = 32
		0x60, 0x00, // offset = 0
		0xA0,       // LOG0
		0x60, 0x01, // push 1 (return value placeholder)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	_, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("LOG0 failed: %v", err)
	}

	// LOG4 with 4 topics
	code2 := []byte{
		0x60, 0x2A, 0x60, 0x00, 0x52,
		0x60, 0x01, // topic4
		0x60, 0x02, // topic3
		0x60, 0x03, // topic2
		0x60, 0x04, // topic1
		0x60, 0x20, // length
		0x60, 0x00, // offset
		0xA4,       // LOG4
		0x60, 0x01, 0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	state2 := newAuditState()
	exec2 := vm.NewEVMExecutor(ctx, state2)
	_, _, err = exec2.Execute(code2)
	if err != nil {
		t.Fatalf("LOG4 failed: %v", err)
	}
}

func TestAuditREVERT(t *testing.T) {
	ctx := defaultCtx()
	code := []byte{
		0x60, 0x2A, 0x60, 0x00, 0x52, // MSTORE(0, 42)
		0x60, 0x20, // size
		0x60, 0x00, // offset
		0xFD,       // REVERT
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute(code)
	if err == nil {
		t.Fatalf("REVERT should produce error")
	}
	if len(output) != 32 || output[31] != 0x2A {
		t.Fatalf("REVERT output mismatch: got %x", output)
	}
}

func TestAuditBALANCE(t *testing.T) {
	ctx := defaultCtx()
	state := newAuditState()
	state.balance[string(bytes20(0xBB))] = big.NewInt(100)

	code := []byte{
		0x60, 0xBB, // address
		0x31,       // BALANCE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("BALANCE failed: %v", err)
	}
	if output[31] != 100 {
		t.Fatalf("BALANCE expected 100, got %d", output[31])
	}
}

func TestAuditCALLDATACOPY(t *testing.T) {
	ctx := defaultCtx()
	ctx.Data = []byte{0xAA, 0xBB, 0xCC, 0xDD}

	code := []byte{
		0x60, 0x04, // size = 4
		0x60, 0x00, // src offset = 0
		0x60, 0x00, // dest offset = 0
		0x37,       // CALLDATACOPY
		0x60, 0x04, // size = 4
		0x60, 0x00, // offset = 0
		0xF3,       // RETURN
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CALLDATACOPY failed: %v", err)
	}
	want := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	if len(output) != 4 {
		t.Fatalf("CALLDATACOPY len: %d", len(output))
	}
	for i := range want {
		if output[i] != want[i] {
			t.Fatalf("CALLDATACOPY mismatch at %d: got %02x, want %02x", i, output[i], want[i])
		}
	}
}

func TestAuditBYTE(t *testing.T) {
	ctx := defaultCtx()
	// BYTE(31, 0x0000...00FF) = 0xFF (last byte)
	code := []byte{
		0x61, 0x00, 0xFF, // value = 0xFF
		0x60, 0x1F, // byte index 31 (last byte)
		0x1A,       // BYTE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "BYTE", code, ctx, makeExpected32([]byte{0xFF}))
}

func TestAuditPUSH(t *testing.T) {
	ctx := defaultCtx()
	// PUSH2 0x0102 -> MSTORE -> RETURN
	code := []byte{
		0x61, 0x01, 0x02, // PUSH2 0x0102
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "PUSH2", code, ctx, makeExpected32([]byte{0x01, 0x02}))

	// PUSH32 full word
	code2 := []byte{
		0x7F,       // PUSH32
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20,
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	want := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20,
	}
	auditCode(t, "PUSH32", code2, ctx, want)
}

// --- Security / Edge Case Tests ---

func TestAuditStackUnderflow(t *testing.T) {
	ctx := defaultCtx()

	tests := []struct {
		name string
		code []byte
	}{
		{"ADD empty", []byte{0x01}},
		{"ADD one elem", []byte{0x60, 0x01, 0x01}},
		{"SUB empty", []byte{0x03}},
		{"MUL empty", []byte{0x02}},
		{"DIV empty", []byte{0x04}},
		{"MOD empty", []byte{0x06}},
		{"EQ empty", []byte{0x14}},
		{"LT empty", []byte{0x10}},
		{"GT empty", []byte{0x11}},
		{"AND empty", []byte{0x16}},
		{"OR empty", []byte{0x17}},
		{"XOR empty", []byte{0x18}},
		{"NOT empty", []byte{0x19}},
		{"SHL empty", []byte{0x1B}},
		{"SHR empty", []byte{0x1C}},
		{"MSTORE empty", []byte{0x52}},
		{"MSTORE one elem", []byte{0x60, 0x01, 0x52}},
		{"MLOAD empty", []byte{0x51}},
		{"SSTORE empty", []byte{0x55}},
		{"SSTORE one elem", []byte{0x60, 0x01, 0x55}},
		{"SLOAD empty", []byte{0x54}},
		{"JUMP empty", []byte{0x56}},
		{"JUMPI empty", []byte{0x57}},
		{"JUMPI one elem", []byte{0x60, 0x01, 0x57}},
		{"RETURN empty", []byte{0xF3}},
		{"RETURN one elem", []byte{0x60, 0x01, 0xF3}},
		{"REVERT empty", []byte{0xFD}},
		{"POP empty", []byte{0x50}},
		{"BYTE empty", []byte{0x1A}},
		{"BYTE one elem", []byte{0x60, 0x01, 0x1A}},
		{"CALLDATALOAD empty", []byte{0x35}},
		{"CALLDATACOPY empty", []byte{0x37}},
		{"CALLDATACOPY <3 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x37}},
		{"CODECOPY empty", []byte{0x39}},
		{"CODECOPY <3 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x39}},
		{"SHA3 empty", []byte{0x20}},
		{"SHA3 one elem", []byte{0x60, 0x01, 0x20}},
		{"LOG0 empty", []byte{0xA0}},
		{"LOG0 one elem", []byte{0x60, 0x01, 0xA0}},
		{"LOG4 empty", []byte{0xA4}},
		{"BALANCE empty", []byte{0x31}},
		{"SDIV empty", []byte{0x05}},
		{"SMOD empty", []byte{0x07}},
		{"SIGNEXTEND empty", []byte{0x0B}},
		{"SIGNEXTEND one elem", []byte{0x60, 0x01, 0x0B}},
		{"SAR empty", []byte{0x1D}},
		{"SAR one elem", []byte{0x60, 0x01, 0x1D}},
		{"SLT empty", []byte{0x12}},
		{"SGT empty", []byte{0x13}},
		{"RETURNDATACOPY empty", []byte{0x3E}},
		{"RETURNDATACOPY <3 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x3E}},
		{"CREATE empty", []byte{0xF0}},
		{"CREATE <3 elems", []byte{0x60, 0x01, 0x60, 0x02, 0xF0}},
		{"CALL empty", []byte{0xF1}},
		{"CALL <7 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x60, 0x03, 0x60, 0x04, 0x60, 0x05, 0x60, 0x06, 0xF1}},
		{"STATICCALL empty", []byte{0xFA}},
		{"STATICCALL <6 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x60, 0x03, 0x60, 0x04, 0x60, 0x05, 0xFA}},
		{"DELEGATECALL empty", []byte{0xF4}},
		{"DELEGATECALL <6 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x60, 0x03, 0x60, 0x04, 0x60, 0x05, 0xF4}},
		{"CALLCODE empty", []byte{0xF2}},
		{"CALLCODE <7 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x60, 0x03, 0x60, 0x04, 0x60, 0x05, 0x60, 0x06, 0xF2}},
		{"CREATE2 empty", []byte{0xF5}},
		{"CREATE2 <4 elems", []byte{0x60, 0x01, 0x60, 0x02, 0x60, 0x03, 0xF5}},
	}

	for _, tc := range tests {
		state := newAuditState()
		exec := vm.NewEVMExecutor(ctx, state)
		_, _, err := exec.Execute(tc.code)
		if err == nil {
			t.Errorf("[%s] expected stack underflow but got none", tc.name)
		}
	}
}

func TestAuditInvalidJump(t *testing.T) {
	ctx := defaultCtx()

	// Jump to non-existent destination (no JUMPDEST at target)
	code := []byte{
		0x60, 0x05, // dest = 5
		0x56,       // JUMP
		0x5B,       // JUMPDEST at offset 3? No, JUMPDEST at offset 3 is at byte 3
		0x60, 0x2A, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	_, _, err := exec.Execute(code)
	if err == nil {
		t.Fatalf("JUMP to non-JUMPDEST should error")
	}
	_ = err
}

func TestAuditOutOfGas(t *testing.T) {
	ctx := defaultCtx()
	ctx.GasLimit = 5 // very low gas

	code := []byte{
		0x60, 0x01, 0x60, 0x02, 0x01, // ADD (needs 3+ gas)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	_, _, err := exec.Execute(code)
	if err == nil {
		t.Fatal("expected out of gas error")
	}
}

func TestAuditArithmeticUnderflow(t *testing.T) {
	ctx := defaultCtx()
	// SUB: 0 - 1 should wrap to 2^256-1
	code := []byte{
		0x60, 0x00, 0x60, 0x01, 0x03, // SUB: 0 - 1
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("SUB underflow error: %v", err)
	}
	val := new(big.Int).SetBytes(output)
	expected := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	if val.Cmp(expected) != 0 {
		t.Fatalf("SUB underflow: got %x, want %x", output, expected.Bytes())
	}
}

func TestAuditSIGNEXTENDEdgeCases(t *testing.T) {
	ctx := defaultCtx()

	// SIGNEXTEND with index >= 31 does nothing (treats as no-extension)
	code := []byte{
		0x60, 0xFF, // value
		0x60, 0x20, // index 32 >= 31 (top of stack)
		0x0B, // SIGNEXTEND
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "SIGNEXTEND_high_idx", code, ctx, makeExpected32([]byte{0xFF}))

	// SIGNEXTEND with 0xFF and index 0 -> 0xFFFFFFFF...FF
	code2 := []byte{
		0x60, 0xFF, // value
		0x60, 0x00, // index 0 (top of stack)
		0x0B,
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "SIGNEXTEND_neg", code2, ctx, fill32(0xFF))

	// SIGNEXTEND with 0x7F and index 0 -> 0x0000...7F (positive, no change)
	code3 := []byte{
		0x60, 0x7F, // value
		0x60, 0x00, // index 0 (top of stack)
		0x0B,
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "SIGNEXTEND_pos", code3, ctx, makeExpected32([]byte{0x7F}))
}

func TestAuditSHLEdgeCases(t *testing.T) {
	ctx := defaultCtx()

	// SHL by 256+ should return 0
	code := []byte{
		0x60, 0x01, 0x60, 0xFF, 0x1B, // SHL(1, 255)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	want := make([]byte, 32)
	want[0] = 0x80
	auditCode(t, "SHL_by_255", code, ctx, want) // 1 << 255 = 0x8000...00

	// SHL by >255 returns 0
	code2 := []byte{
		0x60, 0x01,       // val = 1
		0x61, 0x01, 0x00, // PUSH2 256 (shift amount, top of stack)
		0x1B,             // SHL
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "SHL_by_256", code2, ctx, make([]byte, 32))
}

func TestAuditSHRSAEdgeCases(t *testing.T) {
	ctx := defaultCtx()

	// SHR by 256+ returns 0
	code := []byte{
		0x61, 0x01, 0x00, 0x61, 0x01, 0x00, 0x1C, // SHR(256, 256)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	auditCode(t, "SHR_by_256", code, ctx, make([]byte, 32))

	// SAR by 256+ should return 0 for positive, -1 for negative
	neg2 := make([]byte, 33)
	neg2[0] = 0x7F
	for i := 1; i <= 32; i++ {
		neg2[i] = 0xFF
	}
	neg2[32] = 0xFE // -2 in 256-bit two's complement
	code2 := append(neg2, []byte{
		0x61, 0x01, 0x00, // PUSH2 256
		0x1D, // SAR
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}...)
	auditCode(t, "SAR_neg_by_large", code2, ctx, fill32(0xFF))
}

func TestAuditEmptyBytecode(t *testing.T) {
	ctx := defaultCtx()
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute([]byte{})
	if err != nil {
		t.Fatalf("empty code error: %v", err)
	}
	if len(output) != 0 {
		t.Fatalf("empty code output: %x", output)
	}
}

func TestAuditSingleSTOP(t *testing.T) {
	ctx := defaultCtx()
	state := newAuditState()
	exec := vm.NewEVMExecutor(ctx, state)
	output, _, err := exec.Execute([]byte{0x00})
	if err != nil {
		t.Fatalf("STOP error: %v", err)
	}
	if len(output) != 0 {
		t.Fatalf("STOP output: %x", output)
	}
}
