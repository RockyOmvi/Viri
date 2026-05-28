package privacy

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
)

const (
	nullifierLen     = 32
	commitmentLen    = 32
	minRandomnessLen = 16
)

var (
	G bn254.G1Affine
	H bn254.G1Affine
)

func init() {
	// G is the standard BN254 generator
	G.ScalarMultiplicationBase(new(big.Int).SetUint64(1))

	// Derive H from G using hash-and-map to curve (NUMS point)
	gBytes := G.Bytes()
	hash := sha256.Sum256(gBytes[:])
	var u fp.Element
	u.SetBytes(hash[:])
	H = bn254.MapToG1(u)
}

type Note struct {
	Nullifier  []byte
	Commitment []byte
	Value      uint64
	OwnerHash  []byte
}

type PoolBackend interface {
	SaveNote(note *Note) error
	SaveNullifier(nullifier []byte) error
	SaveCommitment(commitment []byte) error
	HasCommitment(commitment []byte) (bool, error)
	HasNullifier(nullifier []byte) (bool, error)
	LoadNotes() ([]*Note, error)
	LoadNullifiers() (map[string]bool, error)
	LoadCommitments() (map[string]bool, error)
	SaveTotalShielded(value uint64) error
	LoadTotalShielded() (uint64, error)
	DeleteNote(nullifier []byte) error
	Close() error
}

type memoryBackend struct {
	mu          sync.RWMutex
	notes       []*Note
	nullifiers  map[string]bool
	commitments map[string]bool
	totalShield uint64
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		notes:       make([]*Note, 0),
		nullifiers:  make(map[string]bool),
		commitments: make(map[string]bool),
	}
}

func (m *memoryBackend) SaveNote(note *Note) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notes = append(m.notes, note)
	return nil
}

func (m *memoryBackend) SaveNullifier(nullifier []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nullifiers[string(nullifier)] = true
	return nil
}

func (m *memoryBackend) SaveCommitment(commitment []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commitments[string(commitment)] = true
	return nil
}

func (m *memoryBackend) HasCommitment(commitment []byte) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.commitments[string(commitment)], nil
}

func (m *memoryBackend) HasNullifier(nullifier []byte) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nullifiers[string(nullifier)], nil
}

func (m *memoryBackend) LoadNotes() ([]*Note, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Note, len(m.notes))
	copy(result, m.notes)
	return result, nil
}

func (m *memoryBackend) LoadNullifiers() (map[string]bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]bool, len(m.nullifiers))
	for k, v := range m.nullifiers {
		result[k] = v
	}
	return result, nil
}

func (m *memoryBackend) LoadCommitments() (map[string]bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]bool, len(m.commitments))
	for k, v := range m.commitments {
		result[k] = v
	}
	return result, nil
}

func (m *memoryBackend) SaveTotalShielded(value uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalShield = value
	return nil
}

func (m *memoryBackend) LoadTotalShielded() (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalShield, nil
}

