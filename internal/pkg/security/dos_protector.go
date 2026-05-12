package security

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

type CircuitBreaker struct {
	state         CircuitBreakerState
	failCount     int32
	failThreshold int32
	resetTimeout  time.Duration
	lastFail      time.Time
	mu            sync.Mutex
}

func NewCircuitBreaker(threshold int32, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:         CircuitClosed,
		failThreshold: threshold,
		resetTimeout:  resetTimeout,
	}
}

func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	now := time.Now()
	if cb.state == CircuitOpen {
		if now.Sub(cb.lastFail) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failCount = 0
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	atomic.AddInt32(&cb.failCount, 1)
	cb.lastFail = time.Now()
	if cb.failCount >= cb.failThreshold {
		cb.state = CircuitOpen
	}
}

func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == CircuitOpen
}

type IPRateData struct {
	connections []time.Time
	lastClean   time.Time
}

type DoSProtectorConfig struct {
	MaxConnsPerIPPerSec    int
	MaxTotalConnsPerSec    int
	MaxMemoryBytes         int64
	CircuitBreakerThreshold int32
	CircuitBreakerTimeout   time.Duration
}

func DefaultDoSProtectorConfig() *DoSProtectorConfig {
	return &DoSProtectorConfig{
		MaxConnsPerIPPerSec:    5,
		MaxTotalConnsPerSec:    50,
		MaxMemoryBytes:         100 << 20,
		CircuitBreakerThreshold: 50,
		CircuitBreakerTimeout:   1 * time.Minute,
	}
}

type connectionRecord struct {
	peer  peer.ID
	ip    string
	time  time.Time
}

type DoSProtector struct {
	config            *DoSProtectorConfig
	ipData            map[string]*IPRateData
	ipDataMu          sync.RWMutex
	totalConns        []time.Time
	totalConnsMu      sync.Mutex
	memoryUsed        int64
	circuitBreaker    *CircuitBreaker
	emergencyShutdown func()
	shutdownActive    int32
	connRecords       []connectionRecord
	connRecordsMu     sync.Mutex
}

func NewDoSProtector(config *DoSProtectorConfig, onEmergencyShutdown func()) *DoSProtector {
	if config == nil {
		config = DefaultDoSProtectorConfig()
	}
	return &DoSProtector{
		config:            config,
		ipData:            make(map[string]*IPRateData),
		circuitBreaker:    NewCircuitBreaker(config.CircuitBreakerThreshold, config.CircuitBreakerTimeout),
		emergencyShutdown: onEmergencyShutdown,
		connRecords:       make([]connectionRecord, 0, 100),
	}
}

func (d *DoSProtector) AllowConnection(ip string) bool {
	if !d.circuitBreaker.AllowRequest() {
		return false
	}

	d.ipDataMu.Lock()
	data, exists := d.ipData[ip]
	if !exists {
		data = &IPRateData{
			connections: make([]time.Time, 0, d.config.MaxConnsPerIPPerSec),
			lastClean:   time.Now(),
		}
		d.ipData[ip] = data
	}
	d.ipDataMu.Unlock()

	data.connections = append(data.connections, time.Now())

	now := time.Now()
	valid := make([]time.Time, 0, len(data.connections))
	for _, t := range data.connections {
		if now.Sub(t) < time.Second {
			valid = append(valid, t)
		}
	}
	data.connections = valid

	if len(valid) > d.config.MaxConnsPerIPPerSec {
		d.circuitBreaker.RecordFailure()
		return false
	}

	d.totalConnsMu.Lock()
	d.totalConns = append(d.totalConns, now)
	validTotal := make([]time.Time, 0, len(d.totalConns))
	for _, t := range d.totalConns {
		if now.Sub(t) < time.Second {
			validTotal = append(validTotal, t)
		}
	}
	d.totalConns = validTotal
	d.totalConnsMu.Unlock()

	if len(validTotal) > d.config.MaxTotalConnsPerSec {
		d.circuitBreaker.RecordFailure()
		return false
	}

	d.circuitBreaker.RecordSuccess()
	return true
}

func (d *DoSProtector) RecordConnection(peerID peer.ID, ip string) {
	d.connRecordsMu.Lock()
	d.connRecords = append(d.connRecords, connectionRecord{
		peer: peerID,
		ip:   ip,
		time: time.Now(),
	})
	if len(d.connRecords) > 1000 {
		d.connRecords = d.connRecords[100:]
	}
	d.connRecordsMu.Unlock()
}

func (d *DoSProtector) AllocateMemory(size int) bool {
	if size <= 0 {
		return true
	}
	current := atomic.LoadInt64(&d.memoryUsed)
	if current+int64(size) > d.config.MaxMemoryBytes {
		d.circuitBreaker.RecordFailure()
		if d.circuitBreaker.IsOpen() {
			d.triggerEmergencyShutdown()
		}
		return false
	}
	atomic.AddInt64(&d.memoryUsed, int64(size))
	return true
}

func (d *DoSProtector) ReleaseMemory(size int) {
	if size > 0 {
		atomic.AddInt64(&d.memoryUsed, -int64(size))
	}
}

func (d *DoSProtector) MemoryUsed() int64 {
	return atomic.LoadInt64(&d.memoryUsed)
}

func (d *DoSProtector) triggerEmergencyShutdown() {
	if !atomic.CompareAndSwapInt32(&d.shutdownActive, 0, 1) {
		return
	}
	if d.emergencyShutdown != nil {
		go d.emergencyShutdown()
	}
}

// Restart resets the DoS protector state and allows new connections after emergency shutdown
func (d *DoSProtector) Restart() {
	d.ipDataMu.Lock()
	d.ipData = make(map[string]*IPRateData)
	d.totalConns = nil
	d.ipDataMu.Unlock()

	d.totalConnsMu.Lock()
	d.totalConns = nil
	d.totalConnsMu.Unlock()

	d.connRecordsMu.Lock()
	d.connRecords = make([]connectionRecord, 0, 100)
	d.connRecordsMu.Unlock()

	d.circuitBreaker = NewCircuitBreaker(d.config.CircuitBreakerThreshold, d.config.CircuitBreakerTimeout)
	atomic.StoreInt32(&d.shutdownActive, 0)
}

func (d *DoSProtector) IsUnderAttack() bool {
	return d.circuitBreaker.IsOpen()
}

func (d *DoSProtector) RecordAttack() {
	d.circuitBreaker.RecordFailure()
	if d.circuitBreaker.IsOpen() {
		d.triggerEmergencyShutdown()
	}
}

func (d *DoSProtector) SetEmergencyHandler(handler func()) {
	d.emergencyShutdown = handler
}

func (d *DoSProtector) Cleanup() {
	now := time.Now()

	d.ipDataMu.Lock()
	for ip, data := range d.ipData {
		valid := make([]time.Time, 0, len(data.connections))
		for _, t := range data.connections {
			if now.Sub(t) < time.Minute {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(d.ipData, ip)
		} else {
			data.connections = valid
		}
	}
	d.ipDataMu.Unlock()

	d.totalConnsMu.Lock()
	valid := make([]time.Time, 0, len(d.totalConns))
	for _, t := range d.totalConns {
		if now.Sub(t) < time.Minute {
			valid = append(valid, t)
		}
	}
	d.totalConns = valid
	d.totalConnsMu.Unlock()

	d.connRecordsMu.Lock()
	validRec := make([]connectionRecord, 0, len(d.connRecords))
	for _, r := range d.connRecords {
		if now.Sub(r.time) < time.Minute {
			validRec = append(validRec, r)
		}
	}
	d.connRecords = validRec
	d.connRecordsMu.Unlock()
}
