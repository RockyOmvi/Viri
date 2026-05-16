package vm

import (
	"math/big"
	"testing"
)

type testState struct {
	balances  map[string]*big.Int
	nonces    map[string]uint64
	codes     map[string][]byte
	storage   map[string]map[string][]byte
	logs      []logEntry
	journal   []journalEntry
	snapshots []int
}

type logEntry struct {
	addr   []byte
	topics [][]byte
	data   []byte
}

type journalAction uint8

const (
	jBal journalAction = iota
	jStor
	jCreate
)

type journalEntry struct {
	action journalAction
	addr   string
	key    string
	oldVal interface{}
}

func newTestState() *testState {
	return &testState{
		balances: make(map[string]*big.Int),
		nonces:   make(map[string]uint64),
		codes:    make(map[string][]byte),
		storage:  make(map[string]map[string][]byte),
	}
}

func (s *testState) GetBalance(addr []byte) *big.Int {
	b, ok := s.balances[string(addr)]
	if !ok || b == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(b)
}

func (s *testState) GetNonce(addr []byte) uint64 {
	return s.nonces[string(addr)]
}

func (s *testState) GetCode(addr []byte) []byte {
	return s.codes[string(addr)]
}

func (s *testState) GetStorage(addr, key []byte) []byte {
	m, ok := s.storage[string(addr)]
	if !ok {
		return nil
	}
	return m[string(key)]
}

func (s *testState) SetStorage(addr, key, value []byte) {
	am := string(addr)
	m, ok := s.storage[am]
	if !ok {
		m = make(map[string][]byte)
		s.storage[am] = m
	}
	oldVal := m[string(key)]
	m[string(key)] = value
	s.journal = append(s.journal, journalEntry{action: jStor, addr: am, key: string(key), oldVal: oldVal})
}

func (s *testState) Transfer(from, to []byte, amount *big.Int) {
	fs, ts := string(from), string(to)
	oldFrom := new(big.Int).Set(s.GetBalance(from))
	oldTo := new(big.Int).Set(s.GetBalance(to))
	if _, ok := s.balances[fs]; !ok {
		s.balances[fs] = big.NewInt(0)
	}
	if _, ok := s.balances[ts]; !ok {
		s.balances[ts] = big.NewInt(0)
	}
	s.balances[fs] = new(big.Int).Sub(s.balances[fs], amount)
	s.balances[ts] = new(big.Int).Add(s.balances[ts], amount)
	s.journal = append(s.journal, journalEntry{action: jBal, addr: fs, oldVal: oldFrom})
	s.journal = append(s.journal, journalEntry{action: jBal, addr: ts, oldVal: oldTo})
}

func (s *testState) CreateAccount(addr []byte) {
	s.journal = append(s.journal, journalEntry{action: jCreate, addr: string(addr)})
	s.balances[string(addr)] = big.NewInt(0)
}

func (s *testState) AddLog(addr []byte, topics [][]byte, data []byte) {
	s.logs = append(s.logs, logEntry{addr: addr, topics: topics, data: data})
}

func (s *testState) Snapshot() int {
	s.snapshots = append(s.snapshots, len(s.journal))
	return len(s.snapshots) - 1
}

func (s *testState) RevertToSnapshot(id int) {
	if id < 0 || id >= len(s.snapshots) {
		return
	}
	targetLen := s.snapshots[id]
	for len(s.journal) > targetLen {
		entry := s.journal[len(s.journal)-1]
		s.journal = s.journal[:len(s.journal)-1]
		switch entry.action {
		case jBal:
			s.balances[entry.addr] = entry.oldVal.(*big.Int)
		case jStor:
			m, ok := s.storage[entry.addr]
			if !ok {
				m = make(map[string][]byte)
				s.storage[entry.addr] = m
			}
			if entry.oldVal == nil {
				delete(m, entry.key)
			} else {
				m[entry.key] = entry.oldVal.([]byte)
			}
		case jCreate:
			delete(s.balances, entry.addr)
			delete(s.storage, entry.addr)
		}
	}
	s.snapshots = s.snapshots[:id]
}

