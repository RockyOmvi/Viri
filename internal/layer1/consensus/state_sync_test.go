package consensus

import (
	"encoding/binary"
	"encoding/json"
	"sync"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
)

// TestStateSyncGenesisToTip verifies the StateSyncer can catch up from
// genesis to the current chain tip across a large gap (100 blocks).
func TestStateSyncGenesisToTip(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	// Build a source chain with 100 blocks
	srcGenesis := ledger.TestGenesis()
	srcDB := state.NewMemoryStore()
	srcChain, err := ledger.NewPersistentBlockchain(srcGenesis, srcDB)
	if err != nil {
		t.Fatal(err)
	}

	blockData := make(map[uint64][]byte)
	var mu sync.Mutex

	for i := uint64(1); i <= 100; i++ {
		tx, err := ledger.NewTransactionFromKey(i, []byte{0x02}, 100, 100000, 1, nil, srcGenesis.ChainID, key)
		if err != nil {
			t.Fatal(err)
		}
		srcChain.TxPool().Add(tx)

		block, err := ledger.NewBlock(i, srcChain.TipHash(), srcChain.TxPool().GetPending(), key.PubKey().Address(), key)
		if err != nil {
			t.Fatal(err)
		}
		if err := srcChain.AddBlock(block); err != nil {
			t.Fatal(err)
		}

		serialized, err := json.Marshal(block)
		if err != nil {
			t.Fatal(err)
		}
		data := make([]byte, 8+len(serialized))
		binary.BigEndian.PutUint64(data[:8], i)
		copy(data[8:], serialized)
		mu.Lock()
		blockData[i] = data
		mu.Unlock()
	}

	targetHeight := srcChain.Height()
	if targetHeight != 100 {
		t.Fatalf("expected source chain height 100, got %d", targetHeight)
	}
	t.Logf("Source chain built: height=%d tip=%x", targetHeight, srcChain.TipHash())

	// Build an empty destination chain (starting from genesis)
	dstGenesis := ledger.TestGenesis()
	dstDB := state.NewMemoryStore()
	dstChain, err := ledger.NewPersistentBlockchain(dstGenesis, dstDB)
	if err != nil {
		t.Fatal(err)
	}

	if dstChain.Height() != 0 {
		t.Fatalf("expected destination chain height 0, got %d", dstChain.Height())
	}

	// Create a block requester that reads from our blockData map
	blockRequester := func(fromHeight, toHeight uint64) error {
		return nil
	}

	blockApplier := func(data []byte) error {
		if len(data) < 8 {
			return nil
		}
		payload := data[8:]
		var block ledger.Block
		if err := json.Unmarshal(payload, &block); err != nil {
			return err
		}
		if len(block.Transactions) > 0 {
			dstChain.TxPool().Add(block.Transactions[0])
		}
		return dstChain.AddBlock(&block)
	}

	heightGetter := func() uint64 {
		return dstChain.Height() + 1
	}

	syncer := NewStateSyncer(blockRequester, blockApplier, heightGetter, nil)

	// Manually feed blocks to the syncer (simulating P2P responses)
	syncer.StartSync(targetHeight)

	for i := uint64(1); i <= targetHeight; i++ {
		mu.Lock()
		data := blockData[i]
		mu.Unlock()
		if err := syncer.ReceiveBlock(data); err != nil {
			t.Fatalf("ReceiveBlock height=%d: %v", i, err)
		}
	}

	if syncer.IsSyncing() {
		t.Error("expected syncer to be done after all blocks applied")
	}

	finalHeight := dstChain.Height()
	if finalHeight != targetHeight {
		t.Errorf("expected destination chain height %d, got %d", targetHeight, finalHeight)
	}

	// Verify chain integrity - tip hashes should match
	srcTip := srcChain.TipHash()
	dstTip := dstChain.TipHash()
	if len(srcTip) != len(dstTip) {
		t.Fatal("tip hash length mismatch")
	}
	for i := range srcTip {
		if srcTip[i] != dstTip[i] {
			t.Errorf("tip hash mismatch at byte %d: src=%x dst=%x", i, srcTip, dstTip)
			break
		}
	}
	t.Logf("Chain sync verified: src_tip=%x dst_tip=%x height=%d", srcTip, dstTip, finalHeight)
}
