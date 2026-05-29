package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type ConnManagerConfig struct {
	MaxConnections    int
	MinConnections    int
	GracePeriod       time.Duration
	SilenceThreshold  time.Duration
	MaxConnectionsPerPeer int
	HealthCheckInterval time.Duration
}

func DefaultConnManagerConfig() *ConnManagerConfig {
	return &ConnManagerConfig{
		MaxConnections:        100,
		MinConnections:        5,
		GracePeriod:           30 * time.Second,
		SilenceThreshold:      5 * time.Minute,
		MaxConnectionsPerPeer: 3,
		HealthCheckInterval:   30 * time.Second,
	}
}

type ConnRecord struct {
	PeerID        peer.ID
	Connections   []network.Conn
	FirstSeen     time.Time
	LastActivity  time.Time
	BytesIn       uint64
	BytesOut      uint64
	MessagesIn    uint64
	MessagesOut   uint64
	IsInbound     bool
}

type ConnManager struct {
	mu               sync.RWMutex
	config           *ConnManagerConfig
	conns            map[peer.ID]*ConnRecord
	notifee          *viriNotifee
	done             chan struct{}
	host             network.Network
	onPeerConnected  func(peer.ID, multiaddr.Multiaddr, network.Direction)
	onPeerDisconnected func(peer.ID)
}

type viriNotifee struct {
	manager *ConnManager
}

func (n *viriNotifee) Connected(net network.Network, conn network.Conn) {
	n.manager.handleConnected(conn)
}

func (n *viriNotifee) Disconnected(net network.Network, conn network.Conn) {
	n.manager.handleDisconnected(conn)
}

func (n *viriNotifee) OpenedStream(net network.Network, stream network.Stream) {
	n.manager.handleStreamOpened(stream)
}

func (n *viriNotifee) ClosedStream(net network.Network, stream network.Stream) {
	n.manager.handleStreamClosed(stream)
}

func (n *viriNotifee) Listen(net network.Network, addr multiaddr.Multiaddr) {}

func (n *viriNotifee) ListenClose(net network.Network, addr multiaddr.Multiaddr) {}

func NewConnManager(config *ConnManagerConfig) *ConnManager {
	if config == nil {
		config = DefaultConnManagerConfig()
	}

	cm := &ConnManager{
		config: config,
		conns:  make(map[peer.ID]*ConnRecord),
		done:   make(chan struct{}),
	}
	cm.notifee = &viriNotifee{manager: cm}

	return cm
}

func (cm *ConnManager) Start() {
	go cm.healthCheckLoop()
}

func (cm *ConnManager) SetHost(host network.Network) {
	cm.host = host
}

func (cm *ConnManager) healthCheckLoop() {
	ticker := time.NewTicker(cm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.healthCheck()
		case <-cm.done:
			return
		}
	}
}

func (cm *ConnManager) healthCheck() {
	cm.mu.RLock()
	stale := cm.GetStalePeersLocked(cm.config.SilenceThreshold)
	activeCount := cm.getActiveCountLocked()
	cm.mu.RUnlock()

	for _, peerID := range stale {
		cm.mu.Lock()
		delete(cm.conns, peerID)
		cm.mu.Unlock()
	}

	if activeCount < cm.config.MinConnections && cm.host != nil {
		for _, p := range cm.host.Peers() {
			if len(cm.host.ConnsToPeer(p)) == 0 {
				cm.mu.Lock()
				delete(cm.conns, p)
				cm.mu.Unlock()
			}
		}
	}
}

func (cm *ConnManager) Stop() {
	close(cm.done)
}

func (cm *ConnManager) getActiveCountLocked() int {
	count := 0
	for _, record := range cm.conns {
		if len(record.Connections) > 0 {
			count++
		}
	}
	return count
}

func (cm *ConnManager) GetStalePeersLocked(threshold time.Duration) []peer.ID {
	now := time.Now()
	var stale []peer.ID

	for peerID, record := range cm.conns {
		if now.Sub(record.LastActivity) > threshold {
			stale = append(stale, peerID)
		}
	}

	return stale
}

func (cm *ConnManager) Notifee() network.Notifiee {
	return cm.notifee
}

func (cm *ConnManager) OnPeerConnected(cb func(peer.ID, multiaddr.Multiaddr, network.Direction)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onPeerConnected = cb
}

func (cm *ConnManager) handleConnected(conn network.Conn) {
	cm.mu.Lock()

	peerID := conn.RemotePeer()
	now := time.Now()

	record, exists := cm.conns[peerID]
	if !exists {
		record = &ConnRecord{
			PeerID:       peerID,
			FirstSeen:    now,
			LastActivity: now,
			IsInbound:    conn.Stat().Direction == network.DirInbound,
		}
		cm.conns[peerID] = record
	}

	record.Connections = append(record.Connections, conn)

	cb := cm.onPeerConnected
	cm.mu.Unlock()

	if cb != nil {
		cb(peerID, conn.RemoteMultiaddr(), conn.Stat().Direction)
	}
}

