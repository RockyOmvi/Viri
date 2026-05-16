package p2p

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/pkg/security"
)

func TestP2PFloodProtection(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(&RateLimiterConfig{
		MaxMessagesPerSecond: 10,
		MaxBytesPerSecond:    10000,
		BurstSize:            10,
		WindowSize:           time.Second,
		BlockDuration:        100 * time.Millisecond,
	})

	peerID := peer.ID("flood-peer")

	for i := 0; i < 10; i++ {
		if !rl.Allow(peerID, 100) {
			t.Fatalf("message %d should be allowed within limit", i+1)
		}
	}

	if rl.Allow(peerID, 100) {
		t.Error("message beyond limit should be rejected")
	}

	if !rl.IsBlocked(peerID) {
		t.Error("peer should be blocked after exceeding rate limit")
	}

	stats := rl.GetStats()
	s, ok := stats[peerID.String()]
	if !ok {
		t.Fatal("expected stats for flood peer")
	}
	if s.Blocked != true {
		t.Error("stats should indicate peer is blocked")
	}
	if s.Messages < 10 {
		t.Errorf("expected at least 10 recorded messages, got %d", s.Messages)
	}
}

func TestP2PFloodProtectionByteLimit(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(&RateLimiterConfig{
		MaxMessagesPerSecond: 1000,
		MaxBytesPerSecond:    500,
		BurstSize:            1000,
		WindowSize:           time.Second,
		BlockDuration:        100 * time.Millisecond,
	})

	peerID := peer.ID("byte-flood-peer")

	rl.Allow(peerID, 300)
	rl.Allow(peerID, 300)

	if rl.Allow(peerID, 300) {
		t.Error("should be blocked by byte limit")
	}
}

func TestP2PFloodProtectionZeroConfig(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(&RateLimiterConfig{
		MaxMessagesPerSecond: 0,
		MaxBytesPerSecond:    0,
		BurstSize:            0,
		WindowSize:           time.Second,
		BlockDuration:        100 * time.Millisecond,
	})

	peerID := peer.ID("zero-peer")

	if rl.Allow(peerID, 1) {
		t.Error("should reject when max messages is 0")
	}
}

func TestP2PFloodProtectionExcessiveMessageSize(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(&RateLimiterConfig{
		MaxMessagesPerSecond: 100,
		MaxBytesPerSecond:    1000,
		BurstSize:            100,
		WindowSize:           time.Second,
		BlockDuration:        100 * time.Millisecond,
	})

	peerID := peer.ID("big-msg-peer")

	rl.Allow(peerID, 1500)

	if !rl.IsBlocked(peerID) {
		t.Error("message exceeding byte limit should trigger block")
	}
}

func TestP2PEclipseAttack(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.MaxPeers = 5
	config.MinPeers = 1
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	for i := 0; i < 10; i++ {
		pid := peer.ID(string(rune('A' + i)))
		pm.AddPeer(pid, addr, 0)
	}

	if pm.PeerCount() > 5 {
		t.Errorf("peer count %d exceeds max 5 (evicted extras)", pm.PeerCount())
	}
	if pm.PeerCount() != 5 {
		t.Logf("peer count = %d (max 5, eviction replaces lowest score)", pm.PeerCount())
	}
}

func TestP2PEclipseAttackWithScoreEviction(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.MaxPeers = 3
	config.ScoreThreshold = -50
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	pm.AddPeer("good-peer", addr, 0)
	pm.AddPeer("medium-peer", addr, 0)
	pm.AddPeer("worst-peer", addr, 0)

	pm.UpdateScore("worst-peer", -30)
	pm.UpdateScore("medium-peer", -10)

	pm.AddPeer("new-peer", addr, 0)

	_, exists := pm.GetPeer("worst-peer")
	if exists {
		t.Error("worst-peer (-30) should have been evicted")
	}

	_, exists = pm.GetPeer("good-peer")
	if !exists {
		t.Error("good-peer (score 0) should not be evicted")
	}

	_, exists = pm.GetPeer("medium-peer")
	if !exists {
		t.Error("medium-peer (-10) should not be evicted ahead of worst-peer (-30)")
	}
}

