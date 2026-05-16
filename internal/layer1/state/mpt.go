package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
)

// MerklePatriciaTrie is a compact authenticated trie with branch, extension, and leaf nodes.
// Paths are represented as nibble (hex character) sequences.
type MerklePatriciaTrie struct {
	mu       sync.RWMutex
	rootHash []byte
	db       KVStore
}

// Node types
const (
	nodeTypeBranch    = 0x02
	nodeTypeExtension = 0x03
	nodeTypeLeaf      = 0x04
)

// NewMPT creates a new Merkle-Patricia Trie backed by the given KVStore.
func NewMPT(db KVStore) *MerklePatriciaTrie {
	return &MerklePatriciaTrie{db: db}
}

func (m *MerklePatriciaTrie) Root() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rootHash == nil {
		return hashEmptyMPT()
	}
	return m.rootHash
}

func (m *MerklePatriciaTrie) RootHex() string {
	return fmt.Sprintf("%x", m.Root())
}

// Get retrieves a value by key. Returns error if not found.
func (m *MerklePatriciaTrie) Get(key []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rootHash == nil {
		return nil, fmt.Errorf("mpt: empty trie")
	}
	path := keyToNibbles(key)
	return m.getNode(m.rootHash, path)
}

// Update inserts or updates a key-value pair. Empty value deletes.
func (m *MerklePatriciaTrie) Update(key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(value) == 0 {
		return m.delete(key)
	}
	path := keyToNibbles(key)
	newRoot, err := m.upsert(m.rootHash, path, value)
	if err != nil {
		return err
	}
	m.rootHash = newRoot
	return nil
}

// Delete removes a key from the trie.
func (m *MerklePatriciaTrie) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.delete(key)
}

func (m *MerklePatriciaTrie) delete(key []byte) error {
	if m.rootHash == nil {
		return nil
	}
	path := keyToNibbles(key)
	newRoot, err := m.remove(m.rootHash, path)
	if err != nil {
		return err
	}
	m.rootHash = newRoot
	return nil
}

// keyToNibbles converts a byte key to a nibble (hex char) slice.
func keyToNibbles(key []byte) []byte {
	nibbles := make([]byte, 0, len(key)*2)
	for _, b := range key {
		nibbles = append(nibbles, b>>4, b&0x0f)
	}
	return nibbles
}

// nibblesToKey converts nibbles back to bytes (padded).
func nibblesToKey(nibbles []byte) []byte {
	key := make([]byte, (len(nibbles)+1)/2)
	for i := 0; i < len(nibbles); i++ {
		if i%2 == 0 {
			key[i/2] = nibbles[i] << 4
		} else {
			key[i/2] |= nibbles[i]
		}
	}
	return key
}

// commonPrefixLen returns the length of the common nibble prefix.
func commonPrefixLen(a, b []byte) int {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	for i := 0; i < max; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return max
}

// --- Node serialization ---

func hashEmptyMPT() []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	return h.Sum(nil)
}

type hashEntry struct {
	hash   []byte
	nibble byte
}

type node interface {
	serialize() []byte
}

type branchMPTNode struct {
	children [16]hashEntry
	value    []byte
}

type extensionMPTNode struct {
	sharedNibbles []byte
	childHash     []byte
}

type leafMPTNode struct {
	path  []byte
	value []byte
}

// --- Trie operations ---

func (m *MerklePatriciaTrie) getNode(nodeHash []byte, path []byte) ([]byte, error) {
	if len(nodeHash) == 0 {
		return nil, fmt.Errorf("mpt: key not found")
	}
	data, err := m.db.Get(append([]byte("mpt:"), nodeHash...))
	if err != nil {
		return nil, fmt.Errorf("mpt: node not found")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("mpt: empty node")
	}
	switch data[0] {
	case nodeTypeLeaf:
		return m.getLeaf(data, path)
	case nodeTypeExtension:
		return m.getExtension(data, path)
	case nodeTypeBranch:
		return m.getBranch(data, path)
	default:
		return nil, fmt.Errorf("mpt: unknown node type %x", data[0])
	}
}

func (m *MerklePatriciaTrie) getLeaf(data []byte, path []byte) ([]byte, error) {
	nibbles, value := parseLeafData(data)
	if bytes.Equal(nibbles, path) {
		return value, nil
	}
	return nil, fmt.Errorf("mpt: key not found")
}

