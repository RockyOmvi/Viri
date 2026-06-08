package security

import "github.com/prometheus/client_golang/prometheus"

var (
	rateLimitClients = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "viri",
			Subsystem: "rate_limit",
			Name:      "clients",
			Help:      "Current number of clients tracked by the global rate limiter.",
		},
	)
	rateLimitMethods = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "viri",
			Subsystem: "rate_limit",
			Name:      "methods",
			Help:      "Current number of method-specific rate limiters.",
		},
	)
)

func init() {
	prometheus.MustRegister(rateLimitClients, rateLimitMethods)
}

func ExportRateLimitMetrics(rateLimiter *RateLimiter, methodRateLimiter *MethodRateLimiter) {
	if rateLimiter != nil {
		rateLimiter.mu.RLock()
		rateLimitClients.Set(float64(len(rateLimiter.clientBuckets)))
		rateLimiter.mu.RUnlock()
	}

	if methodRateLimiter != nil {
		methodRateLimiter.mu.RLock()
		rateLimitMethods.Set(float64(len(methodRateLimiter.methodLimiters)))
		methodRateLimiter.mu.RUnlock()
	}
}