func addr20(b byte) []byte {
	a := make([]byte, 20)
	a[19] = b
	return a
}

func pad32(v byte) []byte {
	b := make([]byte, 32)
	b[31] = v
	return b
}

func TestVMArithmetic(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x05, // PUSH1 5
		0x60, 0x03, // PUSH1 3
		0x01,       // ADD
		0x60, 0x00, // MSIZE : size for MSTORE (not MSIZE, just "push mstore offset")
		// After ADD: stack=[8], push 0 → stack=[8,0]
		0x52, // MSTORE: pops offset=0(top), value=8(second)
		0x60, 0x20, // size=32 (first push for RETURN)
		0x60, 0x00, // offset=0 (second push, ends up top)
		0xF3, // RETURN: pops offset=0(top), size=32(second)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("ADD failed: %v", err)
	}
	if len(out) != 32 || out[31] != 8 {
		t.Fatalf("ADD expected 8, got %x", out)
	}
}

func TestVMOutOfGas(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 2,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{0x60, 0x01, 0x60, 0x02, 0x01}
	_, _, err := exec.Execute(code)
	if err == nil {
		t.Fatal("expected out of gas")
	}
}

func TestVMMemoryExpansionGas(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x42,       // value
		0x61, 0xFF, 0xFF, // offset=65535
		0x52,             // MSTORE
		0x60, 0x00,       // size (for RETURN)
		0x60, 0x00,       // offset (for RETURN)
		0xF3,
	}
	_, _, err := exec.Execute(code)
	if err == nil {
		t.Fatal("expected out of gas from memory expansion")
	}
}

func TestVMSstoreGas(t *testing.T) {
	ctx := &EVMContext{
		Address:  addr20(0x01),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	// SSTORE: top=key, second=value → push value first, then key
	code := []byte{
		0x60, 0x2A, // value 42
		0x60, 0x01, // key 1
		0x55,       // SSTORE
		0x60, 0x01, // key 1
		0x54,       // SLOAD
		0x60, 0x00, // MSTORE offset
		0x52,       // MSTORE
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0, 32)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("SSTORE failed: %v", err)
	}
	if len(out) != 32 || out[31] != 0x2A {
		t.Fatalf("SSTORE: expected 42, got %x", out)
	}
}

func TestVMLogEmission(t *testing.T) {
	ctx := &EVMContext{
		Address:  addr20(0x01),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	// LOG0: push offset, then size (top=size, second=offset)
	code := []byte{
		0x60, 0x2A, 0x60, 0x00, 0x52, // MSTORE(0, 42)
		// for LOG: push offset first, then size
		0x60, 0x00, // offset
		0x60, 0x20, // size
		0xA0,       // LOG0
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0,32)
	}
	_, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("LOG0 failed: %v", err)
	}
	if len(state.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(state.logs))
	}
	if len(state.logs[0].topics) != 0 {
		t.Fatalf("LOG0 expected 0 topics")
	}
}

func TestVMLog4(t *testing.T) {
	ctx := &EVMContext{
		Address:  addr20(0x01),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	// LOG4: push topics[0..3], then offset, then size (top=size)
	code := []byte{
		0x60, 0x2A, 0x60, 0x00, 0x52, // MSTORE(0, 42)
		0x60, 0x01, // topic0
		0x60, 0x02, // topic1
		0x60, 0x03, // topic2
		0x60, 0x04, // topic3
		0x60, 0x00, // offset
		0x60, 0x20, // size
		0xA4,       // LOG4
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0,32)
	}
	_, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("LOG4 failed: %v", err)
	}
	if len(state.logs) != 1 || len(state.logs[0].topics) != 4 {
		t.Fatalf("LOG4 expected 1 log with 4 topics, got %d", len(state.logs))
	}
}

