package observability

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "viri",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests.",
		},
		[]string{"server", "method", "status"},
	)
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "viri",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"server", "method"},
	)
	InFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "viri",
			Subsystem: "http",
			Name:      "in_flight_requests",
			Help:      "Current in-flight HTTP requests.",
		},
		[]string{"server"},
	)
	BlockHeight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "viri",
			Subsystem: "chain",
			Name:      "block_height",
			Help:      "Current chain height.",
		},
		[]string{"server"},
	)
	PeerCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "viri",
			Subsystem: "p2p",
			Name:      "peer_count",
			Help:      "Current peer count.",
		},
		[]string{"server"},
	)
	ReadyState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "viri",
			Subsystem: "service",
			Name:      "ready",
			Help:      "Service readiness state (1=ready, 0=not ready).",
		},
		[]string{"server"},
	)
)

func init() {
	prometheus.MustRegister(RequestsTotal, RequestDuration, InFlight, BlockHeight, PeerCount, ReadyState)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func InstrumentHandler(server string, next http.Handler, update func()) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if update != nil {
			update()
		}

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		InFlight.WithLabelValues(server).Inc()
		start := time.Now()
		next.ServeHTTP(sw, r)
		duration := time.Since(start).Seconds()
		InFlight.WithLabelValues(server).Dec()

		RequestsTotal.WithLabelValues(server, r.Method, fmt.Sprintf("%d", sw.status)).Inc()
		RequestDuration.WithLabelValues(server, r.Method).Observe(duration)
	})
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func SetChainStats(server string, height uint64, peers int) {
	BlockHeight.WithLabelValues(server).Set(float64(height))
	PeerCount.WithLabelValues(server).Set(float64(peers))
}

func SetReady(server string, ready bool) {
	if ready {
		ReadyState.WithLabelValues(server).Set(1)
		return
	}
	ReadyState.WithLabelValues(server).Set(0)
}

func LocalOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.RemoteAddr
		if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			host = h
		}

		if host == "localhost" {
			next.ServeHTTP(w, r)
			return
		}

		ip := net.ParseIP(host)
		if ip != nil && ip.IsLoopback() {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "forbidden", http.StatusForbidden)
	})
}
