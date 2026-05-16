package zk

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
)

// AddCircuit defines the constraint X + Y == Z.
// X, Y are public inputs; Z is the secret witness.
type AddCircuit struct {
	X frontend.Variable `gnark:",public"`
	Y frontend.Variable `gnark:",public"`
	Z frontend.Variable `gnark:",secret"`
}

func (c *AddCircuit) Define(api frontend.API) error {
	res := api.Add(c.X, c.Y)
	api.AssertIsEqual(res, c.Z)
	return nil
}

// MulCircuit defines the constraint X * Y == Z.
// X, Y are public inputs; Z is the secret witness.
type MulCircuit struct {
	X frontend.Variable `gnark:",public"`
	Y frontend.Variable `gnark:",public"`
	Z frontend.Variable `gnark:",secret"`
}

func (c *MulCircuit) Define(api frontend.API) error {
	res := api.Mul(c.X, c.Y)
	api.AssertIsEqual(res, c.Z)
	return nil
}

// assignFn maps a *big.Int to a frontend.Variable.
func assignFn(n *big.Int) frontend.Variable {
	if n == nil {
		return 0
	}
	return frontend.Variable(new(big.Int).Set(n))
}

// scalarField is the BN254 scalar field used by gnark.
func scalarField() *big.Int {
	return ecc.BN254.ScalarField()
}
