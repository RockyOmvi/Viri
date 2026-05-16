package crypto

import (
	"math/big"
	"testing"
)

func FuzzP256KeyGenSignVerify(f *testing.F) {
	f.Add([]byte("p256 test data"))
	f.Add([]byte{0xde, 0xad, 0xbe, 0xef})
	f.Add(make([]byte, 256))
	f.Add([]byte(""))
	f.Add([]byte{0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		key, err := GenerateP256Key()
		if err != nil {
			t.Skip()
		}
		sig, err := key.Sign(data)
		if err != nil {
			t.Skip()
		}
		if !key.PubKey().Verify(data, sig) {
			t.Errorf("P256 valid signature rejected")
		}
	})
}

func FuzzP256KeySerializationRoundtrip(f *testing.F) {
	f.Add([]byte{0x01, 0x02, 0x03, 0x04})
	f.Add(make([]byte, 32))
	f.Add([]byte{0xff})

	f.Fuzz(func(t *testing.T, seed []byte) {
		if len(seed) == 0 {
			return
		}
		priv, err := P256PrivateKeyFromBytes(seed)
		if err != nil {
			return
		}
		pubBytes := priv.PubKey().Bytes()
		pub2, err := P256PubKeyFromBytes(pubBytes)
		if err != nil {
			return
		}
		sig, signErr := priv.Sign([]byte("msg"))
		if signErr != nil {
			t.Skip()
		}
		if !pub2.Verify([]byte("msg"), sig) {
			t.Errorf("roundtrip pubkey verify failed")
		}
	})
}

func FuzzP256AddressGen(f *testing.F) {
	f.Add([]byte("addr data"))
	f.Add([]byte{0x01, 0x02})
	f.Add(make([]byte, 128))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		key, err := GenerateP256Key()
		if err != nil {
			t.Skip()
		}
		addr := key.PubKey().Address()
		if len(addr) != 20 {
			t.Errorf("address length expected 20 got %d", len(addr))
		}
	})
}

func FuzzKeccak256Properties(f *testing.F) {
	f.Add([]byte("keccak data"))
	f.Add([]byte{})
	f.Add(make([]byte, 10000))
	f.Add([]byte{0x00, 0x01, 0x02, 0x03, 0x04})

	f.Fuzz(func(t *testing.T, data []byte) {
		h := Keccak256(data)
		if len(h) != 32 {
			t.Errorf("keccak256 should produce 32 bytes got %d", len(h))
		}
		h2 := Keccak256(data)
		if string(h) != string(h2) {
			t.Errorf("keccak256 not deterministic")
		}
	})
}

func FuzzDoubleSHA256Properties(f *testing.F) {
	f.Add([]byte("double test"))
	f.Add([]byte{})
	f.Add(make([]byte, 10000))
	f.Add([]byte{0x00})
	f.Add([]byte{0xff, 0xee, 0xdd, 0xcc})

	f.Fuzz(func(t *testing.T, data []byte) {
		h := DoubleSHA256(data)
		if len(h) != 32 {
			t.Errorf("DoubleSHA256 should produce 32 bytes got %d", len(h))
		}
		single := SHA256(SHA256(data))
		if string(h) != string(single) {
			t.Errorf("DoubleSHA256 != SHA256(SHA256())")
		}
	})
}

func FuzzMerkleTreeWithArbitraryLeaves(f *testing.F) {
	f.Add([]byte("leaf1"), []byte("leaf2"), []byte("leaf3"), []byte("leaf4"))
	f.Add([]byte{0x00}, []byte{0x01}, []byte{0x02}, []byte{})
	f.Add([]byte{}, []byte{}, []byte{}, []byte{})

	f.Fuzz(func(t *testing.T, a, b, c, d []byte) {
		leaves := make([][]byte, 0, 4)
		for _, leaf := range [][]byte{a, b, c, d} {
			if len(leaf) > 0 {
				leaves = append(leaves, leaf)
			}
		}
		if len(leaves) < 2 {
			return
		}
		tree, err := NewMerkleTree(leaves)
		if err != nil {
			t.Skip()
		}
		if len(tree.RootHash) == 0 {
			t.Errorf("empty root hash")
		}
		proof, err := tree.GenerateProof(0)
		if err != nil {
			return
		}
		if !VerifyProof(tree.RootHash, leaves[0], proof, 0) {
			t.Errorf("merkle proof verification failed")
		}
	})
}

