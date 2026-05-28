package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func main() {
	privHex := "4bb69e6d57a25516956ba2b5d94697764f1aceef2c79f8a88ae576cc96389d29"

	privKey, err := crypto.HexToECDSA(privHex)
	if err != nil {
		panic(err)
	}

	fromAddr := crypto.PubkeyToAddress(privKey.PublicKey)
	fmt.Printf("From address: %s\n", fromAddr.Hex())

	toAddr := crypto.PubkeyToAddress(privKey.PublicKey)

	nonce := uint64(0)

	chainID := big.NewInt(1986622057)
	gasPrice := big.NewInt(1000000000)
	gasLimit := uint64(30000)
	value := big.NewInt(1000000000000000000)
	data := []byte{}

	// Legacy EIP-155 tx
	txData := map[string]interface{}{
		"nonce":    nonce,
		"gasPrice": gasPrice,
		"gas":      gasLimit,
		"to":       toAddr,
		"value":    value,
		"data":     data,
	}

	// RLP encode the signing fields: [nonce, gasPrice, gasLimit, to, value, data, chainID, 0, 0]
	signFields := []interface{}{
		nonce,
		gasPrice,
		gasLimit,
		toAddr,
		value,
		data,
		chainID,
		uint64(0),
		uint64(0),
	}

	signBytes, err := rlp.EncodeToBytes(signFields)
	if err != nil {
		panic(err)
	}

	// Keccak256 hash
	hash := crypto.Keccak256Hash(signBytes)
	fmt.Printf("Signing hash: %s\n", hash.Hex())

	sig, err := crypto.Sign(hash.Bytes(), privKey)
	if err != nil {
		panic(err)
	}

	// sig is [R(32) || S(32) || V(1)] where V is 0 or 1
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	v := sig[64]

	// EIP-155 V = chainID*2 + 35 + v
	eip155V := chainID.Mul(chainID, big.NewInt(2)).Add(chainID, big.NewInt(35+uint64(v)))

	// Full RLP tx: [nonce, gasPrice, gasLimit, to, value, data, v, r, s]
	txRLP := []interface{}{
		nonce,
		gasPrice,
		gasLimit,
		toAddr,
		value,
		data,
		eip155V,
		r,
		s,
	}

	txBytes, err := rlp.EncodeToBytes(txRLP)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Raw tx hex: 0x%s\n", hex.EncodeToString(txBytes))
	fmt.Printf("Length: %d bytes\n", len(txBytes))

	txHash := crypto.Keccak256Hash(txBytes)
	fmt.Printf("Tx hash: %s\n", txHash.Hex())

	// Verify: recover signer
	// Remove EIP-155 V encoding for recovery
	sigV := sig[64]
	if sigV >= 27 {
		sigV -= 27
	}
	recoverySig := make([]byte, 65)
	copy(recoverySig, sig[:64])
	recoverySig[64] = sigV

	recoveredPub, err := crypto.SigToPub(hash.Bytes(), recoverySig)
	if err != nil {
		panic(err)
	}
	recoveredAddr := crypto.PubkeyToAddress(*recoveredPub)
	fmt.Printf("Recovered address: %s\n", recoveredAddr.Hex())
	fmt.Printf("Match: %v\n", recoveredAddr == fromAddr)

	// Check gas limit vs intrinsic gas
	intrinsicGas := uint64(21000)
	if gasLimit < intrinsicGas {
		fmt.Printf("WARNING: gas limit %d < intrinsic gas %d\n", gasLimit, intrinsicGas)
	}

	fmt.Printf("\n--- Send this raw tx: ---\n")
	fmt.Printf("eth_sendRawTransaction('0x%s')\n", hex.EncodeToString(txBytes))
}

func init() {
	// Recover chainID for re-use
	chainID := big.NewInt(1986622057)

	// Print the chainID for reference
	fmt.Printf("Chain ID: %s (%d)\n", chainID.String(), chainID)
}

var _ = strings.TrimSpace
