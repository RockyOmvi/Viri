package p2p

import (
	"sync"
	"sync/atomic"
	"time"
)

type NetworkStats struct {
	mu                sync.RWMutex
	startTime         time.Time
	TotalMessagesIn   atomic.Int64
	TotalMessagesOut  atomic.Int64
	TotalBytesIn      atomic.Int64
	TotalBytesOut     atomic.Int64
	TotalBlocksIn     atomic.Int64
	TotalBlocksOut    atomic.Int64
	TotalTxsIn        atomic.Int64
	TotalTxsOut       atomic.Int64
	RejectedMessages  atomic.Int64
	DroppedMessages   atomic.Int64
	ActivePeaks       atomic.Int64
	currentPeers      int
	peakPeers         int
}

func NewNetworkStats() *NetworkStats {
	return &NetworkStats{
		startTime: time.Now(),
	}
}

func (ns *NetworkStats) RecordMessageIn(size int) {
	ns.TotalMessagesIn.Add(1)
	ns.TotalBytesIn.Add(int64(size))
}

func (ns *NetworkStats) RecordMessageOut(size int) {
	ns.TotalMessagesOut.Add(1)
	ns.TotalBytesOut.Add(int64(size))
}

func (ns *NetworkStats) RecordBlockIn() {
	ns.TotalBlocksIn.Add(1)
	ns.RecordMessageIn(0)
}

func (ns *NetworkStats) RecordBlockOut() {
	ns.TotalBlocksOut.Add(1)
	ns.RecordMessageOut(0)
}

func (ns *NetworkStats) RecordTxIn() {
	ns.TotalTxsIn.Add(1)
	ns.RecordMessageIn(0)
}

func (ns *NetworkStats) RecordTxOut() {
	ns.TotalTxsOut.Add(1)
	ns.RecordMessageOut(0)
}

func (ns *NetworkStats) RecordRejected() {
	ns.RejectedMessages.Add(1)
}

func (ns *NetworkStats) RecordDropped() {
	ns.DroppedMessages.Add(1)
}

func (ns *NetworkStats) SetPeerCount(count int) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.currentPeers = count
	if count > ns.peakPeers {
		ns.peakPeers = count
		ns.ActivePeaks.Store(int64(count))
	}
}

func (ns *NetworkStats) Snapshot() NetworkStatsSnapshot {
	return NetworkStatsSnapshot{
		Uptime:            time.Since(ns.startTime),
		TotalMessagesIn:   ns.TotalMessagesIn.Load(),
		TotalMessagesOut:  ns.TotalMessagesOut.Load(),
		TotalBytesIn:      ns.TotalBytesIn.Load(),
		TotalBytesOut:     ns.TotalBytesOut.Load(),
		TotalBlocksIn:     ns.TotalBlocksIn.Load(),
		TotalBlocksOut:    ns.TotalBlocksOut.Load(),
		TotalTxsIn:        ns.TotalTxsIn.Load(),
		TotalTxsOut:       ns.TotalTxsOut.Load(),
		RejectedMessages:  ns.RejectedMessages.Load(),
		DroppedMessages:   ns.DroppedMessages.Load(),
		CurrentPeers:      ns.currentPeers,
		PeakPeers:         ns.peakPeers,
		MessagesPerSecond: ns.messagesPerSecond(),
		BytesPerSecondIn:  ns.bytesPerSecondIn(),
		BytesPerSecondOut: ns.bytesPerSecondOut(),
	}
}

func (ns *NetworkStats) messagesPerSecond() float64 {
	elapsed := time.Since(ns.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	total := ns.TotalMessagesIn.Load() + ns.TotalMessagesOut.Load()
	return float64(total) / elapsed
}

func (ns *NetworkStats) bytesPerSecondIn() float64 {
	elapsed := time.Since(ns.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(ns.TotalBytesIn.Load()) / elapsed
}

func (ns *NetworkStats) bytesPerSecondOut() float64 {
	elapsed := time.Since(ns.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(ns.TotalBytesOut.Load()) / elapsed
}

func (ns *NetworkStats) Reset() {
	ns.TotalMessagesIn.Store(0)
	ns.TotalMessagesOut.Store(0)
	ns.TotalBytesIn.Store(0)
	ns.TotalBytesOut.Store(0)
	ns.TotalBlocksIn.Store(0)
	ns.TotalBlocksOut.Store(0)
	ns.TotalTxsIn.Store(0)
	ns.TotalTxsOut.Store(0)
	ns.RejectedMessages.Store(0)
	ns.DroppedMessages.Store(0)
	ns.startTime = time.Now()
}

type NetworkStatsSnapshot struct {
	Uptime            time.Duration
	TotalMessagesIn   int64
	TotalMessagesOut  int64
	TotalBytesIn      int64
	TotalBytesOut     int64
	TotalBlocksIn     int64
	TotalBlocksOut    int64
	TotalTxsIn        int64
	TotalTxsOut       int64
	RejectedMessages  int64
	DroppedMessages   int64
	CurrentPeers      int
	PeakPeers         int
	MessagesPerSecond float64
	BytesPerSecondIn  float64
	BytesPerSecondOut float64
}
