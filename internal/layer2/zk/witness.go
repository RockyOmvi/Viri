package zk

import "math/big"

// Witness provides public and secret inputs to a circuit.
type Witness struct {
	Public []*big.Int
	Secret []*big.Int
}
