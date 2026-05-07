package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsCollector struct {
	ConsensusHeight         prometheus.Gauge
	ConsensusView           prometheus.Gauge
	ConsensusPhase          prometheus.Gauge
	ConsensusValidators     prometheus.Gauge
	ConsensusBlocksFinalized prometheus.Counter
	ConsensusViewChanges    prometheus.Counter
	ConsensusProposals      prometheus.Counter
	ConsensusVotes          prometheus.Counter
	ConsensusInvariantViolations prometheus.Counter
	ConsensusRateLimited    prometheus.Counter

	P2PPeersConnected  prometheus.Gauge
	P2PBytesIn         prometheus.Counter
	P2PBytesOut        prometheus.Counter
	P2PMessagesIn      prometheus.Counter
	P2PMessagesOut     prometheus.Counter

	NodeUptimeSeconds  prometheus.Gauge
	NodeIsSyncing     prometheus.Gauge
	MempoolPendingTxs prometheus.Gauge

	startTime time.Time
	mu        sync.RWMutex

	healthData *HealthData
}

type HealthData struct {
	Height   uint64
	Peers    int
	Syncing  bool
}

type healthResponse struct {
	Status  string `json:"status"`
	Height  uint64 `json:"height"`
	Peers   int    `json:"peers"`
	Syncing bool   `json:"syncing"`
}

func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		startTime: time.Now(),
		healthData: &HealthData{},
	}

	mc.ConsensusHeight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "consensus",
		Name:      "height",
		Help:      "Current block height.",
	})
	mc.ConsensusView = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "consensus",
		Name:      "view",
		Help:      "Current view number.",
	})
	mc.ConsensusPhase = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "consensus",
		Name:      "phase",
		Help:      "Current phase (0=idle, 1=prepare, 2=precommit, 3=commit, 4=decide).",
	})
	mc.ConsensusValidators = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "consensus",
		Name:      "validators",
		Help:      "Number of active validators.",
	})
	mc.ConsensusBlocksFinalized = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "consensus",
		Name:      "block_finalized_total",
		Help:      "Total blocks finalized.",
	})
	mc.ConsensusViewChanges = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "consensus",
		Name:      "view_changes_total",
		Help:      "Total view changes.",
	})
	mc.ConsensusProposals = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "consensus",
		Name:      "proposals_total",
		Help:      "Total proposals sent/received.",
	})
	mc.ConsensusVotes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "consensus",
		Name:      "votes_total",
		Help:      "Total votes sent/received.",
	})
	mc.ConsensusInvariantViolations = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "consensus",
		Name:      "invariant_violations_total",
		Help:      "Total consensus invariant violations detected.",
	})
	mc.ConsensusRateLimited = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "consensus",
		Name:      "rate_limited_total",
		Help:      "Total messages rate limited.",
	})

	mc.P2PPeersConnected = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "p2p",
		Name:      "peers_connected",
		Help:      "Number of connected peers.",
	})
	mc.P2PBytesIn = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "p2p",
		Name:      "bytes_in_total",
		Help:      "Total bytes received.",
	})
	mc.P2PBytesOut = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "p2p",
		Name:      "bytes_out_total",
		Help:      "Total bytes sent.",
	})
	mc.P2PMessagesIn = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "p2p",
		Name:      "messages_in_total",
		Help:      "Total messages received.",
	})
	mc.P2PMessagesOut = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "p2p",
		Name:      "messages_out_total",
		Help:      "Total messages sent.",
	})

	mc.NodeUptimeSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "node",
		Name:      "uptime_seconds",
		Help:      "Node uptime in seconds.",
	})
	mc.NodeIsSyncing = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "node",
		Name:      "is_syncing",
		Help:      "Whether node is syncing (1=true, 0=false).",
	})
	mc.MempoolPendingTxs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "mempool",
		Name:      "pending_txs",
		Help:      "Pending transactions in mempool.",
	})

	prometheus.MustRegister(
		mc.ConsensusHeight,
		mc.ConsensusView,
		mc.ConsensusPhase,
		mc.ConsensusValidators,
		mc.ConsensusBlocksFinalized,
		mc.ConsensusViewChanges,
		mc.ConsensusProposals,
		mc.ConsensusVotes,
		mc.ConsensusInvariantViolations,
		mc.ConsensusRateLimited,
		mc.P2PPeersConnected,
		mc.P2PBytesIn,
		mc.P2PBytesOut,
		mc.P2PMessagesIn,
		mc.P2PMessagesOut,
		mc.NodeUptimeSeconds,
		mc.NodeIsSyncing,
		mc.MempoolPendingTxs,
	)

	return mc
}