func (m *MerklePatriciaTrie) getExtension(data []byte, path []byte) ([]byte, error) {
	shared, childHash := parseExtensionData(data)
	if len(path) < len(shared) || !bytes.Equal(path[:len(shared)], shared) {
		return nil, fmt.Errorf("mpt: key not found")
	}
	return m.getNode(childHash, path[len(shared):])
}

func (m *MerklePatriciaTrie) getBranch(data []byte, path []byte) ([]byte, error) {
	children, value := parseBranchData(data)
	if len(path) == 0 {
		if value != nil {
			return value, nil
		}
		return nil, fmt.Errorf("mpt: key not found")
	}
	idx := path[0]
	if idx >= 16 {
		return nil, fmt.Errorf("mpt: invalid nibble")
	}
	if children[idx].hash == nil {
		return nil, fmt.Errorf("mpt: key not found")
	}
	return m.getNode(children[idx].hash, path[1:])
}

func (m *MerklePatriciaTrie) upsert(nodeHash []byte, path []byte, value []byte) ([]byte, error) {
	if len(nodeHash) == 0 {
		// Create new leaf
		return m.storeNode(&leafMPTNode{path: path, value: value})
	}
	data, err := m.db.Get(append([]byte("mpt:"), nodeHash...))
	if err != nil {
		return nil, fmt.Errorf("mpt: node not found: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("mpt: empty node data")
	}
	switch data[0] {
	case nodeTypeLeaf:
		return m.upsertLeaf(data, path, value)
	case nodeTypeExtension:
		return m.upsertExtension(data, path, value)
	case nodeTypeBranch:
		return m.upsertBranch(data, path, value)
	default:
		return nil, fmt.Errorf("mpt: unknown node type %x", data[0])
	}
}

func (m *MerklePatriciaTrie) upsertLeaf(data []byte, path []byte, value []byte) ([]byte, error) {
	nibbles, oldValue := parseLeafData(data)
	if bytes.Equal(nibbles, path) {
		// Same path — update value
		return m.storeNode(&leafMPTNode{path: path, value: value})
	}
	cp := commonPrefixLen(nibbles, path)
	if cp == 0 {
		// No shared prefix — create branch node
		br := &branchMPTNode{}
		// Insert old leaf
		if len(nibbles) == 0 {
			br.value = oldValue
		} else {
			childHash, err := m.storeNode(&leafMPTNode{path: nibbles[1:], value: oldValue})
			if err != nil {
				return nil, err
			}
			br.children[nibbles[0]] = hashEntry{hash: childHash, nibble: nibbles[0]}
		}
		// Insert new leaf
		if len(path) == 0 {
			br.value = value
		} else {
			childHash, err := m.storeNode(&leafMPTNode{path: path[1:], value: value})
			if err != nil {
				return nil, err
			}
			br.children[path[0]] = hashEntry{hash: childHash, nibble: path[0]}
		}
		return m.storeNode(br)
	}
	// Shared prefix — create extension + branch
	sharedPath := nibbles[:cp]
	remainingOld := nibbles[cp:]
	remainingNew := path[cp:]

	br := &branchMPTNode{}
	if len(remainingOld) == 0 {
		br.value = oldValue
	} else {
		childHash, err := m.storeNode(&leafMPTNode{path: remainingOld[1:], value: oldValue})
		if err != nil {
			return nil, err
		}
		br.children[remainingOld[0]] = hashEntry{hash: childHash, nibble: remainingOld[0]}
	}
	if len(remainingNew) == 0 {
		br.value = value
	} else {
		childHash, err := m.storeNode(&leafMPTNode{path: remainingNew[1:], value: value})
		if err != nil {
			return nil, err
		}
		br.children[remainingNew[0]] = hashEntry{hash: childHash, nibble: remainingNew[0]}
	}

	branchHash, err := m.storeNode(br)
	if err != nil {
		return nil, err
	}

	if cp > 0 {
		return m.storeNode(&extensionMPTNode{sharedNibbles: sharedPath, childHash: branchHash})
	}
	return branchHash, nil
}

func (m *MerklePatriciaTrie) upsertExtension(data []byte, path []byte, value []byte) ([]byte, error) {
	shared, childHash := parseExtensionData(data)
	cp := commonPrefixLen(shared, path)

	if cp == len(shared) {
		// Full match — recurse into child
		newChild, err := m.upsert(childHash, path[cp:], value)
		if err != nil {
			return nil, err
		}
		return m.storeNode(&extensionMPTNode{sharedNibbles: shared, childHash: newChild})
	}

	// Partial match — need to split extension
	commonPath := shared[:cp]
	extRemaining := shared[cp:]
	newPathRemaining := path[cp:]

	br := &branchMPTNode{}
	// Preserve old child under its nibble — the existing childHash stays unchanged
	if len(extRemaining) == 1 {
		br.children[extRemaining[0]] = hashEntry{hash: childHash, nibble: extRemaining[0]}
	} else {
		// extRemaining has more nibbles — wrap in extension preserving the child
		extChild, err := m.storeNode(&extensionMPTNode{sharedNibbles: extRemaining[1:], childHash: childHash})
		if err != nil {
			return nil, err
		}
		br.children[extRemaining[0]] = hashEntry{hash: extChild, nibble: extRemaining[0]}
	}

	// Insert new leaf
	if err := m.insertIntoBranch(br, newPathRemaining, value); err != nil {
		return nil, err
	}

	branchHash, err := m.storeNode(br)
	if err != nil {
		return nil, err
	}

	if len(commonPath) == 0 {
		return branchHash, nil
	}
	return m.storeNode(&extensionMPTNode{sharedNibbles: commonPath, childHash: branchHash})
}

func (m *MerklePatriciaTrie) upsertBranch(data []byte, path []byte, value []byte) ([]byte, error) {
	children, oldValue := parseBranchData(data)
	br := &branchMPTNode{children: children, value: oldValue}

	if len(path) == 0 {
		// Update value at this branch
		br.value = value
		return m.storeNode(br)
	}

	idx := path[0]
	rest := path[1:]

	if br.children[idx].hash != nil {
		newChild, err := m.upsert(br.children[idx].hash, rest, value)
		if err != nil {
			return nil, err
		}
		br.children[idx] = hashEntry{hash: newChild, nibble: idx}
	} else {
		childHash, err := m.storeNode(&leafMPTNode{path: rest, value: value})
		if err != nil {
			return nil, err
		}
		br.children[idx] = hashEntry{hash: childHash, nibble: idx}
	}
	return m.storeNode(br)
}

func (m *MerklePatriciaTrie) insertIntoBranch(br *branchMPTNode, path []byte, value []byte) error {
	if len(path) == 0 {
		br.value = value
		return nil
	}
	idx := path[0]
	rest := path[1:]
	var err error
	if br.children[idx].hash != nil {
		br.children[idx].hash, err = m.upsert(br.children[idx].hash, rest, value)
	} else {
		br.children[idx].hash, err = m.storeNode(&leafMPTNode{path: rest, value: value})
	}
	br.children[idx].nibble = idx
	return err
}

func (m *MerklePatriciaTrie) remove(nodeHash []byte, path []byte) ([]byte, error) {
	if len(nodeHash) == 0 {
		return nil, nil
	}
	data, err := m.db.Get(append([]byte("mpt:"), nodeHash...))
	if err != nil {
		return nodeHash, nil // node doesn't exist — no-op
	}
	if len(data) == 0 {
		return nil, nil
	}
	switch data[0] {
	case nodeTypeLeaf:
		return m.deleteLeaf(data, path)
	case nodeTypeExtension:
		return m.deleteExtension(data, path)
	case nodeTypeBranch:
		return m.deleteBranch(data, path)
	default:
		return nodeHash, nil
	}
}

func (m *MerklePatriciaTrie) deleteLeaf(data []byte, path []byte) ([]byte, error) {
	nibbles, _ := parseLeafData(data)
	if bytes.Equal(nibbles, path) {
		return nil, nil // deleted
	}
	// Key doesn't match — return original hash unchanged
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil), nil
}

