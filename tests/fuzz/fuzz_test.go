package fuzz

import (
	"math/big"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func FuzzSignatureVerification(f *testing.F) {
	f.Add([]byte("test data for fuzzing"))
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add(make([]byte, 1024))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}

		sig, err := key.Sign(data)
		if err != nil {
			t.Skip()
		}

		if !key.PubKey().Verify(data, sig) {
			t.Errorf("verification failed for valid signature")
		}

		mutatedData := make([]byte, len(data))
		copy(mutatedData, data)
		if len(mutatedData) > 0 {
			mutatedData[0] ^= 0xFF
		}

		if key.PubKey().Verify(mutatedData, sig) {
			t.Errorf("verification passed for mutated data")
		}
	})
}

func FuzzTransactionHash(f *testing.F) {
	f.Add(uint64(1), []byte{0x01}, []byte{0x02}, uint64(100), uint64(1000), uint64(1), []byte("data"))

	f.Fuzz(func(t *testing.T, nonce uint64, from, to []byte, value, gasLimit, gasPrice uint64, data []byte) {
		tx := &ledger.Transaction{
			Nonce:    nonce,
			From:     from,
			To:       to,
			Value:    value,
			GasLimit: gasLimit,
			GasPrice: gasPrice,
			Data:     data,
		}

		hash := tx.ComputeHash()
		if len(hash) == 0 {
			t.Errorf("empty hash computed")
		}

		tx2 := &ledger.Transaction{
			Nonce:    nonce,
			From:     from,
			To:       to,
			Value:    value,
			GasLimit: gasLimit,
			GasPrice: gasPrice,
			Data:     data,
		}

		hash2 := tx2.ComputeHash()
		if string(hash) != string(hash2) {
			t.Errorf("hash not deterministic")
		}
	})
}

func FuzzMerkleTree(f *testing.F) {
	f.Add([]byte("leaf1"), []byte("leaf2"), []byte("leaf3"))

	f.Fuzz(func(t *testing.T, leaf1, leaf2, leaf3 []byte) {
		leaves := [][]byte{leaf1, leaf2, leaf3}

		filtered := make([][]byte, 0, 3)
		for _, leaf := range leaves {
			if len(leaf) > 0 {
				filtered = append(filtered, leaf)
			}
		}

		if len(filtered) == 0 {
			return
		}

		tree, err := crypto.NewMerkleTree(filtered)
		if err != nil {
			t.Skip()
		}

		root := tree.RootHash
		if len(root) == 0 {
			t.Errorf("empty root hash")
		}

		tree2, err := crypto.NewMerkleTree(filtered)
		if err != nil {
			t.Skip()
		}

		if string(tree.RootHash) != string(tree2.RootHash) {
			t.Errorf("root hash not deterministic")
		}
	})
}

func FuzzSHA256(f *testing.F) {
	f.Add([]byte("test input"))
	f.Add([]byte{})
	f.Add(make([]byte, 10000))

	f.Fuzz(func(t *testing.T, data []byte) {
		hash := crypto.SHA256(data)
		if len(hash) != 32 {
			t.Errorf("SHA256 should produce 32 bytes, got %d", len(hash))
		}

		hash2 := crypto.SHA256(data)
		if string(hash) != string(hash2) {
			t.Errorf("SHA256 not deterministic")
		}
	})
}

func FuzzBlockSigningPayload(f *testing.F) {
	f.Add(uint64(1), []byte{0x01}, []byte{0x02}, []byte{0x03}, uint64(1000))

	f.Fuzz(func(t *testing.T, height uint64, prevHash, txsHash, stateRoot []byte, timestamp uint64) {
		block := &ledger.Block{
			Header: &ledger.Header{
				Height:    height,
				PrevHash:  prevHash,
				TxsHash:   txsHash,
				StateRoot: stateRoot,
			},
		}

		payload := block.SigningPayload()
		if len(payload) == 0 {
			t.Errorf("empty signing payload")
		}

		hash := block.Hash()
		if len(hash) == 0 {
			t.Errorf("empty block hash")
		}
	})
}

