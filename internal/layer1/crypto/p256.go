package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/sha3"
)

var (
	ErrP256InvalidSignature = errors.New("p256: invalid signature")
	ErrP256InvalidKey       = errors.New("p256: invalid key")
)

type P256PrivateKey struct {
	key *ecdsa.PrivateKey
}

type P256PublicKey struct {
	key *ecdsa.PublicKey
}

type P256Signature struct {
	R, S *big.Int
}

func GenerateP256Key() (*P256PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("p256 generate: %w", err)
	}
	return &P256PrivateKey{key: k}, nil
}

func GenerateP256KeyWithReader(r io.Reader) (*P256PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), r)
	if err != nil {
		return nil, fmt.Errorf("p256 generate: %w", err)
	}
	return &P256PrivateKey{key: k}, nil
}

func (k *P256PrivateKey) PubKey() *P256PublicKey {
	return &P256PublicKey{key: &k.key.PublicKey}
}

func (k *P256PrivateKey) Sign(data []byte) (*P256Signature, error) {
	hash := keccak256P256(data)
	r, s, err := ecdsa.Sign(rand.Reader, k.key, hash)
	if err != nil {
		return nil, fmt.Errorf("p256 sign: %w", err)
	}
	return &P256Signature{R: r, S: s}, nil
}

func (k *P256PrivateKey) SignMessage(data []byte) ([]byte, error) {
	sig, err := k.Sign(data)
	if err != nil {
		return nil, err
	}
	return sig.Bytes(), nil
}

func (k *P256PrivateKey) VerifyMessage(data, sig []byte) bool {
	return k.PubKey().VerifyMessage(data, sig)
}

func (k *P256PrivateKey) Scheme() Scheme       { return SchemeECDSA }
func (k *P256PrivateKey) PublicBytes() []byte   { return k.PubKey().Bytes() }
func (k *P256PrivateKey) Seed() []byte          { return k.key.D.Bytes() }
func (k *P256PrivateKey) PrivateBytes() []byte  { return k.key.D.Bytes() }

func (k *P256PrivateKey) Hex() string {
	return hex.EncodeToString(k.key.D.Bytes())
}

func (pub *P256PublicKey) Verify(data []byte, sig *P256Signature) bool {
	hash := keccak256P256(data)
	return ecdsa.Verify(pub.key, hash, sig.R, sig.S)
}

func (pub *P256PublicKey) VerifyMessage(data, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	return pub.Verify(data, &P256Signature{R: r, S: s})
}

func (pub *P256PublicKey) Scheme() Scheme     { return SchemeECDSA }
func (pub *P256PublicKey) PublicBytes() []byte { return pub.Bytes() }

func (pub *P256PublicKey) Address() []byte {
	raw := elliptic.Marshal(pub.key, pub.key.X, pub.key.Y)
	hash := keccak256P256(raw[1:])
	return hash[12:]
}

func (pub *P256PublicKey) Hex() string {
	return hex.EncodeToString(pub.Bytes())
}

func (pub *P256PublicKey) Bytes() []byte {
	return elliptic.Marshal(elliptic.P256(), pub.key.X, pub.key.Y)
}

func (sig *P256Signature) Bytes() []byte {
	combined := make([]byte, 64)
	rBytes := sig.R.Bytes()
	sBytes := sig.S.Bytes()
	copy(combined[32-len(rBytes):32], rBytes)
	copy(combined[64-len(sBytes):64], sBytes)
	return combined
}

func keccak256P256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func P256PubKeyFromBytes(data []byte) (*P256PublicKey, error) {
	if len(data) == 0 {
		return nil, ErrP256InvalidKey
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), data)
	if x == nil || y == nil {
		return nil, ErrP256InvalidKey
	}
	return &P256PublicKey{key: &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}}, nil
}

func P256PrivateKeyFromBytes(data []byte) (*P256PrivateKey, error) {
	if len(data) == 0 {
		return nil, ErrP256InvalidKey
	}
	k := new(ecdsa.PrivateKey)
	k.Curve = elliptic.P256()
	k.PublicKey.Curve = elliptic.P256()
	k.D = new(big.Int).SetBytes(data)
	k.PublicKey.X, k.PublicKey.Y = k.Curve.ScalarBaseMult(k.D.Bytes())
	if k.PublicKey.X == nil {
		return nil, ErrP256InvalidKey
	}
	return &P256PrivateKey{key: k}, nil
}
