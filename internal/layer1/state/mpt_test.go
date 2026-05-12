package state

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestMPT_EmptyTrie(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	if len(mpt.Root()) == 0 {
		t.Error("empty trie should have a root hash")
	}
	_, err := mpt.Get([]byte("key"))
	if err == nil {
		t.Error("expected error on empty trie")
	}
}

func TestMPT_InsertAndGet(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	key := []byte("foo")
	value := []byte("bar")

	if err := mpt.Update(key, value); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	got, err := mpt.Get(key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("got %s, want %s", got, value)
	}
}

func TestMPT_UpdateExisting(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	key := []byte("key")
	if err := mpt.Update(key, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := mpt.Update(key, []byte("v2")); err != nil {
		t.Fatal(err)
	}
	got, err := mpt.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("v2")) {
		t.Errorf("got %s, want v2", got)
	}
}

func TestMPT_MultipleKeys(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	pairs := map[string]string{
		"apple":  "fruit",
		"app":    "short",
		"apricot": "stone fruit",
		"banana": "yellow",
		"berry":  "small",
		"cat":    "animal",
		"car":    "vehicle",
	}

	for k, v := range pairs {
		if err := mpt.Update([]byte(k), []byte(v)); err != nil {
			t.Fatalf("insert %s failed: %v", k, err)
		}
	}

	for k, v := range pairs {
		got, err := mpt.Get([]byte(k))
		if err != nil {
			t.Fatalf("get %s failed: %v", k, err)
		}
		if string(got) != v {
			t.Errorf("get %s: got %s, want %s", k, got, v)
		}
	}

	// Verify node count is reasonable
	count := mpt.MPTNodeCount()
	if count == 0 {
		t.Error("expected non-zero node count")
	}
	t.Logf("MPT node count for %d entries: %d", len(pairs), count)
}

func TestMPT_NotFound(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	mpt.Update([]byte("existing"), []byte("value"))
	if _, err := mpt.Get([]byte("nonexistent")); err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestMPT_Delete(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	key := []byte("delete-me")

	mpt.Update(key, []byte("value"))
	if err := mpt.Delete(key); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := mpt.Get(key); err == nil {
		t.Error("expected error after delete")
	}
}

func TestMPT_DeleteNonexistent(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	if err := mpt.Delete([]byte("ghost")); err != nil {
		t.Errorf("delete nonexistent should not error: %v", err)
	}
}

func TestMPT_RootHashDeterminism(t *testing.T) {
	pairs := map[string]string{
		"do":   "verb",
		"dog":  "animal",
		"doge": "meme",
	}

	var prev []byte
	for run := 0; run < 3; run++ {
		mpt := NewMPT(NewMemoryStore())
		for k, v := range pairs {
			mpt.Update([]byte(k), []byte(v))
		}
		root := mpt.Root()
		if prev != nil && !bytes.Equal(root, prev) {
			t.Error("root hash not deterministic")
		}
		prev = root
	}
}

func TestMPT_UpdateViaEmptyValueDelete(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	key := []byte("key")

	mpt.Update(key, []byte("value"))
	mpt.Update(key, []byte{})

	if _, err := mpt.Get(key); err == nil {
		t.Error("expected error after update with empty value")
	}
}

func TestMPT_RandomKeys(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	entries := make(map[string]string)

	for i := 0; i < 50; i++ {
		key := make([]byte, 8)
		rand.Read(key)
		val := make([]byte, 16)
		rand.Read(val)
		ks := string(key)
		vs := string(val)
		entries[ks] = vs
		if err := mpt.Update(key, val); err != nil {
			t.Fatalf("insert %x failed: %v", key, err)
		}
	}

	for ks, vs := range entries {
		got, err := mpt.Get([]byte(ks))
		if err != nil {
			t.Fatalf("get %x failed: %v", []byte(ks), err)
		}
		if string(got) != vs {
			t.Errorf("value mismatch for %x", []byte(ks))
		}
	}

	// Verify all entries still exist
	hasAll := true
	for ks := range entries {
		if !mpt.Has([]byte(ks)) {
			hasAll = false
			t.Errorf("Has() returned false for existing key %x", []byte(ks))
		}
	}
	if !hasAll {
		t.Error("not all keys reported as present")
	}
}

func TestMPT_InsertAndDeleteCycle(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	keys := [][]byte{[]byte("a"), []byte("ab"), []byte("abc"), []byte("abcd")}

	// Insert all
	for _, k := range keys {
		if err := mpt.Update(k, []byte("v")); err != nil {
			t.Fatalf("insert %s failed: %v", k, err)
		}
	}

	// Delete all in reverse
	for i := len(keys) - 1; i >= 0; i-- {
		if err := mpt.Delete(keys[i]); err != nil {
			t.Fatalf("delete %s failed: %v", keys[i], err)
		}
	}

	// Trie should be empty
	if mpt.MPTNodeCount() != 0 {
		t.Errorf("expected 0 nodes after full delete cycle, got %d", mpt.MPTNodeCount())
	}
}

func TestMPT_SharedPrefixKeys(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	keys := []string{"a", "aa", "aaa", "aaaa", "aaaaa"}

	for _, k := range keys {
		if err := mpt.Update([]byte(k), []byte(k+"-val")); err != nil {
			t.Fatalf("insert %s failed: %v", k, err)
		}
	}

	for _, k := range keys {
		got, err := mpt.Get([]byte(k))
		if err != nil {
			t.Fatalf("get %s failed: %v", k, err)
		}
		if string(got) != k+"-val" {
			t.Errorf("get %s: got %s, want %s-val", k, got, k)
		}
	}

	t.Logf("MPT node count for 5 shared-prefix keys: %d", mpt.MPTNodeCount())
}

func TestMPT_LargeValue(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())
	largeVal := make([]byte, 4096)
	rand.Read(largeVal)

	if err := mpt.Update([]byte("large"), largeVal); err != nil {
		t.Fatalf("insert large value failed: %v", err)
	}

	got, err := mpt.Get([]byte("large"))
	if err != nil {
		t.Fatalf("get large value failed: %v", err)
	}
	if !bytes.Equal(got, largeVal) {
		t.Error("large value mismatch")
	}
}

func TestMPT_RootChanges(t *testing.T) {
	mpt := NewMPT(NewMemoryStore())

	r0 := mpt.Root()
	mpt.Update([]byte("k1"), []byte("v1"))
	r1 := mpt.Root()
	mpt.Update([]byte("k2"), []byte("v2"))
	r2 := mpt.Root()

	if bytes.Equal(r0, r1) {
		t.Error("root should change after insert")
	}
	if bytes.Equal(r1, r2) {
		t.Error("root should change after second insert")
	}

	mpt.Delete([]byte("k1"))
	r3 := mpt.Root()
	if bytes.Equal(r2, r3) {
		t.Error("root should change after delete")
	}
}
