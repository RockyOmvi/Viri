package crypto

import (
	"io"
	"sync"
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
	default:
		return nil, ErrInvalidKey
	}
}

// NewVerifierFromPublicBytes creates a Verifier from raw public key bytes for a given scheme.
func NewVerifierFromPublicBytes(scheme Scheme, data []byte) (Verifier, error) {
	switch scheme {
	case SchemeECDSA:
		return PubKeyFromBytes(data)
	default:
		return nil, ErrInvalidKey
	}
}
