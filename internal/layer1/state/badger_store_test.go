package state

import (
	"os"
	"testing"
)

func TestBadgerStorePutGet(t *testing.T) {
	dir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	key := []byte("test-key")
	value := []byte("test-value")

	if err := store.Put(key, value); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(value) {
		t.Errorf("expected %s, got %s", value, got)
	}
}

func TestBadgerStoreHasDelete(t *testing.T) {
	dir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	key := []byte("test-key")

	exists, _ := store.Has(key)
	if exists {
		t.Error("expected key to not exist")
	}

	if err := store.Put(key, []byte("value")); err != nil {
		t.Fatal(err)
	}

	exists, _ = store.Has(key)
	if !exists {
		t.Error("expected key to exist")
	}

	if err := store.Delete(key); err != nil {
		t.Fatal(err)
	}

	exists, _ = store.Has(key)
	if exists {
		t.Error("expected key to not exist after delete")
	}
}

func TestBadgerStoreBatch(t *testing.T) {
	dir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	batch := store.Batch()

	for i := 0; i < 10; i++ {
		key := []byte{byte(i)}
		value := []byte{byte(i + 100)}
		if err := batch.Put(key, value); err != nil {
			t.Fatal(err)
		}
	}

	if err := batch.Write(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		key := []byte{byte(i)}
		expected := []byte{byte(i + 100)}

		got, err := store.Get(key)
		if err != nil {
			t.Fatalf("key %d not found", i)
		}

		if string(got) != string(expected) {
			t.Errorf("key %d: expected %v, got %v", i, expected, got)
		}
	}
}

func TestBadgerStoreIterator(t *testing.T) {
	dir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	prefix := []byte("user:")
	for i := 0; i < 5; i++ {
		key := append(prefix, byte(i))
		if err := store.Put(key, []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.Put([]byte("other"), []byte{99}); err != nil {
		t.Fatal(err)
	}

	it, err := store.Iterator(prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()

	count := 0
	for it.Next() {
		count++
	}

	if count != 5 {
		t.Errorf("expected 5 items, got %d", count)
	}
}

func TestBadgerStorePersistence(t *testing.T) {
	dir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store1, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store1.Put([]byte("persist"), []byte("data")); err != nil {
		t.Fatal(err)
	}

	store1.Close()

	store2, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	got, err := store2.Get([]byte("persist"))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "data" {
		t.Errorf("expected 'data', got '%s'", got)
	}
}
