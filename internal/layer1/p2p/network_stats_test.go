package p2p

import (
	"testing"
	"time"
)

func TestNetworkStatsRecord(t *testing.T) {
	ns := NewNetworkStats()

	ns.RecordMessageIn(100)
	ns.RecordMessageOut(200)

	snap := ns.Snapshot()

	if snap.TotalMessagesIn != 1 {
		t.Errorf("Expected 1 message in, got %d", snap.TotalMessagesIn)
	}

	if snap.TotalMessagesOut != 1 {
		t.Errorf("Expected 1 message out, got %d", snap.TotalMessagesOut)
	}

	if snap.TotalBytesIn != 100 {
		t.Errorf("Expected 100 bytes in, got %d", snap.TotalBytesIn)
	}

	if snap.TotalBytesOut != 200 {
		t.Errorf("Expected 200 bytes out, got %d", snap.TotalBytesOut)
	}
}

func TestNetworkStatsBlocksAndTxs(t *testing.T) {
	ns := NewNetworkStats()

	ns.RecordBlockIn()
	ns.RecordBlockOut()
	ns.RecordTxIn()
	ns.RecordTxOut()

	snap := ns.Snapshot()

	if snap.TotalBlocksIn != 1 {
		t.Errorf("Expected 1 block in, got %d", snap.TotalBlocksIn)
	}

	if snap.TotalBlocksOut != 1 {
		t.Errorf("Expected 1 block out, got %d", snap.TotalBlocksOut)
	}

	if snap.TotalTxsIn != 1 {
		t.Errorf("Expected 1 tx in, got %d", snap.TotalTxsIn)
	}

	if snap.TotalTxsOut != 1 {
		t.Errorf("Expected 1 tx out, got %d", snap.TotalTxsOut)
	}
}

func TestNetworkStatsRejectedAndDropped(t *testing.T) {
	ns := NewNetworkStats()

	ns.RecordRejected()
	ns.RecordRejected()
	ns.RecordDropped()

	snap := ns.Snapshot()

	if snap.RejectedMessages != 2 {
		t.Errorf("Expected 2 rejected, got %d", snap.RejectedMessages)
	}

	if snap.DroppedMessages != 1 {
		t.Errorf("Expected 1 dropped, got %d", snap.DroppedMessages)
	}
}

func TestNetworkStatsPeakPeers(t *testing.T) {
	ns := NewNetworkStats()

	ns.SetPeerCount(5)
	ns.SetPeerCount(10)
	ns.SetPeerCount(3)

	snap := ns.Snapshot()

	if snap.CurrentPeers != 3 {
		t.Errorf("Expected current peers 3, got %d", snap.CurrentPeers)
	}

	if snap.PeakPeers != 10 {
		t.Errorf("Expected peak peers 10, got %d", snap.PeakPeers)
	}
}

func TestNetworkStatsReset(t *testing.T) {
	ns := NewNetworkStats()

	ns.RecordMessageIn(100)
	ns.RecordMessageOut(200)
	ns.RecordBlockIn()

	ns.Reset()

	snap := ns.Snapshot()

	if snap.TotalMessagesIn != 0 {
		t.Errorf("Expected 0 messages in after reset, got %d", snap.TotalMessagesIn)
	}

	if snap.TotalMessagesOut != 0 {
		t.Errorf("Expected 0 messages out after reset, got %d", snap.TotalMessagesOut)
	}

	if snap.Uptime < 0 {
		t.Error("Uptime should be non-negative after reset")
	}
}

func TestNetworkStatsUptime(t *testing.T) {
	ns := NewNetworkStats()

	time.Sleep(10 * time.Millisecond)

	snap := ns.Snapshot()

	if snap.Uptime < 10*time.Millisecond {
		t.Errorf("Expected uptime >= 10ms, got %v", snap.Uptime)
	}
}
