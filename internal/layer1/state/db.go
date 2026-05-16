package state

import (
	"errors"
	"sync"
)

var ErrKeyNotFound = errors.New("key not found")

type KVStore interface {
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte) error
	Delete(key []byte) error
	Has(key []byte) (bool, error)
	Close() error
	Batch() Batch
}

type Batch interface {
	Put(key []byte, value []byte) error
	Delete(key []byte) error
	Write() error
	Reset()
}

type IterableKVStore interface {
	KVStore
	Iterator(prefix []byte) (Iterator, error)
}

type Iterator interface {
	Next() bool
	Key() []byte
	Value() []byte
	Error() error
	Close() error
}

type MemoryStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string][]byte),
	}
}

func (m *MemoryStore) Get(key []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, exists := m.data[string(key)]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return val, nil
}

func (m *MemoryStore) Put(key []byte, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := make([]byte, len(value))
	copy(v, value)
	m.data[string(key)] = v
	return nil
}

func (m *MemoryStore) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, string(key))
	return nil
}

func (m *MemoryStore) Has(key []byte) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.data[string(key)]
	return exists, nil
}

func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	return nil
}

func (m *MemoryStore) Batch() Batch {
	return &MemoryBatch{store: m, ops: make([]batchOp, 0)}
}

func (m *MemoryStore) Iterator(prefix []byte) (Iterator, error) {
	return &MemoryIterator{
		prefix: prefix,
		keys:   m.keysWithPrefix(prefix),
		idx:    -1,
		store:  m,
	}, nil
}

func (m *MemoryStore) keysWithPrefix(prefix []byte) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys [][]byte
	prefixStr := string(prefix)
	for k := range m.data {
		if len(k) >= len(prefixStr) && k[:len(prefixStr)] == prefixStr {
			keys = append(keys, []byte(k))
		}
	}
	return keys
}

type batchOp struct {
	key    []byte
	value  []byte
	delete bool
}

type MemoryBatch struct {
	store *MemoryStore
	ops   []batchOp
}

func (b *MemoryBatch) Put(key []byte, value []byte) error {
	k := make([]byte, len(key))
	copy(k, key)
	v := make([]byte, len(value))
	copy(v, value)
	b.ops = append(b.ops, batchOp{key: k, value: v})
	return nil
}

func (b *MemoryBatch) Delete(key []byte) error {
	k := make([]byte, len(key))
	copy(k, key)
	b.ops = append(b.ops, batchOp{key: k, delete: true})
	return nil
}

func (b *MemoryBatch) Write() error {
	b.store.mu.Lock()
	defer b.store.mu.Unlock()
	for _, op := range b.ops {
		if op.delete {
			delete(b.store.data, string(op.key))
		} else {
			v := make([]byte, len(op.value))
			copy(v, op.value)
			b.store.data[string(op.key)] = v
		}
	}
	b.ops = b.ops[:0]
	return nil
}

func (b *MemoryBatch) Reset() {
	b.ops = b.ops[:0]
}

type MemoryIterator struct {
	prefix []byte
	keys   [][]byte
	idx    int
	store  *MemoryStore
}

func (mi *MemoryIterator) Next() bool {
	mi.idx++
	return mi.idx < len(mi.keys)
}

func (mi *MemoryIterator) Key() []byte {
	if mi.idx < 0 || mi.idx >= len(mi.keys) {
		return nil
	}
	return mi.keys[mi.idx]
}

func (mi *MemoryIterator) Value() []byte {
	if mi.idx < 0 || mi.idx >= len(mi.keys) {
		return nil
	}
	val, _ := mi.store.Get(mi.keys[mi.idx])
	return val
}

func (mi *MemoryIterator) Error() error {
	return nil
}

func (mi *MemoryIterator) Close() error {
	return nil
}
