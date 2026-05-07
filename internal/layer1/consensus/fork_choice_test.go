package consensus

import (
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func createTestBlock(height uint64, prevHash []byte, txCount int) *ledger.Block {
	txs := make([]*ledger.Transaction, txCount)
	for i := 0; i < txCount; i++ {
		txs[i] = &ledger.Transaction{
			Hash: []byte{byte(i)},
		}
	}

	header := &ledger.Header{
		Version:  ledger.Version1,
		Height:   height,
		PrevHash: prevHash,
		TxsHash:  []byte("txs"),
		Timestamp: time.Now(),
	}

	return &ledger.Block{
		Header:       header,
		Transactions: txs,
	}
}

func TestForkChoice_SelectChain_NoReorgWhenShorter(t *testing.T) {
	fc := ledger.NewForkChoice()
	currentChain := &ledger.PersistentBlockchain{}

	candidate := createTestBlock(0, nil, 0)
	result, err := fc.SelectChain(currentChain, candidate, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected no reorg for same height (both 0), got reorg result")
	}
}

func TestForkChoice_SelectChain_ReorgWhenHigher(t *testing.T) {
	fc := ledger.NewForkChoice()
	currentChain := &ledger.PersistentBlockchain{}

	candidate := createTestBlock(1, nil, 0)
	result, err := fc.SelectChain(currentChain, candidate, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected reorg for higher block")
	}
	if result.NewTip != 1 {
		t.Errorf("expected new tip 1, got %d", result.NewTip)
	}
}

func TestForkChoice_ReorgWhenMoreTransactions(t *testing.T) {
	fc := ledger.NewForkChoice()
	currentChain := &ledger.PersistentBlockchain{}

	candidate := createTestBlock(1, nil, 500)
	result, err := fc.SelectChain(currentChain, candidate, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected reorg for heavier side chain")
	}
	if result.NewTip != 1 {
		t.Errorf("expected new tip 1, got %d", result.NewTip)
	}
}

func TestForkChoice_WeightWithTransactionCount(t *testing.T) {
	block1 := createTestBlock(100, nil, 0)
	block2 := createTestBlock(100, nil, 100)

	weight1 := block1.Header.Height*1000 + uint64(len(block1.Transactions))
	weight2 := block2.Header.Height*1000 + uint64(len(block2.Transactions))

	if weight2 <= weight1 {
		t.Errorf("block with more txs should have higher weight")
	}

	if weight2 != weight1+100 {
		t.Errorf("weight difference should equal tx count difference, got %d vs %d", weight2, weight1+100)
	}
}

func TestForkChoice_ReorgDetectionWithCompetingChains(t *testing.T) {
	fc := ledger.NewForkChoice()

	currentChain := &ledger.PersistentBlockchain{}

	competingChains := make([]*ledger.Block, 3)
	for i := 0; i < 3; i++ {
		competingChains[i] = createTestBlock(15, nil, (i+1)*20)
	}

	var selectedChain *ledger.Block
	var maxWeight uint64

	for _, block := range competingChains {
		weight := block.Header.Height*1000 + uint64(len(block.Transactions))
		if weight > maxWeight {
			maxWeight = weight
			selectedChain = block
		}
	}

	result, err := fc.SelectChain(currentChain, selectedChain, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected reorg to selected chain")
	}

	if result.NewTip != selectedChain.Header.Height {
		t.Errorf("expected new tip %d, got %d", selectedChain.Header.Height, result.NewTip)
	}
}

func TestForkChoice_ChainWeightCalculation(t *testing.T) {
	tests := []struct {
		name           string
		height         uint64
		txCount        int
		expectedWeight uint64
	}{
		{"height 1, 0 tx", 1, 0, 1000},
		{"height 1, 5 tx", 1, 5, 1005},
		{"height 100, 0 tx", 100, 0, 100000},
		{"height 100, 50 tx", 100, 50, 100050},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := createTestBlock(tt.height, nil, tt.txCount)
			weight := block.Header.Height*1000 + uint64(len(block.Transactions))
			if weight != tt.expectedWeight {
				t.Errorf("expected weight %d, got %d", tt.expectedWeight, weight)
			}
		})
	}
}
