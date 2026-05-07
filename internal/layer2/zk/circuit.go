package zk

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
)

type FieldType uint8

const (
	FieldTypePrime FieldType = iota
	FieldTypeBinary
)

type ConstraintType uint8

const (
	ConstraintTypeAdd ConstraintType = iota
	ConstraintTypeMul
	ConstraintTypeEqual
	ConstraintTypeBool
	ConstraintTypeRange
)

type Constraint struct {
	Type   ConstraintType
	Left   int
	Right  int
	Output int
	Value  *big.Int
	Min    *big.Int
	Max    *big.Int
}

type Circuit struct {
	Name        string
	NumInputs   int
	NumWitness  int
	Constraints []Constraint
	FieldType   FieldType
	Prime       *big.Int
}

type Assignment struct {
	Inputs  []*big.Int
	Witness []*big.Int
}

func NewCircuit(name string, numInputs, numWitness int, fieldType FieldType) *Circuit {
	circuit := &Circuit{
		Name:       name,
		NumInputs:  numInputs,
		NumWitness: numWitness,
		FieldType:  fieldType,
	}

	if fieldType == FieldTypePrime {
		circuit.Prime, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	}

	return circuit
}

func (c *Circuit) AddAddConstraint(left, right, output int) {
	c.Constraints = append(c.Constraints, Constraint{
		Type:   ConstraintTypeAdd,
		Left:   left,
		Right:  right,
		Output: output,
	})
}

func (c *Circuit) AddMulConstraint(left, right, output int) {
	c.Constraints = append(c.Constraints, Constraint{
		Type:   ConstraintTypeMul,
		Left:   left,
		Right:  right,
		Output: output,
	})
}

func (c *Circuit) AddEqualConstraint(left, right int) {
	c.Constraints = append(c.Constraints, Constraint{
		Type:  ConstraintTypeEqual,
		Left:  left,
		Right: right,
	})
}

func (c *Circuit) AddBoolConstraint(variable int) {
	c.Constraints = append(c.Constraints, Constraint{
		Type:   ConstraintTypeBool,
		Left:   variable,
		Output: variable,
	})
}

func (c *Circuit) AddRangeConstraint(variable int, min, max *big.Int) {
	c.Constraints = append(c.Constraints, Constraint{
		Type:  ConstraintTypeRange,
		Left:  variable,
		Min:   min,
		Max:   max,
	})
}

func (c *Circuit) Validate(assignment *Assignment) error {
	if len(assignment.Inputs) != c.NumInputs {
		return fmt.Errorf("expected %d inputs, got %d", c.NumInputs, len(assignment.Inputs))
	}
	if len(assignment.Witness) != c.NumWitness {
		return fmt.Errorf("expected %d witness variables, got %d", c.NumWitness, len(assignment.Witness))
	}

	allValues := make([]*big.Int, c.NumInputs+c.NumWitness)
	copy(allValues, assignment.Inputs)
	copy(allValues[c.NumInputs:], assignment.Witness)

	for i, constraint := range c.Constraints {
		if err := c.checkConstraint(constraint, allValues); err != nil {
			return fmt.Errorf("constraint %d failed: %w", i, err)
		}
	}

	return nil
}

func (c *Circuit) checkConstraint(constraint Constraint, values []*big.Int) error {
	switch constraint.Type {
	case ConstraintTypeAdd:
		left := values[constraint.Left]
		right := values[constraint.Right]
		output := values[constraint.Output]

		sum := new(big.Int).Add(left, right)
		if c.Prime != nil {
			sum.Mod(sum, c.Prime)
		}

		if sum.Cmp(output) != 0 {
			return fmt.Errorf("add constraint: %s + %s != %s", left, right, output)
		}

	case ConstraintTypeMul:
		left := values[constraint.Left]
		right := values[constraint.Right]
		output := values[constraint.Output]

		product := new(big.Int).Mul(left, right)
		if c.Prime != nil {
			product.Mod(product, c.Prime)
		}

		if product.Cmp(output) != 0 {
			return fmt.Errorf("mul constraint: %s * %s != %s", left, right, output)
		}

	case ConstraintTypeEqual:
		left := values[constraint.Left]
		right := values[constraint.Right]

		if left.Cmp(right) != 0 {
			return fmt.Errorf("equal constraint: %s != %s", left, right)
		}

	case ConstraintTypeBool:
		val := values[constraint.Left]

		if val.Cmp(big.NewInt(0)) != 0 && val.Cmp(big.NewInt(1)) != 0 {
			return fmt.Errorf("bool constraint: %s is not 0 or 1", val)
		}

	case ConstraintTypeRange:
		val := values[constraint.Left]

		if constraint.Min != nil && val.Cmp(constraint.Min) < 0 {
			return fmt.Errorf("range constraint: %s < min %s", val, constraint.Min)
		}
		if constraint.Max != nil && val.Cmp(constraint.Max) > 0 {
			return fmt.Errorf("range constraint: %s > max %s", val, constraint.Max)
		}
	}

	return nil
}

func (c *Circuit) ComputeCommitment(assignment *Assignment) []byte {
	h := sha256.New()

	for _, input := range assignment.Inputs {
		h.Write(input.Bytes())
	}

	for _, witness := range assignment.Witness {
		h.Write(witness.Bytes())
	}

	return h.Sum(nil)
}

func (c *Circuit) GenerateConstraintHash() []byte {
	h := sha256.New()

	h.Write([]byte(c.Name))
	h.Write([]byte{byte(c.NumInputs), byte(c.NumWitness)})

	for _, constraint := range c.Constraints {
		buf := make([]byte, 40)
		buf[0] = byte(constraint.Type)
		binary.LittleEndian.PutUint32(buf[1:5], uint32(constraint.Left))
		binary.LittleEndian.PutUint32(buf[5:9], uint32(constraint.Right))
		binary.LittleEndian.PutUint32(buf[9:13], uint32(constraint.Output))
		h.Write(buf)
	}

	return h.Sum(nil)
}

func NewShieldedTransferCircuit() *Circuit {
	circuit := NewCircuit("shielded_transfer", 3, 6, FieldTypePrime)

	circuit.AddMulConstraint(3, 4, 5)
	circuit.AddBoolConstraint(3)
	circuit.AddRangeConstraint(4, big.NewInt(0), new(big.Int).Sub(circuit.Prime, big.NewInt(1)))

	circuit.AddEqualConstraint(0, 6)
	circuit.AddAddConstraint(4, 7, 8)

	return circuit
}

func NewRangeProofCircuit(bits int) *Circuit {
	circuit := NewCircuit(fmt.Sprintf("range_%d", bits), 1, bits, FieldTypePrime)

	for i := 0; i < bits; i++ {
		circuit.AddBoolConstraint(1 + i)
	}

	circuit.AddEqualConstraint(0, 1)

	powerOfTwo := big.NewInt(1)
	for i := 1; i < bits; i++ {
		nextPower := new(big.Int).Lsh(powerOfTwo, 1)
		witnessIdx := 1 + i
		circuit.AddMulConstraint(witnessIdx, witnessIdx, witnessIdx)
		powerOfTwo = nextPower
	}

	maxVal := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	maxVal.Sub(maxVal, big.NewInt(1))
	circuit.AddRangeConstraint(0, big.NewInt(0), maxVal)

	return circuit
}