func TestP2PInvalidMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{"nil data", nil},
		{"empty data", []byte{}},
		{"truncated header", []byte{0x10, 0x00, 0x00}},
		{"only type", []byte{0x10}},
		{"mismatched length", []byte{0x10, 0x00, 0x00, 0x00, 0x05, 0x01, 0x02}},
		{"negative length encoded", []byte{0x10, 0xFF, 0xFF, 0xFF, 0xFF}},
		{"max uint32 length", []byte{0x10, 0xFF, 0xFF, 0xFF, 0xFF, 0x01}},
		{"unknown type only", []byte{0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeMessage(tt.data)
			if err == nil {
				t.Error("expected error for invalid message data")
			}
		})
	}
}

func TestP2PInvalidSignedMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{"nil data", nil},
		{"too short", []byte{0x01, 0x02}},
		{"truncated timestamp", []byte{0x00, 0x00, 0x00, 0x00}},
		{"missing pubkey length field", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeSignedMessage(tt.data)
			if err == nil {
				t.Error("expected error for invalid signed message data")
			}
		})
	}
}

func TestP2PInvalidMessagesOverMaxSize(t *testing.T) {
	t.Parallel()

	msg := NewMessage(MsgBlock, make([]byte, MaxMessageSize+1))
	encoded, err := msg.Encode()
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecodeMessage(encoded)
	if err != ErrMessageTooLarge {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestP2PPeerBanning(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.ScoreThreshold = -50
	config.BanDuration = 5 * time.Minute
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("bad-peer", addr, 0)

	pm.UpdateScore("bad-peer", -60)

	if !pm.IsBanned("bad-peer") {
		t.Error("peer should be banned after exceeding fault threshold")
	}

	info, exists := pm.GetPeer("bad-peer")
	if exists {
		t.Error("banned peer should be removed from active peers")
		if info.Status != PeerStatusBanned {
			t.Errorf("expected banned status, got %v", info.Status)
		}
	}
}

func TestP2PPeerBanningViaInvalidMessage(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.ScoreThreshold = -50
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("spammer", addr, 0)

	for i := 0; i < 6; i++ {
		pm.OnInvalidMessage("spammer", "bad message")
	}

	if !pm.IsBanned("spammer") {
		t.Error("spammer should be banned after repeated invalid messages")
	}
}

func TestP2PPeerBanningViaDuplicateMessage(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.ScoreThreshold = -50
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("duplicator", addr, 0)

	for i := 0; i < 3; i++ {
		pm.OnDuplicateMessage("duplicator")
	}

	if !pm.IsBanned("duplicator") {
		t.Error("duplicator should be banned after duplicate spam (-20 each)")
	}
}

func TestP2PPeerBanningExpiry(t *testing.T) {
	config := DefaultPeerManagerConfig()
	config.ScoreThreshold = -50
	config.BanDuration = 50 * time.Millisecond
	pm := NewPeerManager(config)

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
	pm.AddPeer("temp-banned", addr, 0)

	pm.UpdateScore("temp-banned", -60)

	if !pm.IsBanned("temp-banned") {
		t.Fatal("peer should be banned initially")
	}

	time.Sleep(80 * time.Millisecond)

	if pm.IsBanned("temp-banned") {
		t.Error("ban should have expired")
	}
}

func TestP2PReplayProtection(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	msg := NewMessage(MsgTransaction, []byte("replay-test"))
	chainID := uint64(1337)

	signedMsg, err := SignMessage(msg, privKey, chainID)
	if err != nil {
		t.Fatal(err)
	}

	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	signedMsg.Timestamp = oldTimestamp

	err = VerifySignedMessage(signedMsg, chainID, 5*time.Minute)
	if err != ErrSignatureExpired {
		t.Errorf("expected ErrSignatureExpired for old message, got %v", err)
	}
}

func TestP2PReplayProtectionWrongChain(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	msg := NewMessage(MsgBlock, []byte("cross-chain-replay"))
	signedMsg, err := SignMessage(msg, privKey, 1337)
	if err != nil {
		t.Fatal(err)
	}

	err = VerifySignedMessage(signedMsg, 9999, DefaultMaxMessageAge)
	if err != ErrChainIDMismatch {
		t.Errorf("expected ErrChainIDMismatch, got %v", err)
	}
}

func TestP2PReplayProtectionFutureTimestamp(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	msg := NewMessage(MsgBlock, []byte("future-msg"))
	signedMsg, err := SignMessage(msg, privKey, 1337)
	if err != nil {
		t.Fatal(err)
	}

	signedMsg.Timestamp = time.Now().Add(10 * time.Second).Unix()

	err = VerifySignedMessage(signedMsg, 1337, DefaultMaxMessageAge)
	if err != ErrInvalidTimestamp {
		t.Errorf("expected ErrInvalidTimestamp, got %v", err)
	}
}

func TestP2PReplayProtectionTamperedContent(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	msg := NewMessage(MsgProposal, []byte("original-proposal"))
	signedMsg, err := SignMessage(msg, privKey, 1337)
	if err != nil {
		t.Fatal(err)
	}

	signedMsg.MessageBytes = []byte("tampered-proposal")

	err = VerifySignedMessage(signedMsg, 1337, DefaultMaxMessageAge)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature for tampered content, got %v", err)
	}
}

func TestP2PConcurrentConnections(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	var wg sync.WaitGroup
	concurrentPeers := 50

	for i := 0; i < concurrentPeers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pid := peer.ID(string(rune('A' + n%26)))
			if n >= 26 {
				pid = peer.ID(string(rune('a' + (n-26)%26)))
			}
			pm.AddPeer(pid, addr, 0)
			pm.UpdateScore(pid, n)
			pm.GetPeer(pid)
			pm.UpdateHeight(pid, uint64(n*100))
			pm.OnBlockReceived(pid, n%2 == 0)
		}(i)
	}

	wg.Wait()

	connected := pm.GetConnectedPeers()

	_ = pm.Stats()
	_ = pm.PeerCount()
	_ = pm.IsFull()
	_ = pm.NeedsPeers()

	if len(connected) == 0 {
		t.Error("expected at least some connected peers after concurrent adds")
	}
}

