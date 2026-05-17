package jepsen

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Config struct {
	Endpoints    []string
	ClientCount  int
	OpsPerClient int
	NemesisFreq  int
	TestDuration time.Duration
}

type Worker struct {
	id      int
	client  *Client
	history *History
	cfg     Config
	gen     *WorkloadGen
}

type WorkloadGen struct {
	mu      sync.Mutex
	nonces  map[string]uint64
}

func NewWorkloadGen() *WorkloadGen {
	return &WorkloadGen{nonces: make(map[string]uint64)}
}

type TestResult struct {
	History      []Operation
	CheckResults []CheckerResult
	Duration     time.Duration
	Faults       []string
}

func RunTest(ctx context.Context, cfg Config) (*TestResult, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("at least one endpoint required")
	}
	if cfg.ClientCount == 0 {
		cfg.ClientCount = 3
	}
	if cfg.OpsPerClient == 0 {
		cfg.OpsPerClient = 10
	}
	if cfg.NemesisFreq == 0 {
		cfg.NemesisFreq = 5
	}
	if cfg.TestDuration == 0 {
		cfg.TestDuration = 60 * time.Second
	}

	history := NewHistory()
	gen := NewWorkloadGen()
	nemesis := NewRandomNemesis(NewDockerNemesis("../.."))

	var faults []string
	var wg sync.WaitGroup
	faultCh := make(chan string, 100)

	workerCtx, cancelWorkers := context.WithTimeout(ctx, cfg.TestDuration)
	defer cancelWorkers()

	for i := 0; i < cfg.ClientCount; i++ {
		wg.Add(1)
		endpoint := cfg.Endpoints[i%len(cfg.Endpoints)]
		worker := &Worker{
			id:      i,
			client:  NewClient(endpoint),
			history: history,
			cfg:     cfg,
			gen:     gen,
		}
		go func() {
			defer wg.Done()
			worker.Run(workerCtx, faultCh)
		}()
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				faultDesc := nemesis.Inject(workerCtx)
				faults = append(faults, faultDesc)
				history.Record(OpInvoke, faultDesc, OpInfo, -1)
			}
		}
	}()

	wg.Wait()
	close(faultCh)
	for f := range faultCh {
		faults = append(faults, f)
	}

	allOps := history.Ops()

	checker := NewSafetyChecker(history, cfg.Endpoints)
	checkResults := checker.Check(allOps)

	return &TestResult{
		History:      allOps,
		CheckResults: checkResults,
		Duration:     cfg.TestDuration,
		Faults:       faults,
	}, nil
}

func (w *Worker) Run(ctx context.Context, faultCh chan<- string) {
	ops := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if ops >= w.cfg.OpsPerClient {
			return
		}
		ops++

		height, err := w.client.BlockNumber()
		if err != nil {
			w.history.Record(OpCheckState,
				map[string]interface{}{"error": err.Error(), "height": uint64(0)},
				OpFail, w.id)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		w.history.Record(OpCheckState,
			map[string]interface{}{"height": height, "node": w.cfg.Endpoints[w.id%len(w.cfg.Endpoints)]},
			OpOk, w.id)

		time.Sleep(time.Duration(100+rand.Intn(400)) * time.Millisecond)
	}
}
