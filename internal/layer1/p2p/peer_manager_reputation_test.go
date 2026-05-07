package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func TestReputationScoreCalculation(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	pID := peer.ID("test-peer-1")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	peerInfo, _ := pm.GetPeer(pID)

	score := pm.calculateReputationScore(peerInfo)
	if score != 100 {
		t.Errorf("expected base reputation score 100, got %d", score)
	}
}

func TestOnBlockReceivedValid(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	pID := peer.ID("test-peer")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	initialScore := pm.peers[pID].Score

	pm.OnBlockReceived(pID, true)

	if pm.peers[pID].Score != initialScore+10 {
		t.Errorf("expected score to increase by 10, got %d", pm.peers[pID].Score)
	}

	if pm.peers[pID].Behavior.BlocksValidated != 1 {
		t.Errorf("expected BlocksValidated to be 1, got %d", pm.peers[pID].Behavior.BlocksValidated)
	}
}

func TestOnBlockReceivedInvalid(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	pID := peer.ID("test-peer")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	initialScore := pm.peers[pID].Score

	pm.OnBlockReceived(pID, false)

	if pm.peers[pID].Score != initialScore-10 {
		t.Errorf("expected score to decrease by 10, got %d", pm.peers[pID].Score)
	}

	if pm.peers[pID].Behavior.InvalidMessages != 1 {
		t.Errorf("expected InvalidMessages to be 1, got %d", pm.peers[pID].Behavior.InvalidMessages)
	}
}

func TestOnInvalidMessagePenalty(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	pID := peer.ID("test-peer")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	initialScore := pm.peers[pID].Score

	pm.OnInvalidMessage(pID, "malformed message")

	if pm.peers[pID].Score != initialScore-10 {
		t.Errorf("expected score to decrease by 10, got %d", pm.peers[pID].Score)
	}
}

func TestOnDuplicateMessageSeverePenalty(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	pID := peer.ID("test-peer")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	initialScore := pm.peers[pID].Score

	pm.OnDuplicateMessage(pID)

	if pm.peers[pID].Score != initialScore-20 {
		t.Errorf("expected score to decrease by 20, got %d", pm.peers[pID].Score)
	}

	if pm.peers[pID].Behavior.DuplicateMessages != 1 {
		t.Errorf("expected DuplicateMessages to be 1, got %d", pm.peers[pID].Behavior.DuplicateMessages)
	}
}

func TestScoreDecayAfterInactivity(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	pID := peer.ID("test-peer")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	pm.peers[pID].Behavior.LastSeen = time.Now().Add(-2 * time.Hour)

	score := pm.calculateReputationScore(pm.peers[pID])

	expectedScore := int(float64(100) * 0.9)
	if score != expectedScore {
		t.Errorf("expected decayed score %d, got %d", expectedScore, score)
	}
}

func TestGetHighQualityPeers(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	for i := 0; i < 5; i++ {
		pID := peer.ID("peer-" + string(rune('A'+i)))
		addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
		pm.AddPeer(pID, addr, 0)

		for j := 0; j < i*10; j++ {
			pm.OnBlockReceived(pID, true)
		}
	}

	topPeers := pm.GetHighQualityPeers(3)
	if len(topPeers) != 3 {
		t.Errorf("expected 3 high quality peers, got %d", len(topPeers))
	}

	for i := 0; i < len(topPeers)-1; i++ {
		scoreI := pm.calculateReputationScore(topPeers[i])
		scoreJ := pm.calculateReputationScore(topPeers[i+1])
		if scoreI < scoreJ {
			t.Error("peers not sorted by reputation score")
		}
	}
}

func TestShouldEvictLowScorePeers(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.ScoreThreshold = -200
	pm := NewPeerManager(config)

	pID := peer.ID("low-score-peer")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	for i := 0; i < 10; i++ {
		pm.OnInvalidMessage(pID, "bad message")
	}

	shouldEvict := pm.ShouldEvict(string(pID))
	if !shouldEvict {
		t.Error("expected low-score peer to be evicted")
	}
}

func TestShouldNotEvictHighScorePeers(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	pID := peer.ID("high-score-peer")
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	pm.AddPeer(pID, addr, 0)

	for i := 0; i < 20; i++ {
		pm.OnBlockReceived(pID, true)
	}

	shouldEvict := pm.ShouldEvict(string(pID))
	if shouldEvict {
		t.Error("should not evict high-score peer")
	}
}

func TestGetClassificationThresholds(t *testing.T) {
	tests := []struct {
		name           string
		setupPeer      func(pm *PeerManager, pID peer.ID)
		expectedClass  PeerClassification
	}{
		{
			"trusted peer",
			func(pm *PeerManager, pID peer.ID) {
				pm.peers[pID].Behavior.ConnectedSince = time.Now().Add(-2 * time.Hour)
				for i := 0; i < 60; i++ {
					pm.OnBlockReceived(pID, true)
				}
			},
			PeerTrusted,
		},
		{
			"healthy peer",
			func(pm *PeerManager, pID peer.ID) {
				for i := 0; i < 10; i++ {
					pm.OnBlockReceived(pID, true)
				}
			},
			PeerHealthy,
		},
		{
			"suspicious peer",
			func(pm *PeerManager, pID peer.ID) {
				for i := 0; i < 5; i++ {
					pm.OnInvalidMessage(pID, "bad")
				}
			},
			PeerSuspicious,
		},
		{
			"toxic peer",
			func(pm *PeerManager, pID peer.ID) {
				for i := 0; i < 10; i++ {
					pm.OnInvalidMessage(pID, "bad")
				}
			},
			PeerToxic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPeerManager(DefaultPeerManagerConfig())
			pID := peer.ID("test-peer")
			addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
			pm.AddPeer(pID, addr, 0)

			tt.setupPeer(pm, pID)

			peerInfo, exists := pm.GetPeer(pID)
			if !exists {
				t.Skip("peer was banned and removed")
				return
			}
			classification := pm.GetClassification(peerInfo)
			if classification != tt.expectedClass {
				t.Errorf("expected classification %v, got %v", tt.expectedClass, classification)
			}
		})
	}
}

func TestGetPeersForSync(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	for i := 0; i < 5; i++ {
		pID := peer.ID("peer-" + string(rune('A'+i)))
		addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
		pm.AddPeer(pID, addr, 0)
		pm.UpdateHeight(pID, uint64(100-i*10))
	}

	peersForSync := pm.GetPeersForSync()
	if len(peersForSync) != 5 {
		t.Errorf("expected 5 peers for sync, got %d", len(peersForSync))
	}

	for i := 0; i < len(peersForSync)-1; i++ {
		scoreI := pm.calculateReputationScore(peersForSync[i])
		scoreJ := pm.calculateReputationScore(peersForSync[i+1])
		if scoreI < scoreJ {
			t.Error("peers not sorted by reputation score for sync")
		} else if scoreI == scoreJ {
			if peersForSync[i].Height < peersForSync[i+1].Height {
				t.Error("peers with same score not sorted by height")
			}
		}
	}
}

func TestReputationReport(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())

	for i := 0; i < 3; i++ {
		pID := peer.ID("peer-" + string(rune('A'+i)))
		addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
		pm.AddPeer(pID, addr, 0)
	}

	report := pm.GetReputationReport()
	if len(report) != 3 {
		t.Errorf("expected 3 reputation entries, got %d", len(report))
	}
}
