package crypto

import (
	"fmt"
	"io"

	crand "crypto/rand"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
)

// ML-DSA key pair wrapper. Uses circl's FIPS 204 implementation.

type mldsaKeyPair struct {
	scheme Scheme
	sk     *mldsa44.PrivateKey
	pk     *mldsa44.PublicKey
	pk65   *mldsa65.PublicKey
	sk65   *mldsa65.PrivateKey
	pk87   *mldsa87.PublicKey
	sk87   *mldsa87.PrivateKey
	seed   []byte
}

type mldsaVerifier struct {
	scheme Scheme
	pk44   *mldsa44.PublicKey
	pk65   *mldsa65.PublicKey
	pk87   *mldsa87.PublicKey
}

// MLDSA44GenerateKey generates an ML-DSA-44 key pair (NIST Level 2).
func MLDSA44GenerateKey(rand io.Reader) (KeyPair, error) {
	if rand == nil {
		rand = crand.Reader
	}
	pk, sk, err := mldsa44.GenerateKey(rand)
	if err != nil {
		return nil, fmt.Errorf("mldsa44: %w", err)
	}
	return &mldsaKeyPair{scheme: SchemeMLDSA44, pk: pk, sk: sk}, nil
}

// MLDSA65GenerateKey generates an ML-DSA-65 key pair (NIST Level 3).
func MLDSA65GenerateKey(rand io.Reader) (KeyPair, error) {
	if rand == nil {
		rand = crand.Reader
	}
	pk, sk, err := mldsa65.GenerateKey(rand)
	if err != nil {
		return nil, fmt.Errorf("mldsa65: %w", err)
	}
	return &mldsaKeyPair{scheme: SchemeMLDSA65, pk65: pk, sk65: sk}, nil
}

// MLDSA87GenerateKey generates an ML-DSA-87 key pair (NIST Level 5).
func MLDSA87GenerateKey(rand io.Reader) (KeyPair, error) {
	if rand == nil {
		rand = crand.Reader
	}
	pk, sk, err := mldsa87.GenerateKey(rand)
	if err != nil {
		return nil, fmt.Errorf("mldsa87: %w", err)
	}
	return &mldsaKeyPair{scheme: SchemeMLDSA87, pk87: pk, sk87: sk}, nil
}

func (k *mldsaKeyPair) SignMessage(data []byte) ([]byte, error) {
	sig := make([]byte, k.scheme.SigBytes())
	switch k.scheme {
	case SchemeMLDSA44:
		if err := mldsa44.SignTo(k.sk, data, nil, false, sig); err != nil {
			return nil, err
		}
	case SchemeMLDSA65:
		if err := mldsa65.SignTo(k.sk65, data, nil, false, sig); err != nil {
			return nil, err
		}
	case SchemeMLDSA87:
		if err := mldsa87.SignTo(k.sk87, data, nil, false, sig); err != nil {
			return nil, err
		}
	}
	return sig, nil
}

func (k *mldsaKeyPair) Scheme() Scheme      { return k.scheme }
func (k *mldsaKeyPair) Seed() []byte         { return k.seed }
func (k *mldsaKeyPair) PrivateBytes() []byte { return k.seed }

func (k *mldsaKeyPair) PublicBytes() []byte {
	switch k.scheme {
	case SchemeMLDSA44:
		b, _ := k.pk.MarshalBinary()
		return b
	case SchemeMLDSA65:
		b, _ := k.pk65.MarshalBinary()
		return b
	case SchemeMLDSA87:
		b, _ := k.pk87.MarshalBinary()
		return b
	}
	return nil
}

func (k *mldsaKeyPair) VerifyMessage(data, sig []byte) bool {
	switch k.scheme {
	case SchemeMLDSA44:
		return mldsa44.Verify(k.pk, data, nil, sig)
	case SchemeMLDSA65:
		return mldsa65.Verify(k.pk65, data, nil, sig)
	case SchemeMLDSA87:
		return mldsa87.Verify(k.pk87, data, nil, sig)
	}
	return false
}

func (v *mldsaVerifier) VerifyMessage(data, sig []byte) bool {
	switch v.scheme {
	case SchemeMLDSA44:
		return v.pk44 != nil && mldsa44.Verify(v.pk44, data, nil, sig)
	case SchemeMLDSA65:
		return v.pk65 != nil && mldsa65.Verify(v.pk65, data, nil, sig)
	case SchemeMLDSA87:
		return v.pk87 != nil && mldsa87.Verify(v.pk87, data, nil, sig)
	}
	return false
}

func (v *mldsaVerifier) Scheme() Scheme      { return v.scheme }
func (v *mldsaVerifier) PublicBytes() []byte {
	switch v.scheme {
	case SchemeMLDSA44:
		if v.pk44 == nil {
			return nil
		}
		b, _ := v.pk44.MarshalBinary()
		return b
	case SchemeMLDSA65:
		if v.pk65 == nil {
			return nil
		}
		b, _ := v.pk65.MarshalBinary()
		return b
	case SchemeMLDSA87:
		if v.pk87 == nil {
			return nil
		}
		b, _ := v.pk87.MarshalBinary()
		return b
	}
	return nil
}

// GenerateKeyPair generates a key pair for the given scheme.
func GenerateKeyPair(scheme Scheme, rand io.Reader) (KeyPair, error) {
	switch scheme {
	case SchemeECDSA:
		return GenerateKeyWithReader(rand)
	case SchemeMLDSA44:
		return MLDSA44GenerateKey(rand)
	case SchemeMLDSA65:
		return MLDSA65GenerateKey(rand)
	case SchemeMLDSA87:
		return MLDSA87GenerateKey(rand)
	case SchemeSPHINCS:
		return SphincsGenerateKey(rand)
	default:
		return nil, fmt.Errorf("unknown scheme: %d", scheme)
	}
}
