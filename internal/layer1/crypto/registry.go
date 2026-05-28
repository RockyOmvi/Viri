package crypto

import (
	"fmt"
	"io"
	"sync"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/cloudflare/circl/sign/slhdsa"
)

// SchemeRegistry maps scheme names to their generators.
// It is populated at init time with built-in schemes.
type SchemeRegistry struct {
	mu       sync.RWMutex
	schemes  map[Scheme]KeyGenerator
	names    map[string]Scheme
	defaultScheme Scheme
}

var globalRegistry = &SchemeRegistry{
	schemes: make(map[Scheme]KeyGenerator),
	names:   make(map[string]Scheme),
}

func init() {
	RegisterScheme(SchemeECDSA, &ecdsaGenerator{})
	RegisterScheme(SchemeMLDSA44, &mldsaGenerator{scheme: SchemeMLDSA44})
	RegisterScheme(SchemeMLDSA65, &mldsaGenerator{scheme: SchemeMLDSA65})
	RegisterScheme(SchemeMLDSA87, &mldsaGenerator{scheme: SchemeMLDSA87})
	RegisterScheme(SchemeSPHINCS, &sphincsGenerator{})
	globalRegistry.defaultScheme = SchemeECDSA
}

// RegisterScheme registers a key generator for a scheme.
func RegisterScheme(scheme Scheme, gen KeyGenerator) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.schemes[scheme] = gen
	globalRegistry.names[scheme.String()] = scheme
}

// GetGenerator returns the key generator for the given scheme.
func GetGenerator(scheme Scheme) (KeyGenerator, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	gen, ok := globalRegistry.schemes[scheme]
	return gen, ok
}

// DefaultScheme returns the default signature scheme.
func DefaultScheme() Scheme {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.defaultScheme
}

// SetDefaultScheme sets the default signature scheme.
func SetDefaultScheme(s Scheme) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.defaultScheme = s
}

// ecdsaGenerator implements KeyGenerator for secp256k1 ECDSA.
type ecdsaGenerator struct{}

func (g *ecdsaGenerator) GenerateKey(rand io.Reader) (KeyPair, error) {
	return GenerateKeyWithReader(rand)
}
func (g *ecdsaGenerator) Scheme() Scheme              { return SchemeECDSA }
func (g *ecdsaGenerator) GenerateFromSeed(seed []byte) KeyPair {
	key, err := PrivateKeyFromBytes(seed)
	if err != nil {
		return nil
	}
	return key
}

// mldsaGenerator implements KeyGenerator for ML-DSA.
type mldsaGenerator struct{ scheme Scheme }

func (g *mldsaGenerator) GenerateKey(rand io.Reader) (KeyPair, error) {
	return GenerateKeyPair(g.scheme, rand)
}
func (g *mldsaGenerator) Scheme() Scheme { return g.scheme }
func (g *mldsaGenerator) GenerateFromSeed(seed []byte) KeyPair {
	return nil // ML-DSA requires full key, not just seed
}

// sphincsGenerator implements KeyGenerator for SPHINCS+.
type sphincsGenerator struct{}

func (g *sphincsGenerator) GenerateKey(rand io.Reader) (KeyPair, error) {
	return SphincsGenerateKey(rand)
}
func (g *sphincsGenerator) Scheme() Scheme              { return SchemeSPHINCS }
func (g *sphincsGenerator) GenerateFromSeed(seed []byte) KeyPair {
	return nil // requires full key struct
}

// NewSignerFromPrivateBytes creates a Signer from raw private key bytes for a given scheme.
func NewSignerFromPrivateBytes(scheme Scheme, data []byte) (Signer, error) {
	switch scheme {
	case SchemeECDSA:
		return PrivateKeyFromBytes(data)
	case SchemeMLDSA44:
		var sk mldsa44.PrivateKey
		if err := sk.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("mldsa44: %w", err)
		}
		// ML-DSA private key encoding includes the public key
		seedSize := 32
		if len(data) < seedSize+1312 {
			return nil, fmt.Errorf("mldsa44: invalid key length %d", len(data))
		}
		var pk mldsa44.PublicKey
		if err := pk.UnmarshalBinary(data[seedSize : seedSize+1312]); err != nil {
			return nil, fmt.Errorf("mldsa44: invalid embedded public key: %w", err)
		}
		return &mldsaKeyPair{scheme: SchemeMLDSA44, sk: &sk, pk: &pk, seed: data[:seedSize]}, nil
	case SchemeMLDSA65:
		var sk mldsa65.PrivateKey
		if err := sk.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("mldsa65: %w", err)
		}
		seedSize := 48
		if len(data) < seedSize+1952 {
			return nil, fmt.Errorf("mldsa65: invalid key length %d", len(data))
		}
		var pk mldsa65.PublicKey
		if err := pk.UnmarshalBinary(data[seedSize : seedSize+1952]); err != nil {
			return nil, fmt.Errorf("mldsa65: invalid embedded public key: %w", err)
		}
		return &mldsaKeyPair{scheme: SchemeMLDSA65, sk65: &sk, pk65: &pk, seed: data[:seedSize]}, nil
	case SchemeMLDSA87:
		var sk mldsa87.PrivateKey
		if err := sk.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("mldsa87: %w", err)
		}
		seedSize := 56
		if len(data) < seedSize+2592 {
			return nil, fmt.Errorf("mldsa87: invalid key length %d", len(data))
		}
		var pk mldsa87.PublicKey
		if err := pk.UnmarshalBinary(data[seedSize : seedSize+2592]); err != nil {
			return nil, fmt.Errorf("mldsa87: invalid embedded public key: %w", err)
		}
		return &mldsaKeyPair{scheme: SchemeMLDSA87, sk87: &sk, pk87: &pk, seed: data[:seedSize]}, nil
	case SchemeSPHINCS:
		var priv slhdsa.PrivateKey
		priv.ID = slhdsa.SHA2_128s
		if err := priv.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("sphincs: %w", err)
		}
		pub := priv.PublicKey()
		return &sphincsSigner{sk: priv, pk: pub, seed: data[:sphincsSeedLen]}, nil
	default:
		return nil, ErrInvalidKey
	}
}

// NewVerifierFromPublicBytes creates a Verifier from raw public key bytes for a given scheme.
func NewVerifierFromPublicBytes(scheme Scheme, data []byte) (Verifier, error) {
	switch scheme {
	case SchemeECDSA:
		return PubKeyFromBytes(data)
	case SchemeMLDSA44:
		var pk mldsa44.PublicKey
		if err := pk.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("mldsa44: %w", err)
		}
		return &mldsaVerifier{scheme: SchemeMLDSA44, pk44: &pk}, nil
	case SchemeMLDSA65:
		var pk mldsa65.PublicKey
		if err := pk.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("mldsa65: %w", err)
		}
		return &mldsaVerifier{scheme: SchemeMLDSA65, pk65: &pk}, nil
	case SchemeMLDSA87:
		var pk mldsa87.PublicKey
		if err := pk.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("mldsa87: %w", err)
		}
		return &mldsaVerifier{scheme: SchemeMLDSA87, pk87: &pk}, nil
	case SchemeSPHINCS:
		var pub slhdsa.PublicKey
		pub.ID = slhdsa.SHA2_128s
		if err := pub.UnmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("sphincs: %w", err)
		}
		return &sphincsVerifier{pk: pub}, nil
	default:
		return nil, ErrInvalidKey
	}
}
