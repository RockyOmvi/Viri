package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/viri-chain/viri/internal/layer1/consensus"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/layer1/p2p"
	"github.com/viri-chain/viri/internal/layer1/state"
	syncpkg "github.com/viri-chain/viri/internal/layer1/sync"
)

const shutdownTimeout = 30 * time.Second

type ShutdownManager struct {
	mu            sync.Mutex
	log           *logging.Logger
	dataDir       string
	components    []Component
	shutdownCh    chan struct{}
	doneCh        chan struct{}
	shutdownOnce  sync.Once
	shutdownCBs   []func()
	shutdownState *ShutdownState
}

type Component interface {
	ShutdownPriority() int
	Shutdown(ctx context.Context) error
	Name() string
}

type ComponentFunc struct {
	name     string
	priority int
	fn       func(ctx context.Context) error
}

func (c *ComponentFunc) ShutdownPriority() int {
	return c.priority
}

func (c *ComponentFunc) Shutdown(ctx context.Context) error {
	return c.fn(ctx)
}

func (c *ComponentFunc) Name() string {
	return c.name
}

type ShutdownState struct {
	Timestamp      time.Time                `json:"timestamp"`
	ConsensusState *consensus.ConsensusState `json:"consensus_state,omitempty"`
	PeerList       []string                `json:"peer_list,omitempty"`
	MempoolSize    int                     `json:"mempool_size"`
	SyncState      *SyncState              `json:"sync_state,omitempty"`
	BlockHeight    uint64                  `json:"block_height"`
	TipHash        string                  `json:"tip_hash"`
}

type SyncState struct {
	Syncing       bool   `json:"syncing"`
	CurrentHeight uint64 `json:"current_height"`
	TargetHeight  uint64 `json:"target_height"`
}

type NodeComponents struct {
	Blockchain      *ledger.PersistentBlockchain
	ConsensusEngine *consensus.HotStuffEngine
	Network         *p2p.ViriNetwork
	DB              state.KVStore
	StateMgr        *state.StateManager
	MempoolPersist  *ledger.MempoolPersister
	NodeSyncer      *syncpkg.Syncer
	RPCServer       interface{ Stop() error }
	APIServer       interface{ Stop() error }
	WSServer        interface{ Stop() error }
	MetricsServer   interface{ Close() error }
	AuditLog        interface{ Close() error }
}

func NewShutdownManager(log *logging.Logger, dataDir string) *ShutdownManager {
	return &ShutdownManager{
		log:           log,
		dataDir:       dataDir,
		components:    make([]Component, 0),
		shutdownCh:    make(chan struct{}),
		doneCh:        make(chan struct{}),
		shutdownState: &ShutdownState{},
	}
}

func (sm *ShutdownManager) RegisterComponent(c Component) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.components = append(sm.components, c)
}

func (sm *ShutdownManager) RegisterShutdownCallback(fn func()) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.shutdownCBs = append(sm.shutdownCBs, fn)
}

func (sm *ShutdownManager) SetupSignalHandling() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		sm.log.Info(fmt.Sprintf("Received signal: %s", sig))
		sm.TriggerShutdown()
	}()
}

func (sm *ShutdownManager) TriggerShutdown() {
	sm.shutdownOnce.Do(func() {
		sm.log.Info("Initiating graceful shutdown...")
		close(sm.shutdownCh)
	})
}

func (sm *ShutdownManager) ShutdownCh() <-chan struct{} {
	return sm.shutdownCh
}

func (sm *ShutdownManager) DoneCh() <-chan struct{} {
	return sm.doneCh
}

func (sm *ShutdownManager) WaitForShutdown() {
	<-sm.shutdownCh
}

func (sm *ShutdownManager) RunShutdownSequence(nc *NodeComponents) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	sm.persistStateBeforeShutdown(nc)
	sm.stopAcceptingConnections(nc)
	sm.shutdownComponents(ctx, nc)
	sm.flushMempool(nc)
	sm.closeNetwork(nc, ctx)
	sm.saveShutdownState()
	sm.closeDatabase(nc)
	sm.runCallbacks()

	sm.log.Info("Shutdown complete")
	close(sm.doneCh)
}

func (sm *ShutdownManager) persistStateBeforeShutdown(nc *NodeComponents) {
	sm.log.Info("Persisting state before shutdown...")

	if nc.ConsensusEngine != nil && nc.ConsensusEngine.IsRunning() {
		cs := nc.ConsensusEngine.GetState()
		sm.shutdownState.ConsensusState = cs

		sm.log.Info(fmt.Sprintf("Consensus state saved: height=%d view=%d phase=%d",
			cs.Height, cs.View, cs.Phase))

		nc.ConsensusEngine.Stop()
	}

	if nc.Blockchain != nil {
		sm.shutdownState.BlockHeight = nc.Blockchain.Height()
		tipHash := nc.Blockchain.TipHash()
		sm.shutdownState.TipHash = fmt.Sprintf("%x", tipHash)
	}
}

