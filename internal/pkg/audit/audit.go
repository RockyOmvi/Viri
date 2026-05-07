package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

const (
	EventTypeProposal     = "proposal"
	EventTypeVote         = "vote"
	EventTypeViewChange   = "view_change"
	EventTypeFinalize     = "finalize"
	EventTypeTimeout      = "timeout"
	EventTypeSync         = "sync"
	EventTypeValidator    = "validator"
	MaxFileSize           = 100 * 1024 * 1024
	MaxEntriesPerFile     = 1000000
	DefaultBatchSize      = 100
	DefaultFlushInterval  = 100 * time.Millisecond
)

type EventProposal struct {
	Height    uint64 `json:"height"`
	View      uint64 `json:"view"`
	Proposer  string `json:"proposer"`
	BlockHash string `json:"block_hash"`
}

type EventVote struct {
	Height    uint64 `json:"height"`
	View      uint64 `json:"view"`
	Phase     string `json:"phase"`
	Validator string `json:"validator"`
	BlockHash string `json:"block_hash"`
}

type EventViewChange struct {
	OldView uint64 `json:"old_view"`
	NewView uint64 `json:"new_view"`
	Reason  string `json:"reason"`
}

type EventFinalize struct {
	Height       uint64 `json:"height"`
	Hash         string `json:"hash"`
	Proposer     string `json:"proposer"`
	FinalityProof string `json:"finality_proof,omitempty"`
}

type EventTimeout struct {
	Height       uint64 `json:"height"`
	View         uint64 `json:"view"`
	TimeoutCount uint64 `json:"timeout_count"`
}

type EventSync struct {
	Status   string  `json:"status"`
	Height   uint64  `json:"height,omitempty"`
	Target   uint64  `json:"target,omitempty"`
	Progress float64 `json:"progress,omitempty"`
}

