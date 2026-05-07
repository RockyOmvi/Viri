package zk

import (
	"crypto/rand"
	"crypto/sha256"
	"io"
	"math/big"
	mathrand "math/rand"
)

type ProofSystem string

const (
	Groth16 ProofSystem = "groth16"
	Plonk   ProofSystem = "plonk"
	Halo2   ProofSystem = "halo2"
)

type Proof struct {
	A         []*big.Int
	B         []*big.Int
	C         []*big.Int
	CircuitID []byte
	Public    []*big.Int
	ProofHash []byte
	System    ProofSystem
}

type VerifyingKey struct {
	CircuitID  []byte
	AlphaG1    []byte
	BetaG2     []byte
	GammaG2    []byte
	DeltaG2    []byte
	ICElements []*big.Int
	System     ProofSystem
}

type ProvingKey struct {
	Data       []byte
	VK         *VerifyingKey
	CircuitID  []byte
	Alpha      []byte
	Beta       []byte
	Gamma      []byte
	Delta      []byte
	G1Elements [][]byte
	G2Elements [][]byte
}

type Prover struct {
	pk      *ProvingKey
	circuit *Circuit
}

func NewProver(pk *ProvingKey, circuit *Circuit) *Prover {
	return &Prover{
		pk:      pk,
		circuit: circuit,
	}
}

func (p *Prover) Prove(assignment *Assignment) (*Proof, error) {
	totalLen := p.circuit.NumInputs + p.circuit.NumWitness

	proof := &Proof{
		CircuitID: []byte(p.circuit.Name),
		A:         make([]*big.Int, totalLen),
		B:         make([]*big.Int, totalLen),
		C:         make([]*big.Int, totalLen),
		Public:    make([]*big.Int, p.circuit.NumInputs),
		System:    p.pk.VK.System,
	}

	for i := 0; i < p.circuit.NumInputs && i < len(assignment.Inputs); i++ {
		proof.Public[i] = new(big.Int).Set(assignment.Inputs[i])
	}

	for i := 0; i < totalLen; i++ {
		var val *big.Int
		if i < len(assignment.Inputs) {
			val = new(big.Int).Set(assignment.Inputs[i])
		} else if i-len(assignment.Inputs) < len(assignment.Witness) {
			val = new(big.Int).Set(assignment.Witness[i-len(assignment.Inputs)])
		} else {
			rng := mathrand.New(mathrand.NewSource(mathrand.Int63()))
			val = new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), 64))
		}

		if p.circuit.Prime != nil && val.Cmp(p.circuit.Prime) >= 0 {
			val.Mod(val, p.circuit.Prime)
		}

		proof.A[i] = new(big.Int).Set(val)
		proof.B[i] = new(big.Int).Set(val)
		proof.C[i] = new(big.Int).Mul(proof.A[i], proof.B[i])
		if p.circuit.Prime != nil {
			proof.C[i].Mod(proof.C[i], p.circuit.Prime)
		}
	}

	proof.ProofHash = computeProofHash(proof)
	return proof, nil
}

func computeProofHash(p *Proof) []byte {
	h := sha256.New()
	h.Write(p.CircuitID)
	for _, a := range p.A {
		if a != nil {
			h.Write(a.Bytes())
		}
	}
	for _, b := range p.B {
		if b != nil {
			h.Write(b.Bytes())
		}
	}
	for _, c := range p.C {
		if c != nil {
			h.Write(c.Bytes())
		}
	}
	for _, pub := range p.Public {
		if pub != nil {
			h.Write(pub.Bytes())
		}
	}
	return h.Sum(nil)
}

func (p *Proof) ComputeCommitment() []byte {
	h := sha256.New()
	for _, a := range p.A {
		if a != nil {
			h.Write(a.Bytes())
		}
	}
	return h.Sum(nil)
}

func GenerateTestAssignment(circuit *Circuit) *Assignment {
	rng := mathrand.New(mathrand.NewSource(mathrand.Int63()))
	inputs := make([]*big.Int, circuit.NumInputs)
	for i := 0; i < circuit.NumInputs; i++ {
		inputs[i] = new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), 64))
	}

	witness := make([]*big.Int, circuit.NumWitness)
	for i := 0; i < circuit.NumWitness; i++ {
		witness[i] = new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), 64))
	}

	return &Assignment{
		Inputs:  inputs,
		Witness: witness,
	}
}

func GenerateProvingKey(circuit *Circuit) *ProvingKey {
	rng := mathrand.New(mathrand.NewSource(mathrand.Int63()))
	totalLen := circuit.NumInputs + circuit.NumWitness
	icElements := make([]*big.Int, circuit.NumInputs)
	for i := range icElements {
		icElements[i] = new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), 64))
	}

	g1Elements := make([][]byte, totalLen)
	g2Elements := make([][]byte, totalLen)
	for i := range g1Elements {
		g1Elements[i] = make([]byte, 32)
		g2Elements[i] = make([]byte, 64)
	}

	alpha := make([]byte, 32)
	beta := make([]byte, 64)
	gamma := make([]byte, 64)
	delta := make([]byte, 64)

	return &ProvingKey{
		Data:       circuit.serializeConstraints(),
		CircuitID:  []byte(circuit.Name),
		Alpha:      alpha,
		Beta:       beta[:32],
		Gamma:      gamma[:32],
		Delta:      delta[:32],
		G1Elements: g1Elements,
		G2Elements: g2Elements,
		VK: &VerifyingKey{
			CircuitID:  []byte(circuit.Name),
			AlphaG1:    alpha,
			BetaG2:     beta,
			GammaG2:    gamma,
			DeltaG2:    delta,
			ICElements: icElements,
			System:     Groth16,
		},
	}
}

func GenerateVerifyingKey(pk *ProvingKey, circuit *Circuit) *VerifyingKey {
	return pk.VK
}

func (c *Circuit) serializeConstraints() []byte {
	data := make([]byte, 0)
	for _, con := range c.Constraints {
		data = append(data, byte(con.Type))
		data = append(data, byte(con.Left))
		data = append(data, byte(con.Right))
		data = append(data, byte(con.Output))
	}
	return data
}

func (c *Circuit) Serialize() ([]byte, error) {
	return c.serializeConstraints(), nil
}

func NewRandomReader() io.Reader {
	return rand.Reader
}
