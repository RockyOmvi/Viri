package observability

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

type readinessState struct {
	minPeers        int
	minBlockHeight  uint64
	readyPeers      atomic.Int64
	readyHeight     atomic.Uint64
	forceReady      atomic.Bool
}

var readiness = &readinessState{}

func ConfigureReadiness(minPeers int, minBlockHeight uint64) {
	readiness.minPeers = minPeers
	readiness.minBlockHeight = minBlockHeight
}

func UpdateReadiness(peers int, height uint64) {
	readiness.readyPeers.Store(int64(peers))
	readiness.readyHeight.Store(height)
}

func ForceReady(force bool) {
	readiness.forceReady.Store(force)
}

func IsReady() bool {
	if readiness.forceReady.Load() {
		return true
	}
	peers := int(readiness.readyPeers.Load())
	height := readiness.readyHeight.Load()

	if readiness.minPeers > 0 && peers < readiness.minPeers {
		return false
	}
	if readiness.minBlockHeight > 0 && height < readiness.minBlockHeight {
		return false
	}
	return true
}

func ReadinessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsReady() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "not_ready",
				"current_peers": readiness.readyPeers.Load(),
				"min_peers":     readiness.minPeers,
				"current_height": readiness.readyHeight.Load(),
				"min_height":    readiness.minBlockHeight,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
