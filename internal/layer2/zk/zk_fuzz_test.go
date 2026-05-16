package zk

import (
	"math/big"
	"math/rand"
	"testing"
	"time"
)

func FuzzShieldedTransactionSerialize(f *testing.F) {
	f.Add(uint8(0), []byte("comm"), []byte("null"), []byte("pub"), uint64(100))
	f.Add(uint8(2), []byte{}, []byte{}, []byte{}, uint64(0))
	f.Add(uint8(1), make([]byte, 32), make([]byte, 32), make([]byte, 64), uint64(1<<60))

	f.Fuzz(func(t *testing.T, typ uint8, commitment, nullifier, publicData []byte, amount uint64) {
		tx := &ShieldedTransaction{
			Type:       ShieldedTxType(typ),
			Commitment: commitment,
			Nullifier:  nullifier,
			Proof:      &Proof{},
			PublicData: publicData,
			Amount:     amount,
			Timestamp:  time.Now(),
		}
		data, err := tx.Serialize()
		if err != nil {
			return
		}
		var deser ShieldedTransaction
		if err := deser.Deserialize(data); err != nil {
			t.Errorf("deserialize failed: %v", err)
			return
		}
		if string(deser.Commitment) != string(commitment) {
			t.Errorf("commitment mismatch")
		}
		if deser.Amount != amount {
			t.Errorf("amount mismatch: %d != %d", deser.Amount, amount)
		}
	})
}

func FuzzShieldedTransactionValidate(f *testing.F) {
	f.Add(uint8(0), []byte("comm"), []byte("null"), []byte("pub"), uint64(100))
	f.Add(uint8(1), []byte{}, []byte{}, []byte{}, uint64(0))

	f.Fuzz(func(t *testing.T, typ uint8, commitment, nullifier, publicData []byte, amount uint64) {
		tx := &ShieldedTransaction{
			Type:       ShieldedTxType(typ),
			Commitment: commitment,
			Nullifier:  nullifier,
			Proof:      &Proof{},
			PublicData: publicData,
			Amount:     amount,
		}
		err := tx.Validate()
		_ = err
		hash := tx.ComputeHash()
		if len(hash) == 0 {
			t.Errorf("hash should not be empty")
		}
	})
}

func FuzzCircuitValidateRandomAssignment(f *testing.F) {
	f.Add(3, 6)
	f.Add(1, 1)
	f.Add(0, 0)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 1 || numInputs > 10 || numWitness < 0 || numWitness > 10 {
			return
		}
		circuit := NewCircuit("fuzz", numInputs, numWitness, FieldTypePrime)
		circuit.AddAddConstraint(0, numInputs, numInputs)
		if numInputs+numWitness > 1 {
			circuit.AddEqualConstraint(0, 0)
		}

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		inputs := make([]*big.Int, numInputs)
		for i := range inputs {
			inputs[i] = big.NewInt(int64(rng.Intn(1000)))
		}
		witness := make([]*big.Int, numWitness)
		for i := range witness {
			witness[i] = big.NewInt(int64(rng.Intn(1000)))
		}

		assignment := &Assignment{Inputs: inputs, Witness: witness}
		err := circuit.Validate(assignment)
		_ = err
	})
}

func FuzzCircuitConstraintHash(f *testing.F) {
	f.Add("test", 3, 6)
	f.Add("empty", 0, 0)
	f.Add("large", 10, 10)

	f.Fuzz(func(t *testing.T, name string, numInputs, numWitness int) {
		if numInputs < 0 || numInputs > 20 || numWitness < 0 || numWitness > 20 {
			return
		}
		circuit := NewCircuit(name, numInputs, numWitness, FieldTypePrime)
		hash := circuit.GenerateConstraintHash()
		if len(hash) == 0 {
			t.Errorf("constraint hash should not be empty")
		}
		hash2 := circuit.GenerateConstraintHash()
		if string(hash) != string(hash2) {
			t.Errorf("constraint hash not deterministic")
		}
	})
}

