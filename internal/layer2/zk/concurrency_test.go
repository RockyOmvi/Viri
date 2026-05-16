package zk

import (
	"math/big"
	"sync"
	"testing"
)

func TestGnarkProveConcurrent(t *testing.T) {
	circuit := NewCircuit("concurrent_test", 2, 1, FieldTypePrime)
	circuit.AddAddConstraint(0, 1, 2)

	pk := GenerateProvingKey(circuit)
	vk := GenerateVerifyingKey(pk, circuit)
	prover := NewProver(pk, circuit)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assignment := &Assignment{
				Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5)},
				Witness: []*big.Int{big.NewInt(8)},
			}
			proof, err := prover.Prove(assignment)
			if err != nil {
				t.Errorf("concurrent prove failed: %v", err)
				return
			}
			verifier := NewVerifier(vk, circuit)
			if err := verifier.Verify(proof); err != nil {
				t.Errorf("concurrent verify failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestGnarkCircuitCompileConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			circuit := NewCircuit("g"+"cc", 2, 1, FieldTypePrime)
			circuit.AddMulConstraint(0, 1, 2)

			w := &Witness{
				Public: []*big.Int{big.NewInt(7), big.NewInt(8)},
				Secret: []*big.Int{big.NewInt(56)},
			}

			gp := NewGnarkProver()
			proof, err := gp.Prove(circuit, w)
			if err != nil {
				t.Errorf("concurrent gnark prove: %v", err)
				return
			}

			gv := NewGnarkVerifier()
			if err := gv.Verify(proof, circuit, w); err != nil {
				t.Errorf("concurrent gnark verify: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