func TestVMCreateContractDetectsSuccess(t *testing.T) {
	caller := addr20(0x01)
	ctx := &EVMContext{
		Address:  caller,
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.balances[string(caller)] = big.NewInt(1000)

	exec := NewEVMExecutor(ctx, state)

	// CREATE with empty init code → succeeds, pushes contract address
	code := []byte{
		0x60, 0x00, // value = 0
		0x60, 0x00, // offset = 0
		0x60, 0x00, // size = 0
		0xF0, // CREATE
		0x60, 0x00, // MSTORE offset
		0x52, // MSTORE (pops offset, then addr value)
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0,32)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CREATE failed: %v", err)
	}
	if len(out) != 32 {
		t.Fatalf("CREATE expected 32-byte output, got %d", len(out))
	}
	contractAddr := out[12:] // last 20 bytes
	var zero [20]byte
	if string(contractAddr) == string(zero[:]) {
		t.Fatal("CREATE returned zero address")
	}
}

func TestVMCreateInsufficientBalance(t *testing.T) {
	caller := addr20(0x01)
	ctx := &EVMContext{
		Address:  caller,
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.balances[string(caller)] = big.NewInt(0)

	exec := NewEVMExecutor(ctx, state)

	// CREATE with value > balance → fails, pushes 0
	code := []byte{
		0x60, 0x64, // value = 100
		0x60, 0x00, // offset
		0x60, 0x00, // size
		0xF0, // CREATE
		0x60, 0x00, // MSTORE offset
		0x52, // MSTORE (pops offset 0, then pops 0)
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0,32)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CREATE with insufficient balance: %v", err)
	}
	if len(out) != 32 {
		t.Fatalf("expected 32-byte output from CREATE, got %d", len(out))
	}
}

func TestVMCallRevertSnapshot(t *testing.T) {
	callee := addr20(0x02)
	caller := addr20(0x01)

	ctx := &EVMContext{
		Address:  caller,
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.balances[string(caller)] = big.NewInt(1000)
	state.codes[string(callee)] = []byte{0xFD, 0x00, 0x00, 0x00, 0x00}

	exec := NewEVMExecutor(ctx, state)
	callerBalBefore := new(big.Int).Set(state.balances[string(caller)])

	// CALL: push gas, addr, value, argOff, argSz, retOff, retSz (top=retSz)
	code := []byte{
		0x60, 0x00, // retSz (1st, bottom)
		0x60, 0x00, // retOff
		0x60, 0x00, // argSz
		0x60, 0x00, // argOff
		0x60, 0x64, // value = 100
		0x60, 0x02, // addr
		0x60, 0x00, // gas (last, top)
		0xF1, // CALL
		0x60, 0x00, 0xF3, // RETURN(0,0)
	}
	_, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CALL to reverting contract: %v", err)
	}
	callerBalAfter := state.balances[string(caller)]
	if callerBalAfter.Cmp(callerBalBefore) != 0 {
		t.Fatalf("balance should be unchanged after reverted CALL: before=%s after=%s",
			callerBalBefore, callerBalAfter)
	}
}

func TestVMCallBalanceCheck(t *testing.T) {
	callee := addr20(0x02)

	ctx := &EVMContext{
		Address:  addr20(0x01),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.balances[string(addr20(0x01))] = big.NewInt(10)
	state.codes[string(callee)] = []byte{0x00}

	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x00, // retSz
		0x60, 0x00, // retOff
		0x60, 0x00, // argSz
		0x60, 0x00, // argOff
		0x60, 0x64, // value = 100 (more than balance)
		0x60, 0x02, // addr
		0x60, 0x00, // gas
		0xF1, // CALL
		0x60, 0x00, 0xF3, // RETURN(0,0)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CALL with insufficient balance: %v", err)
	}
	_ = out
}

