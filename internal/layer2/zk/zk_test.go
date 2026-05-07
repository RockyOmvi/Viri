package zk

import (
	"math/big"
	"testing"
)

func TestNewCircuit(t *testing.T) {
	circuit := NewCircuit("test", 2, 3, FieldTypePrime)

	if circuit.Name != "test" {
		t.Errorf("expected name 'test', got %s", circuit.Name)
	}
	if circuit.NumInputs != 2 {
		t.Errorf("expected 2 inputs, got %d", circuit.NumInputs)
	}
	if circuit.NumWitness != 3 {
		t.Errorf("expected 3 witnesses, got %d", circuit.NumWitness)
	}
	if circuit.Prime == nil {
		t.Errorf("prime should not be nil for prime field")
	}
}

func TestCircuitAddConstraint(t *testing.T) {
	circuit := NewCircuit("test", 2, 3, FieldTypePrime)

	circuit.AddAddConstraint(0, 1, 2)
	circuit.AddMulConstraint(1, 2, 3)
	circuit.AddEqualConstraint(0, 1)
	circuit.AddBoolConstraint(0)
	circuit.AddRangeConstraint(0, big.NewInt(0), big.NewInt(100))

	if len(circuit.Constraints) != 5 {
		t.Errorf("expected 5 constraints, got %d", len(circuit.Constraints))
	}
}

func TestCircuitValidation(t *testing.T) {
	circuit := NewCircuit("add_test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	if err := circuit.Validate(assignment); err != nil {
		t.Errorf("valid assignment failed: %v", err)
	}

	invalidAssignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(9)},
	}

	if err := circuit.Validate(invalidAssignment); err == nil {
		t.Errorf("invalid assignment should have failed")
	}
}

func TestCircuitMulValidation(t *testing.T) {
	circuit := NewCircuit("mul_test", 2, 1, FieldTypePrime)
	circuit.AddMulConstraint(0, 1, 2)

	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(4)},
		Witness: []*big.Int{big.NewInt(12)},
	}

	if err := circuit.Validate(assignment); err != nil {
		t.Errorf("valid mul assignment failed: %v", err)
	}

	invalidAssignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(4)},
		Witness: []*big.Int{big.NewInt(13)},
	}

	if err := circuit.Validate(invalidAssignment); err == nil {
		t.Errorf("invalid mul assignment should have failed")
	}
}

func TestCircuitBoolValidation(t *testing.T) {
	circuit := NewCircuit("bool_test", 0, 1, FieldTypePrime)
	circuit.AddBoolConstraint(0)

	validZero := &Assignment{
		Inputs:  []*big.Int{},
		Witness: []*big.Int{big.NewInt(0)},
	}

	if err := circuit.Validate(validZero); err != nil {
		t.Errorf("bool zero failed: %v", err)
	}

	validOne := &Assignment{
		Inputs:  []*big.Int{},
		Witness: []*big.Int{big.NewInt(1)},
	}

	if err := circuit.Validate(validOne); err != nil {
		t.Errorf("bool one failed: %v", err)
	}

	invalid := &Assignment{
		Inputs:  []*big.Int{},
		Witness: []*big.Int{big.NewInt(2)},
	}

	if err := circuit.Validate(invalid); err == nil {
		t.Errorf("bool invalid should have failed")
	}
}

func TestCircuitRangeValidation(t *testing.T) {
	circuit := NewCircuit("range_test", 0, 1, FieldTypePrime)
	circuit.AddRangeConstraint(0, big.NewInt(10), big.NewInt(20))

	valid := &Assignment{
		Inputs:  []*big.Int{},
		Witness: []*big.Int{big.NewInt(15)},
	}

	if err := circuit.Validate(valid); err != nil {
		t.Errorf("valid range failed: %v", err)
	}

	below := &Assignment{
		Inputs:  []*big.Int{},
		Witness: []*big.Int{big.NewInt(5)},
	}

	if err := circuit.Validate(below); err == nil {
		t.Errorf("below range should have failed")
	}

	above := &Assignment{
		Inputs:  []*big.Int{},
		Witness: []*big.Int{big.NewInt(25)},
	}

	if err := circuit.Validate(above); err == nil {
		t.Errorf("above range should have failed")
	}
}