func FuzzCircuitCommitment(f *testing.F) {
	f.Add(2, 3)
	f.Add(1, 0)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 0 || numInputs > 10 || numWitness < 0 || numWitness > 10 {
			return
		}
		circuit := NewCircuit("commit", numInputs, numWitness, FieldTypePrime)
		rng := rand.New(rand.NewSource(42))
		inputs := make([]*big.Int, numInputs)
		for i := range inputs {
			inputs[i] = big.NewInt(int64(rng.Intn(1000)))
		}
		witness := make([]*big.Int, numWitness)
		for i := range witness {
			witness[i] = big.NewInt(int64(rng.Intn(1000)))
		}
		assignment := &Assignment{Inputs: inputs, Witness: witness}
		comm := circuit.ComputeCommitment(assignment)
		if len(comm) == 0 {
			t.Errorf("commitment should not be empty")
		}
		comm2 := circuit.ComputeCommitment(assignment)
		if string(comm) != string(comm2) {
			t.Errorf("commitment not deterministic")
		}
	})
}

func FuzzProverProofGeneration(f *testing.F) {
	f.Add(2, 3)
	f.Add(1, 1)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 1 || numInputs > 5 || numWitness < 0 || numWitness > 5 {
			return
		}
		circuit := NewCircuit("prover_fuzz", numInputs, numWitness, FieldTypePrime)
		pk := GenerateProvingKey(circuit)
		prover := NewProver(pk, circuit)
		assignment := GenerateTestAssignment(circuit)
		proof, err := prover.Prove(assignment)
		if err != nil {
			t.Errorf("prove failed: %v", err)
			return
		}
		if proof == nil {
			t.Errorf("proof should not be nil")
			return
		}
		if len(proof.A) != numInputs+numWitness {
			t.Errorf("proof A length mismatch: %d != %d", len(proof.A), numInputs+numWitness)
		}
	})
}

func FuzzVerifierVerifyProof(f *testing.F) {
	f.Add(2, 3)
	f.Add(1, 1)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 1 || numInputs > 5 || numWitness < 0 || numWitness > 5 {
			return
		}
		circuit := NewCircuit("verifier_fuzz", numInputs, numWitness, FieldTypePrime)
		pk := GenerateProvingKey(circuit)
		vk := GenerateVerifyingKey(pk, circuit)
		verifier := NewVerifier(vk, circuit)
		prover := NewProver(pk, circuit)
		assignment := GenerateTestAssignment(circuit)
		proof, err := prover.Prove(assignment)
		if err != nil {
			t.Skip()
		}
		err = verifier.Verify(proof)
		_ = err
	})
}

func FuzzVerifierVerifyBatch(f *testing.F) {
	f.Add(2, 3, 1)
	f.Add(1, 1, 3)

	f.Fuzz(func(t *testing.T, numInputs, numWitness, batchSize int) {
		if numInputs < 1 || numInputs > 5 || numWitness < 0 || numWitness > 5 || batchSize < 1 || batchSize > 5 {
			return
		}
		circuit := NewCircuit("batch_fuzz", numInputs, numWitness, FieldTypePrime)
		pk := GenerateProvingKey(circuit)
		vk := GenerateVerifyingKey(pk, circuit)
		verifier := NewVerifier(vk, circuit)
		prover := NewProver(pk, circuit)
		var proofs []*Proof
		for i := 0; i < batchSize; i++ {
			assignment := GenerateTestAssignment(circuit)
			proof, err := prover.Prove(assignment)
			if err != nil {
				t.Skip()
			}
			proofs = append(proofs, proof)
		}
		err := verifier.VerifyBatch(proofs)
		_ = err
	})
}

func FuzzProofHashConsistency(f *testing.F) {
	f.Add(2, 3)
	f.Add(3, 4)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 1 || numInputs > 5 || numWitness < 0 || numWitness > 5 {
			return
		}
		circuit := NewCircuit("hash_fuzz", numInputs, numWitness, FieldTypePrime)
		pk := GenerateProvingKey(circuit)
		prover := NewProver(pk, circuit)
		assignment := GenerateTestAssignment(circuit)
		proof1, err := prover.Prove(assignment)
		if err != nil {
			t.Skip()
		}
		proof2, err := prover.Prove(assignment)
		if err != nil {
			t.Skip()
		}
		if string(proof1.ProofHash) != string(proof2.ProofHash) {
			t.Errorf("proof hash not deterministic for same assignment")
		}
	})
}

