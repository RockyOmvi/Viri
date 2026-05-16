package zk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

var (
	schemesMu sync.RWMutex
	schemes   = make(map[string]*groth16Scheme)
)

type groth16Scheme struct {
	ccs        constraint.ConstraintSystem
	pk         groth16.ProvingKey
	vk         groth16.VerifyingKey
	circuitKey string
}

func schemeCacheKey(circuit *Circuit) string {
	return string(circuit.GenerateConstraintHash())
}

func getOrCreateScheme(circuit *Circuit) (*groth16Scheme, error) {
	if circuit == nil {
		return nil, fmt.Errorf("circuit is nil")
	}
	if circuit.NumInputs < 0 || circuit.NumWitness < 0 {
		return nil, fmt.Errorf("invalid circuit: NumInputs=%d, NumWitness=%d", circuit.NumInputs, circuit.NumWitness)
	}
	if circuit.NumInputs+circuit.NumWitness == 0 {
		return nil, fmt.Errorf("circuit must have at least one variable")
	}

	key := schemeCacheKey(circuit)
	schemesMu.RLock()
	sch, ok := schemes[key]
	schemesMu.RUnlock()
	if ok {
		return sch, nil
	}

	schemesMu.Lock()
	defer schemesMu.Unlock()
	if sch, ok := schemes[key]; ok {
		return sch, nil
	}

	gnarkCircuit := NewGenericGnarkCircuit(circuit)
	ccs, err := frontend.Compile(scalarField(), r1cs.NewBuilder, gnarkCircuit)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}

	sch = &groth16Scheme{ccs: ccs, pk: pk, vk: vk, circuitKey: key}
	schemes[key] = sch
	return sch, nil
}

// GnarkProver generates real Groth16 zk-SNARK proofs using gnark.
type GnarkProver struct {
	mu sync.Mutex
}

// NewGnarkProver creates a prover that generates real Groth16 proofs.
func NewGnarkProver() *GnarkProver {
	return &GnarkProver{}
}

// Prove generates a real Groth16 proof for the given circuit and witness.
func (gp *GnarkProver) Prove(circuit *Circuit, witness *Witness) (*Proof, error) {
	if circuit == nil {
		return nil, fmt.Errorf("circuit is nil")
	}
	if witness == nil {
		return nil, fmt.Errorf("witness is nil")
	}
	if len(witness.Public) < circuit.NumInputs {
		return nil, fmt.Errorf("witness has %d public inputs, circuit needs %d", len(witness.Public), circuit.NumInputs)
	}
	if len(witness.Secret) < circuit.NumWitness {
		return nil, fmt.Errorf("witness has %d secret inputs, circuit needs %d", len(witness.Secret), circuit.NumWitness)
	}

	gp.mu.Lock()
	defer gp.mu.Unlock()

	scheme, err := getOrCreateScheme(circuit)
	if err != nil {
		return nil, fmt.Errorf("scheme: %w", err)
	}

	gnarkAssignment := buildAssignment(circuit, witness)
	if gnarkAssignment == nil {
		return nil, fmt.Errorf("failed to build gnark assignment")
	}

	fullWitness, err := frontend.NewWitness(gnarkAssignment, scalarField())
	if err != nil {
		return nil, fmt.Errorf("new witness: %w", err)
	}

	proof, err := groth16.Prove(scheme.ccs, scheme.pk, fullWitness)
	if err != nil {
		return nil, fmt.Errorf("prove: %w", err)
	}

	var buf bytes.Buffer
	if _, err := proof.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write proof: %w", err)
	}

	publicValues := make([]*big.Int, len(witness.Public))
	for i, p := range witness.Public {
		if p == nil {
			publicValues[i] = new(big.Int)
		} else {
			publicValues[i] = new(big.Int).Set(p)
		}
	}

	return &Proof{
		CircuitID: []byte(circuit.Name),
		Public:    publicValues,
		System:    Groth16,
		Raw:       buf.Bytes(),
	}, nil
}

// GnarkVerifier verifies real Groth16 zk-SNARK proofs using gnark.
type GnarkVerifier struct {
	mu sync.Mutex
}

// NewGnarkVerifier creates a verifier that checks real Groth16 proofs.
func NewGnarkVerifier() *GnarkVerifier {
	return &GnarkVerifier{}
}