func TestCircuitValidationInputMismatch(t *testing.T) {
	circuit := NewCircuit("test", 2, 3, FieldTypePrime)

	tooFewInputs := &Assignment{
		Inputs:  []*big.Int{big.NewInt(1)},
		Witness: make([]*big.Int, 3),
	}

	if err := circuit.Validate(tooFewInputs); err == nil {
		t.Errorf("too few inputs should have failed")
	}

	tooManyInputs := &Assignment{
		Inputs:  []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
		Witness: make([]*big.Int, 3),
	}

	if err := circuit.Validate(tooManyInputs); err == nil {
		t.Errorf("too many inputs should have failed")
	}

	tooFewWitnesses := &Assignment{
		Inputs:  []*big.Int{big.NewInt(1), big.NewInt(2)},
		Witness: []*big.Int{big.NewInt(1)},
	}

	if err := circuit.Validate(tooFewWitnesses); err == nil {
		t.Errorf("too few witnesses should have failed")
	}
}

func TestComputeCommitment(t *testing.T) {
	circuit := NewCircuit("test", 2, 3, FieldTypePrime)

	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(1), big.NewInt(2)},
		Witness: []*big.Int{big.NewInt(3), big.NewInt(4), big.NewInt(5)},
	}

	commitment := circuit.ComputeCommitment(assignment)
	if len(commitment) == 0 {
		t.Errorf("commitment should not be empty")
	}

	commitment2 := circuit.ComputeCommitment(assignment)
	if string(commitment) != string(commitment2) {
		t.Errorf("commitments should be deterministic")
	}

	differentAssignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(1), big.NewInt(3)},
		Witness: []*big.Int{big.NewInt(3), big.NewInt(4), big.NewInt(5)},
	}

	differentCommitment := circuit.ComputeCommitment(differentAssignment)
	if string(commitment) == string(differentCommitment) {
		t.Errorf("different assignments should have different commitments")
	}
}

func TestGenerateConstraintHash(t *testing.T) {
	circuit := NewCircuit("test", 2, 3, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	hash := circuit.GenerateConstraintHash()
	if len(hash) == 0 {
		t.Errorf("constraint hash should not be empty")
	}

	hash2 := circuit.GenerateConstraintHash()
	if string(hash) != string(hash2) {
		t.Errorf("constraint hash should be deterministic")
	}

	circuit2 := NewCircuit("test2", 2, 3, FieldTypePrime)
	circuit2.AddAddConstraint(0, 1, 2)

	hash3 := circuit2.GenerateConstraintHash()
	if string(hash) == string(hash3) {
		t.Errorf("different circuits should have different hashes")
	}
}

func TestGenerateProvingKey(t *testing.T) {
	circuit := NewCircuit("test", 2, 3, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)

	if pk == nil {
		t.Fatalf("proving key should not be nil")
	}

	if len(pk.CircuitID) == 0 {
		t.Errorf("circuit ID should not be empty")
	}

	if pk.Alpha == nil || pk.Beta == nil || pk.Gamma == nil || pk.Delta == nil {
		t.Errorf("key parameters should not be nil")
	}

	if len(pk.G1Elements) != 5 {
		t.Errorf("expected 5 G1 elements, got %d", len(pk.G1Elements))
	}

	if len(pk.G2Elements) != 5 {
		t.Errorf("expected 5 G2 elements, got %d", len(pk.G2Elements))
	}
}

func TestGenerateVerifyingKey(t *testing.T) {
	circuit := NewCircuit("test", 2, 3, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	if vk == nil {
		t.Fatalf("verifying key should not be nil")
	}

	if string(vk.CircuitID) != string(pk.CircuitID) {
		t.Errorf("circuit IDs should match")
	}

	if vk.AlphaG1 == nil {
		t.Errorf("AlphaG1 should not be nil")
	}

	if len(vk.ICElements) != 2 {
		t.Errorf("expected 2 IC elements, got %d", len(vk.ICElements))
	}
}

func TestProveAndVerify(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	prover := NewProver(pk, circuit)
	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof generation failed: %v", err)
	}

	if len(proof.A) == 0 || len(proof.B) == 0 || len(proof.C) == 0 {
		t.Errorf("proof elements should not be empty")
	}

	verifier := NewVerifier(vk, circuit)
	if err := verifier.Verify(proof); err != nil {
		t.Errorf("valid proof verification failed: %v", err)
	}
}

func TestVerifyInvalidProof(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	pk := GenerateProvingKey(circuit)

	prover := NewProver(pk, circuit)
	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof generation failed: %v", err)
	}

	wrongCircuit := NewCircuit("wrong", 2, 1, FieldTypePrime)
	wrongCircuit.AddMulConstraint(0, 1, 2)

	wrongPk := GenerateProvingKey(wrongCircuit)
	wrongVk := GenerateVerifyingKey(wrongPk, wrongCircuit)

	verifier := NewVerifier(wrongVk, wrongCircuit)
	if err := verifier.Verify(proof); err == nil {
		t.Errorf("invalid circuit ID should fail verification")
	}
}