type EventValidator struct {
	Action    string `json:"action"`
	Validator string `json:"validator"`
	Stake     uint64 `json:"stake,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type AuditEntry struct {
	SeqNo     uint64      `json:"seq_no"`
	Timestamp  time.Time   `json:"timestamp"`
	EventType  string      `json:"event_type"`
	EventData  interface{} `json:"event_data"`
	PrevHash   []byte      `json:"prev_hash"`
	EntryHash  []byte      `json:"entry_hash"`
	Signature  []byte      `json:"signature,omitempty"`
	rawEventData []byte    `json:"-"`
}

type indexRecord struct {
	SeqNo     uint64 `json:"seq_no"`
	Offset    int64  `json:"offset"`
	FileIndex int    `json:"file_index"`
}

type AuditLogger struct {
	mu            sync.RWMutex
	writeMu       sync.Mutex
	config        *AuditConfig
	file          *os.File
	indexFile     *os.File
	currentSeq    uint64
	lastHash      []byte
	signer        interface{ Sign(data []byte) (*crypto.Signature, error) }
	writeCh       chan *AuditEntry
	batch         []*AuditEntry
	batchSize     int
	flushInterval time.Duration
	stopCh        chan struct{}
	stopped       atomic.Bool
	fileSize      int64
	entriesInFile uint64
	fileIndex     int
}

type AuditConfig struct {
	OutputPath    string
	BatchSize     int
	FlushInterval time.Duration
	Signer        interface{ Sign(data []byte) (*crypto.Signature, error) }
}

type AuditLoggerInterface interface {
	LogProposal(height, view uint64, proposer, blockHash string)
	LogVote(height, view uint64, phase, validator, blockHash string)
	LogViewChange(oldView, newView uint64, reason string)
	LogFinalize(height uint64, hash, proposer, finalityProof string)
	LogTimeout(height, view, timeoutCount uint64)
	LogSync(status string, height, target uint64, progress float64)
	LogValidator(action, validator string, stake uint64, reason string)
	VerifyAuditChain() error
	GetEntry(seq uint64) (*AuditEntry, error)
	ExportAuditLog(from, to uint64) ([]*AuditEntry, error)
	Close() error
}

func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		OutputPath:    "audit",
		BatchSize:     DefaultBatchSize,
		FlushInterval: DefaultFlushInterval,
	}
}

func NewAuditLogger(config *AuditConfig) (*AuditLogger, error) {
	if config == nil {
		config = DefaultAuditConfig()
	}

	if config.BatchSize <= 0 {
		config.BatchSize = DefaultBatchSize
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = DefaultFlushInterval
	}

	if err := os.MkdirAll(config.OutputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}

	al := &AuditLogger{
		config:        config,
		signer:        config.Signer,
		writeCh:       make(chan *AuditEntry, 1000),
		batch:         make([]*AuditEntry, 0, config.BatchSize),
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		stopCh:        make(chan struct{}),
		lastHash:      make([]byte, 32),
	}

	if err := al.openFiles(); err != nil {
		return nil, err
	}

	go al.writeLoop()

	return al, nil
}

func (al *AuditLogger) openFiles() error {
	pattern := filepath.Join(al.config.OutputPath, "audit_*.log")
	files, _ := filepath.Glob(pattern)

	if len(files) > 0 {
		var latest string
		var maxIndex int
		for _, f := range files {
			var idx int
			fmt.Sscanf(filepath.Base(f), "audit_%d.log", &idx)
			if idx > maxIndex {
				maxIndex = idx
				latest = f
			}
		}
		al.fileIndex = maxIndex

		f, err := os.OpenFile(latest, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open audit file: %w", err)
		}
		al.file = f
		al.fileIndex = maxIndex

		info, err := f.Stat()
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to stat audit file: %w", err)
		}
		al.fileSize = info.Size()

		if err := al.recoverState(); err != nil {
			f.Close()
			return fmt.Errorf("failed to recover state: %w", err)
		}
	} else {
		if err := al.rotateFile(); err != nil {
			return err
		}
	}

	indexFilePath := filepath.Join(al.config.OutputPath, "audit.index")
	idxFile, err := os.OpenFile(indexFilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		if al.file != nil {
			al.file.Close()
		}
		return fmt.Errorf("failed to open index file: %w", err)
	}
	al.indexFile = idxFile

	return nil
}

func (al *AuditLogger) recoverState() error {
	al.currentSeq = 0
	al.lastHash = make([]byte, 32)

	info, err := al.file.Stat()
	if err != nil {
		return err
	}

	if info.Size() == 0 {
		return nil
	}

	_, err = al.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(al.file)
	for decoder.More() {
		var rawEntry json.RawMessage
		if err := decoder.Decode(&rawEntry); err != nil {
			break
		}
		al.entriesInFile++

		var seqData struct {
			SeqNo    uint64 `json:"seq_no"`
			EntryHash []byte `json:"entry_hash"`
			EventData json.RawMessage `json:"event_data"`
		}
		if err := json.Unmarshal(rawEntry, &seqData); err != nil {
			break
		}

		var entry AuditEntry
		if err := json.Unmarshal(rawEntry, &entry); err != nil {
			break
		}
		entry.rawEventData = seqData.EventData

		al.currentSeq = entry.SeqNo
		al.lastHash = entry.EntryHash
	}

	return nil
}

func (al *AuditLogger) rotateFile() error {
	if al.file != nil {
		al.file.Close()
	}

	al.fileIndex++
	auditPath := filepath.Join(al.config.OutputPath, fmt.Sprintf("audit_%d.log", al.fileIndex))

	f, err := os.OpenFile(auditPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create audit file: %w", err)
	}

	al.file = f
	al.fileSize = 0
	al.entriesInFile = 0

	return nil
}

func (al *AuditLogger) writeLoop() {
	ticker := time.NewTicker(al.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-al.writeCh:
			al.batch = append(al.batch, entry)
			if len(al.batch) >= al.batchSize {
				al.flushBatch()
			}
		case <-ticker.C:
			if len(al.batch) > 0 {
				al.flushBatch()
			}
		case <-al.stopCh:
			if len(al.batch) > 0 {
				al.flushBatch()
			}
			return
		}
	}
}

func (al *AuditLogger) flushBatch() {
	al.writeMu.Lock()
	defer al.writeMu.Unlock()

	for _, entry := range al.batch {
		al.writeEntryLocked(entry)
	}
	al.batch = al.batch[:0]
}

func (al *AuditLogger) writeEntryLocked(entry *AuditEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	n, err := al.file.Write(append(data, '\n'))
	if err != nil {
		return
	}

	al.fileSize += int64(n)
	al.entriesInFile++

	idxRec := indexRecord{
		SeqNo:     entry.SeqNo,
		Offset:    al.fileSize - int64(n),
		FileIndex: al.fileIndex,
	}
	idxData, _ := json.Marshal(idxRec)
	al.indexFile.Write(append(idxData, '\n'))

	if al.fileSize >= MaxFileSize || al.entriesInFile >= MaxEntriesPerFile {
		al.rotateFile()
	}
}

func (al *AuditLogger) createEntry(eventType string, eventData interface{}) *AuditEntry {
	al.mu.Lock()
	seqNo := al.currentSeq + 1
	al.mu.Unlock()

	rawEventData, _ := json.Marshal(eventData)

	entry := &AuditEntry{
		SeqNo:     seqNo,
		Timestamp:  time.Now().UTC(),
		EventType:  eventType,
		EventData:  eventData,
		PrevHash:   al.lastHash,
		rawEventData: rawEventData,
	}

	entry.EntryHash = al.computeHash(entry)

	if al.signer != nil {
		sig, err := al.signer.Sign(entry.EntryHash)
		if err == nil && sig != nil {
			entry.Signature = sig.Bytes()
		}
	}

	return entry
}

func (al *AuditLogger) computeHash(entry *AuditEntry) []byte {
	h := sha256.New()

	binary.BigEndian.PutUint64(entrySeqBuf[:], entry.SeqNo)
	h.Write(entrySeqBuf[:])

	ts, _ := entry.Timestamp.MarshalBinary()
	h.Write(ts)

	h.Write([]byte(entry.EventType))

	if len(entry.rawEventData) > 0 {
		h.Write(entry.rawEventData)
	} else {
		data, _ := json.Marshal(entry.EventData)
		h.Write(data)
	}

	h.Write(entry.PrevHash)

	return h.Sum(nil)
}

var entrySeqBuf [8]byte

func (al *AuditLogger) LogProposal(height, view uint64, proposer, blockHash string) {
	event := EventProposal{
		Height:    height,
		View:      view,
		Proposer:  proposer,
		BlockHash: blockHash,
	}

	entry := al.createEntry(EventTypeProposal, event)

	al.mu.Lock()
	al.currentSeq = entry.SeqNo
	al.lastHash = entry.EntryHash
	al.mu.Unlock()

	select {
	case al.writeCh <- entry:
	default:
	}
}

func (al *AuditLogger) LogVote(height, view uint64, phase, validator, blockHash string) {
	event := EventVote{
		Height:    height,
		View:      view,
		Phase:     phase,
		Validator: validator,
		BlockHash: blockHash,
	}

	entry := al.createEntry(EventTypeVote, event)

	al.mu.Lock()
	al.currentSeq = entry.SeqNo
	al.lastHash = entry.EntryHash
	al.mu.Unlock()

	select {
	case al.writeCh <- entry:
	default:
	}
}

func (al *AuditLogger) LogViewChange(oldView, newView uint64, reason string) {
	event := EventViewChange{
		OldView: oldView,
		NewView: newView,
		Reason:  reason,
	}

	entry := al.createEntry(EventTypeViewChange, event)

	al.mu.Lock()
	al.currentSeq = entry.SeqNo
	al.lastHash = entry.EntryHash
	al.mu.Unlock()

	select {
	case al.writeCh <- entry:
	default:
	}
}

func (al *AuditLogger) LogFinalize(height uint64, hash, proposer, finalityProof string) {
	event := EventFinalize{
		Height:        height,
		Hash:          hash,
		Proposer:      proposer,
		FinalityProof: finalityProof,
	}

	entry := al.createEntry(EventTypeFinalize, event)

	al.mu.Lock()
	al.currentSeq = entry.SeqNo
	al.lastHash = entry.EntryHash
	al.mu.Unlock()

	select {
	case al.writeCh <- entry:
	default:
	}
}

func (al *AuditLogger) LogTimeout(height, view, timeoutCount uint64) {
	event := EventTimeout{
		Height:       height,
		View:         view,
		TimeoutCount: timeoutCount,
	}

	entry := al.createEntry(EventTypeTimeout, event)

	al.mu.Lock()
	al.currentSeq = entry.SeqNo
	al.lastHash = entry.EntryHash
	al.mu.Unlock()

	select {
	case al.writeCh <- entry:
	default:
	}
}

func (al *AuditLogger) LogSync(status string, height, target uint64, progress float64) {
	event := EventSync{
		Status:   status,
		Height:   height,
		Target:   target,
		Progress: progress,
	}

	entry := al.createEntry(EventTypeSync, event)

	al.mu.Lock()
	al.currentSeq = entry.SeqNo
	al.lastHash = entry.EntryHash
	al.mu.Unlock()

	select {
	case al.writeCh <- entry:
	default:
	}
}

func (al *AuditLogger) LogValidator(action, validator string, stake uint64, reason string) {
	event := EventValidator{
		Action:    action,
		Validator: validator,
		Stake:     stake,
		Reason:    reason,
	}

	entry := al.createEntry(EventTypeValidator, event)

	al.mu.Lock()
	al.currentSeq = entry.SeqNo
	al.lastHash = entry.EntryHash
	al.mu.Unlock()

	select {
	case al.writeCh <- entry:
	default:
	}
}

func (al *AuditLogger) VerifyAuditChain() error {
	al.mu.RLock()
	defer al.mu.RUnlock()

	pattern := filepath.Join(al.config.OutputPath, "audit_*.log")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob audit files: %w", err)
	}

	var prevHash = make([]byte, 32)
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", f, err)
		}

		decoder := json.NewDecoder(file)
		for decoder.More() {
			var rawEntry json.RawMessage
			if err := decoder.Decode(&rawEntry); err != nil {
				file.Close()
				return fmt.Errorf("failed to decode entry: %w", err)
			}

			var entry AuditEntry
			if err := json.Unmarshal(rawEntry, &entry); err != nil {
				file.Close()
				return fmt.Errorf("failed to decode entry: %w", err)
			}

			var rawEventData struct {
				EventData json.RawMessage `json:"event_data"`
			}
			if err := json.Unmarshal(rawEntry, &rawEventData); err == nil {
				entry.rawEventData = rawEventData.EventData
			}

			expectedHash := al.computeHashForVerify(&entry)
			if !bytes.Equal(entry.EntryHash, expectedHash) {
				file.Close()
				return fmt.Errorf("hash mismatch at seq %d in %s", entry.SeqNo, f)
			}

			if !bytes.Equal(entry.PrevHash, prevHash) {
				file.Close()
				return fmt.Errorf("chain break at seq %d in %s", entry.SeqNo, f)
			}

			prevHash = entry.EntryHash
		}
		file.Close()
	}

	return nil
}

func (al *AuditLogger) computeHashForVerify(entry *AuditEntry) []byte {
	h := sha256.New()

	binary.BigEndian.PutUint64(entrySeqBuf[:], entry.SeqNo)
	h.Write(entrySeqBuf[:])

	ts, _ := entry.Timestamp.MarshalBinary()
	h.Write(ts)

	h.Write([]byte(entry.EventType))

	if len(entry.rawEventData) > 0 {
		h.Write(entry.rawEventData)
	} else {
		data, _ := json.Marshal(entry.EventData)
		h.Write(data)
	}

	h.Write(entry.PrevHash)

	return h.Sum(nil)
}

func (al *AuditLogger) GetEntry(seq uint64) (*AuditEntry, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	idxPath := filepath.Join(al.config.OutputPath, "audit.index")
	idxFile, err := os.Open(idxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	defer idxFile.Close()

	decoder := json.NewDecoder(idxFile)
	for decoder.More() {
		var rec indexRecord
		if err := decoder.Decode(&rec); err != nil {
			break
		}
		if rec.SeqNo == seq {
			return al.readEntryFromFile(rec)
		}
	}

	return nil, fmt.Errorf("entry %d not found", seq)
}

func (al *AuditLogger) readEntryFromFile(rec indexRecord) (*AuditEntry, error) {
	auditPath := filepath.Join(al.config.OutputPath, fmt.Sprintf("audit_%d.log", rec.FileIndex))
	file, err := os.Open(auditPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit file: %w", err)
	}
	defer file.Close()

	_, err = file.Seek(rec.Offset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	decoder := json.NewDecoder(file)
	var entry AuditEntry
	if err := decoder.Decode(&entry); err != nil {
		return nil, fmt.Errorf("failed to decode entry: %w", err)
	}

	return &entry, nil
}

func (al *AuditLogger) ExportAuditLog(from, to uint64) ([]*AuditEntry, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	pattern := filepath.Join(al.config.OutputPath, "audit_*.log")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob audit files: %w", err)
	}

	entries := make([]*AuditEntry, 0)
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			continue
		}

		decoder := json.NewDecoder(file)
		for decoder.More() {
			var entry AuditEntry
			if err := decoder.Decode(&entry); err != nil {
				break
			}
			if entry.SeqNo >= from && entry.SeqNo <= to {
				entries = append(entries, &entry)
			}
		}
		file.Close()
	}

	return entries, nil
}

func (al *AuditLogger) Close() error {
	if !al.stopped.CompareAndSwap(false, true) {
		return nil
	}

	close(al.stopCh)

	al.writeMu.Lock()
	defer al.writeMu.Unlock()

	if al.file != nil {
		al.file.Close()
	}
	if al.indexFile != nil {
		al.indexFile.Close()
	}

	return nil
}
