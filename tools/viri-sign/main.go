package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	sececdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

type TxSignature struct {
	R []byte
	S []byte
	V byte
}

type Transaction struct {
	Hash        []byte
	Nonce       uint64
	From        []byte
	To          []byte
	Value       uint64
	GasLimit    uint64
	GasPrice    uint64
	FeeCurrency []byte
	Data        []byte
	Signature   *TxSignature
	ChainID     uint64
}

func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b[i] = byte(n >> (56 - 8*i))
	}
	return b
}

var secp256k1HalfOrder = new(big.Int).Rsh(secp256k1.S256().N, 1)

type SigningInput struct {
	ChainID  uint64 `json:"chain_id"`
	Nonce    uint64 `json:"nonce"`
	To       string `json:"to"`
	Value    string `json:"value"`
	GasLimit uint64 `json:"gas_limit"`
	GasPrice uint64 `json:"gas_price"`
	Data     string `json:"data"`
	PrivKey  string `json:"priv_key"`
}

func parseWei(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

func main() {
	var rawInput []byte
	if len(os.Args) >= 2 {
		rawInput = []byte(os.Args[1])
	} else {
		data, _ := io.ReadAll(os.Stdin)
		rawInput = data
	}
	rawInput = bytes.TrimSpace(rawInput)
	if len(rawInput) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: viri-sign <json-input>")
		os.Exit(1)
	}

	var input SigningInput
	if err := json.Unmarshal(rawInput, &input); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse input: %v\n", err)
		os.Exit(1)
	}

	keyBytes, err := hex.DecodeString(input.PrivKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid private key hex: %v\n", err)
		os.Exit(1)
	}

	privKey := secp256k1.PrivKeyFromBytes(keyBytes)
	pubKey := privKey.PubKey()
	pubKeyBytes := pubKey.SerializeUncompressed()

	toBytes, err := hex.DecodeString(input.To)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid to address hex: %v\n", err)
		os.Exit(1)
	}

	dataBytes, _ := hex.DecodeString(input.Data)

	value, err := parseWei(input.Value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid value: %v\n", err)
		os.Exit(1)
	}

	// Build signing payload: ChainID || Nonce || From || To || Value || GasLimit || GasPrice || Data
	payload := make([]byte, 0)
	payload = append(payload, uint64ToBytes(input.ChainID)...)
	payload = append(payload, uint64ToBytes(input.Nonce)...)
	payload = append(payload, pubKeyBytes...)
	payload = append(payload, toBytes...)
	payload = append(payload, uint64ToBytes(value)...)
	payload = append(payload, uint64ToBytes(input.GasLimit)...)
	payload = append(payload, uint64ToBytes(input.GasPrice)...)
	payload = append(payload, dataBytes...)

	// Sign: Keccak256(payload) then ECDSA
	hash := keccak256(payload)
	sig := sececdsa.Sign(privKey, hash)
	r := sig.R()
	s := sig.S()

	rArr := r.Bytes()
	sArr := s.Bytes()
	rBytes := rArr[:]
	sBytes := sArr[:]

	// Low-S enforcement
	sInt := new(big.Int).SetBytes(sBytes)
	if sInt.Cmp(secp256k1HalfOrder) > 0 {
		sInt.Sub(secp256k1.S256().N, sInt)
		sBytes = sInt.Bytes()
	}

	txSig := &TxSignature{
		R: rBytes,
		S: sBytes,
		V: 0,
	}

	// Compute hash = DoubleSHA256(SigningPayload + R + S + V)
	hashPayload := make([]byte, len(payload))
	copy(hashPayload, payload)
	hashPayload = append(hashPayload, txSig.R...)
	hashPayload = append(hashPayload, txSig.S...)
	hashPayload = append(hashPayload, txSig.V)

	txHash := doubleSHA256(hashPayload)

	tx := &Transaction{
		Hash:        txHash,
		Nonce:       input.Nonce,
		From:        pubKeyBytes,
		To:          toBytes,
		Value:       value,
		GasLimit:    input.GasLimit,
		GasPrice:    input.GasPrice,
		FeeCurrency: nil,
		Data:        dataBytes,
		Signature:   txSig,
		ChainID:     input.ChainID,
	}

	// Gob serialize
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to serialize: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(hex.EncodeToString(buf.Bytes()))
}
