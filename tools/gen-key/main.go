package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"golang.org/x/crypto/sha3"
)

func main() {
	privBytes := make([]byte, 32)
	_, _ = rand.Read(privBytes)
	priv := secp256k1.PrivKeyFromBytes(privBytes)
	pub := priv.PubKey()
	pubUncompressed := pub.SerializeUncompressed()
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubUncompressed[1:])
	addr := hash.Sum(nil)[12:]

	fmt.Println("Private key:", hex.EncodeToString(privBytes))
	fmt.Println("Public key: ", hex.EncodeToString(pubUncompressed))
	fmt.Println("Address:    ", hex.EncodeToString(addr))
}