func TestVMExtCodeOps(t *testing.T) {
	callee := addr20(0x02)

	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.codes[string(callee)] = []byte{0x60, 0x2A, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3}

	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x02, // address
		0x3B,       // EXTCODESIZE
		0x60, 0x00, 0x52, // MSTORE(0, size)
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0,32)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("EXTCODESIZE failed: %v", err)
	}
	if len(out) != 32 || out[31] != 10 {
		t.Fatalf("EXTCODESIZE expected 10, got %d", out[31])
	}

	exec2 := NewEVMExecutor(ctx, state)
	code2 := []byte{
		0x60, 0x02, // address
		0x3F, // EXTCODEHASH
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out2, _, err := exec2.Execute(code2)
	if err != nil {
		t.Fatalf("EXTCODEHASH failed: %v", err)
	}
	val := new(big.Int).SetBytes(out2)
	if val.Sign() == 0 {
		t.Fatal("EXTCODEHASH should return non-zero hash for existing code")
	}

	exec3 := NewEVMExecutor(ctx, state)
	code3 := []byte{
		0x60, 0x03, // non-existent address
		0x3F, // EXTCODEHASH
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out3, _, err := exec3.Execute(code3)
	if err != nil {
		t.Fatalf("EXTCODEHASH non-existent failed: %v", err)
	}
	val3 := new(big.Int).SetBytes(out3)
	if val3.Sign() != 0 {
		t.Fatal("EXTCODEHASH should return 0 for non-existent code")
	}
}

func TestVMSelfdestruct(t *testing.T) {
	addr := addr20(0x01)
	recipient := addr20(0x02)

	ctx := &EVMContext{
		Address:  addr,
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.balances[string(addr)] = big.NewInt(500)
	state.balances[string(recipient)] = big.NewInt(100)

	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x02, // recipient
		0xFF, // SELFDESTRUCT
	}
	_, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("SELFDESTRUCT failed: %v", err)
	}

	bal := state.GetBalance(recipient)
	if bal.Cmp(big.NewInt(600)) != 0 {
		t.Fatalf("recipient balance: expected 600, got %s", bal)
	}
}

func TestVMChainID(t *testing.T) {
	ctx := &EVMContext{
		ChainID:  big.NewInt(12345),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x46, // CHAINID
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("CHAINID failed: %v", err)
	}
	val := new(big.Int).SetBytes(out)
	if val.Cmp(big.NewInt(12345)) != 0 {
		t.Fatalf("CHAINID expected 12345, got %s", val)
	}
}

func TestVMPush0(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x5F,       // PUSH0
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("PUSH0 failed: %v", err)
	}
	val := new(big.Int).SetBytes(out)
	if val.Sign() != 0 {
		t.Fatalf("PUSH0 expected 0, got %s", val)
	}
}

func TestVMBlockEnvironment(t *testing.T) {
	ctx := &EVMContext{
		BlockNum:  42,
		Timestamp: 1000,
		GasPrice:  big.NewInt(0),
		GasLimit:  100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x43, // NUMBER
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("NUMBER failed: %v", err)
	}
	val := new(big.Int).SetBytes(out)
	if val.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("NUMBER expected 42, got %s", val)
	}
}

func TestVMStaticCallRejectsWrite(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)
	exec.staticCall = true

	code := []byte{
		0x60, 0x01,
		0x60, 0x02,
		0x55,
	}
	_, _, err := exec.Execute(code)
	if err == nil {
		t.Fatal("expected error for SSTORE in staticcall context")
	}
}

func TestVMMcopy(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x42, 0x60, 0x00, 0x52, // MSTORE(0, 0x42)
		0x60, 0x20, // size = 32
		0x60, 0x00, // src = 0
		0x60, 0x20, // dst = 32
		0x5E,       // MCOPY
		0x60, 0x20, 0x60, 0x20, 0x51, // MLOAD(32)
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("MCOPY failed: %v", err)
	}
	if len(out) != 32 || out[31] != 0x42 {
		t.Fatalf("MCOPY expected 0x42 at last byte, got %x", out)
	}
}

