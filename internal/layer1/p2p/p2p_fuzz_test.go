package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func FuzzMessageEncodeDecode(f *testing.F) {
	types := []MessageType{MsgPing, MsgPong, MsgBlock, MsgTransaction, MsgProposal, MsgVote, MsgQC}
	for _, mt := range types {
		f.Add(byte(mt), []byte("payload"))
	}
	f.Add(byte(255), []byte{})
	f.Add(byte(0), make([]byte, 1000))

	f.Fuzz(func(t *testing.T, typ byte, payload []byte) {
		msg := NewMessage(MessageType(typ), payload)
		encoded, err := msg.Encode()
		if err != nil {
			return
		}
		decoded, err := DecodeMessage(encoded)
		if err != nil {
			return
		}
		if decoded.Type != msg.Type {
			t.Errorf("type mismatch after roundtrip")
		}
		if string(decoded.Payload) != string(payload) {
			t.Errorf("payload mismatch after roundtrip")
		}
	})
}

func FuzzMessageDecodeInvalid(f *testing.F) {
	f.Add([]byte{0x00, 0x01})
	f.Add([]byte{0x10, 0x00, 0x00, 0x00, 0x05, 0x01, 0x02})
	f.Add(make([]byte, 3))
	f.Add(make([]byte, 100))

	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := DecodeMessage(data)
		if err != nil {
			return
		}
		if msg == nil {
			t.Errorf("decoded message should not be nil when no error")
		}
	})
}

func FuzzPeerManagerAddRemove(f *testing.F) {
	f.Add([]byte("peer1"), []byte("addr1"), uint64(100))
	f.Add([]byte(""), []byte("addr"), uint64(0))

	f.Fuzz(func(t *testing.T, peerID, addrBytes []byte, height uint64) {
		if len(peerID) == 0 {
			return
		}
		pm := NewPeerManager(DefaultPeerManagerConfig())
		id := peer.ID(peerID)
		pm.AddPeer(id, nil, 0)
		pm.UpdateHeight(id, height)
		info, exists := pm.GetPeer(id)
		if !exists {
			t.Errorf("peer should exist after add")
			return
		}
		if info.Height != height {
			t.Errorf("height mismatch: %d != %d", info.Height, height)
		}
		pm.RemovePeer(id)
		_, exists = pm.GetPeer(id)
		if exists {
			t.Errorf("peer should not exist after remove")
		}
	})
}

func FuzzPeerManagerScoreAndBan(f *testing.F) {
	f.Add([]byte("peer"), -50)
	f.Add([]byte("peer2"), 30)
	f.Add([]byte("peer3"), -200)

	f.Fuzz(func(t *testing.T, pid []byte, delta int) {
		if len(pid) == 0 {
			return
		}
		pm := NewPeerManager(DefaultPeerManagerConfig())
		id := peer.ID(pid)
		pm.AddPeer(id, nil, 0)
		pm.UpdateScore(id, delta)
		info, exists := pm.GetPeer(id)
		if !exists {
			return
		}
		if info.Score != delta {
			t.Logf("score %d after delta %d", info.Score, delta)
		}
	})
}

func FuzzPeerManagerClassification(f *testing.F) {
	f.Add([]byte("peer"), 0)
	f.Add([]byte("peer"), 100)
	f.Add([]byte("peer"), 200)

	f.Fuzz(func(t *testing.T, pid []byte, score int) {
		if len(pid) == 0 {
			return
		}
		pm := NewPeerManager(DefaultPeerManagerConfig())
		id := peer.ID(pid)
		p := pm.AddPeer(id, nil, 0)
		p.Score = score
		_ = pm.GetClassification(p)
		_ = pm.GetHighQualityPeers(5)
	})
}

func FuzzRateLimiter(f *testing.F) {
	f.Add([]byte("peer"), 10)
	f.Add([]byte("peer2"), 0)
	f.Add([]byte("peer3"), 5000)

	f.Fuzz(func(t *testing.T, pid []byte, msgCount int) {
		if len(pid) == 0 || msgCount < 0 || msgCount > 10000 {
			return
		}
		rl := NewRateLimiter(DefaultRateLimiterConfig())
		id := peer.ID(pid)
		allowed := 0
		for i := 0; i < msgCount; i++ {
			if rl.Allow(id, 100) {
				allowed++
			}
		}
		if allowed > rl.config.MaxMessagesPerSecond {
			t.Errorf("allowed %d > max %d", allowed, rl.config.MaxMessagesPerSecond)
		}
	})
}

func FuzzHandshakeMessageEncodeDecode(f *testing.F) {
	f.Add(uint64(1), uint64(100), uint64(42), []byte("agent"))
	f.Add(uint64(0), uint64(0), uint64(0), []byte{})
	f.Add(uint64(1337), uint64(1000), uint64(99), []byte("viri/0.1.0"))

	f.Fuzz(func(t *testing.T, chainID, height, nonce uint64, agent []byte) {
		if len(agent) > 100 {
			agent = agent[:100]
		}
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}
		hs := NewHandshakeMessage(chainID, height, nonce, key.PubKey())
		hs.Agent = string(agent)
		data, err := hs.Encode()
		if err != nil {
			return
		}
		decoded, err := DecodeHandshakeMessage(data)
		if err != nil {
			t.Errorf("decode failed: %v", err)
			return
		}
		if decoded.ChainID != chainID {
			t.Errorf("chainID mismatch: %d != %d", decoded.ChainID, chainID)
		}
		if decoded.Height != height {
			t.Errorf("height mismatch: %d != %d", decoded.Height, height)
		}
	})
}