func (m *MerklePatriciaTrie) deleteExtension(data []byte, path []byte) ([]byte, error) {
	shared, childHash := parseExtensionData(data)
	if len(path) < len(shared) || !bytes.Equal(path[:len(shared)], shared) {
		h := sha256.New()
		h.Write(data)
		return h.Sum(nil), nil
	}
	newChild, err := m.remove(childHash, path[len(shared):])
	if err != nil {
		return nil, err
	}
	if newChild == nil {
		return nil, nil
	}
	// Try to collapse extension if child is a leaf
	cd, err := m.db.Get(append([]byte("mpt:"), newChild...))
	if err != nil {
		return m.storeNode(&extensionMPTNode{sharedNibbles: shared, childHash: newChild})
	}
	if len(cd) > 0 {
		switch cd[0] {
		case nodeTypeLeaf:
			cNibbles, cVal := parseLeafData(cd)
			combined := make([]byte, 0, len(shared)+len(cNibbles))
			combined = append(combined, shared...)
			combined = append(combined, cNibbles...)
			return m.storeNode(&leafMPTNode{path: combined, value: cVal})
		case nodeTypeExtension:
			extShared, extChild := parseExtensionData(cd)
			combined := make([]byte, 0, len(shared)+len(extShared))
			combined = append(combined, shared...)
			combined = append(combined, extShared...)
			return m.storeNode(&extensionMPTNode{sharedNibbles: combined, childHash: extChild})
		}
	}
	return m.storeNode(&extensionMPTNode{sharedNibbles: shared, childHash: newChild})
}