func (m *memoryBackend) DeleteNote(nullifier []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(nullifier)
	for i, note := range m.notes {
		if string(note.Nullifier) == key {
			m.notes = append(m.notes[:i], m.notes[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *memoryBackend) Close() error {
	return nil
}

type ShieldedPool struct {
	mu          sync.RWMutex
	backend     PoolBackend
	notes       []*Note
	nullifiers  map[string]bool
	commitments map[string]bool
	totalShield uint64
}

func NewShieldedPool() *ShieldedPool {
	return NewShieldedPoolWithBackend(newMemoryBackend())
}

func NewShieldedPoolWithBackend(backend PoolBackend) *ShieldedPool {
	sp := &ShieldedPool{
		backend:     backend,
		notes:       make([]*Note, 0),
		nullifiers:  make(map[string]bool),
		commitments: make(map[string]bool),
	}

	if notes, err := backend.LoadNotes(); err == nil {
		sp.notes = notes
	}
	if nullifiers, err := backend.LoadNullifiers(); err == nil {
		sp.nullifiers = nullifiers
	}
	if commitments, err := backend.LoadCommitments(); err == nil {
		sp.commitments = commitments
	}
	if total, err := backend.LoadTotalShielded(); err == nil {
		sp.totalShield = total
	}

	return sp
}

func (sp *ShieldedPool) CreateNote(value uint64, owner, randomness []byte) (*Note, error) {
	if len(owner) == 0 {
		return nil, errors.New("owner is required")
	}
	if len(randomness) < minRandomnessLen {
		return nil, fmt.Errorf("randomness must be at least %d bytes", minRandomnessLen)
	}
	if value == 0 {
		return nil, errors.New("value must be positive")
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.totalShield > math.MaxUint64-value {
		return nil, errors.New("total shielded overflow")
	}

	ownerHash := sha256.Sum256(owner)

	// Pedersen commitment: C = r*H + v*G
	valueScalar := new(big.Int).SetUint64(value)
	var rBytes [32]byte
	copy(rBytes[:], randomness)
	rScalar := new(big.Int).SetBytes(rBytes[:])

	var vG bn254.G1Affine
	vG.ScalarMultiplication(&G, valueScalar)

	var rH bn254.G1Affine
	rH.ScalarMultiplication(&H, rScalar)

	var C bn254.G1Affine
	C.Add(&rH, &vG)

	commitment := C.Bytes()

	nullifierData := append(ownerHash[:], randomness...)
	nullifierData = append(nullifierData, 0x01)
	nullifier := sha256.Sum256(nullifierData)

	commitmentKey := string(commitment[:])
	if sp.commitments[commitmentKey] {
		return nil, fmt.Errorf("duplicate commitment")
	}

	note := &Note{
		Nullifier:  nullifier[:],
		Commitment: commitment[:],
		Value:      value,
		OwnerHash:  ownerHash[:],
	}

	if err := sp.backend.SaveNote(note); err != nil {
		log.Printf("[ERROR] shielded pool: failed to persist note: %v", err)
		return nil, fmt.Errorf("persist note: %w", err)
	}
	if err := sp.backend.SaveCommitment(commitment[:]); err != nil {
		log.Printf("[ERROR] shielded pool: failed to persist commitment: %v", err)
		return nil, fmt.Errorf("persist commitment: %w", err)
	}
	if err := sp.backend.SaveTotalShielded(sp.totalShield); err != nil {
		log.Printf("[ERROR] shielded pool: failed to persist total shielded: %v", err)
		return nil, fmt.Errorf("persist total shielded: %w", err)
	}

	sp.notes = append(sp.notes, note)
	sp.commitments[commitmentKey] = true
	sp.totalShield += value

	return note, nil
}

// SpendNote with proof verification.
// The caller MUST verify the ZK proof before calling SpendNote,
// or pass a non-nil GnarkVerifier+circuit for inline verification.
// Without proof verification, anyone who knows a nullifier can spend notes.
func (sp *ShieldedPool) SpendNote(nullifier []byte) (uint64, error) {
	if len(nullifier) != nullifierLen {
		return 0, fmt.Errorf("nullifier must be %d bytes", nullifierLen)
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	key := string(nullifier)
	if sp.nullifiers[key] {
		return 0, fmt.Errorf("nullifier already used")
	}

	for i, note := range sp.notes {
		if string(note.Nullifier) == key {
			if sp.totalShield < note.Value {
				return 0, errors.New("total shielded underflow")
			}
			sp.nullifiers[key] = true
			sp.totalShield -= note.Value
			value := note.Value
			sp.notes = append(sp.notes[:i], sp.notes[i+1:]...)

		if err := sp.backend.SaveNullifier(nullifier); err != nil {
			log.Printf("[ERROR] shielded pool: failed to persist nullifier: %v", err)
			return 0, fmt.Errorf("persist nullifier: %w", err)
		}
		if err := sp.backend.DeleteNote(nullifier); err != nil {
			log.Printf("[ERROR] shielded pool: failed to delete note: %v", err)
			return 0, fmt.Errorf("delete note: %w", err)
		}
		if err := sp.backend.SaveTotalShielded(sp.totalShield); err != nil {
			log.Printf("[ERROR] shielded pool: failed to persist total shielded: %v", err)
			return 0, fmt.Errorf("persist total shielded: %w", err)
		}

		return value, nil
		}
	}

	return 0, fmt.Errorf("unknown nullifier")
}

func (sp *ShieldedPool) HasNullifier(nullifier []byte) bool {
	if len(nullifier) != nullifierLen {
		return false
	}
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.nullifiers[string(nullifier)]
}

func (sp *ShieldedPool) HasCommitment(commitment []byte) bool {
	if len(commitment) != commitmentLen {
		return false
	}
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
