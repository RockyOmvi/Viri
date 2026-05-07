package privacy

import "testing"

func TestCreateAndSpendNote(t *testing.T) {
	pool := NewShieldedPool()

	note, err := pool.CreateNote(100, []byte("owner"), []byte("rand"))
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

	if err := pool.SpendNote(note.Nullifier); err != nil {
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
	_, err := pool.CreateNote(10, []byte("owner"), []byte("rand"))
	if err != nil {
		t.Fatalf("create note failed: %v", err)
	}
	if _, err := pool.CreateNote(10, []byte("owner"), []byte("rand")); err == nil {
		t.Fatalf("expected duplicate commitment error")
	}
}

func TestSpendUnknownNullifier(t *testing.T) {
	pool := NewShieldedPool()
	if err := pool.SpendNote([]byte("missing")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
