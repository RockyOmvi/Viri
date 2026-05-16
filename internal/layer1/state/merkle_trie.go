package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
)

type MerkleTrie struct {
	mu   sync.RWMutex
	root []byte
	db   KVStore
}

type TrieNode struct {
	Key       []byte
	Value     []byte
	LeftHash  []byte
	RightHash []byte
	IsLeaf    bool
}

type entry struct {
	key   []byte
	value []byte
}

func NewMerkleTrie(db KVStore) *MerkleTrie {
	return &MerkleTrie{db: db}
}

func (mt *MerkleTrie) Update(key, value []byte) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if len(value) == 0 {
		return mt.deleteLocked(key)
	}

	return mt.insertLocked(key, value)
}

func (mt *MerkleTrie) Get(key []byte) ([]byte, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	return mt.db.Get(append([]byte("entry:"), key...))
}

func (mt *MerkleTrie) Delete(key []byte) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return mt.deleteLocked(key)
}

func (mt *MerkleTrie) Root() []byte {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	if mt.root == nil {
		return hashEmptyTrie()
	}

	return mt.root
}

func (mt *MerkleTrie) Prove(key []byte) ([][]byte, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	return mt.getProof(key)
}

func (mt *MerkleTrie) VerifyProof(root []byte, key []byte, value []byte, proof [][]byte) bool {
	if len(proof) == 0 {
		computed := hashLeaf(key, value)
		return bytes.Equal(computed, root)
	}

	meta := proof[len(proof)-1]
	if len(meta) < 8 {
		return false
	}
	leafIdx := int(binary.BigEndian.Uint32(meta[0:4]))
	leafCount := int(binary.BigEndian.Uint32(meta[4:8]))
	siblings := proof[:len(proof)-1]

	if leafIdx < 0 || leafIdx >= leafCount || leafCount == 0 {
		return false
	}

	computed := hashLeaf(key, value)
	idx := leafIdx
	count := leafCount

	for count > 1 {
		if idx%2 == 0 {
			if idx+1 < count {
				if len(siblings) == 0 {
					return false
				}
				computed = hashNodes(computed, siblings[0])
				siblings = siblings[1:]
			} else {
				computed = hashNodes(computed, computed)
			}
		} else {
			if len(siblings) == 0 {
				return false
			}
			computed = hashNodes(siblings[0], computed)
			siblings = siblings[1:]
		}
		idx /= 2
		count = (count + 1) / 2
	}

	return bytes.Equal(computed, root)
}

func (mt *MerkleTrie) insertLocked(key, value []byte) error {
	if err := mt.db.Put(append([]byte("entry:"), key...), value); err != nil {
		return fmt.Errorf("failed to store entry: %w", err)
	}
	return mt.rebuild()
}

func (mt *MerkleTrie) deleteLocked(key []byte) error {
	if err := mt.db.Delete(append([]byte("entry:"), key...)); err != nil {
		return err
	}
	return mt.rebuild()
}

func (mt *MerkleTrie) rebuild() error {
	entries, err := mt.loadEntries()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		mt.root = nil
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].key, entries[j].key) < 0
	})

	current := make([][]byte, len(entries))
	for i, e := range entries {
		node := &TrieNode{
			Key:    e.key,
			Value:  e.value,
			IsLeaf: true,
		}
		h := hashNode(node)
		data := serializeNode(node)
		if err := mt.db.Put(append([]byte("trie:"), h...), data); err != nil {
			return fmt.Errorf("failed to store leaf: %w", err)
		}
		current[i] = h
	}

	for len(current) > 1 {
		var next [][]byte
		for i := 0; i < len(current); i += 2 {
			left := current[i]
			right := current[i]
			if i+1 < len(current) {
				right = current[i+1]
			}
			h := hashNodes(left, right)
			node := &TrieNode{
				LeftHash:  left,
				RightHash: right,
				IsLeaf:    false,
			}
			data := serializeNode(node)
			if err := mt.db.Put(append([]byte("trie:"), h...), data); err != nil {
				return fmt.Errorf("failed to store branch: %w", err)
			}
			next = append(next, h)
		}
		current = next
	}

	mt.root = current[0]
	return nil
}

func (mt *MerkleTrie) loadEntries() ([]entry, error) {
	iterStore, ok := mt.db.(IterableKVStore)
	if !ok {
		return nil, fmt.Errorf("store does not support iteration")
	}

	iter, err := iterStore.Iterator([]byte("entry:"))
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var entries []entry
	for iter.Next() {
		key := iter.Key()
		val := iter.Value()
		if len(key) <= 6 {
			continue
		}
		ek := make([]byte, len(key)-6)
		copy(ek, key[6:])
		ev := make([]byte, len(val))
		copy(ev, val)
		entries = append(entries, entry{key: ek, value: ev})
	}
	return entries, iter.Error()
}

