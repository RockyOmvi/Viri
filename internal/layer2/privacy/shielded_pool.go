package privacy

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

type Note struct {
	Nullifier  []byte
	Commitment []byte
	Value      uint64
	Owner      []byte
}

type ShieldedPool struct {
	mu          sync.RWMutex
	notes       []*Note
	nullifiers  map[string]bool
	commitments map[string]bool
	totalShield uint64
}

func NewShieldedPool() *ShieldedPool {
	return &ShieldedPool{
		notes:       make([]*Note, 0),
		nullifiers:  make(map[string]bool),
		commitments: make(map[string]bool),
	}
}

func (sp *ShieldedPool) CreateNote(value uint64, owner []byte, randomness []byte) (*Note, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	commitmentData := append(owner, randomness...)
	commitmentData = append(commitmentData, uint64ToBytes(value)...)
	commitment := sha256.Sum256(commitmentData)

	nullifierData := append(owner, randomness...)
	nullifierData = append(nullifierData, []byte{0x01}...)
	nullifier := sha256.Sum256(nullifierData)

	commitmentKey := string(commitment[:])
	if sp.commitments[commitmentKey] {
		return nil, fmt.Errorf("duplicate commitment")
	}

	note := &Note{
		Nullifier:  nullifier[:],
		Commitment: commitment[:],
		Value:      value,
		Owner:      owner,
	}

	sp.notes = append(sp.notes, note)
	sp.commitments[commitmentKey] = true
	sp.totalShield += value

	return note, nil
}

func (sp *ShieldedPool) SpendNote(nullifier []byte) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	key := string(nullifier)
	if sp.nullifiers[key] {
		return fmt.Errorf("nullifier already used")
	}

	sp.nullifiers[key] = true

	for i, note := range sp.notes {
		if string(note.Nullifier) == key {
			sp.totalShield -= note.Value
			sp.notes = append(sp.notes[:i], sp.notes[i+1:]...)
			break
		}
	}

	return nil
}

func (sp *ShieldedPool) HasNullifier(nullifier []byte) bool {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.nullifiers[string(nullifier)]
}

func (sp *ShieldedPool) HasCommitment(commitment []byte) bool {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.commitments[string(commitment)]
}

func (sp *ShieldedPool) NoteCount() int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return len(sp.notes)
}

func (sp *ShieldedPool) TotalShielded() uint64 {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.totalShield
}

func uint64ToBytes(n uint64) []byte {
	return []byte{
		byte(n >> 56), byte(n >> 48), byte(n >> 40), byte(n >> 32),
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
	}
}