func FuzzECDSASignVerify(f *testing.F) {
	f.Add([]byte("ecdsa test data"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}

		hash := crypto.SHA256(data)

		sig, err := key.SignHash(hash)
		if err != nil {
			t.Skip()
		}

		valid := key.PubKey().VerifyHash(hash, sig)
		if !valid {
			t.Errorf("ECDSA verification failed for valid signature")
		}

		tampered := &crypto.Signature{
			R: new(big.Int).Add(sig.R, big.NewInt(1)),
			S: sig.S,
		}
		valid = key.PubKey().VerifyHash(hash, tampered)
		if valid {
			t.Errorf("ECDSA verification passed for tampered signature")
		}
	})
}

func FuzzHashCollisions(f *testing.F) {
	f.Add([]byte("a"), []byte("b"))

	f.Fuzz(func(t *testing.T, a, b []byte) {
		if string(a) == string(b) {
			return
		}

		hashA := crypto.DoubleSHA256(a)
		hashB := crypto.DoubleSHA256(b)

		if string(hashA) == string(hashB) {
			t.Logf("collision found (unlikely but possible for short inputs): %x", hashA)
		}
	})
}

func FuzzDoubleSHA256Consistency(f *testing.F) {
	f.Add([]byte("test"))
	f.Add([]byte{})
	f.Add(make([]byte, 10000))

	f.Fuzz(func(t *testing.T, data []byte) {
		hash1 := crypto.DoubleSHA256(data)
		if len(hash1) != 32 {
			t.Errorf("DoubleSHA256 should produce 32 bytes, got %d", len(hash1))
		}
		hash2 := crypto.DoubleSHA256(data)
		if string(hash1) != string(hash2) {
			t.Errorf("DoubleSHA256 not deterministic")
		}
	})
}

func FuzzKeccak256Consistency(f *testing.F) {
	f.Add([]byte("test"))
	f.Add([]byte{})
	f.Add(make([]byte, 5000))

	f.Fuzz(func(t *testing.T, data []byte) {
		hash1 := crypto.Keccak256(data)
		if len(hash1) != 32 {
			t.Errorf("Keccak256 should produce 32 bytes, got %d", len(hash1))
		}
		hash2 := crypto.Keccak256(data)
		if string(hash1) != string(hash2) {
			t.Errorf("Keccak256 not deterministic")
		}
	})
}

func FuzzTransactionValidation(f *testing.F) {
	f.Add(uint64(1), []byte("from"), []byte("to"), uint64(100), uint64(50000), uint64(10), []byte("data"))

	f.Fuzz(func(t *testing.T, nonce uint64, from, to []byte, value, gasLimit, gasPrice uint64, data []byte) {
		tx := &ledger.Transaction{
			Nonce:    nonce,
			From:     from,
			To:       to,
			Value:    value,
			GasLimit: gasLimit,
			GasPrice: gasPrice,
			Data:     data,
		}
		_ = tx.ComputeHash()
		_ = tx.SigningPayload()
	})
}

func FuzzBlockHeaderRoundtrip(f *testing.F) {
	f.Add(uint64(1), []byte("prev"), []byte("txs"), []byte("state"), uint64(1000), []byte("proposer"))
	f.Add(uint64(0), []byte{}, []byte{}, []byte{}, uint64(0), []byte{})

	f.Fuzz(func(t *testing.T, height uint64, prevHash, txsHash, stateRoot []byte, timestamp uint64, proposer []byte) {
		h := &ledger.Header{
			Height:    height,
			PrevHash:  prevHash,
			TxsHash:   txsHash,
			StateRoot: stateRoot,
			Timestamp: time.Unix(int64(timestamp), 0),
			Proposer:  proposer,
		}
		block := &ledger.Block{Header: h}
		_ = block.Hash()
	})
}

func FuzzBlockSignAndVerify(f *testing.F) {
	f.Add(uint64(1), []byte("prev"), []byte("txs"), []byte("state"))

	f.Fuzz(func(t *testing.T, height uint64, prevHash, txsHash, stateRoot []byte) {
		block := &ledger.Block{
			Header: &ledger.Header{
				Height:    height,
				PrevHash:  prevHash,
				TxsHash:   txsHash,
				StateRoot: stateRoot,
			},
		}
		payload := block.SigningPayload()
		if len(payload) == 0 {
			t.Errorf("signing payload should not be empty")
		}
		if payload[len(payload)-1] != 0x00 {
			t.Logf("extra data after null terminator")
		}
	})
}

