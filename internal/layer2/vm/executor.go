package vm

// Executor defines the interface for smart contract execution.
type Executor interface {
	Execute(code []byte) ([]byte, uint64, error)
}
