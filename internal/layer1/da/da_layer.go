package da

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

const (
	DefaultMaxBlobSize = 10 * 1024 * 1024
	DefaultMaxBlobs    = 100_000
)

type DataBlob struct {
	Data      []byte
	Hash      []byte
	Submitter []byte
	Timestamp uint64
	Size      uint64
}

type DASample struct {
	BlobHash []byte
	Index    uint32
	Data     []byte
	Proof    []byte
}

type DataAvailabilityLayer struct {
	mu         sync.RWMutex
	blobs      map[string]*DataBlob
	index      map[string][]string
	maxBlobSize int64
	maxBlobs   int
}

func NewDataAvailabilityLayer() *DataAvailabilityLayer {
	return &DataAvailabilityLayer{
		blobs:      make(map[string]*DataBlob),
		index:      make(map[string][]string),
		maxBlobSize: DefaultMaxBlobSize,
		maxBlobs:   DefaultMaxBlobs,
	}
}

func NewDataAvailabilityLayerWithLimit(maxBlobSize int64, maxBlobs int) *DataAvailabilityLayer {
	if maxBlobSize <= 0 {
		maxBlobSize = DefaultMaxBlobSize
	}
	if maxBlobs <= 0 {
		maxBlobs = DefaultMaxBlobs
	}
	return &DataAvailabilityLayer{
		blobs:       make(map[string]*DataBlob),
		index:       make(map[string][]string),
		maxBlobSize: maxBlobSize,
		maxBlobs:    maxBlobs,
	}
}

func (dal *DataAvailabilityLayer) SubmitBlob(data []byte, submitter []byte, timestamp uint64) (*DataBlob, error) {
	dal.mu.Lock()
	defer dal.mu.Unlock()

	if len(data) == 0 {
		return nil, fmt.Errorf("data must not be empty")
	}
	if int64(len(data)) > dal.maxBlobSize {
		return nil, fmt.Errorf("data exceeds max blob size of %d bytes", dal.maxBlobSize)
	}
	if len(submitter) == 0 {
		return nil, fmt.Errorf("submitter must not be empty")
	}
	if len(dal.blobs) >= dal.maxBlobs {
		return nil, fmt.Errorf("blob storage full (%d max)", dal.maxBlobs)
	}

	hash := sha256.Sum256(data)
	hashStr := string(hash[:])

	if _, exists := dal.blobs[hashStr]; exists {
		return nil, fmt.Errorf("blob already exists: hash=%x", hashStr[:])
	}

	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	subCopy := make([]byte, len(submitter))
	copy(subCopy, submitter)
	hashCopy := make([]byte, len(hash[:]))
	copy(hashCopy, hash[:])

	blob := &DataBlob{
		Data:      dataCopy,
		Hash:      hashCopy,
		Submitter: subCopy,
		Timestamp: timestamp,
		Size:      uint64(len(data)),
	}

	dal.blobs[hashStr] = blob

	submitterKey := string(subCopy)
	dal.index[submitterKey] = append(dal.index[submitterKey], hashStr)

	return blob, nil
}

func (dal *DataAvailabilityLayer) GetBlob(hash []byte) (*DataBlob, bool) {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	if len(hash) == 0 {
		return nil, false
	}

	blob, exists := dal.blobs[string(hash)]
	if !exists {
		return nil, false
	}

	cp := &DataBlob{
		Data:      append([]byte(nil), blob.Data...),
		Hash:      append([]byte(nil), blob.Hash...),
		Submitter: append([]byte(nil), blob.Submitter...),
		Timestamp: blob.Timestamp,
		Size:      blob.Size,
	}
	return cp, true
}

func (dal *DataAvailabilityLayer) VerifyAvailability(hash []byte) bool {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	if len(hash) == 0 {
		return false
	}

	_, exists := dal.blobs[string(hash)]
	return exists
}

func (dal *DataAvailabilityLayer) GetSamples(hash []byte, count uint32) ([]*DASample, error) {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	if len(hash) == 0 {
		return nil, fmt.Errorf("hash must not be empty")
	}

	blob, exists := dal.blobs[string(hash)]
	if !exists {
		return nil, fmt.Errorf("blob not found")
	}

	dataLen := len(blob.Data)
	n := int(count)
	if n == 0 || n > dataLen {
		n = dataLen
	}

	samples := make([]*DASample, 0, n)
	for i := 0; i < n && i < dataLen; i++ {
		hashCopy := make([]byte, len(hash))
		copy(hashCopy, hash)
		samples = append(samples, &DASample{
			BlobHash: hashCopy,
			Index:    uint32(i),
			Data:     []byte{blob.Data[i]},
			Proof:    nil,
		})
	}

	return samples, nil
}

func (dal *DataAvailabilityLayer) GetSubmittersBlobs(submitter []byte) []*DataBlob {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	if len(submitter) == 0 {
		return nil
	}

	key := string(submitter)
	hashes, exists := dal.index[key]
	if !exists {
		return nil
	}

	blobs := make([]*DataBlob, 0, len(hashes))
	for _, hashStr := range hashes {
		if blob, exists := dal.blobs[hashStr]; exists {
			cp := &DataBlob{
				Data:      append([]byte(nil), blob.Data...),
				Hash:      append([]byte(nil), blob.Hash...),
				Submitter: append([]byte(nil), blob.Submitter...),
				Timestamp: blob.Timestamp,
				Size:      blob.Size,
			}
			blobs = append(blobs, cp)
		}
	}

	return blobs
}

func (dal *DataAvailabilityLayer) DeleteBlob(hash []byte) error {
	dal.mu.Lock()
	defer dal.mu.Unlock()

	if len(hash) == 0 {
		return fmt.Errorf("hash must not be empty")
	}

	key := string(hash)
	blob, exists := dal.blobs[key]
	if !exists {
		return fmt.Errorf("blob not found")
	}

	submitterKey := string(blob.Submitter)
	if hashes, ok := dal.index[submitterKey]; ok {
		filtered := make([]string, 0, len(hashes))
		for _, h := range hashes {
			if h != key {
				filtered = append(filtered, h)
			}
		}
		if len(filtered) == 0 {
			delete(dal.index, submitterKey)
		} else {
			dal.index[submitterKey] = filtered
		}
	}

	delete(dal.blobs, key)
	return nil
}

func (dal *DataAvailabilityLayer) Prune(target int) int {
	dal.mu.Lock()
	defer dal.mu.Unlock()

	if target < 0 {
		target = 0
	}
	if len(dal.blobs) <= target {
		return 0
	}

	toRemove := len(dal.blobs) - target
	removed := 0

	for key := range dal.blobs {
		if removed >= toRemove {
			break
		}
		delete(dal.blobs, key)
		removed++
	}

	dal.index = make(map[string][]string)
	return removed
}

func (dal *DataAvailabilityLayer) TotalBlobs() int {
	dal.mu.RLock()
	defer dal.mu.RUnlock()
	return len(dal.blobs)
}

func (dal *DataAvailabilityLayer) TotalSize() uint64 {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	var total uint64
	for _, blob := range dal.blobs {
		total += blob.Size
	}

	return total
}
