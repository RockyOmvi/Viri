package ledger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestSerializeBlock(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, nil, key)
	block, _ := NewBlock(1, bc.LatestBlock().Hash(), []*Transaction{tx}, key.PubKey().Address(), key)

	data, err := SerializeBlock(block)
	if err != nil {
		t.Fatalf("Failed to serialize block: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Serialized data is empty")
	}

	restored, err := DeserializeBlock(data)
	if err != nil {
		t.Fatalf("Failed to deserialize block: %v", err)
	}

	if restored.Header.Height != block.Header.Height {
		t.Errorf("Height mismatch: expected %d, got %d", block.Header.Height, restored.Header.Height)
	}

	if !crypto.EqualHash(restored.Header.PrevHash, block.Header.PrevHash) {
		t.Error("PrevHash mismatch")
	}
}

func TestSerializeTransaction(t *testing.T) {
	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, []byte("data"), key)

	data, err := SerializeTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to serialize transaction: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Serialized data is empty")
	}

	restored, err := DeserializeTransaction(data)
	if err != nil {
		t.Fatalf("Failed to deserialize transaction: %v", err)
	}

	if restored.Nonce != tx.Nonce {
		t.Errorf("Nonce mismatch: expected %d, got %d", tx.Nonce, restored.Nonce)
	}

	if restored.Value != tx.Value {
		t.Errorf("Value mismatch: expected %d, got %d", tx.Value, restored.Value)
	}
}

func TestSerializeHeader(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, _ := crypto.GenerateKey()
	block, _ := NewBlock(1, bc.LatestBlock().Hash(), nil, key.PubKey().Address(), key)

	data, err := SerializeHeader(block.Header)
	if err != nil {
		t.Fatalf("Failed to serialize header: %v", err)
	}

	restored, err := DeserializeHeader(data)
	if err != nil {
		t.Fatalf("Failed to deserialize header: %v", err)
	}

	if restored.Height != block.Header.Height {
		t.Errorf("Height mismatch: expected %d, got %d", block.Header.Height, restored.Height)
	}

	if restored.Version != block.Header.Version {
		t.Errorf("Version mismatch: expected %d, got %d", block.Header.Version, restored.Version)
	}
}

func TestBlockToJSON(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, nil, key)
	block, _ := NewBlock(1, bc.LatestBlock().Hash(), []*Transaction{tx}, key.PubKey().Address(), key)

	data, err := BlockToJSON(block)
	if err != nil {
		t.Fatalf("Failed to convert block to JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON output is invalid: %v", err)
	}

	txCount := int(parsed["tx_count"].(float64))
	if txCount != 1 {
		t.Errorf("Expected 1 transaction, got %d", txCount)
	}
}

func TestTransactionToJSON(t *testing.T) {
	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(42, key.PubKey().Address(), 9999, 5000, 20, []byte("test"), key)

	data, err := TransactionToJSON(tx)
	if err != nil {
		t.Fatalf("Failed to convert transaction to JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON output is invalid: %v", err)
	}

	nonce := int(parsed["nonce"].(float64))
	if nonce != 42 {
		t.Errorf("Expected nonce 42, got %d", nonce)
	}
}

func TestBlocksToJSON(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, _ := crypto.GenerateKey()
	block, _ := NewBlock(1, bc.LatestBlock().Hash(), nil, key.PubKey().Address(), key)
	bc.AddBlock(block)

	blocks := []*Block{}
	for h := uint64(0); h <= 1; h++ {
		b, _ := bc.GetBlock(h)
		blocks = append(blocks, b)
	}

	data, err := BlocksToJSON(blocks)
	if err != nil {
		t.Fatalf("Failed to convert blocks to JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON output is invalid: %v", err)
	}

	count := int(parsed["count"].(float64))
	if count != 2 {
		t.Errorf("Expected 2 blocks, got %d", count)
	}
}

func TestWriteBlockToFile(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, _ := crypto.GenerateKey()
	block, _ := NewBlock(1, bc.LatestBlock().Hash(), nil, key.PubKey().Address(), key)

	var buf bytes.Buffer
	if err := WriteBlockToFile(block, &buf); err != nil {
		t.Fatalf("Failed to write block to file: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Written data is empty")
	}
}

func TestReadBlockFromFile(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, _ := crypto.GenerateKey()
	block, _ := NewBlock(1, bc.LatestBlock().Hash(), nil, key.PubKey().Address(), key)

	var buf bytes.Buffer
	WriteBlockToFile(block, &buf)

	restored, err := ReadBlockFromFile(&buf)
	if err != nil {
		t.Fatalf("Failed to read block from file: %v", err)
	}

	if restored.Header.Height != block.Header.Height {
		t.Errorf("Height mismatch: expected %d, got %d", block.Header.Height, restored.Header.Height)
	}
}

func TestBlockJSONContainsHashes(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, nil, key)
	block, _ := NewBlock(1, bc.LatestBlock().Hash(), []*Transaction{tx}, key.PubKey().Address(), key)

	data, _ := BlockToJSON(block)
	jsonStr := string(data)

	if !strings.Contains(jsonStr, "hash") {
		t.Error("JSON should contain block hash")
	}
	if !strings.Contains(jsonStr, "tx_hashes") {
		t.Error("JSON should contain tx_hashes")
	}
	if !strings.Contains(jsonStr, "tx_count") {
		t.Error("JSON should contain tx_count")
	}
}
