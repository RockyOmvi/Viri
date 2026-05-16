package ledger

import (
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestNewBlockchain(t *testing.T) {
	genesis := TestGenesis()
	bc, err := NewBlockchain(genesis)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	if bc.Height() != 0 {
		t.Fatalf("Expected height 0, got %d", bc.Height())
	}

	if bc.LatestBlock() == nil {
		t.Fatal("Latest block is nil")
	}
}

func TestAddBlock(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	tx, err := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 1, nil, 1337, key)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	block, err := NewBlock(1, bc.LatestBlock().Hash(), []*Transaction{tx}, key.PubKey().Address(), key)
	if err != nil {
		t.Fatalf("Failed to create block: %v", err)
	}

	if err := bc.AddBlock(block); err != nil {
		t.Fatalf("Failed to add block: %v", err)
	}

	if bc.Height() != 1 {
		t.Fatalf("Expected height 1, got %d", bc.Height())
	}
}

func TestInvalidBlockHeight(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	block, _ := NewBlock(5, bc.LatestBlock().Hash(), nil, key.PubKey().Address(), key)

	err = bc.AddBlock(block)
	if err == nil {
		t.Fatal("Should have failed for invalid height")
	}
}

func TestInvalidPrevHash(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	block, _ := NewBlock(1, crypto.SHA256([]byte("wrong")), nil, key.PubKey().Address(), key)

	err = bc.AddBlock(block)
	if err == nil {
		t.Fatal("Should have failed for invalid prev hash")
	}
}

func TestGetBlock(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	block, err := bc.GetBlock(0)
	if err != nil {
		t.Fatalf("Failed to get genesis block: %v", err)
	}

	if block.Header.Height != 0 {
		t.Fatalf("Expected height 0, got %d", block.Header.Height)
	}

	_, err = bc.GetBlock(999)
	if err == nil {
		t.Fatal("Should have failed for non-existent block")
	}
}

func TestValidate(t *testing.T) {
	genesis := TestGenesis()
	bc, _ := NewBlockchain(genesis)

	if !bc.Validate() {
		t.Fatal("Blockchain validation failed")
	}
}
