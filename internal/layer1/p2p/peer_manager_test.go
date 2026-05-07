package p2p

import (
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func TestPeerManagerAddPeer(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	p := pm.AddPeer("peer1", addr, 0)

	if p == nil {
		t.Fatal("Failed to add peer")
	}

	if p.Status != PeerStatusConnected {
		t.Errorf("Expected status Connected, got %v", p.Status)
	}

	if pm.PeerCount() != 1 {
		t.Errorf("Expected 1 peer, got %d", pm.PeerCount())
	}
}

func TestPeerManagerRemovePeer(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	pm.RemovePeer("peer1")

	if pm.PeerCount() != 0 {
		t.Errorf("Expected 0 peers after removal, got %d", pm.PeerCount())
	}
}

func TestPeerManagerBanPeer(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	pm.BanPeer("peer1", "test ban")

	if pm.PeerCount() != 0 {
		t.Error("Banned peer should be removed from active peers")
	}

	if !pm.IsBanned("peer1") {
		t.Error("Peer should be banned")
	}
}

func TestPeerManagerIsBannedExpired(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.BanDuration = 50 * time.Millisecond
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)
	pm.BanPeer("peer1", "short ban")

	time.Sleep(100 * time.Millisecond)

	if pm.IsBanned("peer1") {
		t.Error("Ban should have expired")
	}
}

func TestPeerManagerScoreUpdate(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	pm.UpdateScore("peer1", 10)

	peerInfo, _ := pm.GetPeer("peer1")
	if peerInfo.Score != 10 {
		t.Errorf("Expected score 10, got %d", peerInfo.Score)
	}
}

func TestPeerManagerScoreBan(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.ScoreThreshold = -10
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	pm.UpdateScore("peer1", -20)

	if !pm.IsBanned("peer1") {
		t.Error("Peer should be banned due to low score")
	}
}

func TestPeerManagerConnectedPeers(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)
	pm.AddPeer("peer2", addr, 0)
	pm.AddPeer("peer3", addr, 0)

	connected := pm.GetConnectedPeers()
	if len(connected) != 3 {
		t.Errorf("Expected 3 connected peers, got %d", len(connected))
	}

	pm.RemovePeer("peer2")

	connected = pm.GetConnectedPeers()
	if len(connected) != 2 {
		t.Errorf("Expected 2 connected peers, got %d", len(connected))
	}
}

func TestPeerManagerPeerCount(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	for i := 0; i < 10; i++ {
		pm.AddPeer(peer.ID(string(rune(i))), addr, 0)
	}

	if pm.PeerCount() != 10 {
		t.Errorf("Expected 10 peers, got %d", pm.PeerCount())
	}
}

func TestPeerManagerIsFull(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.MaxPeers = 2
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	pm.AddPeer("peer1", addr, 0)
	pm.AddPeer("peer2", addr, 0)

	if !pm.IsFull() {
		t.Error("Peer manager should be full")
	}
}

func TestPeerManagerNeedsPeers(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.MinPeers = 5
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	if !pm.NeedsPeers() {
		t.Error("Peer manager should need more peers")
	}
}

func TestPeerManagerStats(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)
	pm.AddPeer("peer2", addr, 0)

	stats := pm.Stats()

	if stats.TotalPeers != 2 {
		t.Errorf("Expected total peers 2, got %d", stats.TotalPeers)
	}

	if stats.Connected != 2 {
		t.Errorf("Expected connected 2, got %d", stats.Connected)
	}
}

func TestPeerManagerEvents(t *testing.T) {
	pm := NewPeerManager(nil)

	var events []PeerEvent
	var mu sync.Mutex

	pm.Subscribe(func(event PeerEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(events) == 0 {
		t.Error("Expected at least one event")
	}

	if events[0].Type != PeerConnectedEvent {
		t.Errorf("Expected connected event, got %s", events[0].Type)
	}
	mu.Unlock()
}

func TestPeerManagerClose(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	pm.Close()

	if pm.PeerCount() != 0 {
		t.Error("Peer count should be 0 after close")
	}
}

func TestPeerManagerUpdateHeight(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	pm.UpdateHeight("peer1", 1000)

	peerInfo, _ := pm.GetPeer("peer1")
	if peerInfo.Height != 1000 {
		t.Errorf("Expected height 1000, got %d", peerInfo.Height)
	}
}

func TestPeerManagerEviction(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.MaxPeers = 2
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)
	pm.AddPeer("peer2", addr, 0)

	pm.UpdateScore("peer1", -100)

	pm.AddPeer("peer3", addr, 0)

	if pm.PeerCount() != 2 {
		t.Errorf("Expected 2 peers after eviction, got %d", pm.PeerCount())
	}
}

func TestPeerManagerPenalizePeer(t *testing.T) {
	pm := NewPeerManager(nil)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("peer1", addr, 0)

	pm.PenalizePeer("peer1", "spam")

	peerInfo, _ := pm.GetPeer("peer1")
	if peerInfo.Status != PeerStatusPenalized {
		t.Errorf("Expected penalized status, got %v", peerInfo.Status)
	}
}

func TestDefaultPeerManagerConfig(t *testing.T) {
	config := DefaultPeerManagerConfig()

	if config.MaxPeers != 50 {
		t.Errorf("Expected MaxPeers 50, got %d", config.MaxPeers)
	}

	if config.MinPeers != 5 {
		t.Errorf("Expected MinPeers 5, got %d", config.MinPeers)
	}

	if config.BanDuration != 24*time.Hour {
		t.Errorf("Expected BanDuration 24h, got %v", config.BanDuration)
	}
}
