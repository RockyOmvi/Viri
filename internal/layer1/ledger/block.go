package ledger

import (
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func headerTimestamp() time.Time {
	return time.Now()
}

func NewBlock(height uint64, prevHash []byte, txs []*Transaction, proposer []byte, key *crypto.PrivateKey) (*Block, error) {
	txsHash := ComputeTransactionsHash(txs)

	header := &Header{
		Version:      Version1,
		Height:       height,
		PrevHash:     prevHash,
		TxsHash:      txsHash,
		Timestamp:    headerTimestamp(),
		Proposer:     proposer,
	}

	block := &Block{
		Header:       header,
		Transactions: txs,
	}

	payload := block.SigningPayload()
	sig, err := key.Sign(payload)
	if err != nil {
		return nil, err
	}
	block.Header.Signature = sig.Bytes()

	return block, nil
}

func (b *Block) SigningPayload() []byte {
	payload := make([]byte, 0)
	payload = append(payload, byte(b.Header.Version))
	payload = append(payload, uint64ToBytes(b.Header.Height)...)
	payload = append(payload, b.Header.PrevHash...)
	payload = append(payload, b.Header.TxsHash...)
	payload = append(payload, uint64ToBytes(uint64(b.Header.Timestamp.Unix()))...)
	payload = append(payload, b.Header.Proposer...)
	return payload
}

func (b *Block) Hash() []byte {
	if len(b.ConsensusHash) > 0 {
		return b.ConsensusHash
	}
	return crypto.DoubleSHA256(b.SigningPayload())
}

func (b *Block) Verify() bool {
	if b.Header == nil {
		return false
	}

	if len(b.Header.PrevHash) == 0 && b.Header.Height > 0 {
		return false
	}

	// Reject timestamps more than 1 hour in the future (clock drift tolerance)
	if time.Since(b.Header.Timestamp) < -time.Hour {
		return false
	}

	// Genesis block (height 0) is trusted; only check signature on non-genesis blocks
	if b.Header.Height > 0 && len(b.Header.Signature) != 64 {
		return false
	}

	for _, tx := range b.Transactions {
		if !tx.Verify() {
			return false
		}
	}

	computedTxsHash := ComputeTransactionsHash(b.Transactions)
	if !crypto.EqualHash(computedTxsHash, b.Header.TxsHash) {
		return false
	}

	return true
}

// VerifyWithProposer verifies the block including the proposer's signature.
// The pubKey must be the 65-byte uncompressed public key of the block proposer.
func (b *Block) VerifyWithProposer(pubKey []byte) bool {
	if !b.Verify() {
		return false
	}
	if len(pubKey) == 0 {
		return false
	}
	payload := b.SigningPayload()
	sig := b.Header.Signature
	pub, err := crypto.PubKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	return pub.VerifyMessage(payload, sig)
}

func ComputeTransactionsHash(txs []*Transaction) []byte {
	if len(txs) == 0 {
		return crypto.SHA256([]byte{})
	}

	var hashes [][]byte
	for _, tx := range txs {
		hashes = append(hashes, tx.Hash)
	}

	tree, err := crypto.NewMerkleTree(hashes)
	if err != nil {
		return crypto.SHA256([]byte{})
	}

	return tree.RootHash
}
