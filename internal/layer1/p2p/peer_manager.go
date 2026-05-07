package p2p

import (
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type PeerStatus uint8

const (
	PeerStatusConnected PeerStatus = iota
	PeerStatusDisconnected
	PeerStatusBanned
	PeerStatusPenalized
)

type PeerClassification uint8

const (
	PeerTrusted PeerClassification = iota
	PeerHealthy
	PeerSuspicious
	PeerToxic
	PeerBannedClassification
)

type PeerBehavior struct {
	MessagesSent        int
	MessagesReceived    int
	BlocksProposed      int
	BlocksValidated     int
	TransactionsRelayed int
	InvalidMessages     int
	LateResponses       int
	Timeouts            int
	DuplicateMessages   int
	LastSeen            time.Time
	ConnectedSince      time.Time
	Uptime              time.Duration
}

type PeerInfo struct {
	ID          peer.ID
	Address     multiaddr.Multiaddr
	Status      PeerStatus
	Score       int
	ConnectedAt time.Time
	LastSeen    time.Time
	Direction   network.Direction
	Version     string
	Height      uint64
	Agent       string
	PubKey      []byte
	Behavior    PeerBehavior
}

type PeerManagerConfig struct {
	MaxPeers       int
	MinPeers       int
	MaxConnections int
	BanDuration    time.Duration
	ScoreThreshold int
}

func DefaultPeerManagerConfig() *PeerManagerConfig {
	return &PeerManagerConfig{
		MaxPeers:       50,
		MinPeers:       5,
		MaxConnections: 100,
		BanDuration:    24 * time.Hour,
		ScoreThreshold: -100,
	}
}

type PeerManager struct {
	mu       sync.RWMutex
	peers    map[peer.ID]*PeerInfo
	banned   map[peer.ID]time.Time
	config   *PeerManagerConfig
	handlers []PeerEventHandler
}

type PeerEventHandler func(event PeerEvent)

type PeerEventType string

const (
	PeerConnectedEvent    PeerEventType = "connected"
	PeerDisconnectedEvent PeerEventType = "disconnected"
	PeerBannedEvent       PeerEventType = "banned"
	PeerPenalizedEvent    PeerEventType = "penalized"
	PeerScoredEvent       PeerEventType = "scored"
)

type PeerEvent struct {
	Type    PeerEventType
	Peer    *PeerInfo
	Reason  string
	Score   int
	Time    time.Time
}

func NewPeerManager(config *PeerManagerConfig) *PeerManager {
	if config == nil {
		config = DefaultPeerManagerConfig()
	}

	return &PeerManager{
		peers:  make(map[peer.ID]*PeerInfo),
		banned: make(map[peer.ID]time.Time),
		config: config,
	}
}

func (pm *PeerManager) AddPeer(id peer.ID, addr multiaddr.Multiaddr, direction network.Direction) *PeerInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.banned[id]; exists {
		return nil
	}

	if len(pm.peers) >= pm.config.MaxPeers {
		pm.evictLowestScorePeer()
	}

	now := time.Now()
	peerInfo := &PeerInfo{
		ID:          id,
		Address:     addr,
		Status:      PeerStatusConnected,
		Score:       0,
		ConnectedAt: now,
		LastSeen:    now,
		Direction:   direction,
		Behavior: PeerBehavior{
			ConnectedSince: now,
			LastSeen:       now,
		},
	}

	pm.peers[id] = peerInfo
	pm.notifyHandlers(PeerEvent{
		Type: PeerConnectedEvent,
		Peer: peerInfo,
		Time: now,
	})

	return peerInfo
}