// Verify checks a real Groth16 proof against a circuit and public witness.
func (gv *GnarkVerifier) Verify(proof *Proof, circuit *Circuit, publicWitness *Witness) error {
	if proof == nil {
		return fmt.Errorf("proof is nil")
	}
	if circuit == nil {
		return fmt.Errorf("circuit is nil")
	}
	if publicWitness == nil {
		return fmt.Errorf("public witness is nil")
	}
	if proof.Raw == nil {
		return fmt.Errorf("proof missing serialized raw data")
	}
	if len(proof.Raw) == 0 {
		return fmt.Errorf("proof raw data is empty")
	}

	gv.mu.Lock()
	defer gv.mu.Unlock()

	scheme, err := getOrCreateScheme(circuit)
	if err != nil {
		return fmt.Errorf("scheme: %w", err)
	}

	groth16Proof := groth16.NewProof(ecc.BN254)
	if _, err := groth16Proof.ReadFrom(bytes.NewReader(proof.Raw)); err != nil {
		return fmt.Errorf("read proof: %w", err)
	}

	if len(publicWitness.Public) < circuit.NumInputs {
		return fmt.Errorf("public witness has %d inputs, circuit needs %d", len(publicWitness.Public), circuit.NumInputs)
	}

	pubAssignment := buildPublicAssignment(circuit, publicWitness)
	if pubAssignment == nil {
		return fmt.Errorf("failed to build public assignment")
	}

	pubWitness, err := frontend.NewWitness(pubAssignment, scalarField(), frontend.PublicOnly())
	if err != nil {
		return fmt.Errorf("new public witness: %w", err)
	}

	if err := groth16.Verify(groth16Proof, scheme.vk, pubWitness); err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	return nil
}

func buildAssignment(circuit *Circuit, w *Witness) frontend.Circuit {
	if w == nil {
		return nil
	}
	public := make([]frontend.Variable, circuit.NumInputs)
	secret := make([]frontend.Variable, circuit.NumWitness)

	for i := 0; i < circuit.NumInputs && i < len(w.Public); i++ {
		public[i] = assignFn(w.Public[i])
	}
	for i := 0; i < circuit.NumWitness && i < len(w.Secret); i++ {
		secret[i] = assignFn(w.Secret[i])
	}

	return &GenericGnarkCircuit{
		Public: public,
		Secret: secret,
	}
}

func buildPublicAssignment(circuit *Circuit, w *Witness) frontend.Circuit {
	if w == nil {
		return nil
	}
	public := make([]frontend.Variable, circuit.NumInputs)
	for i := 0; i < circuit.NumInputs && i < len(w.Public); i++ {
		public[i] = assignFn(w.Public[i])
	}
	secret := make([]frontend.Variable, circuit.NumWitness)
	return &GenericGnarkCircuit{
		Public: public,
		Secret: secret,
	}
}

// GnarkProofSize returns estimated serialized groth16 proof size on BN254.
const GnarkProofSize = 256

// SerializeProofForTx encodes a proof and its public inputs into transaction data.
func SerializeProofForTx(proof *Proof, publicInputs []*big.Int) ([]byte, error) {
	if proof == nil {
		return nil, fmt.Errorf("proof is nil")
	}
	if proof.Raw == nil {
		return nil, fmt.Errorf("proof missing raw data")
	}
	if len(publicInputs) == 0 {
		return nil, fmt.Errorf("public inputs required")
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(proof.Raw))); err != nil {
		return nil, err
	}
	buf.Write(proof.Raw)
	for _, pi := range publicInputs {
		if pi == nil {
			padded := make([]byte, 32)
			buf.Write(padded)
			continue
		}
		padded := make([]byte, 32)
		pi.FillBytes(padded)
		buf.Write(padded)
	}
	return buf.Bytes(), nil
}

// DeserializeProofFromTx reconstructs a Proof and public witness from transaction data.
func DeserializeProofFromTx(txData []byte, circuit *Circuit) (*Proof, *Witness, error) {
	if circuit == nil {
		return nil, nil, fmt.Errorf("circuit is nil")
	}
	if len(txData) < 4 {
		return nil, nil, fmt.Errorf("data too short for proof length header")
	}
	proofLen := binary.BigEndian.Uint32(txData[:4])
	if proofLen == 0 {
		return nil, nil, fmt.Errorf("proof length is zero")
	}
	end := int(4 + proofLen)
	if end > len(txData) {
		return nil, nil, fmt.Errorf("data too short for proof body: need %d, have %d", end, len(txData))
	}
	numPublic := circuit.NumInputs
	if numPublic < 1 {
		return nil, nil, fmt.Errorf("circuit must have at least 1 public input")
	}
	publicOffset := int(4 + proofLen)
	neededSize := numPublic * 32
	if publicOffset+neededSize > len(txData) {
		return nil, nil, fmt.Errorf("data too short for public inputs: need %d, have %d", publicOffset+neededSize, len(txData))
	}
	publicBytes := txData[publicOffset:]
	publicInputs := make([]*big.Int, numPublic)
	for i := 0; i < numPublic; i++ {
		start := i * 32
		publicInputs[i] = new(big.Int).SetBytes(publicBytes[start : start+32])
	}
	return &Proof{
		Raw:       txData[4 : 4+proofLen],
		CircuitID: []byte(circuit.Name),
		Public:    publicInputs,
		System:    Groth16,
	}, &Witness{Public: publicInputs, Secret: []*big.Int{}}, nil
}
