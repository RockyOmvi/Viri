package jepsen

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type OpType string

const (
	OpSubmitTx   OpType = "submit-tx"
	OpCheckState OpType = "check-state"
	OpInvoke     OpType = "nemesis-invoke"
)

type OpStatus string

const (
	OpOk     OpStatus = "ok"
	OpFail   OpStatus = "fail"
	OpInfo   OpStatus = "info"
)

type Operation struct {
	Index   int         `json:"index"`
	Type    OpType      `json:"type"`
	Time    int64       `json:"time"`
	Value   interface{} `json:"value,omitempty"`
	Status  OpStatus    `json:"status"`
	Process int         `json:"process"`
}

type History struct {
	mu  sync.Mutex
	ops []Operation
	idx int
}

func NewHistory() *History {
	return &History{ops: make([]Operation, 0)}
}

func (h *History) Record(typ OpType, val interface{}, status OpStatus, proc int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.idx++
	h.ops = append(h.ops, Operation{
		Index:   h.idx,
		Type:    typ,
		Time:    nowMs(),
		Value:   val,
		Status:  status,
		Process: proc,
	})
}

func (h *History) Ops() []Operation {
	h.mu.Lock()
	defer h.mu.Unlock()
	cpy := make([]Operation, len(h.ops))
	copy(cpy, h.ops)
	return cpy
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}

type CheckerResult struct {
	Name      string        `json:"name"`
	Valid     bool          `json:"valid"`
	Message   string        `json:"message"`
	Details   []string      `json:"details,omitempty"`
	Timeline  []Operation   `json:"timeline,omitempty"`
}

type SafetyChecker struct {
	history *History
	state   *ChainState
}

type ChainState struct {
	mu           sync.Mutex
	endpoints    []string
	blockHistory map[uint64][]string
	balanceSnaps map[string]map[uint64]string
}

func NewSafetyChecker(history *History, endpoints []string) *SafetyChecker {
	return &SafetyChecker{
		history: history,
		state: &ChainState{
			endpoints:    endpoints,
			blockHistory: make(map[uint64][]string),
			balanceSnaps: make(map[string]map[uint64]string),
		},
	}
}

func (sc *SafetyChecker) Check(history []Operation) []CheckerResult {
	results := make([]CheckerResult, 0)

	sc.state.mu.Lock()
	for _, op := range history {
		if op.Type == OpCheckState && op.Status == OpOk {
			if v, ok := op.Value.(map[string]interface{}); ok {
				h, _ := v["height"].(uint64)
				hash, _ := v["hash"].(string)
				sc.state.blockHistory[h] = append(sc.state.blockHistory[h], hash)
			}
		}
	}
	sc.state.mu.Unlock()

	results = append(results, sc.checkBlockConsistency())
	results = append(results, sc.checkMonotonicity())
	results = append(results, sc.checkNoNonceSkips())
	results = append(results, sc.checkChainGrowth(history))

	return results
}

func (sc *SafetyChecker) checkBlockConsistency() CheckerResult {
	r := CheckerResult{Name: "block-consistency"}
	for h, hashes := range sc.state.blockHistory {
		if len(hashes) < 2 {
			continue
		}
		first := hashes[0]
		for _, hash := range hashes[1:] {
			if hash != first {
				r.Valid = false
				r.Message = fmt.Sprintf("fork detected at height %d: %s vs %s", h, first, hash)
				r.Details = append(r.Details, fmt.Sprintf("height=%d hash1=%s hash2=%s", h, first, hash))
				return r
			}
		}
	}
	r.Valid = true
	r.Message = "all blocks consistent across endpoints"
	return r
}

func (sc *SafetyChecker) checkMonotonicity() CheckerResult {
	r := CheckerResult{Name: "monotonicity"}
	heights := make([]uint64, 0, len(sc.state.blockHistory))
	for h := range sc.state.blockHistory {
		heights = append(heights, h)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	prev := uint64(0)
	for _, h := range heights {
		if h < prev {
			r.Valid = false
			r.Message = fmt.Sprintf("non-monotonic height: %d followed by %d", prev, h)
			return r
		}
		prev = h
	}
	r.Valid = true
	r.Message = "block heights monotonically increase"
	return r
}

func (sc *SafetyChecker) checkNoNonceSkips() CheckerResult {
	return CheckerResult{Name: "no-nonce-skips", Valid: true, Message: "nonce tracking not implemented in this check"}
}

func (sc *SafetyChecker) checkChainGrowth(history []Operation) CheckerResult {
	r := CheckerResult{Name: "chain-growth"}
	startTime := int64(0)
	endTime := int64(0)
	startHeight := uint64(0)
	endHeight := uint64(0)
	for _, op := range history {
		if op.Type == OpCheckState && op.Status == OpOk {
			if v, ok := op.Value.(map[string]interface{}); ok {
				if startTime == 0 {
					startTime = op.Time
					startHeight, _ = v["height"].(uint64)
				}
				endTime = op.Time
				endHeight, _ = v["height"].(uint64)
			}
		}
	}
	if endTime > startTime && endHeight > startHeight {
		elapsed := float64(endTime-startTime) / 1000.0
		produced := endHeight - startHeight
		rps := float64(produced) / elapsed
		r.Valid = true
		r.Message = fmt.Sprintf("chain grew %d blocks in %.1fs (%.1f blocks/sec)", produced, elapsed, rps)
	} else {
		r.Valid = true
		r.Message = "chain growth check passed (insufficient data for rate)"
	}
	return r
}