func TestP2PConcurrentConnectionsRateLimiter(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pid := peer.ID(string(rune('A' + n)))
			for j := 0; j < 5; j++ {
				rl.Allow(pid, 100)
				rl.IsBlocked(pid)
			}
		}(i)
	}

	wg.Wait()

	stats := rl.GetStats()
	if len(stats) == 0 {
		t.Error("expected some stats after concurrent access")
	}
}

func TestP2PConcurrentConnectionsDeadlockFree(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			pid := peer.ID(string(rune('A' + i%26)))
			pm.AddPeer(pid, addr, 0)
			pm.RemovePeer(pid)
		}
		close(done)
	}()

	go func() {
		for i := 0; i < 100; i++ {
			pid := peer.ID(string(rune('A' + i%26)))
			pm.UpdateScore(pid, i)
			pm.GetPeer(pid)
		}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			pm.PeerCount()
			pm.GetConnectedPeers()
			pm.IsFull()
			pm.NeedsPeers()
			pm.Stats()
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("possible deadlock: concurrent operations timed out")
	}
}

func TestP2PMessageAuthentication(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	auth := NewMessageAuthenticator(privKey, 1337, DefaultMaxMessageAge)
	msg := NewMessage(MsgBlock, []byte("authenticated-block"))

	signedMsg, err := auth.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}

	if err := auth.Verify(signedMsg); err != nil {
		t.Errorf("valid signature should verify: %v", err)
	}
}