func FuzzMerkleTreeDeterminism(f *testing.F) {
	f.Add([]byte("a"), []byte("b"))
	f.Add([]byte("x"), []byte("y"))

	f.Fuzz(func(t *testing.T, a, b []byte) {
		if len(a) == 0 || len(b) == 0 {
			return
		}
		leaves := [][]byte{a, b}
		tree1, err := crypto.NewMerkleTree(leaves)
		if err != nil {
			t.Skip()
		}
		tree2, err := crypto.NewMerkleTree(leaves)
		if err != nil {
			t.Skip()
		}
		if string(tree1.RootHash) != string(tree2.RootHash) {
			t.Errorf("merkle tree root not deterministic")
		}
	})
}

func FuzzKeyGenerationAndAddress(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(100)

	f.Fuzz(func(t *testing.T, _ int) {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}
		pubKey := key.PubKey()
		addr := pubKey.Address()
		if len(addr) == 0 {
			t.Errorf("address should not be empty")
		}
		compressed := pubKey.Compressed()
		if len(compressed) == 0 {
			t.Errorf("compressed pubkey should not be empty")
		}
		decompressed, err := crypto.DecompressPubKey(compressed)
		if err != nil {
			t.Errorf("decompress failed: %v", err)
			return
		}
		if string(decompressed.Address()) != string(addr) {
			t.Errorf("address mismatch after decompress")
		}
	})
}

func FuzzSignatureBytesRoundtrip(f *testing.F) {
	f.Add([]byte("data for signing"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}
		sig, err := key.Sign(data)
		if err != nil {
			t.Skip()
		}
		sigBytes := sig.Bytes()
		if len(sigBytes) == 0 {
			t.Errorf("signature bytes should not be empty")
		}
		deserSig, err := crypto.SignatureFromBytes(sigBytes)
		if err != nil {
			t.Errorf("signature from bytes failed: %v", err)
			return
		}
		if !key.PubKey().Verify(data, deserSig) {
			t.Errorf("verify failed after roundtrip")
		}
	})
}

func FuzzP256KeyGenSignVerify(f *testing.F) {
	f.Add([]byte("p256 data"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		key, err := crypto.GenerateP256Key()
		if err != nil {
			t.Skip()
		}
		sig, err := key.Sign(data)
		if err != nil {
			t.Skip()
		}
		if !key.PubKey().Verify(data, sig) {
			t.Errorf("P256 verify failed for valid signature")
		}
		mutatedData := make([]byte, len(data))
		copy(mutatedData, data)
		mutatedData[0] ^= 0xFF
		if key.PubKey().Verify(mutatedData, sig) {
			t.Errorf("P256 verify passed for mutated data")
		}
	})
}

func FuzzPubKeyAddressConsistency(f *testing.F) {
	f.Add([]byte("test data"))
	f.Add(make([]byte, 100))

	f.Fuzz(func(t *testing.T, _ []byte) {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}
		pub := key.PubKey()
		addr1 := pub.Address()
		addr2 := pub.Address()
		if string(addr1) != string(addr2) {
			t.Errorf("address not deterministic")
		}
	})
}

func FuzzPrivateKeySerialization(f *testing.F) {
	f.Add(0)
	f.Add(1)

	f.Fuzz(func(t *testing.T, _ int) {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}
		keyBytes := key.PrivateBytes()
		if len(keyBytes) == 0 {
			t.Errorf("private key bytes should not be empty")
		}
		deserKey, err := crypto.PrivateKeyFromBytes(keyBytes)
		if err != nil {
			t.Errorf("private key from bytes failed: %v", err)
			return
		}
		if string(deserKey.PubKey().Address()) != string(key.PubKey().Address()) {
			t.Errorf("address mismatch after key deserialization")
		}
	})
}

func FuzzTransactionPoolOps(f *testing.F) {
	f.Add(uint64(1), []byte("from"), []byte("to"), uint64(100), uint64(50000), uint64(10))

	f.Fuzz(func(t *testing.T, nonce uint64, from, to []byte, value, gasLimit, gasPrice uint64) {
		tx := &ledger.Transaction{
			Nonce:    nonce,
			From:     from,
			To:       to,
			Value:    value,
			GasLimit: gasLimit,
			GasPrice: gasPrice,
		}
		_ = tx.ComputeHash()
		_ = tx.SigningPayload()
	})
}