func TestVerifyBatchProofs(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	prover := NewProver(pk, circuit)
	verifier := NewVerifier(vk, circuit)

	assignments := []*Assignment{
		{Inputs: []*big.Int{big.NewInt(1), big.NewInt(2)}, Witness: []*big.Int{big.NewInt(3)}},
		{Inputs: []*big.Int{big.NewInt(3), big.NewInt(4)}, Witness: []*big.Int{big.NewInt(7)}},
		{Inputs: []*big.Int{big.NewInt(5), big.NewInt(6)}, Witness: []*big.Int{big.NewInt(11)}},
	}

	var proofs []*Proof
	for _, assignment := range assignments {
		proof, err := prover.Prove(assignment)
		if err != nil {
			t.Fatalf("proof generation failed: %v", err)
		}
		proofs = append(proofs, proof)
	}

	if err := verifier.VerifyBatch(proofs); err != nil {
		t.Errorf("batch verification failed: %v", err)
	}

	if err := verifier.VerifyBatch([]*Proof{}); err == nil {
		t.Errorf("empty batch should fail")
	}
}

func TestShieldedTransactionSerialize(t *testing.T) {
	tx := &ShieldedTransaction{
		Type:       ShieldedTxTypeDeposit,
		Commitment: []byte{0x01, 0x02, 0x03},
		Nullifier:  []byte{0x04, 0x05, 0x06},
		Amount:     1000,
		Nonce:      42,
	}

	data, err := tx.Serialize()
	if err != nil {
		t.Fatalf("serialization failed: %v", err)
	}

	var tx2 ShieldedTransaction
	if err := tx2.Deserialize(data); err != nil {
		t.Fatalf("deserialization failed: %v", err)
	}

	if tx2.Type != tx.Type {
		t.Errorf("type mismatch after serialization")
	}
	if tx2.Amount != tx.Amount {
		t.Errorf("amount mismatch after serialization")
	}
	if tx2.Nonce != tx.Nonce {
		t.Errorf("nonce mismatch after serialization")
	}
}

func TestShieldedTransactionComputeHash(t *testing.T) {
	tx := &ShieldedTransaction{
		Type:       ShieldedTxTypeDeposit,
		Commitment: []byte{0x01, 0x02, 0x03},
		Nullifier:  []byte{0x04, 0x05, 0x06},
		Amount:     1000,
		Nonce:      42,
	}

	hash := tx.ComputeHash()
	if len(hash) == 0 {
		t.Errorf("transaction hash should not be empty")
	}

	hash2 := tx.ComputeHash()
	if string(hash) != string(hash2) {
		t.Errorf("transaction hash should be deterministic")
	}

	tx2 := &ShieldedTransaction{
		Type:       ShieldedTxTypeWithdraw,
		Commitment: []byte{0x01, 0x02, 0x03},
		Nullifier:  []byte{0x04, 0x05, 0x06},
		Amount:     1000,
		Nonce:      42,
	}

	hash3 := tx2.ComputeHash()
	if string(hash) == string(hash3) {
		t.Errorf("different transaction types should have different hashes")
	}
}

func TestShieldedPoolProcessDeposit(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	pool := NewShieldedPool(circuit, vk)

	prover := NewProver(pk, circuit)
	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof generation failed: %v", err)
	}

	tx, err := pool.ProcessDeposit(1000, []byte("sender1"), proof)
	if err != nil {
		t.Fatalf("deposit processing failed: %v", err)
	}

	if tx.Type != ShieldedTxTypeDeposit {
		t.Errorf("transaction type should be deposit")
	}
	if tx.Amount != 1000 {
		t.Errorf("transaction amount should be 1000")
	}

	if pool.GetCommitmentCount() != 1 {
		t.Errorf("expected 1 commitment, got %d", pool.GetCommitmentCount())
	}
	if pool.GetNullifierCount() != 1 {
		t.Errorf("expected 1 nullifier, got %d", pool.GetNullifierCount())
	}
}