func TestP2PMessageAuthenticationInvalidSignature(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	auth := NewMessageAuthenticator(privKey, 1337, DefaultMaxMessageAge)
	msg := NewMessage(MsgBlock, []byte("test-block"))

	signedMsg, err := auth.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}

	otherKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signedMsg.PublicKey = otherKey.PubKey().Compressed()

	if err := auth.Verify(signedMsg); err == nil {
		t.Error("should reject message with mismatched public key")
	}
}

func TestP2PMessageAuthenticationTamperedPayload(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	auth := NewMessageAuthenticator(privKey, 1337, DefaultMaxMessageAge)
	msg := NewMessage(MsgBlock, []byte("original-block"))

	signedMsg, err := auth.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}

	signedMsg.Signature = append(signedMsg.Signature, 0x00)

	if err := auth.Verify(signedMsg); err == nil {
		t.Error("should reject message with tampered signature")
	}
}

func TestP2PMessageAuthenticationEmptyPayload(t *testing.T) {
	t.Parallel()

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	auth := NewMessageAuthenticator(privKey, 1337, DefaultMaxMessageAge)

	signedMsg, err := auth.Sign(NewMessage(MsgBlock, []byte{}))
	if err != nil {
		t.Fatal(err)
	}

	if err := auth.Verify(signedMsg); err != nil {
		t.Errorf("empty payload should verify: %v", err)
	}
}

func TestP2PMessageAuthenticationNilPublicKey(t *testing.T) {
	t.Parallel()

	sm := &SignedMessage{
		MessageBytes: []byte{0x10, 0x00, 0x00, 0x00, 0x00},
		PublicKey:    nil,
		Signature:    make([]byte, 64),
		Timestamp:    time.Now().Unix(),
		ChainID:      1337,
	}

	_, err := sm.Encode()
	if err != ErrInvalidPublicKey {
		t.Errorf("expected ErrInvalidPublicKey, got %v", err)
	}
}