func (m *MerklePatriciaTrie) deleteBranch(data []byte, path []byte) ([]byte, error) {
	children, value := parseBranchData(data)
	br := &branchMPTNode{children: children, value: value}

	if len(path) == 0 {
		br.value = nil
	} else {
		idx := path[0]
		if br.children[idx].hash == nil {
			h := sha256.New()
			h.Write(data)
			return h.Sum(nil), nil
		}
		newChild, err := m.remove(br.children[idx].hash, path[1:])
		if err != nil {
			return nil, err
		}
		if newChild == nil {
			br.children[idx] = hashEntry{}
		} else {
			br.children[idx] = hashEntry{hash: newChild, nibble: idx}
		}
	}

	// Count remaining children
	var remaining []struct {
		idx  byte
		hash []byte
	}
	hasValue := br.value != nil
	for i := 0; i < 16; i++ {
		if br.children[i].hash != nil {
			remaining = append(remaining, struct {
				idx  byte
				hash []byte
			}{byte(i), br.children[i].hash})
		}
	}

	switch {
	case hasValue && len(remaining) == 0:
		// Only value remains — convert to leaf with empty path
		return m.storeNode(&leafMPTNode{value: br.value})
	case !hasValue && len(remaining) == 1:
		// Only one child — try to collapse extension
		childData, err := m.db.Get(append([]byte("mpt:"), remaining[0].hash...))
		if err != nil || len(childData) == 0 {
			return m.storeNode(br)
		}
		if childData[0] == nodeTypeLeaf {
			nibbles, val := parseLeafData(childData)
			combined := make([]byte, 0, 1+len(nibbles))
			combined = append(combined, remaining[0].idx)
			combined = append(combined, nibbles...)
			return m.storeNode(&leafMPTNode{path: combined, value: val})
		}
		if childData[0] == nodeTypeExtension {
			shared, childHash := parseExtensionData(childData)
			combined := make([]byte, 0, 1+len(shared))
			combined = append(combined, remaining[0].idx)
			combined = append(combined, shared...)
			return m.storeNode(&extensionMPTNode{sharedNibbles: combined, childHash: childHash})
		}
		return m.storeNode(br)
	case !hasValue && len(remaining) == 0:
		return nil, nil
	default:
		return m.storeNode(br)
	}
}

// --- Node storage ---
// WARNING: Orphaned nodes are never deleted. Old node versions accumulate in the
// database on every Update/Delete. A production system needs reference counting
// or epoch-based garbage collection.

func (m *MerklePatriciaTrie) storeNode(n node) ([]byte, error) {
	data := n.serialize()
	hash := sha256.Sum256(data)
	if err := m.db.Put(append([]byte("mpt:"), hash[:]...), data); err != nil {
		return nil, err
	}
	return hash[:], nil
}

func (n *branchMPTNode) serialize() []byte {
	var buf bytes.Buffer
	buf.WriteByte(nodeTypeBranch)
	// children
	for i := 0; i < 16; i++ {
		if n.children[i].hash != nil {
			buf.WriteByte(byte(len(n.children[i].hash)))
			buf.Write(n.children[i].hash)
		} else {
			buf.WriteByte(0)
		}
	}
	// value — use explicit present flag so empty values are preserved
	hasValue := n.value != nil
	if hasValue {
		buf.WriteByte(0x01)
		vl := make([]byte, 4)
		binary.BigEndian.PutUint32(vl, uint32(len(n.value)))
		buf.Write(vl)
		buf.Write(n.value)
	} else {
		buf.WriteByte(0x00)
	}
	return buf.Bytes()
}

