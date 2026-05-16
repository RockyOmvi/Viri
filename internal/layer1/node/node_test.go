package node

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/logging"
)

// ----- LightMode tests -----

func TestDefaultLightMode(t *testing.T) {
	lm := DefaultLightMode()
	if lm == nil {
		t.Fatal("expected non-nil LightMode")
	}
	if lm.MaxPeers != 4 {
		t.Fatalf("expected MaxPeers=4, got %d", lm.MaxPeers)
	}
	if lm.MaxMempoolSize != 1000 {
		t.Fatalf("expected MaxMempoolSize=1000, got %d", lm.MaxMempoolSize)
	}
	if lm.BadgerCacheSize != 256 {
		t.Fatalf("expected BadgerCacheSize=256, got %d", lm.BadgerCacheSize)
	}
	if lm.PruneAfterEpochs != 1000 {
		t.Fatalf("expected PruneAfterEpochs=1000, got %d", lm.PruneAfterEpochs)
	}
	if !lm.DisableHistoricRPC {
		t.Fatal("expected DisableHistoricRPC=true")
	}
	if lm.StateCacheSize != 10_000 {
		t.Fatalf("expected StateCacheSize=10000, got %d", lm.StateCacheSize)
	}
}

func TestLightModeEnableDisable(t *testing.T) {
	lm := DefaultLightMode()
	if lm.IsEnabled() {
		t.Fatal("should not be enabled initially")
	}
	lm.Enable()
	if !lm.IsEnabled() {
		t.Fatal("should be enabled after Enable()")
	}
}

func TestLightModeConcurrency(t *testing.T) {
	lm := DefaultLightMode()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lm.Enable()
			_ = lm.IsEnabled()
		}()
	}
	wg.Wait()
	if !lm.IsEnabled() {
		t.Fatal("should be enabled after concurrent Enable()")
	}
}

// ----- StatePruner tests -----

type mockStateDeleter struct {
	mu            sync.Mutex
	deletedBefore uint64
	deleteErr     error
}

func (m *mockStateDeleter) DeleteBefore(epoch uint64) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedBefore = epoch
	return epoch, m.deleteErr
}

func TestNewStatePruner(t *testing.T) {
	md := &mockStateDeleter{}
	sp := NewStatePruner(md, 100)
	if sp == nil {
		t.Fatal("expected non-nil StatePruner")
	}
}

func TestStatePrunerNoPruneBelowKeep(t *testing.T) {
	md := &mockStateDeleter{}
	sp := NewStatePruner(md, 100)

	count, err := sp.Prune(50)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pruned blocks, got %d", count)
	}
	md.mu.Lock()
	if md.deletedBefore != 0 {
		t.Fatalf("expected DeleteBefore not called, got DeleteBefore(%d)", md.deletedBefore)
	}
	md.mu.Unlock()
}

func TestStatePrunerExactlyKeep(t *testing.T) {
	md := &mockStateDeleter{}
	sp := NewStatePruner(md, 100)

	count, err := sp.Prune(100)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pruned blocks, got %d", count)
	}
}

func TestStatePrunerPrunes(t *testing.T) {
	md := &mockStateDeleter{}
	sp := NewStatePruner(md, 100)

	count, err := sp.Prune(200)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	if count != 100 {
		t.Fatalf("expected 100 pruned blocks, got %d", count)
	}
	md.mu.Lock()
	if md.deletedBefore != 100 {
		t.Fatalf("expected DeleteBefore(100), got DeleteBefore(%d)", md.deletedBefore)
	}
	md.mu.Unlock()
}

func TestStatePrunerError(t *testing.T) {
	md := &mockStateDeleter{deleteErr: errors.New("db error")}
	sp := NewStatePruner(md, 100)

	if _, err := sp.Prune(200); err == nil {
		t.Fatal("expected error from pruner")
	}
}

func TestStatePrunerStats(t *testing.T) {
	md := &mockStateDeleter{}
	sp := NewStatePruner(md, 100)

	blocks, lastTime := sp.Stats()
	if blocks != 0 {
		t.Fatalf("expected 0 blocks, got %d", blocks)
	}
	if !lastTime.IsZero() {
		t.Fatal("expected zero time initially")
	}

	sp.Prune(200)
	blocks, lastTime = sp.Stats()
	if blocks != 100 {
		t.Fatalf("expected 100 blocks, got %d", blocks)
	}
	if lastTime.IsZero() {
		t.Fatal("expected non-zero time after prune")
	}
}

func TestStatePrunerConcurrency(t *testing.T) {
	md := &mockStateDeleter{}
	sp := NewStatePruner(md, 100)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(epoch uint64) {
			defer wg.Done()
			sp.Prune(epoch)
		}(uint64(200 + i))
	}
	wg.Wait()

	blocks, _ := sp.Stats()
	if blocks == 0 {
		t.Fatal("expected some blocks to be pruned")
	}
}

// ----- ShutdownManager tests -----

type mockComponent struct {
	name     string
	priority int
	err      error
}

func (m *mockComponent) ShutdownPriority() int        { return m.priority }
func (m *mockComponent) Name() string                 { return m.name }
func (m *mockComponent) Shutdown(ctx context.Context) error { return m.err }