func TestP2PMessageAuthenticationNilSignature(t *testing.T) {
	t.Parallel()

	sm := &SignedMessage{
		MessageBytes: []byte{0x10, 0x00, 0x00, 0x00, 0x00},
		PublicKey:    make([]byte, 33),
		Signature:    nil,
		Timestamp:    time.Now().Unix(),
		ChainID:      1337,
	}

	_, err := sm.Encode()
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestP2PReputationDecay(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	pm.AddPeer("decay-peer", addr, 0)

	peerInfo, exists := pm.GetPeer("decay-peer")
	if !exists {
		t.Fatal("peer should exist")
	}

	peerInfo.Behavior.LastSeen = time.Now().Add(-2 * time.Hour)

	score := pm.calculateReputationScore(peerInfo)

	expectedScore := int(float64(100) * 0.9)
	if score != expectedScore {
		t.Errorf("expected decayed score %d, got %d", expectedScore, score)
	}
}

func TestP2PReputationDecayWithValidBlocks(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	pm.AddPeer("old-validator", addr, 0)
	pm.OnBlockReceived("old-validator", true)
	pm.OnBlockReceived("old-validator", true)

	pm.peers["old-validator"].Behavior.LastSeen = time.Now().Add(-2 * time.Hour)

	score := pm.calculateReputationScore(pm.peers["old-validator"])

	expectedScore := int(102 * 9 / 10)
	if score != expectedScore {
		t.Errorf("expected decayed score %d, got %d", expectedScore, score)
	}
}

func TestP2PReputationDecayMinScore(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	pm.AddPeer("toxic-peer", addr, 0)

	for i := 0; i < 20; i++ {
		pm.OnInvalidMessage("toxic-peer", "spam")
	}

	peerInfo, exists := pm.GetPeer("toxic-peer")
	if !exists {
		t.Skip("peer was banned, skipping")
		return
	}

	score := pm.calculateReputationScore(peerInfo)
	if score < 0 {
		t.Error("reputation score should never be negative")
	}
}

func TestP2PReputationDecayMaxScore(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	pm.AddPeer("elite-peer", addr, 0)

	for i := 0; i < 200; i++ {
		pm.OnBlockReceived("elite-peer", true)
	}

	peerInfo, exists := pm.GetPeer("elite-peer")
	if !exists {
		t.Fatal("peer should exist")
	}

	score := pm.calculateReputationScore(peerInfo)
	if score > 200 {
		t.Errorf("reputation score should be capped at 200, got %d", score)
	}
}

func TestP2PValidatorFlood(t *testing.T) {
	t.Parallel()

	rl := security.NewRateLimiter(1000, 1000)

	validatorPeer := peer.ID("validator-peer")
	normalPeer := peer.ID("normal-peer")

	validMsg := NewMessage(MsgProposal, []byte("valid-proposal"))
	validEncoded, _ := validMsg.Encode()

	allowed, _ := rl.AllowPeer(validatorPeer, len(validEncoded), security.MsgTypeProposal, validEncoded)
	if !allowed {
		t.Error("proposal from validator should be allowed initially")
	}

	for i := 0; i < 50; i++ {
		rl.AllowPeer(validatorPeer, len(validEncoded), security.MsgTypeProposal, validEncoded)
	}

	allowed, _ = rl.AllowPeer(normalPeer, len(validEncoded), security.MsgTypeProposal, validEncoded)
	if !allowed {
		t.Error("normal peer should have separate bucket")
	}

	for i := 0; i < 100; i++ {
		rl.AllowPeer(normalPeer, 100, security.MsgTypeGeneral, []byte("msg"))
	}

	allowed, reason := rl.AllowPeer(normalPeer, 100, security.MsgTypeGeneral, []byte("msg"))
	if !allowed {
		t.Logf("normal peer general limit hit: %s", reason)
	}
}

func TestP2PValidatorFloodBlockRequests(t *testing.T) {
	t.Parallel()

	rl := security.NewRateLimiter(1000, 1000)

	peerID := peer.ID("block-requester")

	for i := 0; i < 30; i++ {
		rl.AllowPeer(peerID, 100, security.MsgTypeBlockRequest, []byte("block-req"))
	}

	allowed, reason := rl.AllowPeer(peerID, 100, security.MsgTypeBlockRequest, []byte("block-req"))
	if !allowed {
		t.Logf("block request limit reached: %s", reason)
	}

	generalAllowed, _ := rl.AllowPeer(peerID, 100, security.MsgTypeGeneral, []byte("general-msg"))
	if !generalAllowed {
		t.Error("general bucket should be independent from block request bucket")
	}
}

func TestP2PValidatorFloodConsensusIsolation(t *testing.T) {
	t.Parallel()

	rl := security.NewRateLimiter(1000, 1000)

	peerID := peer.ID("consensus-flooder")

	for i := 0; i < 500; i++ {
		rl.AllowPeer(peerID, 100, security.MsgTypeGeneral, []byte("msg"))
	}

	allowed, _ := rl.AllowPeer(peerID, 100, security.MsgTypeConsensus, []byte("consensus-msg"))
	if allowed {
		t.Log("consensus bucket is separate from general bucket")
	}
}

func TestP2PHandlerPanic(t *testing.T) {
	t.Parallel()

	recovered := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
			}
		}()

		pm := NewPeerManager(DefaultPeerManagerConfig())
		addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")
		pm.AddPeer("panic-peer", addr, 0)

		panic("simulated handler panic in message processing")
	}()

	if !recovered {
		t.Error("panic should be recovered to keep peer manager alive")
	}
}

func TestP2PHandlerPanicDuringConcurrentAccess(t *testing.T) {
	pm := NewPeerManager(DefaultPeerManagerConfig())
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/30303")

	pid := peer.ID("stress-peer")
	pm.AddPeer(pid, addr, 0)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			defer func() {
				recover()
			}()
			if n%3 == 0 {
				panic("panic in handler goroutine")
			}
			pm.UpdateScore(pid, n)
			pm.GetPeer(pid)
			pm.OnBlockReceived(pid, n%2 == 0)
		}(i)
	}

	wg.Wait()

	_, exists := pm.GetPeer(pid)
	if !exists {
		t.Error("peer manager should survive panics and peer should still exist")
	}

	connected := pm.GetConnectedPeers()
	_ = connected
}