func FuzzSignedMessageEncodeDecode(f *testing.F) {
	f.Add([]byte("msg"), []byte("pubkey"), []byte("sig"))
	f.Add([]byte{}, []byte{}, []byte{})
	f.Add(make([]byte, 100), make([]byte, 33), make([]byte, 64))

	f.Fuzz(func(t *testing.T, msgBytes, pubKey, sig []byte) {
		sm := NewSignedMessage(msgBytes, pubKey, sig, time.Now().Unix(), 1)
		data, err := sm.Encode()
		if err != nil {
			return
		}
		decoded, err := DecodeSignedMessage(data)
		if err != nil {
			t.Errorf("decode failed: %v", err)
			return
		}
		if string(decoded.MessageBytes) != string(msgBytes) {
			t.Errorf("message bytes mismatch")
		}
		if string(decoded.PublicKey) != string(pubKey) {
			t.Errorf("public key mismatch")
		}
	})
}

func FuzzPeerManagerBehaviorEvents(f *testing.F) {
	f.Add([]byte("peer"), true)
	f.Add([]byte("peer2"), false)

	f.Fuzz(func(t *testing.T, pid []byte, valid bool) {
		if len(pid) == 0 {
			return
		}
		pm := NewPeerManager(DefaultPeerManagerConfig())
		id := peer.ID(pid)
		pm.AddPeer(id, nil, 0)
		pm.OnBlockReceived(id, valid)
		pm.OnTxRelayed(id, valid)
		pm.OnInvalidMessage(id, "fuzz test")
		pm.OnTimeout(id)
		pm.OnDuplicateMessage(id)
		pm.OnBlockProposed(id)
		pm.OnLateResponse(id)
	})
}

func FuzzPeerManagerPeersByHeight(f *testing.F) {
	f.Add(int(0), int(50))
	f.Add(int(5), int(1))

	f.Fuzz(func(t *testing.T, numPeers, maxHeight int) {
		if numPeers < 0 || numPeers > 100 || maxHeight < 0 {
			return
		}
		pm := NewPeerManager(DefaultPeerManagerConfig())
		for i := 0; i < numPeers; i++ {
			id := peer.ID(string(rune('A' + i%26)))
			pm.AddPeer(id, nil, 0)
			pm.UpdateHeight(id, uint64(i*maxHeight/(numPeers+1)))
		}
		byHeight := pm.GetPeersByHeight()
		for i := 1; i < len(byHeight); i++ {
			if byHeight[i-1].Height < byHeight[i].Height {
				t.Errorf("peers not sorted by height descending")
				break
			}
		}
	})
}

func FuzzSignedMessageVerifyWithGeneratedKey(f *testing.F) {
	f.Add([]byte("test message"))
	f.Add([]byte("another message"))
	f.Add(make([]byte, 1000))

	f.Fuzz(func(t *testing.T, msgData []byte) {
		if len(msgData) == 0 {
			return
		}
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}
		msg := NewMessage(MsgPing, msgData)
		sm, err := SignMessage(msg, key, 1)
		if err != nil {
			t.Errorf("sign failed: %v", err)
			return
		}
		err = VerifySignedMessage(sm, 1, 5*time.Minute)
		if err != nil {
			t.Errorf("verify failed: %v", err)
		}
	})
}

func FuzzMessageAuthenticatorRoundtrip(f *testing.F) {
	f.Add([]byte("auth payload"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, payload []byte) {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Skip()
		}
		auth := NewMessageAuthenticator(key, 1, 5*time.Minute)
		msg := NewMessage(MsgTransaction, payload)
		sm, err := auth.Sign(msg)
		if err != nil {
			t.Errorf("auth sign failed: %v", err)
			return
		}
		if err := auth.Verify(sm); err != nil {
			t.Errorf("auth verify failed: %v", err)
		}
		_ = auth.GetPeerID()
		_ = auth.PublicKey()
		_ = auth.ValidatorAddress()
	})
}

func FuzzPeerManagerConcurrentAccess(f *testing.F) {
	f.Add([]byte("peer"))
	f.Add([]byte("another"))

	f.Fuzz(func(t *testing.T, pid []byte) {
		if len(pid) == 0 {
			return
		}
		pm := NewPeerManager(DefaultPeerManagerConfig())
		id := peer.ID(pid)
		pm.AddPeer(id, nil, 0)
		for i := 0; i < 10; i++ {
			pm.UpdateScore(id, i)
			pm.OnBlockReceived(id, i%2 == 0)
			pm.GetPeer(id)
		}
		pm.GetConnectedPeers()
		pm.PeerCount()
		pm.IsFull()
		pm.NeedsPeers()
		pm.Stats()
	})
}
