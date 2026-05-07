package metrics

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

var (
	sharedMC     *MetricsCollector
	sharedMCOnce sync.Once
)

func getSharedCollector() *MetricsCollector {
	sharedMCOnce.Do(func() {
		sharedMC = NewMetricsCollector()
	})
	return sharedMC
}

func TestSetConsensusHeight(t *testing.T) {
	mc := getSharedCollector()
	mc.SetConsensusHeight(100)
}

func TestSetConsensusView(t *testing.T) {
	mc := getSharedCollector()
	mc.SetConsensusView(5)
}

func TestSetConsensusPhase(t *testing.T) {
	mc := getSharedCollector()
	mc.SetConsensusPhase(1)
}

func TestSetConsensusValidators(t *testing.T) {
	mc := getSharedCollector()
	mc.SetConsensusValidators(4)
}

func TestIncConsensusBlocksFinalized(t *testing.T) {
	mc := getSharedCollector()
	mc.IncConsensusBlocksFinalized()
	mc.IncConsensusBlocksFinalized()
}

func TestIncConsensusViewChanges(t *testing.T) {
	mc := getSharedCollector()
	mc.IncConsensusViewChanges()
}

func TestIncConsensusProposals(t *testing.T) {
	mc := getSharedCollector()
	mc.IncConsensusProposals()
}

func TestIncConsensusVotes(t *testing.T) {
	mc := getSharedCollector()
	mc.IncConsensusVotes()
}

func TestIncConsensusInvariantViolations(t *testing.T) {
	mc := getSharedCollector()
	mc.IncConsensusInvariantViolations()
}

func TestIncRateLimitedMessages(t *testing.T) {
	mc := getSharedCollector()
	mc.IncRateLimitedMessages()
}

func TestSetP2PPeersConnected(t *testing.T) {
	mc := getSharedCollector()
	mc.SetP2PPeersConnected(10)
}

func TestAddP2PBytesIn(t *testing.T) {
	mc := getSharedCollector()
	mc.AddP2PBytesIn(1024)
}

func TestAddP2PBytesOut(t *testing.T) {
	mc := getSharedCollector()
	mc.AddP2PBytesOut(2048)
}

func TestIncP2PMessagesIn(t *testing.T) {
	mc := getSharedCollector()
	mc.IncP2PMessagesIn()
}

func TestIncP2PMessagesOut(t *testing.T) {
	mc := getSharedCollector()
	mc.IncP2PMessagesOut()
}

func TestSetNodeIsSyncing(t *testing.T) {
	mc := getSharedCollector()
	mc.SetNodeIsSyncing(true)
	mc.SetNodeIsSyncing(false)
}

func TestSetMempoolPendingTxs(t *testing.T) {
	mc := getSharedCollector()
	mc.SetMempoolPendingTxs(500)
}

func TestUpdateUptime(t *testing.T) {
	mc := getSharedCollector()
	mc.UpdateUptime()
}

func TestSetHealthData(t *testing.T) {
	mc := getSharedCollector()
	mc.SetHealthData(100, 5, false)

	mc.SetHealthData(100, 5, true)
	if mc.healthData.Syncing != true {
		t.Error("expected syncing to be true")
	}
	if mc.healthData.Height != 100 {
		t.Errorf("expected height 100, got %d", mc.healthData.Height)
	}
	if mc.healthData.Peers != 5 {
		t.Errorf("expected 5 peers, got %d", mc.healthData.Peers)
	}
}

func TestHealthHandler(t *testing.T) {
	mc := getSharedCollector()
	mc.SetHealthData(100, 5, false)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mc.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHealthReadyHandler_Ready(t *testing.T) {
	mc := getSharedCollector()
	mc.SetHealthData(100, 5, false)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	mc.healthReadyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHealthReadyHandler_NotReady(t *testing.T) {
	mc := getSharedCollector()
	mc.SetHealthData(0, 0, true)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	mc.healthReadyHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestStartMetricsServer(t *testing.T) {
	mc := getSharedCollector()
	server := mc.StartMetricsServer(9997)
	if server == nil {
		t.Fatal("expected server to be created")
	}

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get("http://localhost:9997/health")
	if err != nil {
		t.Fatalf("expected server to be running, got error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	server.Close()
}

func TestMetricsEndpoint(t *testing.T) {
	mc := getSharedCollector()
	server := mc.StartMetricsServer(9996)
	defer server.Close()

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get("http://localhost:9996/metrics")
	if err != nil {
		t.Fatalf("expected metrics endpoint to be available, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestConcurrentMetricUpdates(t *testing.T) {
	mc := getSharedCollector()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				mc.SetConsensusHeight(uint64(j))
				mc.IncConsensusVotes()
				mc.AddP2PBytesIn(100)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
