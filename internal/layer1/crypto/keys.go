package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
)

var ErrInvalidSignature = errors.New("invalid signature")
var ErrInvalidKey = errors.New("invalid key")

type PrivateKey struct {
	*ecdsa.PrivateKey
}

type PublicKey struct {
	*ecdsa.PublicKey
}

type Signature struct {
	R, S *big.Int
}

func GenerateKey() (*PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{PrivateKey: key}, nil
}

func (k *PrivateKey) PubKey() *PublicKey {
	return &PublicKey{PublicKey: &k.PrivateKey.PublicKey}
}

func (k *PrivateKey) Sign(data []byte) (*Signature, error) {
	hash := SHA256(data)
	r, s, err := ecdsa.Sign(rand.Reader, k.PrivateKey, hash)
	if err != nil {
		return nil, err
	}
	return &Signature{R: r, S: s}, nil
}

func (k *PrivateKey) Hex() string {
	return hex.EncodeToString(k.D.Bytes())
}

func (pub *PublicKey) Verify(data []byte, sig *Signature) bool {
	hash := SHA256(data)
	return ecdsa.Verify(pub.PublicKey, hash, sig.R, sig.S)
}

func (pub *PublicKey) Address() []byte {
	hash := SHA256(pub.Bytes())
	ripemd := ripemd160.New()
	ripemd.Write(hash)
	return ripemd.Sum(nil)
}

func (pub *PublicKey) Hex() string {
	return hex.EncodeToString(pub.Bytes())
}

func (pub *PublicKey) Bytes() []byte {
	return elliptic.Marshal(pub.PublicKey, pub.X, pub.Y)
}

func (sig *Signature) Bytes() []byte {
	// Fixed 64-byte encoding: 32 bytes for R + 32 bytes for S, left zero-padded
	combined := make([]byte, 64)
	rBytes := sig.R.Bytes()
	sBytes := sig.S.Bytes()
	copy(combined[32-len(rBytes):32], rBytes)
	copy(combined[64-len(sBytes):64], sBytes)
	return combined
}

// SignatureFromBytes deserializes a 64-byte signature (32B R + 32B S).
func SignatureFromBytes(data []byte) (*Signature, error) {
	if len(data) != 64 {
		return nil, fmt.Errorf("invalid signature length: expected 64, got %d", len(data))
	}
	return &Signature{
		R: new(big.Int).SetBytes(data[:32]),
		S: new(big.Int).SetBytes(data[32:64]),
	}, nil
}

func SHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func DoubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// Keccak256 computes the Keccak-256 hash (used for Ethereum-compatible addressing).
func Keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func GenerateAddress() ([]byte, error) {
	key, err := GenerateKey()
	if err != nil {
		return nil, err
	}
	return key.PubKey().Address(), nil
}

func PubKeyFromBytes(data []byte) (*PublicKey, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), data)
	if x == nil {
		return nil, ErrInvalidKey
	}
	return &PublicKey{PublicKey: &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}}, nil
}

func PrivateKeyFromBytes(data []byte) (*PrivateKey, error) {
	if len(data) < 32 {
		return nil, ErrInvalidKey
	}
	d := new(big.Int).SetBytes(data)
	key := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
		},
		D: d,
	}
	key.PublicKey.X, key.PublicKey.Y = key.PublicKey.Curve.ScalarBaseMult(data)
	return &PrivateKey{PrivateKey: key}, nil
}
