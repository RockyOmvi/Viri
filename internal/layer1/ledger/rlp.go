package ledger

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	sececdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type RLPTx struct {
	Nonce    uint64
	GasPrice uint64
	GasLimit uint64
	To       []byte
	Value    uint64
	Data     []byte
	V        byte
	R        []byte
	S        []byte
	ChainID  uint64
	txType   byte // 0 = legacy, 1 = EIP-2930, 2 = EIP-1559
}

func decodeRLPItem(data []byte) ([]byte, []byte, error) {
	if len(data) == 0 {
		return nil, nil, errors.New("empty RLP data")
	}
	prefix := data[0]
	if prefix < 0x80 {
		return data[0:1], data[1:], nil
	}
	if prefix <= 0xB7 {
		length := int(prefix - 0x80)
		if len(data) < 1+length {
			return nil, nil, fmt.Errorf("short RLP string: need %d bytes, have %d", 1+length, len(data))
		}
		return data[1 : 1+length], data[1+length:], nil
	}
	if prefix <= 0xBF {
		lenLen := int(prefix - 0xB7)
		if len(data) < 1+lenLen {
			return nil, nil, errors.New("short RLP string length")
		}
		length := 0
		for i := 0; i < lenLen; i++ {
			length = (length << 8) | int(data[1+i])
		}
		if len(data) < 1+lenLen+length {
			return nil, nil, fmt.Errorf("short RLP string: need %d bytes, have %d", 1+lenLen+length, len(data))
		}
		return data[1+lenLen : 1+lenLen+length], data[1+lenLen+length:], nil
	}
	if prefix <= 0xF7 {
		length := int(prefix - 0xC0)
		if len(data) < 1+length {
			return nil, nil, fmt.Errorf("short RLP list: need %d bytes, have %d", 1+length, len(data))
		}
		return data[1 : 1+length], data[1+length:], nil
	}
	lenLen := int(prefix - 0xF7)
	if len(data) < 1+lenLen {
		return nil, nil, errors.New("short RLP list length")
	}
	length := 0
	for i := 0; i < lenLen; i++ {
		length = (length << 8) | int(data[1+i])
	}
	if len(data) < 1+lenLen+length {
		return nil, nil, fmt.Errorf("short RLP list: need %d bytes, have %d", 1+lenLen+length, len(data))
	}
	return data[1+lenLen : 1+lenLen+length], data[1+lenLen+length:], nil
}

func decodeRLPBigInt(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	var val uint64
	for _, b := range data {
		val = (val << 8) | uint64(b)
	}
	return val
}

func DecodeRLPTransaction(raw []byte) (*RLPTx, error) {
	// Handle EIP-2718 typed transactions (0x01 = EIP-2930, 0x02 = EIP-1559)
	var txType byte
	if len(raw) > 0 && raw[0] >= 0x01 && raw[0] <= 0x02 {
		txType = raw[0]
		raw = raw[1:]
	}

	listData, rest, err := decodeRLPItem(raw)
	if err != nil {
		return nil, fmt.Errorf("decode RLP list: %w", err)
	}
	if len(rest) > 0 {
		return nil, errors.New("trailing data after RLP transaction")
	}

	items := make([][]byte, 0, 12)
	for len(listData) > 0 {
		var item []byte
		item, listData, err = decodeRLPItem(listData)
		if err != nil {
			return nil, fmt.Errorf("decode RLP item %d: %w", len(items), err)
		}
		items = append(items, item)
	}
	if len(items) < 9 || len(items) > 12 {
		return nil, fmt.Errorf("expected 9-12 RLP items, got %d", len(items))
	}

	tx := &RLPTx{txType: txType}

	if txType == 2 || txType == 1 {
		// EIP-1559: [chainID, nonce, maxPriorityFee, maxFeePerGas, gasLimit, to, value, data, accessList, v, r, s]
		if len(items) < 12 {
			return nil, fmt.Errorf("typed tx needs 12 items, got %d", len(items))
		}
		tx.ChainID = decodeRLPBigInt(items[0])
		tx.Nonce = decodeRLPBigInt(items[1])
		tx.GasPrice = decodeRLPBigInt(items[3]) // maxFeePerGas
		tx.GasLimit = decodeRLPBigInt(items[4])
		tx.To = items[5]
		tx.Value = decodeRLPBigInt(items[6])
		tx.Data = items[7]
		tx.R = items[10]
		tx.S = items[11]
		v := decodeRLPBigInt(items[9])
		if v >= 27 {
			tx.V = byte(v - 27)
		} else {
			tx.V = byte(v)
		}
	} else {
		// Legacy: [nonce, gasPrice, gasLimit, to, value, data, v, r, s]
		if len(items) < 9 {
			return nil, fmt.Errorf("legacy tx needs 9 items, got %d", len(items))
		}
		tx.Nonce = decodeRLPBigInt(items[0])
		tx.GasPrice = decodeRLPBigInt(items[1])
		tx.GasLimit = decodeRLPBigInt(items[2])
		tx.To = items[3]
		tx.Value = decodeRLPBigInt(items[4])
		tx.Data = items[5]
		tx.R = items[7]
		tx.S = items[8]

		v := decodeRLPBigInt(items[6])
		if v >= 35 {
			tx.ChainID = (v - 35) / 2
			tx.V = byte(v - 35 - 2*tx.ChainID)
		} else if v >= 27 {
			tx.ChainID = 0
			tx.V = byte(v - 27)
		} else {
			tx.ChainID = 0
			tx.V = byte(v)
		}
	}

	return tx, nil
}