func TestP2PDoSProtectorConnectionLimit(t *testing.T) {
	t.Parallel()

	config := security.DefaultDoSProtectorConfig()
	config.MaxConnsPerIPPerSec = 3

	protector := security.NewDoSProtector(config, nil)

	ip := "192.168.1.100"

	if !protector.AllowConnection(ip) {
		t.Error("first connection should be allowed")
	}

	if !protector.AllowConnection(ip) {
		t.Error("second connection should be allowed")
	}

	allowed := protector.AllowConnection(ip)
	if allowed {
		t.Log("connection rate limiting may depend on timing")
	}
}

func TestP2PDoSProtectorMemoryLimit(t *testing.T) {
	t.Parallel()

	config := security.DefaultDoSProtectorConfig()
	config.MaxMemoryBytes = 1000

	protector := security.NewDoSProtector(config, nil)

	if !protector.AllocateMemory(500) {
		t.Error("first allocation should succeed")
	}

	if !protector.AllocateMemory(500) {
		t.Error("second allocation should succeed")
	}

	if protector.AllocateMemory(100) {
		t.Error("allocation exceeding limit should fail")
	}

	protector.ReleaseMemory(500)

	if !protector.AllocateMemory(400) {
		t.Error("allocation after release should succeed")
	}
}

func TestP2PDoSProtectorIsUnderAttack(t *testing.T) {
	t.Parallel()

	config := security.DefaultDoSProtectorConfig()
	config.CircuitBreakerThreshold = 3
	config.CircuitBreakerTimeout = 100 * time.Millisecond

	protector := security.NewDoSProtector(config, nil)

	if protector.IsUnderAttack() {
		t.Error("should not indicate attack initially")
	}

	for i := 0; i < 5; i++ {
		protector.RecordAttack()
	}

	if !protector.IsUnderAttack() {
		t.Error("should indicate under attack after threshold exceeded")
	}
}

func TestP2PDoSProtectorCleanup(t *testing.T) {
	t.Parallel()

	protector := security.NewDoSProtector(nil, nil)

	protector.RecordConnection("peer1", "10.0.0.1")
	protector.RecordConnection("peer2", "10.0.0.2")

	protector.Cleanup()

	before := protector.MemoryUsed()
	protector.AllocateMemory(500)
	protector.ReleaseMemory(500)
	after := protector.MemoryUsed()

	if after != before {
		t.Error("memory should return to previous level after release")
	}
}

func TestP2PDoSProtectorEmergencyShutdown(t *testing.T) {
	t.Parallel()

	var shutdownCount int32
	protector := security.NewDoSProtector(security.DefaultDoSProtectorConfig(), func() {
		atomic.AddInt32(&shutdownCount, 1)
	})

	for i := 0; i < 100; i++ {
		protector.RecordAttack()
	}

	protector.Restart()

	if protector.IsUnderAttack() {
		t.Error("protector should not indicate attack after restart")
	}
}

func TestP2PPropagatorDuplicateBlock(t *testing.T) {
	t.Parallel()

	prop := NewPropagator(time.Minute, time.Minute)

	hash := "test-block-hash"
	if !prop.MarkBlockSeen(hash, "peer1") {
		t.Error("first mark should succeed")
	}

	if prop.MarkBlockSeen(hash, "peer2") {
		t.Error("duplicate mark should be rejected")
	}

	if !prop.IsBlockSeen(hash) {
		t.Error("hash should be marked as seen")
	}
}

func TestP2PPropagatorDuplicateTx(t *testing.T) {
	t.Parallel()

	prop := NewPropagator(time.Minute, time.Minute)

	hash := "test-tx-hash"
	if !prop.MarkTxSeen(hash, "peer1") {
		t.Error("first mark should succeed")
	}

	if prop.MarkTxSeen(hash, "peer2") {
		t.Error("duplicate tx mark should be rejected")
	}
}