func (sm *ShutdownManager) stopAcceptingConnections(nc *NodeComponents) {
	sm.log.Info("Stopping RPC/API servers...")

	if nc.RPCServer != nil {
		if err := nc.RPCServer.Stop(); err != nil {
			sm.log.Error(fmt.Sprintf("Error stopping RPC server: %v", err))
		}
	}

	if nc.WSServer != nil {
		if err := nc.WSServer.Stop(); err != nil {
			sm.log.Error(fmt.Sprintf("Error stopping WebSocket server: %v", err))
		}
	}

	if nc.APIServer != nil {
		if err := nc.APIServer.Stop(); err != nil {
			sm.log.Error(fmt.Sprintf("Error stopping API server: %v", err))
		}
	}
}

func (sm *ShutdownManager) shutdownComponents(ctx context.Context, nc *NodeComponents) {
	sm.log.Info("Shutting down components...")

	sm.mu.Lock()
	components := make([]Component, len(sm.components))
	copy(components, sm.components)
	sm.mu.Unlock()

	for _, c := range components {
		sm.log.Info(fmt.Sprintf("Shutting down component: %s", c.Name()))
		if err := c.Shutdown(ctx); err != nil {
			sm.log.Error(fmt.Sprintf("Error shutting down %s: %v", c.Name(), err))
		}
	}

	if nc.NodeSyncer != nil {
		sm.saveSyncState(nc.NodeSyncer)
		nc.NodeSyncer.Stop()
	}
}

func (sm *ShutdownManager) flushMempool(nc *NodeComponents) {
	if nc.MempoolPersist != nil && nc.Blockchain != nil {
		sm.log.Info("Flushing mempool to disk...")
		if err := nc.MempoolPersist.Save(nc.Blockchain.TxPool()); err != nil {
			sm.log.Warn(fmt.Sprintf("Failed to save mempool: %v", err))
		} else {
			sm.shutdownState.MempoolSize = nc.Blockchain.TxPool().Size()
			sm.log.Info(fmt.Sprintf("Mempool saved: %d transactions", sm.shutdownState.MempoolSize))
		}
	}
}

func (sm *ShutdownManager) closeNetwork(nc *NodeComponents, ctx context.Context) {
	if nc.Network != nil {
		sm.log.Info("Saving peer list...")
		sm.savePeerList(nc.Network)

		sm.log.Info("Closing P2P network...")
		if err := nc.Network.Close(); err != nil {
			sm.log.Error(fmt.Sprintf("Error closing network: %v", err))
		}
	}
}

func (sm *ShutdownManager) savePeerList(net *p2p.ViriNetwork) {
	peers := net.Peers()
	peerList := make([]string, 0, len(peers))

	for _, p := range peers {
		peerList = append(peerList, p.ID.String())
	}

	sm.shutdownState.PeerList = peerList

	peerFile := filepath.Join(sm.dataDir, "peers.json")
	data, err := json.MarshalIndent(peerList, "", "  ")
	if err != nil {
		sm.log.Warn(fmt.Sprintf("Failed to marshal peer list: %v", err))
		return
	}

	if err := os.WriteFile(peerFile, data, 0644); err != nil {
		sm.log.Warn(fmt.Sprintf("Failed to save peer list: %v", err))
	}
}

func (sm *ShutdownManager) saveSyncState(syncer *syncpkg.Syncer) {
	if syncer == nil {
		return
	}

	progress := syncer.Progress()
	sm.shutdownState.SyncState = &SyncState{
		Syncing:       syncer.IsSyncing(),
		CurrentHeight: progress.CurrentHeight,
		TargetHeight:  progress.HighestHeight,
	}
}

func (sm *ShutdownManager) saveShutdownState() {
	sm.shutdownState.Timestamp = time.Now()

	stateFile := filepath.Join(sm.dataDir, "shutdown_state.json")
	data, err := json.MarshalIndent(sm.shutdownState, "", "  ")
	if err != nil {
		sm.log.Warn(fmt.Sprintf("Failed to marshal shutdown state: %v", err))
		return
	}

	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		sm.log.Warn(fmt.Sprintf("Failed to save shutdown state: %v", err))
	}
}

func (sm *ShutdownManager) closeDatabase(nc *NodeComponents) {
	if nc.StateMgr != nil {
		if err := nc.StateMgr.Close(); err != nil {
			sm.log.Error(fmt.Sprintf("Error closing state manager: %v", err))
		}
	}

	if nc.DB != nil {
		if err := nc.DB.Close(); err != nil {
			sm.log.Error(fmt.Sprintf("Error closing database: %v", err))
		}
	}
}

func (sm *ShutdownManager) runCallbacks() {
	sm.mu.Lock()
	cbs := make([]func(), len(sm.shutdownCBs))
	copy(cbs, sm.shutdownCBs)
	sm.mu.Unlock()

	for _, cb := range cbs {
		cb()
	}
}

func (sm *ShutdownManager) StartShutdownMonitor() {
	go func() {
		select {
		case <-sm.shutdownCh:
			select {
			case <-time.After(shutdownTimeout):
				sm.log.Error("Graceful shutdown timed out, forcing exit")
				os.Exit(1)
			case <-sm.doneCh:
				return
			}
		case <-sm.doneCh:
			return
		}
	}()
}