func (tx *RLPTx) SigningHash() []byte {
	var buf []byte
	if tx.txType == 2 || tx.txType == 1 {
		// EIP-1559/2930: 0x02/0x01 || RLP([chainID, nonce, maxPriorityFee, maxFee, gasLimit, to, value, data, accessList])
		// Use tx.txType as the prefix byte and chainID as first field
		buf = append([]byte{tx.txType}, rlpEncodeList(
			rlpEncodeUint64(tx.ChainID),
			rlpEncodeUint64(tx.Nonce),
			rlpEncodeUint64(0), // maxPriorityFeePerGas = 0
			rlpEncodeUint64(tx.GasPrice), // maxFeePerGas
			rlpEncodeUint64(tx.GasLimit),
			rlpEncodeBytes(tx.To),
			rlpEncodeUint64(tx.Value),
			rlpEncodeBytes(tx.Data),
			rlpEncodeBytes(nil), // empty accessList
		)...)
	} else {
		// Legacy EIP-155: RLP([nonce, gasPrice, gasLimit, to, value, data, chainID, 0, 0])
		buf = rlpEncodeList(
			rlpEncodeUint64(tx.Nonce),
			rlpEncodeUint64(tx.GasPrice),
			rlpEncodeUint64(tx.GasLimit),
			rlpEncodeBytes(tx.To),
			rlpEncodeUint64(tx.Value),
			rlpEncodeBytes(tx.Data),
			rlpEncodeUint64(tx.ChainID),
			rlpEncodeUint64(0),
			rlpEncodeUint64(0),
		)
	}
	return crypto.Keccak256(buf)
}

func (tx *RLPTx) TxHash() []byte {
	var buf []byte
	if tx.txType == 2 || tx.txType == 1 {
		// Typed tx: type_byte || RLP([chainID, nonce, maxPriorityFee, maxFee, gasLimit, to, value, data, accessList, v, r, s])
		buf = append([]byte{tx.txType}, rlpEncodeList(
			rlpEncodeUint64(tx.ChainID),
			rlpEncodeUint64(tx.Nonce),
			rlpEncodeUint64(0),
			rlpEncodeUint64(tx.GasPrice),
			rlpEncodeUint64(tx.GasLimit),
			rlpEncodeBytes(tx.To),
			rlpEncodeUint64(tx.Value),
			rlpEncodeBytes(tx.Data),
			rlpEncodeBytes(nil),
			rlpEncodeUint64(uint64(tx.V)),
			rlpEncodeBytes(tx.R),
			rlpEncodeBytes(tx.S),
		)...)
	} else {
		vVal := uint64(tx.V)
		if tx.ChainID > 0 {
			vVal = tx.ChainID*2 + 35 + uint64(tx.V)
		}
		buf = rlpEncodeList(
			rlpEncodeUint64(tx.Nonce),
			rlpEncodeUint64(tx.GasPrice),
			rlpEncodeUint64(tx.GasLimit),
			rlpEncodeBytes(tx.To),
			rlpEncodeUint64(tx.Value),
			rlpEncodeBytes(tx.Data),
			rlpEncodeUint64(vVal),
			rlpEncodeBytes(tx.R),
			rlpEncodeBytes(tx.S),
		)
	}
	return crypto.Keccak256(buf)
}

