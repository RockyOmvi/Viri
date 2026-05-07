package crypto

type MerkleTree struct {
	RootHash []byte
	Leaves   [][]byte
}

func NewMerkleTree(data [][]byte) (*MerkleTree, error) {
	if len(data) == 0 {
		return &MerkleTree{}, nil
	}

	hashes := make([][]byte, len(data))
	for i, d := range data {
		hashes[i] = SHA256(d)
	}

	for len(hashes) > 1 {
		if len(hashes)%2 != 0 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		var nextLevel [][]byte
		for i := 0; i < len(hashes); i += 2 {
			combined := append(hashes[i], hashes[i+1]...)
			nextLevel = append(nextLevel, SHA256(combined))
		}
		hashes = nextLevel
	}

	return &MerkleTree{
		RootHash: hashes[0],
		Leaves:   data,
	}, nil
}

func (m *MerkleTree) GenerateProof(index int) ([][]byte, error) {
	if index < 0 || index >= len(m.Leaves) {
		return nil, ErrInvalidKey
	}

	var proof [][]byte
	currentIndex := index

	hashes := make([][]byte, len(m.Leaves))
	for i := range m.Leaves {
		hashes[i] = SHA256(m.Leaves[i])
	}

	for len(hashes) > 1 {
		if len(hashes)%2 != 0 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		var nextLevel [][]byte
		for i := 0; i < len(hashes); i += 2 {
			if i == currentIndex && i+1 < len(hashes) {
				proof = append(proof, hashes[i+1])
			} else if i+1 == currentIndex {
				proof = append(proof, hashes[i])
			}
			combined := append(hashes[i], hashes[i+1]...)
			nextLevel = append(nextLevel, SHA256(combined))
		}

		if currentIndex%2 == 0 {
			currentIndex = currentIndex / 2
		} else {
			currentIndex = (currentIndex - 1) / 2
		}
		hashes = nextLevel
	}

	return proof, nil
}

func VerifyProof(rootHash []byte, data []byte, proof [][]byte, index int) bool {
	currentHash := SHA256(data)

	for _, proofHash := range proof {
		if index%2 == 0 {
			combined := append(currentHash, proofHash...)
			currentHash = SHA256(combined)
		} else {
			combined := append(proofHash, currentHash...)
			currentHash = SHA256(combined)
		}
		index /= 2
	}

	return EqualHash(currentHash, rootHash)
}

func EqualHash(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