func TestShieldedPoolDuplicateCommitment(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	pool := NewShieldedPool(circuit, vk)

	prover := NewProver(pk, circuit)
	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof generation failed: %v", err)
	}

	_, err = pool.ProcessDeposit(1000, []byte("sender1"), proof)
	if err != nil {
		t.Fatalf("first deposit failed: %v", err)
	}

	_, err = pool.ProcessDeposit(1000, []byte("sender1"), proof)
	if err == nil {
		t.Errorf("duplicate commitment should fail")
	}
}

func TestShieldedPoolProcessWithdraw(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	pool := NewShieldedPool(circuit, vk)

	prover := NewProver(pk, circuit)
	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof generation failed: %v", err)
	}

	depositTx, err := pool.ProcessDeposit(1000, []byte("sender1"), proof)
	if err != nil {
		t.Fatalf("deposit failed: %v", err)
	}

	nullifier := depositTx.Nullifier

	tx, err := pool.ProcessWithdraw(500, []byte("receiver1"), nullifier, proof)
	if err != nil {
		t.Fatalf("withdraw processing failed: %v", err)
	}

	if tx.Type != ShieldedTxTypeWithdraw {
		t.Errorf("transaction type should be withdraw")
	}
	if tx.Amount != 500 {
		t.Errorf("transaction amount should be 500")
	}
}

func TestShieldedPoolProcessTransfer(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	pool := NewShieldedPool(circuit, vk)

	prover := NewProver(pk, circuit)
	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof generation failed: %v", err)
	}

	tx, err := pool.ProcessTransfer(100, []byte("sender1"), []byte("receiver1"), proof)
	if err != nil {
		t.Fatalf("transfer processing failed: %v", err)
	}

	if tx.Type != ShieldedTxTypeTransfer {
		t.Errorf("transaction type should be transfer")
	}
	if tx.Amount != 100 {
		t.Errorf("transaction amount should be 100")
	}

	if !pool.HasCommitment(tx.Commitment) {
		t.Errorf("commitment should be registered")
	}
	if !pool.HasNullifier(tx.Nullifier) {
		t.Errorf("nullifier should be registered")
	}
}

func TestShieldedPoolTransactionCount(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	pool := NewShieldedPool(circuit, vk)

	prover := NewProver(pk, circuit)

	proof1, err := prover.Prove(&Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	})
	if err != nil {
		t.Fatalf("proof 1 generation failed: %v", err)
	}

	proof2, err := prover.Prove(&Assignment{
		Inputs:  []*big.Int{big.NewInt(1), big.NewInt(2)},
		Witness: []*big.Int{big.NewInt(3)},
	})
	if err != nil {
		t.Fatalf("proof 2 generation failed: %v", err)
	}

	_, err = pool.ProcessDeposit(1000, []byte("sender1"), proof1)
	if err != nil {
		t.Fatalf("deposit failed: %v", err)
	}

	_, err = pool.ProcessTransfer(100, []byte("sender2"), []byte("receiver1"), proof2)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	if len(pool.GetTransactions()) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(pool.GetTransactions()))
	}
}

func TestNewShieldedTransferCircuit(t *testing.T) {
	circuit := NewShieldedTransferCircuit()

	if circuit == nil {
		t.Fatalf("shielded transfer circuit should not be nil")
	}

	if circuit.Name != "shielded_transfer" {
		t.Errorf("expected circuit name 'shielded_transfer', got %s", circuit.Name)
	}

	if len(circuit.Constraints) == 0 {
		t.Errorf("circuit should have constraints")
	}
}

func TestNewRangeProofCircuit(t *testing.T) {
	circuit := NewRangeProofCircuit(8)

	if circuit == nil {
		t.Fatalf("range proof circuit should not be nil")
	}

	if circuit.NumWitness != 8 {
		t.Errorf("expected 8 witness variables, got %d", circuit.NumWitness)
	}
}

func TestVerifyQuick(t *testing.T) {
	circuit := NewCircuit("test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	assignment := &Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
		Witness: []*big.Int{big.NewInt(8)},
	}

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)

	prover := NewProver(pk, circuit)
	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("proof generation failed: %v", err)
	}

	if err := VerifyQuick(proof, vk, circuit); err != nil {
		t.Errorf("quick verification failed: %v", err)
	}
}
