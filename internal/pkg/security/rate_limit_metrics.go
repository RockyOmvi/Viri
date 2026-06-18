package security

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	rpcRateLimitHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "viri",
			Subsystem: "rpc",
			Name:      "rate_limit_hits_total",
			Help:      "RPC rate limit denials by JSON-RPC method.",
		},
		[]string{"method"},
	)
	rpcRateLimitThrottledTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "viri",
			Subsystem: "rpc",
			Name:      "rate_limit_throttled_total",
			Help:      "HTTP 429 responses from rate limit middleware.",
		},
		[]string{"limiter"},
	)
	rpcRateLimitTokenRatio = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "viri",
			Subsystem: "rpc",
			Name:      "rate_limit_token_ratio",
			Help:      "Average token fill ratio (tokens/capacity) across active client buckets.",
		},
		[]string{"limiter"},
	)
)

func init() {
	prometheus.MustRegister(rpcRateLimitHitsTotal, rpcRateLimitThrottledTotal, rpcRateLimitTokenRatio)
}

func recordRateLimitHit(method string) {
	if method == "" {
		method = "unknown"
	}
	rpcRateLimitHitsTotal.WithLabelValues(method).Inc()
}

func recordThrottled(limiter string) {
	rpcRateLimitThrottledTotal.WithLabelValues(limiter).Inc()
}

// ExportRateLimitMetrics updates token-bucket fill gauges for Prometheus scraping.
func ExportRateLimitMetrics(global *RateLimiter, mrl *MethodRateLimiter) {
	if global != nil {
		rpcRateLimitTokenRatio.WithLabelValues("client").Set(global.avgTokenRatio())
	}
	if mrl == nil {
		return
	}

	mrl.mu.RLock()
	defer mrl.mu.RUnlock()

	rpcRateLimitTokenRatio.WithLabelValues("default").Set(mrl.defaultLimiter.avgTokenRatio())
	for method, limiter := range mrl.methodLimiters {
		rpcRateLimitTokenRatio.WithLabelValues(method).Set(limiter.avgTokenRatio())
	}
}

func (rl *RateLimiter) avgTokenRatio() float64 {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if len(rl.clientBuckets) == 0 {
		return 1
	}

	var sum float64
	for _, bucket := range rl.clientBuckets {
		if bucket.capacity <= 0 {
			continue
		}
		sum += bucket.tokens / bucket.capacity
	}
	return sum / float64(len(rl.clientBuckets))
}