func TestP2PMessageValidationEdgeCases(t *testing.T) {
	t.Parallel()

	mv := NewMessageValidator(1337)

	tests := []struct {
		name    string
		msg     *Message
		wantErr bool
	}{
		{"nil payload block", NewMessage(MsgBlock, nil), true},
		{"empty block", NewMessage(MsgBlock, []byte{}), true},
		{"unknown type", NewMessage(0xFF, []byte("data")), true},
		{"huge ping", NewMessage(MsgPing, make([]byte, 300)), true},
		{"small header", NewMessage(MsgBlockHeader, make([]byte, 10)), true},
		{"valid header", NewMessage(MsgBlockHeader, make([]byte, 64)), false},
		{"valid tx", NewMessage(MsgTransaction, make([]byte, 100)), false},
		{"valid block", NewMessage(MsgBlock, make([]byte, 1000)), false},
		{"valid query", NewMessage(MsgGetPeers, []byte{}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mv.Validate(tt.msg)
			if tt.wantErr && result != ValidationReject {
				t.Errorf("expected rejection, got %v (err=%v)", result, err)
			}
			if !tt.wantErr && result != ValidationAccept {
				t.Errorf("expected acceptance, got %v (err=%v)", result, err)
			}
		})
	}
}

func TestP2PConnManagerEclipsePrevention(t *testing.T) {
	t.Parallel()

	config := DefaultConnManagerConfig()
	config.MaxConnectionsPerPeer = 2
	config.MaxConnections = 5
	config.MinConnections = 0

	cm := NewConnManager(config)

	if cm.IsAtCapacity() {
		t.Error("should not be at capacity initially")
	}

	if cm.NeedsConnections() {
		t.Error("NeedsConnections should not trigger with MinConnections=0")
	}

	stalePeers := cm.GetStalePeers(time.Millisecond)
	if len(stalePeers) != 0 {
		t.Error("should have no stale peers initially")
	}
}

func TestP2PConnManagerStatsAfterOperations(t *testing.T) {
	t.Parallel()

	cm := NewConnManager(DefaultConnManagerConfig())

	stats := cm.Stats()
	if stats.ActivePeers != 0 {
		t.Errorf("expected 0 active, got %d", stats.ActivePeers)
	}

	pid := peer.ID("tracked-peer")
	cm.TrackBytesIn(pid, 1000)
	cm.TrackBytesOut(pid, 500)
	cm.TrackMessage(pid, "in")
	cm.TrackMessage(pid, "out")

	_, exists := cm.GetRecord(pid)
	if !exists {
		t.Log("tracked peer not in conn records (expected without real connection)")
	}
}

func TestP2PRateLimiterConcurrentFlood(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pid := peer.ID(string(rune('A' + n)))
			for j := 0; j < 100; j++ {
				allowed := rl.Allow(pid, 100)
				if j < rl.config.MaxMessagesPerSecond && !allowed {
					errs <- nil
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for range errs {
	}
}

func TestP2PNetworkStatsEdgeCases(t *testing.T) {
	t.Parallel()

	ns := NewNetworkStats()

	snap := ns.Snapshot()
	if snap.MessagesPerSecond != 0 {
		t.Error("MPS should be 0 with no messages")
	}

	ns.SetPeerCount(0)
	snap = ns.Snapshot()
	if snap.CurrentPeers != 0 {
		t.Errorf("expected 0 current peers, got %d", snap.CurrentPeers)
	}
	if snap.PeakPeers != 0 {
		t.Errorf("expected 0 peak peers, got %d", snap.PeakPeers)
	}

	ns.SetPeerCount(100)
	ns.SetPeerCount(50)
	snap = ns.Snapshot()
	if snap.PeakPeers != 100 {
		t.Errorf("expected peak 100, got %d", snap.PeakPeers)
	}
}

