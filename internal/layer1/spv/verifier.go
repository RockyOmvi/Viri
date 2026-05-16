package spv

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/state"
)

type SPVVerifier struct {
}

func NewSPVVerifier() *SPVVerifier {
	return &SPVVerifier{}
}

type InclusionProof struct {
	TxHash    string   `json:"tx_hash"`
	BlockHash string   `json:"block_hash"`
	Height    uint64   `json:"height"`
	Index     int      `json:"index"`
	MerkleProof [][]byte `json:"merkle_proof"`
}

type StateProof struct {
	Address   string   `json:"address"`
	RootHash  string   `json:"root_hash"`
	Balance   string   `json:"balance"`
	Nonce     uint64   `json:"nonce"`
	Proof     [][]byte `json:"proof"`
}

func (spv *SPVVerifier) VerifyTransactionInclusion(proof *InclusionProof, blockTxHashes []string) bool {
	if proof == nil {
		return false
	}
	if len(proof.MerkleProof) == 0 {
		return false
	}

	// Verify the claimed tx hash actually appears in the block
	found := false
	for _, txHash := range blockTxHashes {
		if txHash == proof.TxHash {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	computedBytes, err := hex.DecodeString(proof.TxHash)
	if err != nil {
		return false
	}
	if len(computedBytes) != 32 {
		return false
	}

	for _, sibling := range proof.MerkleProof {
		var left, right []byte
		if bytes.Compare(computedBytes, sibling) <= 0 {
			left = computedBytes
			right = sibling
		} else {
			left = sibling
			right = computedBytes
		}

		combined := append([]byte{0x00}, left...)
		combined = append(combined, right...)
		computedBytes = hashData(combined)
	}

	rootBytes, err := hex.DecodeString(proof.BlockHash)
	if err != nil {
		return false
	}

	return bytes.Equal(computedBytes, rootBytes)
}

func (spv *SPVVerifier) VerifyStateProof(proof *StateProof, trie *state.MerkleTrie) bool {
	if proof == nil {
		return false
	}

	claimedRoot, err := hex.DecodeString(proof.RootHash)
	if err != nil {
		return false
	}

	addrBytes, err := hex.DecodeString(proof.Address)
	if err != nil {
		return false
	}

	return trie.VerifyProof(claimedRoot, addrBytes, []byte(proof.Balance), proof.Proof)
}

func (spv *SPVVerifier) GenerateTransactionMerkleProof(txs [][]byte, targetIndex int) ([][]byte, error) {
	if targetIndex < 0 || targetIndex >= len(txs) {
		return nil, fmt.Errorf("invalid transaction index")
	}

	hashes := make([][]byte, len(txs))
	for i, tx := range txs {
		hashes[i] = hashData(tx)
	}

	return buildMerkleProof(hashes, targetIndex), nil
}

func buildMerkleProof(hashes [][]byte, index int) [][]byte {
	if len(hashes) == 0 {
		return nil
	}

	var proof [][]byte
	current := make([][]byte, len(hashes))
	copy(current, hashes)

	idx := index

	for len(current) > 1 {
		if idx%2 == 0 {
			if idx+1 < len(current) {
				proof = append(proof, current[idx+1])
			}
		} else {
			proof = append(proof, current[idx-1])
		}

		var next [][]byte
		for i := 0; i < len(current); i += 2 {
			if i+1 < len(current) {
				left := current[i]
				right := current[i+1]
				if bytes.Compare(left, right) > 0 {
					left, right = right, left
				}
				combined := append([]byte{0x00}, left...)
				combined = append(combined, right...)
				next = append(next, hashData(combined))
			} else {
				next = append(next, current[i])
			}
		}

		current = next
		idx /= 2
	}

	return proof
}

func hashData(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func doubleHash(data []byte) []byte {
	return crypto.DoubleSHA256(data)
}

func (spv *SPVVerifier) VerifyBlockHeader(header []byte, trustedRoot []byte) bool {
	if len(header) == 0 || len(trustedRoot) == 0 {
		return false
	}
	computedHash := doubleHash(header)
	return bytes.Equal(computedHash, trustedRoot)
}

func (spv *SPVVerifier) BuildLightClientProof(txs [][]byte, txIndex int, address []byte, balance []byte, trie *state.MerkleTrie) (*LightClientProof, error) {
	if txIndex < 0 || txIndex >= len(txs) {
		return nil, fmt.Errorf("invalid transaction index %d (txs len=%d)", txIndex, len(txs))
	}
	txProof, err := spv.GenerateTransactionMerkleProof(txs, txIndex)
	if err != nil {
		return nil, err
	}

	stateProof, err := trie.Prove(address)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state proof: %w", err)
	}

	stateRoot := make([]byte, len(trie.Root()))
	copy(stateRoot, trie.Root())

	return &LightClientProof{
		TxMerkleProof:    txProof,
		StateMerkleProof: stateProof,
		StateRoot:        stateRoot,
	}, nil
}

type LightClientProof struct {
	TxMerkleProof    [][]byte `json:"tx_merkle_proof"`
	StateMerkleProof [][]byte `json:"state_merkle_proof"`
	StateRoot        []byte   `json:"state_root"`
}