func TestVMRevertReturnsData(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x42, 0x60, 0x00, 0x52, // MSTORE(0, 0x42)
		0x60, 0x20, 0x60, 0x00, // size=32, offset=0
		0xFD, // REVERT
	}
	out, _, err := exec.Execute(code)
	if err == nil {
		t.Fatal("REVERT should return error")
	}
	if len(out) != 32 || out[31] != 0x42 {
		t.Fatalf("REVERT output: expected 0x42, got %x", out)
	}
}

func TestVMStackUnderflow(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	_, _, err := exec.Execute([]byte{0x01})
	if err == nil {
		t.Fatal("expected stack underflow")
	}
}

func TestVMInvalidOpcodes(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	_, _, err := exec.Execute([]byte{0x0C})
	if err == nil {
		t.Fatal("expected invalid opcode error")
	}
}

func TestVMGasOpcode(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 1000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x5A, // GAS
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("GAS failed: %v", err)
	}
	val := new(big.Int).SetBytes(out)
	if val.Sign() <= 0 {
		t.Fatal("GAS should return positive remaining gas")
	}
	if val.Cmp(new(big.Int).SetUint64(1000)) >= 0 {
		t.Fatal("GAS should return less than gas limit (some gas was consumed)")
	}
}

func TestVMBaseFee(t *testing.T) {
	ctx := &EVMContext{
		BaseFee:  big.NewInt(100),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x48, // BASEFEE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("BASEFEE failed: %v", err)
	}
	val := new(big.Int).SetBytes(out)
	if val.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("BASEFEE expected 100, got %s", val)
	}
}

func TestVMCoinbase(t *testing.T) {
	cb := addr20(0xAA)
	ctx := &EVMContext{
		Coinbase: cb,
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x41, // COINBASE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("COINBASE failed: %v", err)
	}
	if out[31] != 0xAA {
		t.Fatalf("COINBASE expected 0xAA, got %x", out[31])
	}
}

func TestVMSelfBalance(t *testing.T) {
	addr := addr20(0x01)
	ctx := &EVMContext{
		Address:  addr,
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.balances[string(addr)] = big.NewInt(999)

	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x47, // SELFBALANCE
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("SELFBALANCE failed: %v", err)
	}
	val := new(big.Int).SetBytes(out)
	if val.Cmp(big.NewInt(999)) != 0 {
		t.Fatalf("SELFBALANCE expected 999, got %s", val)
	}
}

func TestVMGasLimitOpcode(t *testing.T) {
	ctx := &EVMContext{
		BlockGasLimit: 30000000,
		GasPrice:      big.NewInt(0),
		GasLimit:      100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x45, // GASLIMIT
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("GASLIMIT failed: %v", err)
	}
	val := new(big.Int).SetBytes(out)
	if val.Cmp(big.NewInt(30000000)) != 0 {
		t.Fatalf("GASLIMIT expected 30000000, got %s", val)
	}
}

func TestVMBlockHash(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
		GetBlockHash: func(u uint64) []byte {
			if u == 1 {
				b := make([]byte, 32)
				b[31] = 0xAB
				return b
			}
			return nil
		},
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x01,
		0x40, // BLOCKHASH
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("BLOCKHASH failed: %v", err)
	}
	if out[31] != 0xAB {
		t.Fatalf("BLOCKHASH expected 0xAB, got %x", out[31])
	}
}

