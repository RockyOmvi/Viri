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
	ccs  constraint.ConstraintSystem
	pk   groth16.ProvingKey
	vk   groth16.VerifyingKey
	name string
}

func getOrCreateScheme(circuit *Circuit) (*groth16Scheme, error) {
	circuitType := detectGnarkCircuit(circuit)
	schemesMu.RLock()
	sch, ok := schemes[circuitType]
	schemesMu.RUnlock()
	if ok {
		return sch, nil
	}

	schemesMu.Lock()
	defer schemesMu.Unlock()
	if sch, ok := schemes[circuitType]; ok {
		return sch, nil
	}

	var gnarkCircuit frontend.Circuit
	switch circuitType {
	case "add":
		gnarkCircuit = &AddCircuit{}
	case "mul":
		gnarkCircuit = &MulCircuit{}
	default:
		return nil, fmt.Errorf("unsupported circuit type: %s", circuitType)
	}

	ccs, err := frontend.Compile(scalarField(), r1cs.NewBuilder, gnarkCircuit)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}

	sch = &groth16Scheme{ccs: ccs, pk: pk, vk: vk, name: circuitType}
	schemes[circuitType] = sch
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
	gp.mu.Lock()
	defer gp.mu.Unlock()

	scheme, err := getOrCreateScheme(circuit)
	if err != nil {
		return nil, fmt.Errorf("scheme: %w", err)
	}

	gnarkAssignment := buildAssignment(scheme.name, witness)
	if gnarkAssignment == nil {
		return nil, fmt.Errorf("unsupported circuit type: %s", scheme.name)
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
		publicValues[i] = new(big.Int).Set(p)
	}

	return &Proof{
		A:         []*big.Int{new(big.Int).Set(witness.Public[0])},
		B:         []*big.Int{new(big.Int).Set(witness.Public[1])},
		C:         []*big.Int{new(big.Int).Set(witness.Secret[0])},
		CircuitID: []byte(circuit.Name),
		Public:    publicValues,
		System:    Groth16,
		Raw:       buf.Bytes(),
		ProofHash: nil,
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
	gv.mu.Lock()
	defer gv.mu.Unlock()

	if proof.Raw == nil {
		return fmt.Errorf("proof missing serialized raw data")
	}

	scheme, err := getOrCreateScheme(circuit)
	if err != nil {
		return fmt.Errorf("scheme: %w", err)
	}

	groth16Proof := groth16.NewProof(ecc.BN254)
	if _, err := groth16Proof.ReadFrom(bytes.NewReader(proof.Raw)); err != nil {
		return fmt.Errorf("read proof: %w", err)
	}

	pubAssignment := buildPublicAssignment(scheme.name, publicWitness)
	if pubAssignment == nil {
		return fmt.Errorf("unsupported circuit type: %s", scheme.name)
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

func buildAssignment(circuitType string, w *Witness) frontend.Circuit {
	switch circuitType {
	case "add":
		return &AddCircuit{
			X: assignFn(w.Public[0]),
			Y: assignFn(w.Public[1]),
			Z: assignFn(w.Secret[0]),
		}
	case "mul":
		return &MulCircuit{
			X: assignFn(w.Public[0]),
			Y: assignFn(w.Public[1]),
			Z: assignFn(w.Secret[0]),
		}
	default:
		return nil
	}
}

func buildPublicAssignment(circuitType string, w *Witness) frontend.Circuit {
	switch circuitType {
	case "add":
		return &AddCircuit{
			X: assignFn(w.Public[0]),
			Y: assignFn(w.Public[1]),
			Z: 0,
		}
	case "mul":
		return &MulCircuit{
			X: assignFn(w.Public[0]),
			Y: assignFn(w.Public[1]),
			Z: 0,
		}
	default:
		return nil
	}
}

// GnarkProofSize returns estimated serialized groth16 proof size on BN254.
const GnarkProofSize = 256

// SerializeProofForTx encodes a proof and its public inputs into transaction data.
func SerializeProofForTx(proof *Proof, publicInputs []*big.Int) ([]byte, error) {
	if proof.Raw == nil {
		return nil, fmt.Errorf("proof missing raw data")
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(proof.Raw))); err != nil {
		return nil, err
	}
	buf.Write(proof.Raw)
	for _, pi := range publicInputs {
		padded := make([]byte, 32)
		pi.FillBytes(padded)
		buf.Write(padded)
	}
	return buf.Bytes(), nil
}

// DeserializeProofFromTx reconstructs a Proof and public witness from transaction data.
func DeserializeProofFromTx(txData []byte, circuit *Circuit) (*Proof, *Witness, error) {
	if len(txData) < 4 {
		return nil, nil, fmt.Errorf("data too short for proof length header")
	}
	proofLen := binary.BigEndian.Uint32(txData[:4])
	if len(txData) < int(4+proofLen) {
		return nil, nil, fmt.Errorf("data too short for proof body")
	}
	numPublic := circuit.NumInputs
	publicOffset := 4 + int(proofLen)
	publicBytes := txData[publicOffset:]
	publicInputs := make([]*big.Int, numPublic)
	for i := 0; i < numPublic; i++ {
		start := i * 32
		if start+32 > len(publicBytes) {
			publicInputs[i] = big.NewInt(0)
			continue
		}
		publicInputs[i] = new(big.Int).SetBytes(publicBytes[start : start+32])
	}
	return &Proof{
		Raw:       txData[4 : 4+proofLen],
		CircuitID: []byte(circuit.Name),
		Public:    publicInputs,
		System:    Groth16,
	}, &Witness{Public: publicInputs, Secret: []*big.Int{}}, nil
}
