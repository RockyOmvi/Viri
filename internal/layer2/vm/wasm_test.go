package vm

import (
	"testing"
)

func TestWasmVMConstants(t *testing.T) {
	vm := NewWasmVM(100000)

	if vm.GasUsed() != 0 {
		t.Error("new VM should have 0 gas used")
	}

	if vm.GasUsed() > vm.gasLimit {
		t.Error("gas used should not exceed limit on init")
	}
}

func TestWasmVMMemory(t *testing.T) {
	vm := NewWasmVM(100000)

	data := []byte{0x01, 0x02, 0x03, 0x04}
	if err := vm.SetMemory(0, data); err != nil {
		t.Fatal(err)
	}

	got, err := vm.GetMemory(0, 4)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(data) {
		t.Errorf("expected %v, got %v", data, got)
	}
}

func TestWasmVMMemoryOutOfBounds(t *testing.T) {
	vm := NewWasmVM(100000)

	data := make([]byte, 64*1024+1)
	if err := vm.SetMemory(0, data); err == nil {
		t.Error("expected out of bounds error")
	}
}

func TestWasmVMReset(t *testing.T) {
	vm := NewWasmVM(100000)

	vm.PushI32(42)
	vm.PushI64(100)

	if len(vm.stack) != 2 {
		t.Errorf("expected 2 items on stack, got %d", len(vm.stack))
	}

	vm.Reset()

	if len(vm.stack) != 0 {
		t.Error("stack should be empty after reset")
	}

	if vm.GasUsed() != 0 {
		t.Error("gas used should be 0 after reset")
	}
}

func TestWasmVMStackOperations(t *testing.T) {
	vm := NewWasmVM(100000)

	vm.PushI32(100)
	vm.PushI64(200)

	val32, err := vm.PopI32()
	if err != nil {
		t.Fatal(err)
	}
	if val32 != 200 {
		t.Errorf("expected 200, got %d", val32)
	}

	val64, err := vm.PopI64()
	if err != nil {
		t.Fatal(err)
	}
	if val64 != 100 {
		t.Errorf("expected 100, got %d", val64)
	}
}

func TestWasmVMEmptyStackPop(t *testing.T) {
	vm := NewWasmVM(100000)

	if _, err := vm.PopI32(); err == nil {
		t.Error("expected error on empty stack pop")
	}

	if _, err := vm.PopI64(); err == nil {
		t.Error("expected error on empty stack pop")
	}
}

func TestWasmVMEncodingHelpers(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []byte
	}{
		{"u32", U32(0x12345678), []byte{0x78, 0x56, 0x34, 0x12}},
		{"u64", U64(0x123456789ABCDEF0), []byte{0xF0, 0xDE, 0xBC, 0x9A, 0x78, 0x56, 0x34, 0x12}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.data) != string(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, tt.data)
			}
		})
	}
}

func TestWasmVMInvalidBinary(t *testing.T) {
	vm := NewWasmVM(100000)

	result := vm.Execute([]byte{0x00}, nil)
	if result.Err == nil {
		t.Error("expected error for invalid wasm binary")
	}

	result = vm.Execute([]byte{0x01, 0x02, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}, nil)
	if result.Err == nil {
		t.Error("expected error for invalid wasm version")
	}
}

func TestWasmVMGasLimit(t *testing.T) {
	vm := NewWasmVM(5)

	if err := vm.consumeGas(3); err != nil {
		t.Errorf("should not fail with 3 gas: %v", err)
	}

	if err := vm.consumeGas(3); err == nil {
		t.Error("should fail when exceeding gas limit")
	}
}

func TestWasmVMRegisterImport(t *testing.T) {
	vm := NewWasmVM(100000)

	callCount := 0
	vm.RegisterImport("func_0", func(vm *WasmVM, args []int64) ([]int64, error) {
		callCount++
		return []int64{args[0] * 2}, nil
	})

	if _, exists := vm.imports["func_0"]; !exists {
		t.Error("import should be registered")
	}
}
