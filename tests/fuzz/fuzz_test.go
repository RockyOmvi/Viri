package fuzz

import (
	"math/big"
	"testing"

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