func (n *extensionMPTNode) serialize() []byte {
	var buf bytes.Buffer
	buf.WriteByte(nodeTypeExtension)
	pl := make([]byte, 4)
	binary.BigEndian.PutUint32(pl, uint32(len(n.sharedNibbles)))
	buf.Write(pl)
	buf.Write(n.sharedNibbles)
	cl := make([]byte, 4)
	binary.BigEndian.PutUint32(cl, uint32(len(n.childHash)))
	buf.Write(cl)
	buf.Write(n.childHash)
	return buf.Bytes()
}

func (n *leafMPTNode) serialize() []byte {
	var buf bytes.Buffer
	buf.WriteByte(nodeTypeLeaf)
	pl := make([]byte, 4)
	binary.BigEndian.PutUint32(pl, uint32(len(n.path)))
	buf.Write(pl)
	buf.Write(n.path)
	vl := make([]byte, 4)
	binary.BigEndian.PutUint32(vl, uint32(len(n.value)))
	buf.Write(vl)
	buf.Write(n.value)
	return buf.Bytes()
}

// --- Parsing ---

func parseLeafData(data []byte) (nibbles []byte, value []byte) {
	if len(data) < 9 {
		return nil, nil
	}
	pos := 1
	pl := binary.BigEndian.Uint32(data[pos:])
	pos += 4
	if uint32(pos)+pl > uint32(len(data)) {
		return nil, nil
	}
	nibbles = data[pos : pos+int(pl)]
	pos += int(pl)
	if pos+4 > len(data) {
		return nil, nil
	}
	vl := binary.BigEndian.Uint32(data[pos:])
	pos += 4
	if uint32(pos)+vl > uint32(len(data)) {
		return nil, nil
	}
	value = data[pos : pos+int(vl)]
	return nibbles, value
}

func parseExtensionData(data []byte) (shared []byte, childHash []byte) {
	if len(data) < 9 {
		return nil, nil
	}
	pos := 1
	pl := binary.BigEndian.Uint32(data[pos:])
	pos += 4
	if uint32(pos)+pl > uint32(len(data)) {
		return nil, nil
	}
	shared = data[pos : pos+int(pl)]
	pos += int(pl)
	if pos+4 > len(data) {
		return nil, nil
	}
	cl := binary.BigEndian.Uint32(data[pos:])
	pos += 4
	if uint32(pos)+cl > uint32(len(data)) {
		return nil, nil
	}
	childHash = data[pos : pos+int(cl)]
	return shared, childHash
}

func parseBranchData(data []byte) (children [16]hashEntry, value []byte) {
	if len(data) < 18 {
		return children, nil
	}
	pos := 1
	for i := 0; i < 16; i++ {
		hashLen := int(data[pos])
		pos++
		if hashLen > 0 && pos+hashLen <= len(data) {
			children[i] = hashEntry{
				hash:   append([]byte(nil), data[pos:pos+hashLen]...),
				nibble: byte(i),
			}
			pos += hashLen
		}
	}
	if pos < len(data) {
		hasValue := data[pos] == 0x01
		pos++
		if hasValue && pos+4 <= len(data) {
			vl := binary.BigEndian.Uint32(data[pos:])
			pos += 4
			if pos+int(vl) <= len(data) {
				value = data[pos : pos+int(vl)]
			}
		}
	}
	return children, value
}

// MPTNodeCount returns the number of nodes in the trie by walking the tree.
func (m *MerklePatriciaTrie) MPTNodeCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rootHash == nil {
		return 0
	}
	return countNodes(m.db, m.rootHash)
}

func countNodes(db KVStore, hash []byte) int {
	data, err := db.Get(append([]byte("mpt:"), hash...))
	if err != nil || len(data) == 0 {
		return 0
	}
	count := 1
	switch data[0] {
	case nodeTypeLeaf:
	case nodeTypeExtension:
		_, childHash := parseExtensionData(data)
		count += countNodes(db, childHash)
	case nodeTypeBranch:
		children, _ := parseBranchData(data)
		for i := 0; i < 16; i++ {
			if children[i].hash != nil {
				count += countNodes(db, children[i].hash)
			}
		}
	}
	return count
}

func (m *MerklePatriciaTrie) Has(key []byte) bool {
	_, err := m.Get(key)
	return err == nil
}