func FuzzNewRangeProofCircuit(f *testing.F) {
	f.Add(8)
	f.Add(1)
	f.Add(0)
	f.Add(256)

	f.Fuzz(func(t *testing.T, bits int) {
		circuit := NewRangeProofCircuit(bits)
		if circuit == nil {
			t.Errorf("range circuit should not be nil")
			return
		}
		if bits < 1 {
			bits = 1
		}
		if circuit.NumInputs != 1 {
			t.Errorf("range circuit should have 1 input")
		}
	})
}

func FuzzNewShieldedTransferCircuit(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(100)

	f.Fuzz(func(t *testing.T, _ int) {
		circuit := NewShieldedTransferCircuit()
		if circuit == nil {
			t.Errorf("shielded transfer circuit should not be nil")
			return
		}
		if circuit.NumInputs != 3 {
			t.Errorf("shielded transfer circuit should have 3 inputs, got %d", circuit.NumInputs)
		}
	})
}

func FuzzProofComputeCommitment(f *testing.F) {
	f.Add(2, 3)
	f.Add(1, 2)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 1 || numInputs > 5 || numWitness < 0 || numWitness > 5 {
			return
		}
		circuit := NewCircuit("comm_fuzz", numInputs, numWitness, FieldTypePrime)
		pk := GenerateProvingKey(circuit)
		prover := NewProver(pk, circuit)
		assignment := GenerateTestAssignment(circuit)
		proof, err := prover.Prove(assignment)
		if err != nil {
			t.Skip()
		}
		comm := proof.ComputeCommitment()
		if len(comm) == 0 {
			t.Errorf("proof commitment should not be empty")
		}
	})
}

func FuzzGenerateProvingKey(f *testing.F) {
	f.Add(3, 6)
	f.Add(1, 0)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 0 || numInputs > 10 || numWitness < 0 || numWitness > 10 {
			return
		}
		if numInputs == 0 {
			return
		}
		circuit := NewCircuit("pk_fuzz", numInputs, numWitness, FieldTypePrime)
		pk := GenerateProvingKey(circuit)
		if pk == nil {
			t.Errorf("proving key should not be nil")
			return
		}
		if pk.VK == nil {
			t.Errorf("verifying key should not be nil")
		}
		if string(pk.CircuitID) != circuit.Name {
			t.Errorf("circuit ID mismatch")
		}
	})
}

func FuzzQuickVerify(f *testing.F) {
	f.Add(2, 3)
	f.Add(1, 2)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 1 || numInputs > 5 || numWitness < 0 || numWitness > 5 {
			return
		}
		circuit := NewCircuit("quick_fuzz", numInputs, numWitness, FieldTypePrime)
		pk := GenerateProvingKey(circuit)
		vk := GenerateVerifyingKey(pk, circuit)
		prover := NewProver(pk, circuit)
		assignment := GenerateTestAssignment(circuit)
		proof, err := prover.Prove(assignment)
		if err != nil {
			t.Skip()
		}
		err = VerifyQuick(proof, vk, circuit)
		_ = err
	})
}

func FuzzCircuitSerialize(f *testing.F) {
	f.Add(3, 6)
	f.Add(1, 1)

	f.Fuzz(func(t *testing.T, numInputs, numWitness int) {
		if numInputs < 0 || numInputs > 10 || numWitness < 0 || numWitness > 10 {
			return
		}
		circuit := NewCircuit("serialize", numInputs, numWitness, FieldTypePrime)
		data, err := circuit.Serialize()
		if err != nil {
			t.Errorf("serialize failed: %v", err)
			return
		}
		if data == nil {
			t.Errorf("serialized data should not be nil")
		}
	})
}
