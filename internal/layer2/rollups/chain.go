package rollups

import (
	"fmt"
	"sync"
)

type RollupType uint8

const (
	RollupTypeOptimistic RollupType = iota
	RollupTypeZK
	RollupTypeValidium
)

type Batch struct {
	SequenceNumber uint64
	Data           []byte
	Submitter      []byte
	Timestamp      uint64
	Status         BatchStatus
}

type BatchStatus uint8

const (
	BatchStatusPending BatchStatus = iota
	BatchStatusSubmitted
	BatchStatusConfirmed
	BatchStatusChallenged
)

type RollupChain struct {
	mu           sync.RWMutex
	id           string
	rollupType   RollupType
	batches      []*Batch
	nextSeq      uint64
	challengePeriod uint64
}

func NewRollupChain(id string, rollupType RollupType, challengePeriod uint64) *RollupChain {
	return &RollupChain{
		id:              id,
		rollupType:      rollupType,
		batches:         make([]*Batch, 0),
		challengePeriod: challengePeriod,
	}
}

func (rc *RollupChain) SubmitBatch(data []byte, submitter []byte, timestamp uint64) (*Batch, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	batch := &Batch{
		SequenceNumber: rc.nextSeq,
		Data:           data,
		Submitter:      submitter,
		Timestamp:      timestamp,
		Status:         BatchStatusPending,
	}

	rc.batches = append(rc.batches, batch)
	rc.nextSeq++

	return batch, nil
}

func (rc *RollupChain) GetBatch(seq uint64) (*Batch, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	for _, batch := range rc.batches {
		if batch.SequenceNumber == seq {
			return batch, nil
		}
	}

	return nil, fmt.Errorf("batch not found")
}

func (rc *RollupChain) ChallengeBatch(seq uint64) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	for _, batch := range rc.batches {
		if batch.SequenceNumber == seq {
			batch.Status = BatchStatusChallenged
			return nil
		}
	}

	return fmt.Errorf("batch not found")
}

func (rc *RollupChain) ConfirmBatch(seq uint64) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	for _, batch := range rc.batches {
		if batch.SequenceNumber == seq {
			if batch.Status == BatchStatusChallenged {
				return fmt.Errorf("cannot confirm challenged batch")
			}
			batch.Status = BatchStatusConfirmed
			return nil
		}
	}

	return fmt.Errorf("batch not found")
}

func (rc *RollupChain) GetPendingBatches() []*Batch {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	var pending []*Batch
	for _, batch := range rc.batches {
		if batch.Status == BatchStatusPending {
			pending = append(pending, batch)
		}
	}

	return pending
}

func (rc *RollupChain) ID() string {
	return rc.id
}

func (rc *RollupChain) Type() RollupType {
	return rc.rollupType
}

func (rc *RollupChain) BatchCount() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.batches)
}