func (pm *PeerManager) AddPeerWithPubKey(id peer.ID, addr multiaddr.Multiaddr, direction network.Direction, pubKey []byte) *PeerInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.banned[id]; exists {
		return nil
	}

	if len(pm.peers) >= pm.config.MaxPeers {
		pm.evictLowestScorePeer()
	}

	now := time.Now()
	peerInfo := &PeerInfo{
		ID:          id,
		Address:     addr,
		Status:      PeerStatusConnected,
		Score:       0,
		ConnectedAt: now,
		LastSeen:    now,
		Direction:   direction,
		PubKey:      pubKey,
		Behavior: PeerBehavior{
			ConnectedSince: now,
			LastSeen:       now,
		},
	}

	pm.peers[id] = peerInfo
	pm.notifyHandlers(PeerEvent{
		Type: PeerConnectedEvent,
		Peer: peerInfo,
		Time: now,
	})

	return peerInfo
}

func (pm *PeerManager) RemovePeer(id peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	peerInfo, exists := pm.peers[id]
	if !exists {
		return
	}

	peerInfo.Status = PeerStatusDisconnected
	delete(pm.peers, id)

	pm.notifyHandlers(PeerEvent{
		Type: PeerDisconnectedEvent,
		Peer: peerInfo,
		Time: time.Now(),
	})
}

func (pm *PeerManager) BanPeer(id peer.ID, reason string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.banned[id] = time.Now().Add(pm.config.BanDuration)

	peerInfo, exists := pm.peers[id]
	if exists {
		peerInfo.Status = PeerStatusBanned
		delete(pm.peers, id)
	}

	pm.notifyHandlers(PeerEvent{
		Type:   PeerBannedEvent,
		Peer:   peerInfo,
		Reason: reason,
		Time:   time.Now(),
	})
}

func (pm *PeerManager) IsBanned(id peer.ID) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	banExpiry, exists := pm.banned[id]
	if !exists {
		return false
	}

	if time.Now().After(banExpiry) {
		go pm.unbanPeer(id)
		return false
	}

	return true
}

func (pm *PeerManager) UpdateScore(id peer.ID, delta int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	peerInfo, exists := pm.peers[id]
	if !exists {
		return
	}

	peerInfo.Score += delta
	peerInfo.LastSeen = time.Now()

	if peerInfo.Score <= pm.config.ScoreThreshold {
		pm.banPeerLocked(id, "score below threshold")
	}

	pm.notifyHandlers(PeerEvent{
		Type:  PeerScoredEvent,
		Peer:  peerInfo,
		Score: peerInfo.Score,
		Time:  time.Now(),
	})
}

func (pm *PeerManager) PenalizePeer(id peer.ID, reason string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	peerInfo, exists := pm.peers[id]
	if !exists {
		return
	}

	peerInfo.Status = PeerStatusPenalized
	peerInfo.LastSeen = time.Now()

	pm.notifyHandlers(PeerEvent{
		Type:   PeerPenalizedEvent,
		Peer:   peerInfo,
		Reason: reason,
		Time:   time.Now(),
	})
}

func (pm *PeerManager) UpdateHeight(id peer.ID, height uint64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	peerInfo, exists := pm.peers[id]
	if !exists {
		return
	}

	peerInfo.Height = height
	peerInfo.LastSeen = time.Now()
}

func (pm *PeerManager) GetPeer(id peer.ID) (*PeerInfo, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peerInfo, exists := pm.peers[id]
	return peerInfo, exists
}

func (pm *PeerManager) GetConnectedPeers() []*PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var connected []*PeerInfo
	for _, p := range pm.peers {
		if p.Status == PeerStatusConnected {
			connected = append(connected, p)
		}
	}

	return connected
}

func (pm *PeerManager) GetPeersByHeight() []*PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var peers []*PeerInfo
	for _, p := range pm.peers {
		if p.Status == PeerStatusConnected {
			peers = append(peers, p)
		}
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Height > peers[j].Height
	})

	return peers
}

func (pm *PeerManager) PeerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers)
}

func (pm *PeerManager) IsFull() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers) >= pm.config.MaxPeers
}

func (pm *PeerManager) NeedsPeers() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers) < pm.config.MinPeers
}

func (pm *PeerManager) Subscribe(handler PeerEventHandler) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.handlers = append(pm.handlers, handler)
}

