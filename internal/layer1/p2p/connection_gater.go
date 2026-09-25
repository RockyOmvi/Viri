package p2p

import (
	"net"
	"sync"

	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

type ConnectionGater struct {
	mu          sync.RWMutex
	blockedIPs  map[string]bool
	blockedSubs []*net.IPNet
	blockedPeers map[peer.ID]bool
}

func NewConnectionGater() *ConnectionGater {
	return &ConnectionGater{
		blockedIPs:   make(map[string]bool),
		blockedSubs:  make([]*net.IPNet, 0),
		blockedPeers: make(map[peer.ID]bool),
	}
}

func (cg *ConnectionGater) BlockAddr(ip net.IP) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.blockedIPs[ip.String()] = true
	return nil
}

func (cg *ConnectionGater) UnblockAddr(ip net.IP) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	delete(cg.blockedIPs, ip.String())
	return nil
}

func (cg *ConnectionGater) BlockSubnet(ipnet *net.IPNet) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.blockedSubs = append(cg.blockedSubs, ipnet)
	return nil
}

func (cg *ConnectionGater) BlockPeer(p peer.ID) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.blockedPeers[p] = true
	return nil
}

func (cg *ConnectionGater) UnblockPeer(p peer.ID) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	delete(cg.blockedPeers, p)
	return nil
}

func (cg *ConnectionGater) isBlockedIP(ipStr string) bool {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	if cg.blockedIPs[ipStr] {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, subnet := range cg.blockedSubs {
		if subnet.Contains(ip) {
			return true
		}
	}
	return false
}

func (cg *ConnectionGater) isBlockedPeer(p peer.ID) bool {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.blockedPeers[p]
}

func extractIPAddr(addr multiaddr.Multiaddr) (string, bool) {
	ip, err := manet.ToIP(addr)
	if err != nil {
		return "", false
	}
	return ip.String(), true
}

func (cg *ConnectionGater) InterceptAccept(cma network.ConnMultiaddrs) (allow bool) {
	addr := cma.RemoteMultiaddr()
	if ipStr, ok := extractIPAddr(addr); ok {
		if cg.isBlockedIP(ipStr) {
			return false
		}
	}
	return true
}

func (cg *ConnectionGater) InterceptSecured(_ network.Direction, p peer.ID, cma network.ConnMultiaddrs) (allow bool) {
	if cg.isBlockedPeer(p) {
		return false
	}
	addr := cma.RemoteMultiaddr()
	if ipStr, ok := extractIPAddr(addr); ok {
		if cg.isBlockedIP(ipStr) {
			return false
		}
	}
	return true
}

func (cg *ConnectionGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

func (cg *ConnectionGater) InterceptAddrDial(_ peer.ID, addr multiaddr.Multiaddr) (allow bool) {
	if ipStr, ok := extractIPAddr(addr); ok {
		if cg.isBlockedIP(ipStr) {
			return false
		}
	}
	return true
}

func (cg *ConnectionGater) InterceptPeerDial(p peer.ID) (allow bool) {
	return !cg.isBlockedPeer(p)
}
