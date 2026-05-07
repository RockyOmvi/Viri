package zk

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
)

type Verifier struct {
	vk      *VerifyingKey
	circuit *Circuit
}

func NewVerifier(vk *VerifyingKey, circuit *Circuit) *Verifier {
	return &Verifier{
		vk:      vk,
		circuit: circuit,
	}
}

func (v *Verifier) Verify(proof *Proof) error {
	if !bytes.Equal(proof.CircuitID, v.vk.CircuitID) {
		return fmt.Errorf("circuit ID mismatch")
	}

	if len(proof.Public) != v.circuit.NumInputs {
		return fmt.Errorf("expected %d public inputs, got %d", v.circuit.NumInputs, len(proof.Public))
	}

	if err := v.verifyProofStructure(proof); err != nil {
		return fmt.Errorf("invalid proof structure: %w", err)
	}

	if err := v.verifyPairing(proof); err != nil {
		return fmt.Errorf("pairing check failed: %w", err)
	}

	if err := v.verifyPublicInputs(proof); err != nil {
		return fmt.Errorf("public input verification failed: %w", err)
	}

	computedHash := proof.computeHash()
	if !bytes.Equal(computedHash, proof.ProofHash) {
		return fmt.Errorf("proof hash mismatch")
	}

	return nil
}

func (v *Verifier) verifyProofStructure(proof *Proof) error {
	expectedLen := v.circuit.NumInputs + v.circuit.NumWitness

	if len(proof.A) != expectedLen {
		return fmt.Errorf("invalid A length: expected %d, got %d", expectedLen, len(proof.A))
	}
	if len(proof.B) != expectedLen {
		return fmt.Errorf("invalid B length: expected %d, got %d", expectedLen, len(proof.B))
	}
	if len(proof.C) != expectedLen {
		return fmt.Errorf("invalid C length: expected %d, got %d", expectedLen, len(proof.C))
	}

	for i := 0; i < len(proof.A); i++ {
		if proof.A[i] == nil {
			return fmt.Errorf("nil A element at index %d", i)
		}
		if v.circuit.Prime != nil && proof.A[i].Cmp(v.circuit.Prime) >= 0 {
			return fmt.Errorf("A[%d] out of field range", i)
		}
	}

	for i := 0; i < len(proof.B); i++ {
		if proof.B[i] == nil {
			return fmt.Errorf("nil B element at index %d", i)
		}
		if v.circuit.Prime != nil && proof.B[i].Cmp(v.circuit.Prime) >= 0 {
			return fmt.Errorf("B[%d] out of field range", i)
		}
	}

	for i := 0; i < len(proof.C); i++ {
		if proof.C[i] == nil {
			return fmt.Errorf("nil C element at index %d", i)
		}
		if v.circuit.Prime != nil && proof.C[i].Cmp(v.circuit.Prime) >= 0 {
			return fmt.Errorf("C[%d] out of field range", i)
		}
	}

	return nil
}

func (v *Verifier) verifyPairing(proof *Proof) error {
	if v.circuit.Prime == nil {
		return nil
	}

	proofElements := new(big.Int).SetUint64(0)
	for i := 0; i < len(proof.A) && i < len(proof.B) && i < len(proof.C); i++ {
		if proof.A[i] != nil && proof.B[i] != nil && proof.C[i] != nil {
			ab := new(big.Int).Mul(proof.A[i], proof.B[i])
			ab.Mod(ab, v.circuit.Prime)

			c := new(big.Int).Set(proof.C[i])

			if ab.Cmp(c) == 0 {
				proofElements.Add(proofElements, big.NewInt(1))
			}
		}
	}

	if proofElements.Cmp(big.NewInt(0)) == 0 && len(proof.A) > 0 {
		return fmt.Errorf("no valid proof element relationships found")
	}

	return nil
}

func (v *Verifier) verifyPublicInputs(proof *Proof) error {
	for i, input := range proof.Public {
		if i >= len(v.vk.ICElements) {
			continue
		}

		ic := v.vk.ICElements[i]

		if v.circuit.Prime != nil {
			inputCopy := new(big.Int).Set(input)
			icCopy := new(big.Int).Set(ic)

			if inputCopy.Cmp(v.circuit.Prime) >= 0 {
				return fmt.Errorf("public input %d out of field range", i)
			}

			product := new(big.Int).Mul(inputCopy, icCopy)
			product.Mod(product, v.circuit.Prime)

			if product.Cmp(big.NewInt(0)) == 0 && inputCopy.Cmp(big.NewInt(0)) != 0 && icCopy.Cmp(big.NewInt(0)) != 0 {
				return fmt.Errorf("public input %d failed consistency check", i)
			}
		}
	}

	return nil
}

func (v *Verifier) VerifyBatch(proofs []*Proof) error {
	if len(proofs) == 0 {
		return fmt.Errorf("no proofs to verify")
	}

	for i, proof := range proofs {
		if err := v.Verify(proof); err != nil {
			return fmt.Errorf("proof %d verification failed: %w", i, err)
		}
	}

	return nil
}

func (p *Proof) computeHash() []byte {
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

func VerifyQuick(proof *Proof, vk *VerifyingKey, circuit *Circuit) error {
	verifier := NewVerifier(vk, circuit)
	return verifier.Verify(proof)
}
