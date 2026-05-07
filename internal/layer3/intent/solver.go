package intent

import (
	"fmt"
	"sync"
	"time"
)

type IntentType uint8

const (
	IntentTypeSwap IntentType = iota
	IntentTypeTransfer
	IntentTypeLend
	IntentTypeBorrow
	IntentTypeComplex
)

type UserIntent struct {
	ID         string
	User       []byte
	Type       IntentType
	Input      []byte
	Output     []byte
	MaxSlippage float64
	Deadline   uint64
	Fee        uint64
	Status     IntentStatus
	Solver     []byte
	CreatedAt  time.Time
	SolvedAt   time.Time
}

type IntentStatus uint8

const (
	IntentStatusOpen IntentStatus = iota
	IntentStatusSolved
	IntentStatusFilled
	IntentStatusExpired
	IntentStatusCancelled
)

type Solver struct {
	ID          string
	Address     []byte
	Reputation  uint64
	TotalSolved uint64
	IsActive    bool
}

type IntentSolver struct {
	mu       sync.RWMutex
	intents  map[string]*UserIntent
	solvers  map[string]*Solver
	handlers map[IntentType]IntentHandler
}

type IntentHandler func(intent *UserIntent) (*UserIntent, error)

func NewIntentSolver() *IntentSolver {
	return &IntentSolver{
		intents:  make(map[string]*UserIntent),
		solvers:  make(map[string]*Solver),
		handlers: make(map[IntentType]IntentHandler),
	}
}

func (is *IntentSolver) SubmitIntent(user []byte, intentType IntentType, input, output []byte, maxSlippage float64, deadline uint64, fee uint64) (*UserIntent, error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	id := fmt.Sprintf("%x-%d", user[:8], len(is.intents))

	intent := &UserIntent{
		ID:          id,
		User:        user,
		Type:        intentType,
		Input:       input,
		Output:      output,
		MaxSlippage: maxSlippage,
		Deadline:    deadline,
		Fee:         fee,
		Status:      IntentStatusOpen,
		CreatedAt:   time.Now(),
	}

	is.intents[id] = intent
	return intent, nil
}

func (is *IntentSolver) RegisterSolver(id string, address []byte) *Solver {
	is.mu.Lock()
	defer is.mu.Unlock()

	solver := &Solver{
		ID:       id,
		Address:  address,
		IsActive: true,
	}

	is.solvers[id] = solver
	return solver
}

func (is *IntentSolver) SolveIntent(intentID, solverID string) (*UserIntent, error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	intent, exists := is.intents[intentID]
	if !exists {
		return nil, fmt.Errorf("intent not found")
	}

	if intent.Status != IntentStatusOpen {
		return nil, fmt.Errorf("intent not open")
	}

	solver, exists := is.solvers[solverID]
	if !exists {
		return nil, fmt.Errorf("solver not found")
	}

	if !solver.IsActive {
		return nil, fmt.Errorf("solver inactive")
	}

	handler, exists := is.handlers[intent.Type]
	if exists {
		result, err := handler(intent)
		if err != nil {
			return nil, err
		}
		intent = result
	}

	intent.Status = IntentStatusSolved
	intent.Solver = solver.Address
	intent.SolvedAt = time.Now()

	solver.TotalSolved++
	solver.Reputation += 10

	return intent, nil
}

func (is *IntentSolver) FillIntent(intentID string) error {
	is.mu.Lock()
	defer is.mu.Unlock()

	intent, exists := is.intents[intentID]
	if !exists {
		return fmt.Errorf("intent not found")
	}

	if intent.Status != IntentStatusSolved {
		return fmt.Errorf("intent not solved")
	}

	intent.Status = IntentStatusFilled
	return nil
}

func (is *IntentSolver) RegisterHandler(intentType IntentType, handler IntentHandler) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.handlers[intentType] = handler
}

func (is *IntentSolver) GetOpenIntents() []*UserIntent {
	is.mu.RLock()
	defer is.mu.RUnlock()

	var open []*UserIntent
	for _, intent := range is.intents {
		if intent.Status == IntentStatusOpen {
			open = append(open, intent)
		}
	}

	return open
}

func (is *IntentSolver) GetIntent(id string) (*UserIntent, bool) {
	is.mu.RLock()
	defer is.mu.RUnlock()

	intent, exists := is.intents[id]
	return intent, exists
}

func (is *IntentSolver) CleanupExpired() int {
	is.mu.Lock()
	defer is.mu.Unlock()

	now := uint64(time.Now().Unix())
	count := 0

	for _, intent := range is.intents {
		if intent.Status == IntentStatusOpen && now > intent.Deadline {
			intent.Status = IntentStatusExpired
			count++
		}
	}

	return count
}