func (mc *MetricsCollector) SetConsensusHeight(height uint64) {
	mc.ConsensusHeight.Set(float64(height))
}

func (mc *MetricsCollector) SetConsensusView(view uint64) {
	mc.ConsensusView.Set(float64(view))
}

func (mc *MetricsCollector) SetConsensusPhase(phase float64) {
	mc.ConsensusPhase.Set(phase)
}

func (mc *MetricsCollector) SetConsensusValidators(count int) {
	mc.ConsensusValidators.Set(float64(count))
}

func (mc *MetricsCollector) IncConsensusBlocksFinalized() {
	mc.ConsensusBlocksFinalized.Inc()
}

func (mc *MetricsCollector) IncConsensusViewChanges() {
	mc.ConsensusViewChanges.Inc()
}

func (mc *MetricsCollector) IncConsensusProposals() {
	mc.ConsensusProposals.Inc()
}

func (mc *MetricsCollector) IncConsensusVotes() {
	mc.ConsensusVotes.Inc()
}

func (mc *MetricsCollector) IncConsensusInvariantViolations() {
	mc.ConsensusInvariantViolations.Inc()
}

func (mc *MetricsCollector) IncRateLimitedMessages() {
	mc.ConsensusRateLimited.Inc()
}

func (mc *MetricsCollector) SetP2PPeersConnected(count int) {
	mc.P2PPeersConnected.Set(float64(count))
}

func (mc *MetricsCollector) AddP2PBytesIn(n int) {
	mc.P2PBytesIn.Add(float64(n))
}

func (mc *MetricsCollector) AddP2PBytesOut(n int) {
	mc.P2PBytesOut.Add(float64(n))
}

func (mc *MetricsCollector) IncP2PMessagesIn() {
	mc.P2PMessagesIn.Inc()
}

func (mc *MetricsCollector) IncP2PMessagesOut() {
	mc.P2PMessagesOut.Inc()
}

func (mc *MetricsCollector) SetNodeIsSyncing(syncing bool) {
	if syncing {
		mc.NodeIsSyncing.Set(1)
	} else {
		mc.NodeIsSyncing.Set(0)
	}
}

func (mc *MetricsCollector) SetMempoolPendingTxs(count int) {
	mc.MempoolPendingTxs.Set(float64(count))
}

func (mc *MetricsCollector) UpdateUptime() {
	mc.NodeUptimeSeconds.Set(time.Since(mc.startTime).Seconds())
}

func (mc *MetricsCollector) SetHealthData(height uint64, peers int, syncing bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.healthData.Height = height
	mc.healthData.Peers = peers
	mc.healthData.Syncing = syncing
}

func (mc *MetricsCollector) healthHandler(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	resp := healthResponse{
		Status:  "ok",
		Height:  mc.healthData.Height,
		Peers:   mc.healthData.Peers,
		Syncing: mc.healthData.Syncing,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (mc *MetricsCollector) healthReadyHandler(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if mc.healthData.Syncing || mc.healthData.Height == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		resp := healthResponse{
			Status:  "not ready",
			Height:  mc.healthData.Height,
			Peers:   mc.healthData.Peers,
			Syncing: mc.healthData.Syncing,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
	resp := healthResponse{
		Status:  "ok",
		Height:  mc.healthData.Height,
		Peers:   mc.healthData.Peers,
		Syncing: mc.healthData.Syncing,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (mc *MetricsCollector) StartMetricsServer(port int) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", mc.healthHandler)
	mux.HandleFunc("/health/ready", mc.healthReadyHandler)
	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		}
	}()
	return server
}