func (mt *MerkleTrie) getProof(key []byte) ([][]byte, error) {
	if mt.root == nil {
		return nil, fmt.Errorf("trie not initialized")
	}

	entries, err := mt.loadEntries()
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].key, entries[j].key) < 0
	})

	leafIdx := -1
	for i, e := range entries {
		if bytes.Equal(e.key, key) {
			leafIdx = i
			break
		}
	}
	if leafIdx == -1 {
		return nil, fmt.Errorf("key not found")
	}

	hashes := make([][]byte, len(entries))
	for i, e := range entries {
		hashes[i] = hashLeaf(e.key, e.value)
	}

	var proof [][]byte
	idx := leafIdx
	current := hashes

	for len(current) > 1 {
		var next [][]byte
		for i := 0; i < len(current); i += 2 {
			left := current[i]
			right := current[i]
			if i+1 < len(current) {
				right = current[i+1]
			}

			if i == idx {
				if i+1 < len(current) {
					proof = append(proof, right)
				}
			} else if i+1 == idx {
				proof = append(proof, left)
			}

			next = append(next, hashNodes(left, right))
		}
		idx /= 2
		current = next
	}

	meta := make([]byte, 8)
	binary.BigEndian.PutUint32(meta[0:4], uint32(leafIdx))
	binary.BigEndian.PutUint32(meta[4:8], uint32(len(entries)))
	proof = append(proof, meta)

	return proof, nil
}

func hashNode(node *TrieNode) []byte {
	if node.IsLeaf {
		return hashLeaf(node.Key, node.Value)
	}
	return hashNodes(node.LeftHash, node.RightHash)
}

func hashLeaf(key, value []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(key)
	h.Write(value)
	return h.Sum(nil)
}

func hashNodes(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

func hashEmptyTrie() []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	return h.Sum(nil)
}

func serializeNode(node *TrieNode) []byte {
	if node.IsLeaf {
		data := []byte{0x01}
		kl := make([]byte, 4)
		binary.BigEndian.PutUint32(kl, uint32(len(node.Key)))
		data = append(data, kl...)
		data = append(data, node.Key...)
		vl := make([]byte, 4)
		binary.BigEndian.PutUint32(vl, uint32(len(node.Value)))
		data = append(data, vl...)
		data = append(data, node.Value...)
		return data
	}

	data := []byte{0x00}
	data = append(data, node.LeftHash...)
	data = append(data, node.RightHash...)
	return data
}

func deserializeNode(data []byte) (*TrieNode, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("empty node data")
	}

	isLeaf := data[0] == 0x01
	pos := 1

	if isLeaf {
		if pos+4 > len(data) {
			return nil, fmt.Errorf("truncated key length")
		}
		keyLen := int(binary.BigEndian.Uint32(data[pos:]))
		pos += 4
		if uint32(pos)+uint32(keyLen) > uint32(len(data)) {
			return nil, fmt.Errorf("key length %d exceeds data", keyLen)
		}
		key := make([]byte, keyLen)
		copy(key, data[pos:pos+keyLen])
		pos += keyLen

		if pos+4 > len(data) {
			return nil, fmt.Errorf("truncated value length")
		}
		valLen := int(binary.BigEndian.Uint32(data[pos:]))
		pos += 4
		if uint32(pos)+uint32(valLen) > uint32(len(data)) {
			return nil, fmt.Errorf("value length %d exceeds data", valLen)
		}
		value := make([]byte, valLen)
		copy(value, data[pos:pos+valLen])

		return &TrieNode{
			Key:    key,
			Value:  value,
			IsLeaf: true,
		}, nil
	}

	if pos+64 > len(data) {
		return nil, fmt.Errorf("truncated branch node: need 64 bytes for children, have %d", len(data)-pos)
	}

	left := make([]byte, 32)
	copy(left, data[pos:pos+32])
	right := make([]byte, 32)
	copy(right, data[pos+32:pos+64])

	return &TrieNode{
		LeftHash:  left,
		RightHash: right,
		IsLeaf:    false,
	}, nil
}

func (mt *MerkleTrie) RootHex() string {
	return hex.EncodeToString(mt.Root())
}

func (mt *MerkleTrie) Size() int {
	mt.mu.RLock()
	if mt.root == nil {
		mt.mu.RUnlock()
		return 0
	}
	mt.mu.RUnlock()

	entries, err := mt.loadEntries()
	if err != nil {
		return 0
	}
	return len(entries)
}
