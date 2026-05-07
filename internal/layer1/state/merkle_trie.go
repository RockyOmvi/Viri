package state

import (
	"bytes"
	"crypto/sha256"
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
	Key      []byte
	Value    []byte
	Children map[string][]byte
	IsLeaf   bool
}

func NewMerkleTrie(db KVStore) *MerkleTrie {
	return &MerkleTrie{
		db: db,
	}
}

func (mt *MerkleTrie) Update(key, value []byte) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if len(value) == 0 {
		return mt.delete(key)
	}

	return mt.insert(key, value)
}

func (mt *MerkleTrie) Get(key []byte) ([]byte, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	return mt.lookup(key)
}

func (mt *MerkleTrie) Delete(key []byte) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	return mt.delete(key)
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
	computed := hashLeaf(key, value)

	for i := len(proof) - 1; i >= 0; i-- {
		layerHash := proof[i]
		if bytes.Equal(layerHash, computed) {
			continue
		}

		left := hashLeaf(nil, nil)
		if len(proof) > i+1 {
			left = proof[i+1]
		}

		computed = hashNodes(left, computed)
	}

	return bytes.Equal(computed, root)
}

func (mt *MerkleTrie) insert(key, value []byte) error {
	node := &TrieNode{
		Key:      key,
		Value:    value,
		Children: make(map[string][]byte),
		IsLeaf:   true,
	}

	nodeHash := hashNode(node)

	if err := mt.db.Put(append([]byte("trie:"), nodeHash...), serializeNode(node)); err != nil {
		return fmt.Errorf("failed to store trie node: %w", err)
	}

	mt.root = nodeHash
	return nil
}

func (mt *MerkleTrie) lookup(key []byte) ([]byte, error) {
	if mt.root == nil {
		return nil, fmt.Errorf("trie not initialized")
	}

	data, err := mt.db.Get(append([]byte("trie:"), mt.root...))
	if err != nil {
		return nil, fmt.Errorf("root node not found")
	}

	node, err := deserializeNode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize root node: %w", err)
	}

	if node.IsLeaf && bytes.Equal(node.Key, key) {
		return node.Value, nil
	}

	return nil, fmt.Errorf("key not found")
}

func (mt *MerkleTrie) delete(key []byte) error {
	if mt.root == nil {
		return nil
	}

	mt.root = nil
	return nil
}

func (mt *MerkleTrie) getProof(key []byte) ([][]byte, error) {
	if mt.root == nil {
		return nil, fmt.Errorf("trie not initialized")
	}

	return [][]byte{mt.root}, nil
}

func hashNode(node *TrieNode) []byte {
	h := sha256.New()
	h.Write(node.Key)
	h.Write(node.Value)

	keys := make([]string, 0, len(node.Children))
	for k := range node.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(k))
		h.Write(node.Children[k])
	}

	if node.IsLeaf {
		h.Write([]byte{0x01})
	}

	return h.Sum(nil)
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
	data := []byte{}

	data = append(data, byte(len(node.Key)))
	data = append(data, node.Key...)

	data = append(data, byte(len(node.Value)))
	data = append(data, node.Value...)

	if node.IsLeaf {
		data = append(data, 0x01)
	} else {
		data = append(data, 0x00)
	}

	data = append(data, byte(len(node.Children)))
	for k, v := range node.Children {
		data = append(data, []byte(k)...)
		data = append(data, ':')
		data = append(data, v...)
		data = append(data, ';')
	}

	return data
}

func deserializeNode(data []byte) (*TrieNode, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty node data")
	}

	pos := 0
	keyLen := int(data[pos])
	pos++

	key := data[pos : pos+keyLen]
	pos += keyLen

	valLen := int(data[pos])
	pos++

	value := data[pos : pos+valLen]
	pos += valLen

	isLeaf := data[pos] == 0x01
	pos++

	childrenCount := int(data[pos])
	pos++

	children := make(map[string][]byte)
	for i := 0; i < childrenCount; i++ {
		colonIdx := bytes.IndexByte(data[pos:], ':')
		if colonIdx == -1 {
			break
		}
		childKey := string(data[pos : pos+colonIdx])
		pos += colonIdx + 1

		semiIdx := bytes.IndexByte(data[pos:], ';')
		if semiIdx == -1 {
			break
		}
		childHash := data[pos : pos+semiIdx]
		pos += semiIdx + 1

		children[childKey] = childHash
	}

	return &TrieNode{
		Key:      key,
		Value:    value,
		Children: children,
		IsLeaf:   isLeaf,
	}, nil
}

func (mt *MerkleTrie) RootHex() string {
	return hex.EncodeToString(mt.Root())
}

func (mt *MerkleTrie) Size() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	if mt.root == nil {
		return 0
	}
	return 1
}
