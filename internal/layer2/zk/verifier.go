package zk

import (
	"bytes"
	"fmt"
	"math/big"
)

// Deprecated: test-only simulated verifier. Use GnarkVerifier for production.
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

	if proof.ProofHash != nil {
		computedHash := computeProofHash(proof)
		if !bytes.Equal(computedHash, proof.ProofHash) {
			return fmt.Errorf("proof hash mismatch")
		}
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

	for i := 0; i < expectedLen; i++ {
		if proof.A[i] == nil {
			return fmt.Errorf("nil A element at index %d", i)
		}
		if v.circuit.Prime != nil && proof.A[i].Cmp(v.circuit.Prime) >= 0 {
			return fmt.Errorf("A[%d] out of field range", i)
		}
	}

	for i := 0; i < expectedLen; i++ {
		if proof.B[i] == nil {
			return fmt.Errorf("nil B element at index %d", i)
		}
		if v.circuit.Prime != nil && proof.B[i].Cmp(v.circuit.Prime) >= 0 {
			return fmt.Errorf("B[%d] out of field range", i)
		}
	}

	for i := 0; i < expectedLen; i++ {
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

	elementsOk := 0
	for i := 0; i < len(proof.A); i++ {
		if proof.A[i] != nil && proof.B[i] != nil && proof.C[i] != nil {
			ab := new(big.Int).Mul(proof.A[i], proof.B[i])
			ab.Mod(ab, v.circuit.Prime)

			if ab.Cmp(proof.C[i]) == 0 {
				elementsOk++
			}
		}
	}

	if elementsOk == 0 && len(proof.A) > 0 {
		return fmt.Errorf("no valid proof element relationships found")
	}

	return nil
}

func (v *Verifier) verifyPublicInputs(proof *Proof) error {
	for i, input := range proof.Public {
		if i >= len(v.vk.ICElements) {
			continue
		}

		if input == nil {
			return fmt.Errorf("public input %d is nil", i)
		}

		if v.circuit.Prime != nil && input.Cmp(v.circuit.Prime) >= 0 {
			return fmt.Errorf("public input %d out of field range", i)
		}

		if i < len(proof.A) {
			if input.Cmp(proof.A[i]) != 0 {
				return fmt.Errorf("public input %d does not match proof data", i)
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
	return computeProofHash(p)
}

func VerifyQuick(proof *Proof, vk *VerifyingKey, circuit *Circuit) error {
	verifier := NewVerifier(vk, circuit)
	return verifier.Verify(proof)
}