func (cm *ConnManager) OnPeerDisconnected(cb func(peer.ID)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onPeerDisconnected = cb
}

func (cm *ConnManager) handleDisconnected(conn network.Conn) {
	cm.mu.Lock()

	peerID := conn.RemotePeer()
	record, exists := cm.conns[peerID]
	if !exists {
		cm.mu.Unlock()
		return
	}

	var remaining []network.Conn
	for _, c := range record.Connections {
		if c.ID() != conn.ID() {
			remaining = append(remaining, c)
		}
	}
	record.Connections = remaining

	noConnsLeft := len(remaining) == 0
	if noConnsLeft {
		delete(cm.conns, peerID)
	}

	cb := cm.onPeerDisconnected
	cm.mu.Unlock()

	if cb != nil && noConnsLeft {
		cb(peerID)
	}
}

func (cm *ConnManager) handleStreamOpened(stream network.Stream) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	peerID := stream.Conn().RemotePeer()
	if record, exists := cm.conns[peerID]; exists {
		record.LastActivity = time.Now()
	}
}

func (cm *ConnManager) handleStreamClosed(stream network.Stream) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	peerID := stream.Conn().RemotePeer()
	if record, exists := cm.conns[peerID]; exists {
		record.LastActivity = time.Now()
	}
}

func (cm *ConnManager) TrackBytesIn(peerID peer.ID, bytes uint64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if record, exists := cm.conns[peerID]; exists {
		record.BytesIn += bytes
		record.LastActivity = time.Now()
	}
}

func (cm *ConnManager) TrackBytesOut(peerID peer.ID, bytes uint64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if record, exists := cm.conns[peerID]; exists {
		record.BytesOut += bytes
		record.LastActivity = time.Now()
	}
}

func (cm *ConnManager) TrackMessage(peerID peer.ID, direction string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if record, exists := cm.conns[peerID]; exists {
		if direction == "in" {
			record.MessagesIn++
		} else {
			record.MessagesOut++
		}
		record.LastActivity = time.Now()
	}
}

func (cm *ConnManager) GetRecord(peerID peer.ID) (*ConnRecord, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	record, exists := cm.conns[peerID]
	return record, exists
}

func (cm *ConnManager) GetActivePeers() []peer.ID {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var peers []peer.ID
	for peerID, record := range cm.conns {
		if len(record.Connections) > 0 {
			peers = append(peers, peerID)
		}
	}

	return peers
}

func (cm *ConnManager) GetStalePeers(threshold time.Duration) []peer.ID {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	now := time.Now()
	var stale []peer.ID

	for peerID, record := range cm.conns {
		if now.Sub(record.LastActivity) > threshold {
			stale = append(stale, peerID)
		}
	}

	return stale
}

func (cm *ConnManager) NeedsConnections() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	activeCount := 0
	for _, record := range cm.conns {
		if len(record.Connections) > 0 {
			activeCount++
		}
	}

	return activeCount < cm.config.MinConnections
}

func (cm *ConnManager) IsAtCapacity() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	totalConns := 0
	for _, record := range cm.conns {
		totalConns += len(record.Connections)
	}

	return totalConns >= cm.config.MaxConnections
}

func (cm *ConnManager) Stats() ConnManagerStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var totalIn, totalOut, totalMsgIn, totalMsgOut uint64
	activeCount := 0

	for _, record := range cm.conns {
		if len(record.Connections) > 0 {
			activeCount++
			totalIn += record.BytesIn
			totalOut += record.BytesOut
			totalMsgIn += record.MessagesIn
			totalMsgOut += record.MessagesOut
		}
	}

	return ConnManagerStats{
		ActivePeers:   activeCount,
		TotalPeers:    len(cm.conns),
		TotalBytesIn:  totalIn,
		TotalBytesOut: totalOut,
		TotalMsgIn:    totalMsgIn,
		TotalMsgOut:   totalMsgOut,
	}
}

func (cm *ConnManager) Close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, record := range cm.conns {
		for _, conn := range record.Connections {
			conn.Close()
		}
	}

	cm.conns = make(map[peer.ID]*ConnRecord)
}

type ConnManagerStats struct {
	ActivePeers   int
	TotalPeers    int
	TotalBytesIn  uint64
	TotalBytesOut uint64
	TotalMsgIn    uint64
	TotalMsgOut   uint64
}
