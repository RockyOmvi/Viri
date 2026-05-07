package da

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

type DataBlob struct {
	Data     []byte
	Hash     []byte
	Submitter []byte
	Timestamp uint64
	Size     uint32
}

type DASample struct {
	BlobHash []byte
	Index    uint32
	Data     []byte
	Proof    []byte
}

type DataAvailabilityLayer struct {
	mu    sync.RWMutex
	blobs map[string]*DataBlob
	index map[string][]string
}

func NewDataAvailabilityLayer() *DataAvailabilityLayer {
	return &DataAvailabilityLayer{
		blobs: make(map[string]*DataBlob),
		index: make(map[string][]string),
	}
}

func (dal *DataAvailabilityLayer) SubmitBlob(data []byte, submitter []byte, timestamp uint64) (*DataBlob, error) {
	dal.mu.Lock()
	defer dal.mu.Unlock()

	hash := sha256.Sum256(data)
	hashStr := string(hash[:])

	if _, exists := dal.blobs[hashStr]; exists {
		return nil, fmt.Errorf("blob already exists")
	}

	blob := &DataBlob{
		Data:      data,
		Hash:      hash[:],
		Submitter: submitter,
		Timestamp: timestamp,
		Size:      uint32(len(data)),
	}

	dal.blobs[hashStr] = blob

	submitterKey := string(submitter)
	dal.index[submitterKey] = append(dal.index[submitterKey], hashStr)

	return blob, nil
}

func (dal *DataAvailabilityLayer) GetBlob(hash []byte) (*DataBlob, bool) {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	blob, exists := dal.blobs[string(hash)]
	if !exists {
		return nil, false
	}

	return blob, true
}

func (dal *DataAvailabilityLayer) VerifyAvailability(hash []byte) bool {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	_, exists := dal.blobs[string(hash)]
	return exists
}

func (dal *DataAvailabilityLayer) GetSamples(hash []byte, count uint32) ([]*DASample, error) {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	blob, exists := dal.blobs[string(hash)]
	if !exists {
		return nil, fmt.Errorf("blob not found")
	}

	if count == 0 || count > uint32(len(blob.Data)) {
		count = uint32(len(blob.Data))
	}

	samples := make([]*DASample, 0, count)
	for i := uint32(0); i < count && i < uint32(len(blob.Data)); i++ {
		samples = append(samples, &DASample{
			BlobHash: hash,
			Index:    i,
			Data:     []byte{blob.Data[i]},
			Proof:    nil,
		})
	}

	return samples, nil
}

func (dal *DataAvailabilityLayer) GetSubmittersBlobs(submitter []byte) []*DataBlob {
	dal.mu.RLock()
	defer dal.mu.RUnlock()

	key := string(submitter)
	hashes, exists := dal.index[key]
	if !exists {
		return nil
	}

	blobs := make([]*DataBlob, 0, len(hashes))
	for _, hashStr := range hashes {
		if blob, exists := dal.blobs[hashStr]; exists {
			blobs = append(blobs, blob)
		}
	}

	return blobs
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
		total += uint64(blob.Size)
	}

	return total
}
