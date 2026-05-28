package crypto

import "io"

// Scheme identifies a signature algorithm.
type Scheme uint8

const (
	SchemeECDSA   Scheme = 0 // secp256k1 ECDSA (default, EVM-compatible)
	SchemeMLDSA44 Scheme = 1 // ML-DSA-44 (FIPS 204, NIST PQC Level 2)
	SchemeMLDSA65 Scheme = 2 // ML-DSA-65 (FIPS 204, NIST PQC Level 3)
	SchemeMLDSA87 Scheme = 3 // ML-DSA-87 (FIPS 204, NIST PQC Level 5)
	SchemeSPHINCS  Scheme = 4 // SPHINCS+-SHA256-128s (FIPS 205, NIST PQC Level 1)
)

func (s Scheme) String() string {
	switch s {
	case SchemeECDSA:
		return "secp256k1"
	case SchemeMLDSA44:
		return "mldsa44"
	case SchemeMLDSA65:
		return "mldsa65"
	case SchemeMLDSA87:
		return "mldsa87"
	case SchemeSPHINCS:
		return "sphincs-sha256-128s"
	default:
		return "unknown"
	}
}

// PrivateBytes returns the size of the private key for this scheme.
func (s Scheme) PrivateBytes() int {
	switch s {
	case SchemeECDSA:
		return 32
	case SchemeMLDSA44:
		return 2560
	case SchemeMLDSA65:
		return 4032
	case SchemeMLDSA87:
		return 4896
	case SchemeSPHINCS:
		return 64
	default:
		return 0
	}
}

// PublicBytes returns the size of the public key for this scheme.
func (s Scheme) PublicBytes() int {
	switch s {
	case SchemeECDSA:
		return 65 // uncompressed secp256k1
	case SchemeMLDSA44:
		return 1312
	case SchemeMLDSA65:
		return 1952
	case SchemeMLDSA87:
		return 2592
	case SchemeSPHINCS:
		return 32
	default:
		return 0
	}
}

// SigBytes returns the maximum signature size for this scheme.
func (s Scheme) SigBytes() int {
	switch s {
	case SchemeECDSA:
		return 64
	case SchemeMLDSA44:
		return 2420
	case SchemeMLDSA65:
		return 3309
	case SchemeMLDSA87:
		return 4627
	case SchemeSPHINCS:
		return 7856
	default:
		return 0
	}
}

// Signer produces signatures.
type Signer interface {
	SignMessage(data []byte) ([]byte, error)
	Scheme() Scheme
	PublicBytes() []byte
}

// Verifier verifies signatures.
type Verifier interface {
	VerifyMessage(data, sig []byte) bool
	Scheme() Scheme
	PublicBytes() []byte
}

// KeyPair is a signer that can also reveal its private key and public verifier.
type KeyPair interface {
	Signer
	Verifier
	Seed() []byte
	PrivateBytes() []byte
}

// KeyGenerator creates new key pairs for a given scheme.
type KeyGenerator interface {
	GenerateKey(rand io.Reader) (KeyPair, error)
	Scheme() Scheme
	GenerateFromSeed(seed []byte) KeyPair
}

// SignatureEnvelope carries a signature alongside its scheme, public key, and
// optional context, allowing receivers to verify regardless of scheme.
type SignatureEnvelope struct {
	Scheme    Scheme `json:"scheme"`
	PublicKey []byte `json:"public_key"`
	Signature []byte `json:"signature"`
}

// Encode serializes the envelope. Format: scheme(1) || len(pubkey)(2) || pubkey || signature.
func (e *SignatureEnvelope) Encode() []byte {
	pkLen := len(e.PublicKey)
	sigLen := len(e.Signature)
	out := make([]byte, 1+2+pkLen+sigLen)
	out[0] = byte(e.Scheme)
	out[1] = byte(pkLen >> 8)
	out[2] = byte(pkLen)
	copy(out[3:], e.PublicKey)
	copy(out[3+pkLen:], e.Signature)
	return out
}

// DecodeSignatureEnvelope deserializes an envelope.
func DecodeSignatureEnvelope(data []byte) (*SignatureEnvelope, error) {
	if len(data) < 3 {
		return nil, ErrInvalidSignature
	}
	pkLen := int(data[1])<<8 | int(data[2])
	if len(data) < 3+pkLen+1 {
		return nil, ErrInvalidSignature
	}
	return &SignatureEnvelope{
		Scheme:    Scheme(data[0]),
		PublicKey: data[3 : 3+pkLen],
		Signature: data[3+pkLen:],
	}, nil
}

// ParseScheme parses a scheme name string.
func ParseScheme(s string) (Scheme, bool) {
	switch s {
	case "secp256k1", "ecdsa":
		return SchemeECDSA, true
	case "mldsa44", "mldsa-44":
		return SchemeMLDSA44, true
	case "mldsa65", "mldsa-65":
		return SchemeMLDSA65, true
	case "mldsa87", "mldsa-87":
		return SchemeMLDSA87, true
	case "sphincs", "sphincs-sha256-128s":
		return SchemeSPHINCS, true
	default:
		return 0, false
	}
}