func TestVMReturnDataCopy(t *testing.T) {
	callee := addr20(0x02)

	ctx := &EVMContext{
		Address:  addr20(0x01),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.balances[string(addr20(0x01))] = big.NewInt(100)
	state.codes[string(callee)] = []byte{
		0x60, 0x42, 0x60, 0x00, 0x52, // MSTORE(0, 0x42)
		0x60, 0x20, 0x60, 0x00, 0xF3, // RETURN(0, 32)
	}

	exec := NewEVMExecutor(ctx, state)

	// CALL: gas, addr, value, argOff, argSz, retOff, retSz
	code := []byte{
		0x60, 0x00, // retSz (1st push → bottom)
		0x60, 0x00, // retOff
		0x60, 0x00, // argSz
		0x60, 0x00, // argOff
		0x60, 0x00, // value = 0
		0x60, 0x02, // addr
		0x60, 0xFF, // gas (last push → top)
		0xF1, // CALL
		0x50, // POP
		0x3D, // RETURNDATASIZE → stack = [32]
		0x60, 0x00, // srcOffset = 0 (push first for size to be 2nd from top)
		0x60, 0x00, // destOffset = 0 (push last → top)
		0x3E, // RETURNDATACOPY: pops destOffset=0(top), srcOffset=0, size=32(3rd)
		0x60, 0x20, // size (first push for RETURN)
		0x60, 0x00, // offset (last push → top)
		0xF3, // RETURN: top=offset(0), second=size(32)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("RETURNDATACOPY via CALL failed: %v", err)
	}
	if len(out) != 32 || out[31] != 0x42 {
		t.Fatalf("RETURNDATACOPY expected 0x42, got %x", out)
	}
}

func TestVMTstoreTload(t *testing.T) {
	ctx := &EVMContext{
		Address:  addr20(0x01),
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x2A, // value 42
		0x60, 0x01, // key 1
		0x5D,       // TSTORE
		0x60, 0x01, // key 1
		0x5C,       // TLOAD
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("TLOAD/TSTORE failed: %v", err)
	}
	if out[31] != 0x2A {
		t.Fatalf("TLOAD expected 0x2A, got %x", out[31])
	}
}

func TestVMSarNegative(t *testing.T) {
	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	neg2 := make([]byte, 33)
	neg2[0] = 0x7F
	for i := 1; i <= 32; i++ {
		neg2[i] = 0xFF
	}
	neg2[32] = 0xFE

	code := append(neg2, []byte{
		0x60, 0x01, // shift by 1
		0x1D, // SAR
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}...)
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("SAR negative failed: %v", err)
	}
	if out[31] != 0xFF {
		t.Fatalf("SAR(-2,1): expected 0xFF...FF (-1), got %x", out)
	}
}

func TestVMPrevRandao(t *testing.T) {
	ctx := &EVMContext{
		PrevRandao: pad32(0x77),
		GasPrice:   big.NewInt(0),
		GasLimit:   100000,
	}
	state := newTestState()
	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x44, // PREVRANDAO
		0x60, 0x00, 0x52,
		0x60, 0x20, 0x60, 0x00, 0xF3,
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("PREVRANDAO failed: %v", err)
	}
	if out[31] != 0x77 {
		t.Fatalf("PREVRANDAO expected 0x77, got %x", out[31])
	}
}

func TestVMCodecopyExternal(t *testing.T) {
	extAddr := addr20(0xAA)

	ctx := &EVMContext{
		GasPrice: big.NewInt(0),
		GasLimit: 100000,
	}
	state := newTestState()
	state.codes[string(extAddr)] = []byte{0xDE, 0xAD, 0xBE, 0xEF}

	exec := NewEVMExecutor(ctx, state)

	code := []byte{
		0x60, 0x04, // size
		0x60, 0x00, // code offset
		0x60, 0x00, // mem offset
		0x60, 0xAA, // address
		0x3C,       // EXTCODECOPY
		0x60, 0x04, 0x60, 0x00, 0xF3, // RETURN(0, 4)
	}
	out, _, err := exec.Execute(code)
	if err != nil {
		t.Fatalf("EXTCODECOPY failed: %v", err)
	}
	if len(out) != 4 || out[0] != 0xDE || out[1] != 0xAD || out[2] != 0xBE || out[3] != 0xEF {
		t.Fatalf("EXTCODECOPY expected DEADBEEF, got %x", out)
	}
}
