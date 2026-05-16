package zk

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark/frontend"
)

type GenericGnarkCircuit struct {
	Public      []frontend.Variable `gnark:",public"`
	Secret      []frontend.Variable `gnark:",secret"`
	Constraints []Constraint
}

func NewGenericGnarkCircuit(circuit *Circuit) *GenericGnarkCircuit {
	public := make([]frontend.Variable, circuit.NumInputs)
	for i := range public {
		public[i] = 0
	}
	secret := make([]frontend.Variable, circuit.NumWitness)
	for i := range secret {
		secret[i] = 0
	}

	cons := make([]Constraint, len(circuit.Constraints))
	copy(cons, circuit.Constraints)

	return &GenericGnarkCircuit{
		Public:      public,
		Secret:      secret,
		Constraints: cons,
	}
}

func (c *GenericGnarkCircuit) Define(api frontend.API) error {
	allVars := append(c.Public, c.Secret...)

	for i, con := range c.Constraints {
		if err := c.applyConstraint(api, con, allVars); err != nil {
			return fmt.Errorf("constraint %d: %w", i, err)
		}
	}
	return nil
}

func (c *GenericGnarkCircuit) applyConstraint(api frontend.API, con Constraint, allVars []frontend.Variable) error {
	switch con.Type {
	case ConstraintTypeAdd:
		if con.Left >= len(allVars) || con.Right >= len(allVars) || con.Output >= len(allVars) {
			return fmt.Errorf("add: indices out of range")
		}
		res := api.Add(allVars[con.Left], allVars[con.Right])
		api.AssertIsEqual(res, allVars[con.Output])

	case ConstraintTypeMul:
		if con.Left >= len(allVars) || con.Right >= len(allVars) || con.Output >= len(allVars) {
			return fmt.Errorf("mul: indices out of range")
		}
		res := api.Mul(allVars[con.Left], allVars[con.Right])
		api.AssertIsEqual(res, allVars[con.Output])

	case ConstraintTypeEqual:
		if con.Left >= len(allVars) || con.Right >= len(allVars) {
			return fmt.Errorf("equal: indices out of range")
		}
		api.AssertIsEqual(allVars[con.Left], allVars[con.Right])

	case ConstraintTypeBool:
		if con.Left >= len(allVars) {
			return fmt.Errorf("bool: index out of range")
		}
		api.AssertIsBoolean(allVars[con.Left])

	case ConstraintTypeRange:
		if con.Left >= len(allVars) {
			return fmt.Errorf("range: index out of range")
		}
		checkRange(api, allVars[con.Left], con.Min, con.Max)

	default:
		return fmt.Errorf("unknown constraint type %d", con.Type)
	}
	return nil
}

func checkRange(api frontend.API, v frontend.Variable, min, max *big.Int) {
	if min != nil {
		api.AssertIsLessOrEqual(min, v)
	}
	if max != nil {
		api.AssertIsLessOrEqual(v, max)
	}
}
