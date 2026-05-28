package crypto

import (
	crand "crypto/rand"
	"io"

	"github.com/cloudflare/circl/sign/slhdsa"
)

const (
	sphincsPKBytes  = 32
	sphincsSKBytes  = 64
	sphincsSigBytes = 7856
	sphincsSeedLen  = 16
)

type sphincsSigner struct {
	sk   slhdsa.PrivateKey
	pk   slhdsa.PublicKey
	seed []byte
}

type sphincsVerifier struct {
	pk slhdsa.PublicKey
}

func SphincsGenerateKey(rand io.Reader) (KeyPair, error) {
	if rand == nil {
		rand = crand.Reader
	}
	pub, priv, err := slhdsa.GenerateKey(rand, slhdsa.SHA2_128s)
	if err != nil {
		return nil, err
	}
	b, _ := priv.MarshalBinary()
	return &sphincsSigner{sk: priv, pk: pub, seed: b[:sphincsSeedLen]}, nil
}

func (s *sphincsSigner) SignMessage(data []byte) ([]byte, error) {
	return slhdsa.SignDeterministic(&s.sk, slhdsa.NewMessage(data), nil)
}

func (s *sphincsSigner) Scheme() Scheme { return SchemeSPHINCS }

func (s *sphincsSigner) PublicBytes() []byte {
	b, _ := s.pk.MarshalBinary()
	return b
}

func (s *sphincsSigner) VerifyMessage(data, sig []byte) bool {
	return slhdsa.Verify(&s.pk, slhdsa.NewMessage(data), sig, nil)
}

func (s *sphincsSigner) Seed() []byte { return s.seed }

func (s *sphincsSigner) PrivateBytes() []byte {
	b, _ := s.sk.MarshalBinary()
	return b
}

func (s *sphincsVerifier) VerifyMessage(data, sig []byte) bool {
	return slhdsa.Verify(&s.pk, slhdsa.NewMessage(data), sig, nil)
}

func (s *sphincsVerifier) Scheme() Scheme { return SchemeSPHINCS }

func (s *sphincsVerifier) PublicBytes() []byte {
	b, _ := s.pk.MarshalBinary()
	return b
}
