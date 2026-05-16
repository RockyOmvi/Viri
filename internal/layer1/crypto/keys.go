package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	sececdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidKey       = errors.New("invalid key")
)

// secp256k1HalfOrder is (N-1)/2 used for low-S canonical signature enforcement (EIP-2).
var secp256k1HalfOrder = new(big.Int).Rsh(secp256k1.S256().N, 1)

type PrivateKey struct {
	key *secp256k1.PrivateKey
}

type PublicKey struct {
	key *secp256k1.PublicKey
}

type Signature struct {
	R, S *big.Int
}

func GenerateKey() (*PrivateKey, error) {
	k, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &PrivateKey{key: k}, nil
}

func GenerateKeyWithReader(rand io.Reader) (*PrivateKey, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand, buf); err != nil {
		return nil, err
	}
	privKey := secp256k1.PrivKeyFromBytes(buf)
	return &PrivateKey{key: privKey}, nil
}

func (k *PrivateKey) PubKey() *PublicKey {
	return &PublicKey{key: k.key.PubKey()}
}

func (k *PrivateKey) Sign(data []byte) (*Signature, error) {
	hash := Keccak256(data)
	return k.SignHash(hash)
}

func (k *PrivateKey) SignHash(hash []byte) (*Signature, error) {
	sig := sececdsa.Sign(k.key, hash)
	r := sig.R()
	s := sig.S()
	rBytes := r.Bytes()
	sBytes := s.Bytes()

	// Enforce low-S for EIP-2 / EVM compatibility
	// If S > N/2, replace with N - S to prevent signature malleability
	sInt := new(big.Int).SetBytes(sBytes[:])
	if sInt.Cmp(secp256k1HalfOrder) > 0 {
		sInt.Sub(secp256k1.S256().N, sInt)
		copy(sBytes[:], sInt.FillBytes(make([]byte, 32)))
	}

	return &Signature{
		R: new(big.Int).SetBytes(rBytes[:]),
		S: new(big.Int).SetBytes(sBytes[:]),
	}, nil
}

func (k *PrivateKey) VerifyMessage(data, sig []byte) bool {
	return k.PubKey().VerifyMessage(data, sig)
}

func (k *PrivateKey) SignMessage(data []byte) ([]byte, error) {
	sig, err := k.Sign(data)
	if err != nil {
		return nil, err
	}
	return sig.Bytes(), nil
}

func (k *PrivateKey) Scheme() Scheme        { return SchemeECDSA }
func (k *PrivateKey) PublicBytes() []byte   { return k.PubKey().Bytes() }
func (k *PrivateKey) Seed() []byte          { return k.key.Serialize() }
func (k *PrivateKey) PrivateBytes() []byte  { return k.key.Serialize() }

func (k *PrivateKey) Hex() string {
	return hex.EncodeToString(k.key.Serialize())
}

func (pub *PublicKey) Verify(data []byte, sig *Signature) bool {
	hash := Keccak256(data)
	return pub.VerifyHash(hash, sig)
}

func (pub *PublicKey) VerifyHash(hash []byte, sig *Signature) bool {
	sigObj := sececdsa.NewSignature(bigIntToModNScalar(sig.R), bigIntToModNScalar(sig.S))
	return sigObj.Verify(hash, pub.key)
}

func (pub *PublicKey) VerifyMessage(data, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	return pub.Verify(data, &Signature{R: r, S: s})
}

func (pub *PublicKey) Scheme() Scheme      { return SchemeECDSA }
func (pub *PublicKey) PublicBytes() []byte  { return pub.Bytes() }

func (pub *PublicKey) Address() []byte {
	raw := pub.key.SerializeUncompressed()
	hash := Keccak256(raw[1:])
	return hash[12:]
}

func (pub *PublicKey) Hex() string {
	return hex.EncodeToString(pub.key.SerializeUncompressed())
}

func (pub *PublicKey) Bytes() []byte {
	return pub.key.SerializeUncompressed()
}

func (pub *PublicKey) Compressed() []byte {
	return pub.key.SerializeCompressed()
}

func (sig *Signature) Bytes() []byte {
	combined := make([]byte, 64)
	rBytes := sig.R.Bytes()
	sBytes := sig.S.Bytes()
	copy(combined[32-len(rBytes):32], rBytes)
	copy(combined[64-len(sBytes):64], sBytes)
	return combined
}

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
	k, err := secp256k1.ParsePubKey(data)
	if err != nil {
		return nil, ErrInvalidKey
	}
	return &PublicKey{key: k}, nil
}

func DecompressPubKey(data []byte) (*PublicKey, error) {
	return PubKeyFromBytes(data)
}

func PrivateKeyFromBytes(data []byte) (*PrivateKey, error) {
	if len(data) == 0 {
		return nil, ErrInvalidKey
	}
	privKey := secp256k1.PrivKeyFromBytes(data)
	return &PrivateKey{key: privKey}, nil
}

func modNScalarToBigInt(s secp256k1.ModNScalar) *big.Int {
	b := s.Bytes()
	return new(big.Int).SetBytes(b[:])
}

func bigIntToModNScalar(n *big.Int) *secp256k1.ModNScalar {
	var s secp256k1.ModNScalar
	b := n.Bytes()
	var arr [32]byte
	copy(arr[32-len(b):], b)
	s.SetBytes(&arr)
	return &s
}
