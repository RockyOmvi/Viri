package crypto

import (
	"testing"
)

func TestMerkleTreeCreation(t *testing.T) {
	data := [][]byte{
		[]byte("tx1"),
		[]byte("tx2"),
		[]byte("tx3"),
		[]byte("tx4"),
	}

	tree, err := NewMerkleTree(data)
	if err != nil {
		t.Fatalf("Failed to create Merkle tree: %v", err)
	}

	if len(tree.RootHash) == 0 {
		t.Fatal("Root hash is empty")
	}
}

func TestMerkleTreeEmpty(t *testing.T) {
	tree, err := NewMerkleTree([][]byte{})
	if err != nil {
		t.Fatalf("Failed to create empty Merkle tree: %v", err)
	}

	if len(tree.RootHash) != 0 {
		t.Fatal("Root hash should be empty for empty tree")
	}
}

func TestMerkleProof(t *testing.T) {
	data := [][]byte{
		[]byte("tx1"),
		[]byte("tx2"),
		[]byte("tx3"),
		[]byte("tx4"),
	}

	tree, _ := NewMerkleTree(data)

	proof, err := tree.GenerateProof(0)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	if len(proof) == 0 {
		t.Fatal("Proof is empty")
	}

	if !VerifyProof(tree.RootHash, data[0], proof, 0) {
		t.Fatal("Merkle proof verification failed")
	}
}

func TestMerkleProofInvalid(t *testing.T) {
	data := [][]byte{
		[]byte("tx1"),
		[]byte("tx2"),
	}

	tree, _ := NewMerkleTree(data)
	proof, _ := tree.GenerateProof(0)

	if VerifyProof(tree.RootHash, []byte("fake-tx"), proof, 0) {
		t.Fatal("Should have failed for invalid data")
	}
}