func (pm *PeerManager) Stats() PeerManagerStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var connected, disconnected, banned, penalized int
	for _, p := range pm.peers {
		switch p.Status {
		case PeerStatusConnected:
			connected++
		case PeerStatusDisconnected:
			disconnected++
		case PeerStatusBanned:
			banned++
		case PeerStatusPenalized:
			penalized++
		}
	}

	return PeerManagerStats{
		TotalPeers:    len(pm.peers),
		Connected:     connected,
		Disconnected:  disconnected,
		Banned:        banned + len(pm.banned),
		Penalized:     penalized,
		MaxPeers:      pm.config.MaxPeers,
		MinPeers:      pm.config.MinPeers,
	}
}

func (pm *PeerManager) calculateReputationScore(p *PeerInfo) int {
	score := 100

	score += p.Behavior.BlocksValidated
	score += int(float64(p.Behavior.TransactionsRelayed) * 0.5)

	score -= p.Behavior.InvalidMessages * 10
	score -= p.Behavior.Timeouts * 5
	score -= p.Behavior.LateResponses * 2
	score -= p.Behavior.DuplicateMessages * 20

	if time.Since(p.Behavior.LastSeen) > time.Hour {
		score = int(float64(score) * 0.9)
	}

	if score < 0 {
		score = 0
	}
	if score > 200 {
		score = 200
	}

	return score
}

func (pm *PeerManager) GetClassification(p *PeerInfo) PeerClassification {
	if p.Status == PeerStatusBanned {
		return PeerBannedClassification
	}

	score := pm.calculateReputationScore(p)
	uptime := time.Since(p.Behavior.ConnectedSince)

	switch {
	case score > 150 && uptime > time.Hour:
		return PeerTrusted
	case score >= 80 && score <= 150:
		return PeerHealthy
	case score >= 50 && score < 80:
		return PeerSuspicious
	default:
		return PeerToxic
	}
}

func (pm *PeerManager) GetHighQualityPeers(n int) []*PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(pm.peers))
	for _, p := range pm.peers {
		if p.Status == PeerStatusConnected {
			peers = append(peers, p)
		}
	}

	sort.Slice(peers, func(i, j int) bool {
		scoreI := pm.calculateReputationScore(peers[i])
		scoreJ := pm.calculateReputationScore(peers[j])
		return scoreI > scoreJ
	})

	if n > len(peers) {
		n = len(peers)
	}

	return peers[:n]
}

func (pm *PeerManager) GetPeersForSync() []*PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(pm.peers))
	for _, p := range pm.peers {
		if p.Status == PeerStatusConnected {
			peers = append(peers, p)
		}
	}

	sort.Slice(peers, func(i, j int) bool {
		scoreI := pm.calculateReputationScore(peers[i])
		scoreJ := pm.calculateReputationScore(peers[j])
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return peers[i].Height > peers[j].Height
	})

	return peers
}

func (pm *PeerManager) ShouldEvict(peerID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	p, exists := pm.peers[peer.ID(peerID)]
	if !exists {
		return false
	}

	score := pm.calculateReputationScore(p)
	if score < 30 {
		return true
	}

	classification := pm.GetClassification(p)
	return classification == PeerToxic
}

func (pm *PeerManager) OnBlockReceived(peerID peer.ID, valid bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, exists := pm.peers[peerID]
	if !exists {
		return
	}

	if valid {
		p.Behavior.BlocksValidated++
		p.Score += 10
	} else {
		p.Behavior.InvalidMessages++
		p.Score -= 10
	}

	p.Behavior.LastSeen = time.Now()

	if p.Score <= pm.config.ScoreThreshold {
		pm.banPeerLocked(peerID, "score below threshold")
	}
}

func (pm *PeerManager) OnTxRelayed(peerID peer.ID, valid bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, exists := pm.peers[peerID]
	if !exists {
		return
	}

	if valid {
		p.Behavior.TransactionsRelayed++
		p.Score += 5
	} else {
		p.Behavior.InvalidMessages++
		p.Score -= 5
	}

	p.Behavior.LastSeen = time.Now()
}

