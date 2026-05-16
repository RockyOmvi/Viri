package privacy

import "testing"

func TestCreateAndSpendNote(t *testing.T) {
	pool := NewShieldedPool()

	note, err := pool.CreateNote(100, []byte("owner"), []byte("rand1234567890123"))
	if err != nil {
		t.Fatalf("create note failed: %v", err)
	}

	if pool.NoteCount() != 1 {
		t.Fatalf("expected 1 note")
	}
	if pool.TotalShielded() != 100 {
		t.Fatalf("expected total 100")
	}

	if !pool.HasCommitment(note.Commitment) {
		t.Fatalf("commitment missing")
	}

	if _, err := pool.SpendNote(note.Nullifier); err != nil {
		t.Fatalf("spend failed: %v", err)
	}
	if pool.TotalShielded() != 0 {
		t.Fatalf("expected total 0")
	}
	if !pool.HasNullifier(note.Nullifier) {
		t.Fatalf("nullifier should be marked")
	}
}

func TestDuplicateCommitment(t *testing.T) {
	pool := NewShieldedPool()
	_, err := pool.CreateNote(10, []byte("owner"), []byte("rand1234567890123"))
	if err != nil {
		t.Fatalf("create note failed: %v", err)
	}
	if _, err := pool.CreateNote(10, []byte("owner"), []byte("rand1234567890123")); err == nil {
		t.Fatalf("expected duplicate commitment error")
	}
}

func TestSpendUnknownNullifier(t *testing.T) {
	pool := NewShieldedPool()
	if _, err := pool.SpendNote([]byte("missing")); err == nil {
		t.Fatalf("expected error for unknown nullifier")
	}
}

func TestSpendNullifierWrongLength(t *testing.T) {
	pool := NewShieldedPool()
	if _, err := pool.SpendNote([]byte("short")); err == nil {
		t.Fatalf("expected error for short nullifier")
	}
}

func TestSpendDoubleSpend(t *testing.T) {
	pool := NewShieldedPool()
	note, err := pool.CreateNote(50, []byte("owner"), []byte("rand1234567890123"))
	if err != nil {
		t.Fatalf("create note failed: %v", err)
	}
	if _, err := pool.SpendNote(note.Nullifier); err != nil {
		t.Fatalf("first spend failed: %v", err)
	}
	if _, err := pool.SpendNote(note.Nullifier); err == nil {
		t.Fatalf("double spend should fail")
	}
}

func TestCreateNoteEmptyOwner(t *testing.T) {
	pool := NewShieldedPool()
	if _, err := pool.CreateNote(10, nil, []byte("rand1234567890123")); err == nil {
		t.Fatalf("expected error for nil owner")
	}
	if _, err := pool.CreateNote(10, []byte{}, []byte("rand1234567890123")); err == nil {
		t.Fatalf("expected error for empty owner")
	}
}

func TestCreateNoteShortRandomness(t *testing.T) {
	pool := NewShieldedPool()
	if _, err := pool.CreateNote(10, []byte("owner"), []byte("short")); err == nil {
		t.Fatalf("expected error for short randomness")
	}
}

func TestCreateNoteZeroValue(t *testing.T) {
	pool := NewShieldedPool()
	if _, err := pool.CreateNote(0, []byte("owner"), []byte("rand1234567890123")); err == nil {
		t.Fatalf("expected error for zero value")
	}
}

func TestOwnerHashed(t *testing.T) {
	pool := NewShieldedPool()
	note, err := pool.CreateNote(10, []byte("my-owner"), []byte("rand1234567890123"))
	if err != nil {
		t.Fatalf("create note failed: %v", err)
	}
	if string(note.OwnerHash) == string([]byte("my-owner")) {
		t.Fatalf("owner should be hashed, not stored in plaintext")
	}
	if len(note.OwnerHash) != 32 {
		t.Fatalf("owner hash should be 32 bytes")
	}
}

func TestTotalShieldedOverflow(t *testing.T) {
	pool := NewShieldedPool()
	_, err := pool.CreateNote(100, []byte("owner"), []byte("rand1234567890123"))
	if err != nil {
		t.Fatalf("first note failed: %v", err)
	}
	overflowVal := ^uint64(0) - 100 + 1
	if _, err := pool.CreateNote(overflowVal, []byte("owner2"), []byte("rand1234567890123")); err == nil {
		t.Fatalf("expected overflow error")
	}
}

func TestHasNullifierShortInput(t *testing.T) {
	pool := NewShieldedPool()
	if pool.HasNullifier([]byte("short")) {
		t.Fatalf("short nullifier should return false")
	}
}

func TestHasCommitmentShortInput(t *testing.T) {
	pool := NewShieldedPool()
	if pool.HasCommitment([]byte("short")) {
		t.Fatalf("short commitment should return false")
	}
}
