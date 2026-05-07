package ledger

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func NewTransaction(nonce uint64, from, to []byte, value, gasLimit, gasPrice uint64, data []byte, key *crypto.PrivateKey) (*Transaction, error) {
	tx := &Transaction{
		Nonce:    nonce,
		From:     from,
		To:       to,
		Value:    value,
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	}

	payload := tx.SigningPayload()
	sig, err := key.Sign(payload)
	if err != nil {
		return nil, err
	}

	tx.Signature = &TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
		V: 0,
	}

	tx.Hash = tx.ComputeHash()
	return tx, nil
}

func NewTransactionFromKey(nonce uint64, to []byte, value, gasLimit, gasPrice uint64, data []byte, key *crypto.PrivateKey) (*Transaction, error) {
	pubKeyBytes := key.PubKey().Bytes()

	tx := &Transaction{
		Nonce:    nonce,
		From:     pubKeyBytes,
		To:       to,
		Value:    value,
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	}

	payload := tx.SigningPayload()
	sig, err := key.Sign(payload)
	if err != nil {
		return nil, err
	}

	tx.Signature = &TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
		V: 0,
	}

	tx.Hash = tx.ComputeHash()
	return tx, nil
}

func (tx *Transaction) SigningPayload() []byte {
	payload := make([]byte, 0)
	payload = append(payload, uint64ToBytes(tx.Nonce)...)
	payload = append(payload, tx.From...)
	payload = append(payload, tx.To...)
	payload = append(payload, uint64ToBytes(tx.Value)...)
	payload = append(payload, uint64ToBytes(tx.GasLimit)...)
	payload = append(payload, uint64ToBytes(tx.GasPrice)...)
	payload = append(payload, tx.Data...)
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
	pubKey := &crypto.PublicKey{
		PublicKey: &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(tx.From[1:33]),
			Y:     new(big.Int).SetBytes(tx.From[33:]),
		},
	}
	return pubKey.Address()
}

func (tx *Transaction) Verify() bool {
	if tx.Signature == nil || len(tx.From) < 65 {
		return false
	}

	sig := &crypto.Signature{
		R: new(big.Int).SetBytes(tx.Signature.R),
		S: new(big.Int).SetBytes(tx.Signature.S),
	}

	pubKey := &crypto.PublicKey{
		PublicKey: &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(tx.From[1:33]),
			Y:     new(big.Int).SetBytes(tx.From[33:]),
		},
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
