package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Propagator struct {
	mu             sync.RWMutex
	seenBlocks     map[string]time.Time
	seenTxs        map[string]time.Time
	blockTTL       time.Duration
	txTTL          time.Duration
	pendingBlocks  map[string][]peer.ID
	pendingTxs     map[string][]peer.ID
}

func NewPropagator(blockTTL, txTTL time.Duration) *Propagator {
	if blockTTL == 0 {
		blockTTL = 30 * time.Minute
	}
	if txTTL == 0 {
		txTTL = 10 * time.Minute
	}

	p := &Propagator{
		seenBlocks:    make(map[string]time.Time),
		seenTxs:       make(map[string]time.Time),
		blockTTL:      blockTTL,
		txTTL:         txTTL,
		pendingBlocks: make(map[string][]peer.ID),
		pendingTxs:    make(map[string][]peer.ID),
	}

	go p.cleanupLoop()

	return p
}

func (p *Propagator) IsBlockSeen(hash string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.seenBlocks[hash]
	return exists
}

func (p *Propagator) IsTxSeen(hash string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.seenTxs[hash]
	return exists
}

func (p *Propagator) MarkBlockSeen(hash string, from peer.ID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.seenBlocks[hash]; exists {
		return false
	}

	p.seenBlocks[hash] = time.Now()
	return true
}

func (p *Propagator) MarkTxSeen(hash string, from peer.ID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.seenTxs[hash]; exists {
		return false
	}

	p.seenTxs[hash] = time.Now()
	return true
}

func (p *Propagator) GetPeersToPropagate(hash string, isBlock bool, exclude peer.ID) []peer.ID {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var pending []peer.ID
	if isBlock {
		for _, pid := range p.pendingBlocks[hash] {
			if pid != exclude {
				pending = append(pending, pid)
			}
		}
	} else {
		for _, pid := range p.pendingTxs[hash] {
			if pid != exclude {
				pending = append(pending, pid)
			}
		}
	}

	return pending
}

func (p *Propagator) AddPendingBlock(hash string, peers []peer.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pendingBlocks[hash] = peers
}

func (p *Propagator) AddPendingTx(hash string, peers []peer.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pendingTxs[hash] = peers
}

func (p *Propagator) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.cleanup()
	}
}

func (p *Propagator) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	for hash, seenAt := range p.seenBlocks {
		if now.Sub(seenAt) > p.blockTTL {
			delete(p.seenBlocks, hash)
			delete(p.pendingBlocks, hash)
		}
	}

	for hash, seenAt := range p.seenTxs {
		if now.Sub(seenAt) > p.txTTL {
			delete(p.seenTxs, hash)
			delete(p.pendingTxs, hash)
		}
	}
}

func (p *Propagator) Stats() PropagatorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PropagatorStats{
		SeenBlocks:    len(p.seenBlocks),
		SeenTxs:       len(p.seenTxs),
		PendingBlocks: len(p.pendingBlocks),
		PendingTxs:    len(p.pendingTxs),
	}
}

type PropagatorStats struct {
	SeenBlocks    int
	SeenTxs       int
	PendingBlocks int
	PendingTxs    int
}