func (pm *PeerManager) OnInvalidMessage(peerID peer.ID, reason string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, exists := pm.peers[peerID]
	if !exists {
		return
	}

	p.Behavior.InvalidMessages++
	p.Score -= 10
	p.Behavior.LastSeen = time.Now()

	if p.Score <= pm.config.ScoreThreshold {
		pm.banPeerLocked(peerID, reason)
	}
}

func (pm *PeerManager) OnTimeout(peerID peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, exists := pm.peers[peerID]
	if !exists {
		return
	}

	p.Behavior.Timeouts++
	p.Score -= 5
	p.Behavior.LastSeen = time.Now()
}

func (pm *PeerManager) OnDuplicateMessage(peerID peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, exists := pm.peers[peerID]
	if !exists {
		return
	}

	p.Behavior.DuplicateMessages++
	p.Score -= 20
	p.Behavior.LastSeen = time.Now()

	if p.Score <= pm.config.ScoreThreshold {
		pm.banPeerLocked(peerID, "spam detected")
	}
}

func (pm *PeerManager) OnBlockProposed(peerID peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, exists := pm.peers[peerID]
	if !exists {
		return
	}

	p.Behavior.BlocksProposed++
	p.Score += 1
	p.Behavior.LastSeen = time.Now()
}

func (pm *PeerManager) OnLateResponse(peerID peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, exists := pm.peers[peerID]
	if !exists {
		return
	}

	p.Behavior.LateResponses++
	p.Score -= 2
	p.Behavior.LastSeen = time.Now()
}

type PeerReputationEntry struct {
	PeerID         string
	Score          int
	Classification PeerClassification
	Uptime         time.Duration
	MessagesSent   int
	InvalidMessages int
	LastSeen       time.Time
}

func (pm *PeerManager) GetReputationReport() []PeerReputationEntry {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	report := make([]PeerReputationEntry, 0, len(pm.peers))
	for _, p := range pm.peers {
		score := pm.calculateReputationScore(p)
		classification := pm.GetClassification(p)
		uptime := time.Since(p.Behavior.ConnectedSince)

		report = append(report, PeerReputationEntry{
			PeerID:         p.ID.String(),
			Score:          score,
			Classification: classification,
			Uptime:         uptime,
			MessagesSent:   p.Behavior.MessagesSent,
			InvalidMessages: p.Behavior.InvalidMessages,
			LastSeen:       p.Behavior.LastSeen,
		})
	}

	return report
}

func (pm *PeerManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, p := range pm.peers {
		p.Status = PeerStatusDisconnected
	}

	pm.peers = make(map[peer.ID]*PeerInfo)
	pm.banned = make(map[peer.ID]time.Time)
}

func (pm *PeerManager) evictLowestScorePeer() {
	var lowestID peer.ID
	lowestScore := int(^uint(0) >> 1)

	for id, p := range pm.peers {
		if p.Score < lowestScore {
			lowestScore = p.Score
			lowestID = id
		}
	}

	if lowestID != "" {
		peerInfo := pm.peers[lowestID]
		peerInfo.Status = PeerStatusDisconnected
		delete(pm.peers, lowestID)
	}
}

func (pm *PeerManager) banPeerLocked(id peer.ID, reason string) {
	pm.banned[id] = time.Now().Add(pm.config.BanDuration)

	peerInfo := pm.peers[id]
	if peerInfo != nil {
		peerInfo.Status = PeerStatusBanned
		delete(pm.peers, id)
	}
}

func (pm *PeerManager) unbanPeer(id peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.banned, id)
}

func (pm *PeerManager) notifyHandlers(event PeerEvent) {
	for _, handler := range pm.handlers {
		go handler(event)
	}
}

type PeerManagerStats struct {
	TotalPeers   int
	Connected    int
	Disconnected int
	Banned       int
	Penalized    int
	MaxPeers     int
	MinPeers     int
}