type stoppableServer struct {
	stopped bool
}

func (s *stoppableServer) Stop() error {
	s.stopped = true
	return nil
}

func (s *stoppableServer) Close() error {
	s.stopped = true
	return nil
}

func testLogger(t *testing.T) *logging.Logger {
	t.Helper()
	return logging.NewLogger("test", logging.ERROR, "text")
}

func TestNewShutdownManager(t *testing.T) {
	dir := t.TempDir()
	sm := NewShutdownManager(testLogger(t), dir)
	if sm == nil {
		t.Fatal("expected non-nil ShutdownManager")
	}
}

func TestShutdownManagerRegisterComponent(t *testing.T) {
	sm := NewShutdownManager(testLogger(t), t.TempDir())
	c := &mockComponent{name: "test", priority: 1}
	sm.RegisterComponent(c)
}

func TestShutdownManagerRegisterCallback(t *testing.T) {
	sm := NewShutdownManager(testLogger(t), t.TempDir())
	called := false
	sm.RegisterShutdownCallback(func() { called = true })
	sm.TriggerShutdown()
	sm.runCallbacks()
	if !called {
		t.Fatal("shutdown callback was not called")
	}
}

func TestTriggerShutdownOnce(t *testing.T) {
	sm := NewShutdownManager(testLogger(t), t.TempDir())
	sm.TriggerShutdown()
	sm.TriggerShutdown()
}

func TestShutdownChBlocks(t *testing.T) {
	sm := NewShutdownManager(testLogger(t), t.TempDir())

	triggered := make(chan bool)
	go func() {
		sm.WaitForShutdown()
		triggered <- true
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-triggered:
		t.Fatal("WaitForShutdown returned before TriggerShutdown")
	default:
	}

	sm.TriggerShutdown()
	select {
	case <-triggered:
	case <-time.After(time.Second):
		t.Fatal("WaitForShutdown did not return after TriggerShutdown")
	}
}

func TestShutdownComponentsOrder(t *testing.T) {
	sm := NewShutdownManager(testLogger(t), t.TempDir())
	var order []string
	var mu sync.Mutex

	c1 := ComponentFunc{
		name:     "c1",
		priority: 10,
		fn: func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "c1")
			mu.Unlock()
			return nil
		},
	}
	c2 := ComponentFunc{
		name:     "c2",
		priority: 5,
		fn: func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "c2")
			mu.Unlock()
			return nil
		},
	}

	sm.RegisterComponent(&c1)
	sm.RegisterComponent(&c2)

	nc := &NodeComponents{}
	sm.RunShutdownSequence(nc)

	mu.Lock()
	got := order
	mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("expected 2 components shut down, got %d", len(got))
	}
	if got[0] != "c1" || got[1] != "c2" {
		t.Fatalf("expected [c1 c2], got %v", got)
	}
}

func TestShutdownComponentError(t *testing.T) {
	sm := NewShutdownManager(testLogger(t), t.TempDir())
	c := &mockComponent{name: "err", priority: 1, err: errors.New("fail")}
	sm.RegisterComponent(c)
	nc := &NodeComponents{}
	sm.RunShutdownSequence(nc)
}

func TestShutdownStopsServers(t *testing.T) {
	sm := NewShutdownManager(testLogger(t), t.TempDir())
	rpc := &stoppableServer{}
	api := &stoppableServer{}
	ws := &stoppableServer{}
	metrics := &stoppableServer{}
	audit := &stoppableServer{}

	nc := &NodeComponents{
		RPCServer:     rpc,
		APIServer:     api,
		WSServer:      ws,
		MetricsServer: metrics,
		AuditLog:      audit,
	}
	sm.RunShutdownSequence(nc)

	if !rpc.stopped {
		t.Fatal("RPC server was not stopped")
	}
	if !api.stopped {
		t.Fatal("API server was not stopped")
	}
	if !ws.stopped {
		t.Fatal("WebSocket server was not stopped")
	}
}

func TestShutdownSaveState(t *testing.T) {
	dir := t.TempDir()
	sm := NewShutdownManager(testLogger(t), dir)
	nc := &NodeComponents{}
	sm.RunShutdownSequence(nc)

	stateFile := filepath.Join(dir, "shutdown_state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatal("shutdown state file was not created")
	}
}

func TestStartShutdownMonitorTimeout(t *testing.T) {
	if os.Getenv("TEST_SHUTDOWN_TIMEOUT") == "" {
		t.Skip("set TEST_SHUTDOWN_TIMEOUT to run")
	}

	sm := NewShutdownManager(testLogger(t), t.TempDir())
	sm.StartShutdownMonitor()
	sm.TriggerShutdown()

	select {
	case <-sm.doneCh:
	case <-time.After(2 * time.Second):
		sm.log.Error("timeout waiting for done")
	}
}

func TestSavePeerList(t *testing.T) {
	dir := t.TempDir()
	sm := NewShutdownManager(testLogger(t), dir)
	nc := &NodeComponents{}
	sm.RunShutdownSequence(nc)

	peerFile := filepath.Join(dir, "peers.json")
	if _, err := os.Stat(peerFile); !os.IsNotExist(err) {
		t.Fatal("peers.json should not exist when no network")
	}
}
