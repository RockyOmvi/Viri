package privacy

import (
	"sync"
	"testing"
)

func TestShieldedPoolConcurrent(t *testing.T) {
	pool := NewShieldedPool()

	var notesMu sync.Mutex
	notes := make([]*Note, 0, 10)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(val uint64, idx int) {
			defer wg.Done()
			randBytes := []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, byte(idx),
			}
			note, err := pool.CreateNote(val, []byte("owner"), randBytes)
			if err != nil {
				t.Errorf("concurrent create note: %v", err)
				return
			}
			notesMu.Lock()
			notes = append(notes, note)
			notesMu.Unlock()
		}(uint64(i+1)*100, i)
	}
	wg.Wait()

	wg = sync.WaitGroup{}
	for _, note := range notes {
		wg.Add(1)
		go func(n *Note, expected uint64) {
			defer wg.Done()
			spent, err := pool.SpendNote(n.Nullifier)
			if err != nil {
				t.Errorf("spend note: %v", err)
				return
			}
			if spent != expected {
				t.Errorf("spent %d != value %d", spent, expected)
			}
		}(note, note.Value)
	}
	wg.Wait()

	if pool.NoteCount() != 0 {
		t.Errorf("expected 0 notes after spending all, got %d", pool.NoteCount())
	}
}

func TestShieldedPoolConcurrentReads(t *testing.T) {
	pool := NewShieldedPool()

	note, err := pool.CreateNote(1000, []byte("owner"), []byte("rand12345678901234"))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.NoteCount()
			pool.TotalShielded()
			pool.HasCommitment(note.Commitment)
			pool.HasNullifier(note.Nullifier)
		}()
	}
	wg.Wait()
}
