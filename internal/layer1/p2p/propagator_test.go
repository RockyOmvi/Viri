package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestPropagatorIsSeen(t *testing.T) {
	prop := NewPropagator(time.Minute, time.Minute)

	if prop.IsBlockSeen("block1") {
		t.Error("Block should not be seen initially")
	}

	if prop.IsTxSeen("tx1") {
		t.Error("Tx should not be seen initially")
	}
}

func TestPropagatorMarkBlockSeen(t *testing.T) {
	prop := NewPropagator(time.Minute, time.Minute)

	if !prop.MarkBlockSeen("block1", "peer1") {
		t.Error("First mark should succeed")
	}

	if prop.MarkBlockSeen("block1", "peer2") {
		t.Error("Second mark should fail (already seen)")
	}

	if !prop.IsBlockSeen("block1") {
		t.Error("Block should be marked as seen")
	}
}

func TestPropagatorMarkTxSeen(t *testing.T) {
	prop := NewPropagator(time.Minute, time.Minute)

	if !prop.MarkTxSeen("tx1", "peer1") {
		t.Error("First mark should succeed")
	}

	if prop.MarkTxSeen("tx1", "peer2") {
		t.Error("Second mark should fail (already seen)")
	}

	if !prop.IsTxSeen("tx1") {
		t.Error("Tx should be marked as seen")
	}
}

func TestPropagatorGetPeersToPropagate(t *testing.T) {
	prop := NewPropagator(time.Minute, time.Minute)

	prop.AddPendingBlock("block1", []peer.ID{"peer1", "peer2", "peer3"})

	peers := prop.GetPeersToPropagate("block1", true, "peer2")

	if len(peers) != 2 {
		t.Errorf("Expected 2 peers (excluding peer2), got %d", len(peers))
	}

	for _, p := range peers {
		if p == "peer2" {
			t.Error("Excluded peer should not be in result")
		}
	}
}

func TestPropagatorStats(t *testing.T) {
	prop := NewPropagator(time.Minute, time.Minute)

	prop.MarkBlockSeen("b1", "p1")
	prop.MarkBlockSeen("b2", "p1")
	prop.MarkTxSeen("t1", "p1")

	stats := prop.Stats()

	if stats.SeenBlocks != 2 {
		t.Errorf("Expected 2 seen blocks, got %d", stats.SeenBlocks)
	}

	if stats.SeenTxs != 1 {
		t.Errorf("Expected 1 seen tx, got %d", stats.SeenTxs)
	}
}

func TestPropagatorDefaultTTL(t *testing.T) {
	prop := NewPropagator(0, 0)

	if prop.blockTTL != 30*time.Minute {
		t.Errorf("Expected default blockTTL 30m, got %v", prop.blockTTL)
	}

	if prop.txTTL != 10*time.Minute {
		t.Errorf("Expected default txTTL 10m, got %v", prop.txTTL)
	}
}
