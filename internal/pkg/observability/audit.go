package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/logging"
)

type AuditLogger struct {
	mu       sync.Mutex
	logger   *logging.Logger
	file     *os.File
	encoder  *json.Encoder
	maxSize  int64
	currentSize int64
	backupCount int
}

func NewAuditLogger(logDir string, maxSizeMB int, maxBackups int) (*AuditLogger, error) {
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "audit.log")
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &AuditLogger{
		logger:      logging.NewLogger("audit", logging.INFO, "json"),
		file:        file,
		encoder:     json.NewEncoder(file),
		maxSize:     int64(maxSizeMB) * 1024 * 1024,
		backupCount: maxBackups,
	}, nil
}

type AuditEvent struct {
	Timestamp string                 `json:"timestamp"`
	Event     string                 `json:"event"`
	RequestID string                 `json:"request_id,omitempty"`
	ClientIP  string                 `json:"client_ip,omitempty"`
	Method    string                 `json:"method,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

func (al *AuditLogger) Log(event, requestID, clientIP, method string, details map[string]interface{}) {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.rotateIfNeeded()

	entry := AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Event:     event,
		RequestID: requestID,
		ClientIP:  clientIP,
		Method:    method,
		Details:   details,
	}

	if err := al.encoder.Encode(entry); err != nil {
		al.logger.WithField("error", err.Error()).Warn("Failed to write audit log")
	}
}

func (al *AuditLogger) rotateIfNeeded() {
	if al.file == nil || al.maxSize == 0 {
		return
	}

	stat, err := al.file.Stat()
	if err != nil {
		return
	}

	al.currentSize = stat.Size()
	if al.currentSize < al.maxSize {
		return
	}

	al.file.Close()

	for i := al.backupCount - 1; i > 0; i-- {
		old := fmt.Sprintf("%s.%d", al.file.Name(), i)
		new := fmt.Sprintf("%s.%d", al.file.Name(), i+1)
		os.Rename(old, new)
	}

	os.Rename(al.file.Name(), al.file.Name()+".1")

	file, err := os.OpenFile(al.file.Name(), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}

	al.file = file
	al.encoder = json.NewEncoder(file)
	al.currentSize = 0
}

func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.file != nil {
		return al.file.Close()
	}
	return nil
}

func (al *AuditLogger) LogTransaction(requestID, clientIP, txHash string) {
	al.Log("transaction_submitted", requestID, clientIP, "eth_sendRawTransaction", map[string]interface{}{
		"tx_hash": txHash,
	})
}

func (al *AuditLogger) LogConsensusChange(requestID, clientIP, action string, details map[string]interface{}) {
	al.Log("consensus_action", requestID, clientIP, "viri_consensus", details)
}

func (al *AuditLogger) LogAuthFailure(requestID, clientIP, reason string) {
	al.Log("auth_failure", requestID, clientIP, "", map[string]interface{}{
		"reason": reason,
	})
}