func (tx *RLPTx) RecoverPubKey() ([]byte, error) {
	hash := tx.SigningHash()
	r := new(big.Int).SetBytes(tx.R)
	s := new(big.Int).SetBytes(tx.S)
	if r.Sign() == 0 || s.Sign() == 0 {
		return nil, errors.New("zero R or S")
	}
	rBytes := tx.R
	sBytes := tx.S
	if len(rBytes) > 32 || len(sBytes) > 32 {
		return nil, errors.New("R or S too long")
	}
	compactSig := make([]byte, 65)
	compactSig[0] = 27 + tx.V
	copy(compactSig[33-len(rBytes):33], rBytes)
	copy(compactSig[65-len(sBytes):65], sBytes)

	pubKey, _, err := sececdsa.RecoverCompact(compactSig, hash)
	if err != nil {
		return nil, fmt.Errorf("recover pubkey: %w", err)
	}
	raw := pubKey.SerializeUncompressed()
	return raw, nil
}

func (tx *RLPTx) ToTransaction() (*Transaction, error) {
	pubKeyBytes, err := tx.RecoverPubKey()
	if err != nil {
		return nil, err
	}
	pubKey, err := crypto.PubKeyFromBytes(pubKeyBytes)
	if err != nil {
		return nil, err
	}
	hash := tx.TxHash()
	return &Transaction{
		Hash:     hash,
		Nonce:    tx.Nonce,
		From:     pubKey.Address(),
		To:       tx.To,
		Value:    tx.Value,
		GasLimit: tx.GasLimit,
		GasPrice: tx.GasPrice,
		Data:     tx.Data,
		ChainID:  tx.ChainID,
		Signature: &TxSignature{
			R: tx.R,
			S: tx.S,
			V: tx.V,
		},
	}, nil
}

func rlpEncodeList(items ...[]byte) []byte {
	var payload []byte
	for _, item := range items {
		payload = append(payload, item...)
	}
	return rlpEncodeWithPrefix(payload, 0xC0)
}

func rlpEncodeBytes(data []byte) []byte {
	if len(data) == 0 {
		return []byte{0x80}
	}
	if len(data) == 1 && data[0] < 0x80 {
		return data
	}
	return rlpEncodeWithPrefix(data, 0x80)
}

func rlpEncodeUint64(val uint64) []byte {
	if val == 0 {
		return []byte{0x80}
	}
	var buf [8]byte
	var n int
	binary.BigEndian.PutUint64(buf[:], val)
	for n = 0; n < 8; n++ {
		if buf[n] != 0 {
			break
		}
	}
	data := buf[n:]
	if len(data) == 1 && data[0] < 0x80 {
		return data
	}
	return rlpEncodeWithPrefix(data, 0x80)
}

func rlpEncodeWithPrefix(data []byte, base byte) []byte {
	l := len(data)
	if l == 0 {
		return []byte{base}
	}
	if l <= 55 {
		return append([]byte{base + byte(l)}, data...)
	}
	lenBytes := make([]byte, 8)
	var n int
	binary.BigEndian.PutUint64(lenBytes, uint64(l))
	for n = 0; n < 8; n++ {
		if lenBytes[n] != 0 {
			break
		}
	}
	lenPrefix := lenBytes[n:]
	return append(append([]byte{base + 55 + byte(len(lenPrefix))}, lenPrefix...), data...)
}
