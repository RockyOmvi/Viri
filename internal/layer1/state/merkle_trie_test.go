package state

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestMerkleTrie_InsertAndRetrieve(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("test-key")
	value := []byte("test-value")

	err := mt.Update(key, value)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	retrieved, err := mt.Get(key)
	if err != nil {
		t.Fatalf("failed to retrieve: %v", err)
	}

	if !bytes.Equal(retrieved, value) {
		t.Errorf("expected %s, got %s", value, retrieved)
	}
}

func TestMerkleTrie_NotFound(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	_, err := mt.Get([]byte("non-existent"))
	if err == nil {
		t.Error("expected error for non-existent key")
	}
}

func TestMerkleTrie_Delete(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("test-key")
	value := []byte("test-value")

	mt.Update(key, value)
	err := mt.Delete(key)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = mt.Get(key)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestMerkleTrie_EmptyTrie(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	root := mt.Root()
	expectedRoot := hashEmptyTrie()

	if !bytes.Equal(root, expectedRoot) {
		t.Errorf("empty trie root mismatch")
	}

	if mt.Size() != 0 {
		t.Errorf("expected size 0 for empty trie, got %d", mt.Size())
	}
}

func TestMerkleTrie_ProofGenerationAndVerification(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("proof-key")
	value := []byte("proof-value")

	mt.Update(key, value)
	root := mt.Root()

	proof, err := mt.Prove(key)
	if err != nil {
		t.Fatalf("failed to generate proof: %v", err)
	}

	valid := mt.VerifyProof(root, key, value, proof)
	if !valid {
		t.Error("VerifyProof should return true for valid proof (single-entry trie)")
	}
}

func TestMerkleTrie_ProofVerificationTamperedData(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("proof-key")
	value := []byte("proof-value")

	mt.Update(key, value)
	root := mt.Root()

	proof, _ := mt.Prove(key)

	tamperedValue := []byte("tampered-value")
	valid := mt.VerifyProof(root, key, tamperedValue, proof)
	if valid {
		t.Error("proof should fail with tampered data")
	}
}

func TestMerkleTrie_ProofVerificationTamperedKey(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("proof-key")
	value := []byte("proof-value")

	mt.Update(key, value)
	root := mt.Root()

	proof, _ := mt.Prove(key)

	tamperedKey := []byte("tampered-key")
	valid := mt.VerifyProof(root, tamperedKey, value, proof)
	if valid {
		t.Error("proof should fail with tampered key")
	}
}

func TestMerkleTrie_RootHashDeterminism(t *testing.T) {
	store1 := NewMemoryStore()
	mt1 := NewMerkleTrie(store1)

	store2 := NewMemoryStore()
	mt2 := NewMerkleTrie(store2)

	key := []byte("test-key")
	value := []byte("test-value")

	mt1.Update(key, value)
	mt2.Update(key, value)

	root1 := mt1.Root()
	root2 := mt2.Root()

	if !bytes.Equal(root1, root2) {
		t.Error("root hash should be deterministic for same data")
	}
}

func TestMerkleTrie_UpdateExistingKey(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("test-key")
	value1 := []byte("value1")
	value2 := []byte("value2")

	mt.Update(key, value1)
	mt.Update(key, value2)

	retrieved, err := mt.Get(key)
	if err != nil {
		t.Fatalf("failed to retrieve: %v", err)
	}

	if !bytes.Equal(retrieved, value2) {
		t.Errorf("expected updated value %s, got %s", value2, retrieved)
	}
}

func TestMerkleTrie_EmptyValueDelete(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("test-key")
	value := []byte("test-value")

	mt.Update(key, value)

	err := mt.Update(key, []byte{})
	if err != nil {
		t.Fatalf("failed to update with empty value (delete): %v", err)
	}

	_, err = mt.Get(key)
	if err == nil {
		t.Error("expected error after updating with empty value (delete)")
	}
}

func TestMerkleTrie_MultiEntryInsertAndRetrieve(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	entries := map[string]string{
		"key-a": "value-a",
		"key-b": "value-b",
		"key-c": "value-c",
	}

	for k, v := range entries {
		if err := mt.Update([]byte(k), []byte(v)); err != nil {
			t.Fatalf("failed to insert %s: %v", k, err)
		}
	}

	for k, v := range entries {
		got, err := mt.Get([]byte(k))
		if err != nil {
			t.Fatalf("failed to get %s: %v", k, err)
		}
		if string(got) != v {
			t.Errorf("expected %s for key %s, got %s", v, k, got)
		}
	}
}

func TestMerkleTrie_MultiEntryProofAllPositions(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	keys := []string{"alpha", "beta", "gamma", "delta"}
	vals := []string{"10", "20", "30", "40"}

	for i := range keys {
		if err := mt.Update([]byte(keys[i]), []byte(vals[i])); err != nil {
			t.Fatalf("failed to insert %s: %v", keys[i], err)
		}
	}

	root := mt.Root()

	for i := range keys {
		proof, err := mt.Prove([]byte(keys[i]))
		if err != nil {
			t.Fatalf("Prove(%s) failed: %v", keys[i], err)
		}

		if !mt.VerifyProof(root, []byte(keys[i]), []byte(vals[i]), proof) {
			t.Errorf("VerifyProof failed for key %s at position %d", keys[i], i)
		}
	}
}

func TestMerkleTrie_MultiEntryProofOddCount(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	keys := []string{"x", "y", "z"}
	vals := []string{"1", "2", "3"}

	for i := range keys {
		mt.Update([]byte(keys[i]), []byte(vals[i]))
	}

	root := mt.Root()

	for i := range keys {
		proof, err := mt.Prove([]byte(keys[i]))
		if err != nil {
			t.Fatalf("Prove(%s) failed: %v", keys[i], err)
		}
		if !mt.VerifyProof(root, []byte(keys[i]), []byte(vals[i]), proof) {
			t.Errorf("VerifyProof failed for odd-count entry %s", keys[i])
		}
	}
}

func TestMerkleTrie_MultiEntryTamperedData(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	mt.Update([]byte("key1"), []byte("val1"))
	mt.Update([]byte("key2"), []byte("val2"))
	mt.Update([]byte("key3"), []byte("val3"))

	root := mt.Root()
	proof, _ := mt.Prove([]byte("key2"))

	if mt.VerifyProof(root, []byte("key2"), []byte("tampered"), proof) {
		t.Error("expected false for tampered value in multi-entry trie")
	}

	if mt.VerifyProof(root, []byte("wrong-key"), []byte("val2"), proof) {
		t.Error("expected false for wrong key in multi-entry trie")
	}
}

func TestMerkleTrie_SequentialDeleteAndReinsert(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	mt.Update([]byte("a"), []byte("1"))
	mt.Update([]byte("b"), []byte("2"))
	mt.Update([]byte("c"), []byte("3"))

	root1 := mt.Root()

	mt.Delete([]byte("b"))

	_, err := mt.Get([]byte("b"))
	if err == nil {
		t.Error("expected error after delete")
	}

	if mt.Size() != 2 {
		t.Errorf("expected size 2 after delete, got %d", mt.Size())
	}

	mt.Update([]byte("b"), []byte("2"))

	got, _ := mt.Get([]byte("b"))
	if string(got) != "2" {
		t.Errorf("expected '2' after reinsert, got '%s'", got)
	}

	root2 := mt.Root()
	if !bytes.Equal(root1, root2) {
		t.Error("root should match after delete+reinsert of same entry")
	}
}

func TestMerkleTrie_MultipleUpdatesAcrossKeys(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	mt.Update([]byte("k1"), []byte("v1"))
	mt.Update([]byte("k2"), []byte("v2"))
	root1 := mt.Root()

	mt.Update([]byte("k1"), []byte("v1-updated"))
	root2 := mt.Root()

	if bytes.Equal(root1, root2) {
		t.Error("root should change when a value is updated")
	}

	got, _ := mt.Get([]byte("k1"))
	if string(got) != "v1-updated" {
		t.Errorf("expected 'v1-updated', got '%s'", got)
	}

	proof, _ := mt.Prove([]byte("k1"))
	if !mt.VerifyProof(root2, []byte("k1"), []byte("v1-updated"), proof) {
		t.Error("proof should verify for updated value")
	}
}

func TestMerkleTrie_RootDeterminismMultiEntry(t *testing.T) {
	s1, s2 := NewMemoryStore(), NewMemoryStore()
	mt1, mt2 := NewMerkleTrie(s1), NewMerkleTrie(s2)

	inserts := []struct{ k, v string }{
		{"c", "3"}, {"a", "1"}, {"b", "2"},
	}

	for _, in := range inserts {
		mt1.Update([]byte(in.k), []byte(in.v))
	}
	for _, in := range inserts {
		mt2.Update([]byte(in.k), []byte(in.v))
	}

	if !bytes.Equal(mt1.Root(), mt2.Root()) {
		t.Error("roots should be deterministic regardless of insertion order")
	}
}

func TestMerkleTrie_SizeAfterMultiEntry(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	if mt.Size() != 0 {
		t.Errorf("expected 0, got %d", mt.Size())
	}

	mt.Update([]byte("a"), []byte("1"))
	if mt.Size() != 1 {
		t.Errorf("expected 1, got %d", mt.Size())
	}

	mt.Update([]byte("b"), []byte("2"))
	if mt.Size() != 2 {
		t.Errorf("expected 2, got %d", mt.Size())
	}

	mt.Delete([]byte("a"))
	if mt.Size() != 1 {
		t.Errorf("expected 1 after delete, got %d", mt.Size())
	}

	mt.Delete([]byte("b"))
	if mt.Size() != 0 {
		t.Errorf("expected 0 after all deletes, got %d", mt.Size())
	}
}

func TestMerkleTrie_VerifyProofEmptyCompat(t *testing.T) {
	store := NewMemoryStore()
	mt := NewMerkleTrie(store)

	key := []byte("compat-key")
	value := []byte("compat-value")
	mt.Update(key, value)
	root := mt.Root()

	valid := mt.VerifyProof(root, key, value, [][]byte{})
	if !valid {
		t.Error("empty proof should still work for single-entry backward compat")
	}
}

func TestHashFunctions(t *testing.T) {
	key := []byte("test")
	value := []byte("value")

	leafHash := hashLeaf(key, value)
	if len(leafHash) != 32 {
		t.Errorf("expected hash length 32, got %d", len(leafHash))
	}

	emptyHash := hashEmptyTrie()
	if len(emptyHash) != 32 {
		t.Errorf("expected empty hash length 32, got %d", len(emptyHash))
	}

	left := sha256.Sum256([]byte("left"))
	right := sha256.Sum256([]byte("right"))
	combined := hashNodes(left[:], right[:])
	if len(combined) != 32 {
		t.Errorf("expected combined hash length 32, got %d", len(combined))
	}
}