func FuzzMerkleProofTamperDetection(f *testing.F) {
	f.Add([]byte("good"), []byte("bad"))
	f.Add([]byte{0x01}, []byte{0x02})

	f.Fuzz(func(t *testing.T, good, bad []byte) {
		if len(good) == 0 || len(bad) == 0 {
			return
		}
		tree, err := NewMerkleTree([][]byte{good, bad})
		if err != nil {
			t.Skip()
		}
		proof, err := tree.GenerateProof(0)
		if err != nil {
			return
		}
		tamperedData := make([]byte, len(good))
		copy(tamperedData, good)
		if len(tamperedData) > 0 {
			tamperedData[0] ^= 0xFF
		}
		if VerifyProof(tree.RootHash, tamperedData, proof, 0) {
			t.Errorf("proof accepted for tampered data")
		}
	})
}

func FuzzSignatureFromBytesRoundtrip(f *testing.F) {
	f.Add([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f})
	f.Add(make([]byte, 64))
	f.Add([]byte{0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) != 64 {
			_, err := SignatureFromBytes(data)
			if err == nil {
				t.Errorf("expected error for len %d", len(data))
			}
			return
		}
		sig, err := SignatureFromBytes(data)
		if err != nil {
			t.Errorf("unexpected error for 64-byte input: %v", err)
			return
		}
		recovered := sig.Bytes()
		if string(data) != string(recovered) {
			t.Errorf("signature roundtrip mismatch")
		}
	})
}

func FuzzECDSASignTampered(f *testing.F) {
	f.Add([]byte("ecdsa tamper"))
	f.Add([]byte{0x01, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		key, err := GenerateKey()
		if err != nil {
			t.Skip()
		}
		hash := SHA256(data)
		sig, err := key.SignHash(hash)
		if err != nil {
			t.Skip()
		}
		tampered := &Signature{
			R: new(big.Int).Add(sig.R, big.NewInt(1)),
			S: sig.S,
		}
		if key.PubKey().VerifyHash(hash, tampered) {
			t.Errorf("verify passed for tampered sig")
		}
		tampered2 := &Signature{
			R: sig.R,
			S: new(big.Int).Add(sig.S, big.NewInt(1)),
		}
		if key.PubKey().VerifyHash(hash, tampered2) {
			t.Errorf("verify passed for tampered sig2")
		}
	})
}

func FuzzPrivateKeyFromBytesEdgeCases(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add(make([]byte, 31))
	f.Add(make([]byte, 33))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			_, err := PrivateKeyFromBytes(data)
			if err == nil {
				t.Errorf("expected error for empty key")
			}
			return
		}
		key, err := PrivateKeyFromBytes(data)
		if err != nil {
			return
		}
		if key.PubKey() == nil {
			t.Errorf("pubkey is nil for valid privkey")
		}
	})
}

func FuzzPubKeyFromBytesEdgeCases(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add(make([]byte, 64))
	f.Add([]byte{0x04, 0x00, 0x01, 0x02})

	f.Fuzz(func(t *testing.T, data []byte) {
		pub, err := PubKeyFromBytes(data)
		if err != nil {
			return
		}
		if pub.Address() == nil {
			t.Errorf("nil address from pubkey")
		}
		if len(pub.Address()) != 20 {
			t.Errorf("address length expected 20 got %d", len(pub.Address()))
		}
	})
}
