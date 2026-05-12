package vm

// Executor defines the interface for smart contract execution.
// Both EVM and WASM executors implement this interface.
type Executor interface {
	Execute(code []byte) ([]byte, uint64, error)
}

// WasmAdapter wraps WasmVM to implement the Executor interface.
type WasmAdapter struct {
	vm *WasmVM
}

// NewWasmAdapter creates an adapter that exposes a WasmVM as an Executor.
func NewWasmAdapter(gasLimit uint64) *WasmAdapter {
	return &WasmAdapter{vm: NewWasmVM(gasLimit)}
}

// Execute runs the code with no arguments and returns the result.
func (a *WasmAdapter) Execute(code []byte) ([]byte, uint64, error) {
	result := a.vm.Execute(code, nil)
	if result.Err != nil {
		return nil, result.GasUsed, result.Err
	}
	return result.ReturnData, result.GasUsed, nil
}

// VM returns the underlying WasmVM for configuration (e.g. registering imports).
func (a *WasmAdapter) VM() *WasmVM {
	return a.vm
}
