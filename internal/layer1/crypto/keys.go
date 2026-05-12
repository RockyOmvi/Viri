package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/sha3"
)

// Note: PrivateKey implements Signer/KeyPair via SignMessage, Scheme, PublicBytes, Seed, PrivateBytes.
// PublicKey implements Verifier via VerifyMessage, Scheme, PublicBytes.
// Compile-time interface assertions are omitted because Sign/Verify take concrete *Signature types
// rather than []byte for backward compatibility.

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
	return GenerateKeyWithReader(crand.Reader)
}

func GenerateKeyWithReader(rand io.Reader) (*PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{PrivateKey: key}, nil
}

func (k *PrivateKey) PubKey() *PublicKey {
	return &PublicKey{PublicKey: &k.PrivateKey.PublicKey}
}

// Sign returns the raw 64-byte ECDSA signature.
func (k *PrivateKey) Sign(data []byte) (*Signature, error) {
	hash := SHA256(data)
	r, s, err := ecdsa.Sign(crand.Reader, k.PrivateKey, hash)
	if err != nil {
		return nil, err
	}
	return &Signature{R: r, S: s}, nil
}

// VerifyMessage implements the Verifier interface on PrivateKey (delegates to public key).
func (k *PrivateKey) VerifyMessage(data, sig []byte) bool {
	return k.PubKey().VerifyMessage(data, sig)
}

// SignMessage implements the Signer interface.
func (k *PrivateKey) SignMessage(data []byte) ([]byte, error) {
	sig, err := k.Sign(data)
	if err != nil {
		return nil, err
	}
	return sig.Bytes(), nil
}

// Scheme returns SchemeECDSA for backward compatibility.
func (k *PrivateKey) Scheme() Scheme { return SchemeECDSA }

// PublicBytes returns the uncompressed public key bytes.
func (k *PrivateKey) PublicBytes() []byte { return k.PubKey().Bytes() }

// Seed returns the private key scalar as 32 bytes.
func (k *PrivateKey) Seed() []byte { return k.D.Bytes() }

// PrivateBytes returns the private key bytes.
func (k *PrivateKey) PrivateBytes() []byte { return k.D.Bytes() }

func (k *PrivateKey) Hex() string {
	return hex.EncodeToString(k.D.Bytes())
}

func (pub *PublicKey) Verify(data []byte, sig *Signature) bool {
	hash := SHA256(data)
	return ecdsa.Verify(pub.PublicKey, hash, sig.R, sig.S)
}

// VerifyMessage implements the Verifier interface, accepting raw bytes.
func (pub *PublicKey) VerifyMessage(data, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	hash := SHA256(data)
	return ecdsa.Verify(pub.PublicKey, hash, r, s)
}

// Scheme returns SchemeECDSA.
func (pub *PublicKey) Scheme() Scheme { return SchemeECDSA }

// PublicBytes returns the uncompressed public key bytes.
func (pub *PublicKey) PublicBytes() []byte { return pub.Bytes() }

func (pub *PublicKey) Address() []byte {
	hash := Keccak256(pub.Bytes())
	return hash[12:]
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
