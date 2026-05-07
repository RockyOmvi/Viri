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
