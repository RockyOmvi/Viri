package privacy

import "testing"

func TestNoteCountAndHasFlags(t *testing.T) {
	pool := NewShieldedPool()
	if pool.NoteCount() != 0 {
		t.Fatalf("expected 0 notes")
	}

	note, _ := pool.CreateNote(5, []byte("owner"), []byte("rand2"))
	if pool.NoteCount() != 1 {
		t.Fatalf("expected 1 note")
	}
	if !pool.HasCommitment(note.Commitment) {
		t.Fatalf("commitment missing")
	}
	if pool.HasNullifier(note.Nullifier) {
		t.Fatalf("nullifier should not be marked")
	}
}
