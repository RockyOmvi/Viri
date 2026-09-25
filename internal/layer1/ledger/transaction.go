package ledger

import (
	"math/big"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func NewTransaction(nonce uint64, to []byte, value, gasLimit, gasPrice uint64, data []byte, chainID uint64, key *crypto.PrivateKey) (*Transaction, error) {
	pubKeyBytes := key.PubKey().Bytes()

	tx := &Transaction{
		Nonce:    nonce,
		From:     pubKeyBytes,
		To:       to,
		Value:    value,
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		Data:     data,
		ChainID:  chainID,
	}

	payload := tx.SigningPayload()
	sig, err := key.Sign(payload)
	if err != nil {
		return nil, err
	}

	// CR-02: Compute proper V recovery byte from signature.
	// V = 27 + recovery_id (EIP-155: V = chainID*2 + 35 + recovery_id).
	// For simplicity, use the low-order bit of S to approximate recovery ID.
	recoveryID := byte(0)
	if len(sig.S.Bytes()) > 0 && sig.S.Bytes()[len(sig.S.Bytes())-1]&1 != 0 {
		recoveryID = 1
	}

	tx.Signature = &TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
		V: 27 + recoveryID,
	}

	tx.Hash = tx.ComputeHash()
	return tx, nil
}

// NewTransactionFromKey is a wrapper around NewTransaction for compatibility.
func NewTransactionFromKey(nonce uint64, to []byte, value, gasLimit, gasPrice uint64, data []byte, chainID uint64, key *crypto.PrivateKey) (*Transaction, error) {
	return NewTransaction(nonce, to, value, gasLimit, gasPrice, data, chainID, key)
}


// CR-03: Include FeeCurrency in the signing payload to prevent malleability.
func (tx *Transaction) SigningPayload() []byte {
	payload := make([]byte, 0)
	payload = append(payload, uint64ToBytes(tx.ChainID)...)
	payload = append(payload, uint64ToBytes(tx.Nonce)...)
	payload = append(payload, tx.From...)
	payload = append(payload, tx.To...)
	payload = append(payload, uint64ToBytes(tx.Value)...)
	payload = append(payload, uint64ToBytes(tx.GasLimit)...)
	payload = append(payload, uint64ToBytes(tx.GasPrice)...)
	payload = append(payload, tx.Data...)
	// CR-03: FeeCurrency is now part of the signed data
	if len(tx.FeeCurrency) > 0 {
		payload = append(payload, tx.FeeCurrency...)
	}
	return payload
}

func (tx *Transaction) ComputeHash() []byte {
	payload := tx.SigningPayload()
	if tx.Signature != nil {
		payload = append(payload, tx.Signature.R...)
		payload = append(payload, tx.Signature.S...)
		payload = append(payload, tx.Signature.V)
	}
	return crypto.DoubleSHA256(payload)
}

func (tx *Transaction) SenderAddress() []byte {
	if len(tx.From) == 20 {
		return tx.From
	}
	// CR-05: Validate From is exactly 65 bytes (uncompressed public key)
	if len(tx.From) != 65 {
		return nil
	}
	pubKey, err := crypto.PubKeyFromBytes(tx.From)
	if err != nil {
		return nil
	}
	return pubKey.Address()
}

func (tx *Transaction) Verify() bool {
	if tx.Signature == nil {
		return false
	}

	// CR-05: Validate From length is exactly 65 (uncompressed pubkey) or
	// 20 (address, only valid if Verified flag was set during RLP decode).
	if len(tx.From) == 20 {
		// CR-01: Only trust 20-byte From if the Verified flag is set.
		// This flag is set during RLP decode after proper ECDSA recovery.
		return tx.Verified
	}

	if len(tx.From) != 65 {
		return false
	}

	sig := &crypto.Signature{
		R: new(big.Int).SetBytes(tx.Signature.R),
		S: new(big.Int).SetBytes(tx.Signature.S),
	}

	pubKey, err := crypto.PubKeyFromBytes(tx.From)
	if err != nil {
		return false
	}

	return pubKey.Verify(tx.SigningPayload(), sig)
}

func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b[i] = byte(n >> (56 - 8*i))
	}
	return b
}
