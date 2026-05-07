package intent

import (
	"testing"
	"time"
)

func TestSubmitAndGetIntent(t *testing.T) {
	solver := NewIntentSolver()
	intent, err := solver.SubmitIntent([]byte("user12345678"), IntentTypeSwap, []byte("in"), []byte("out"), 0.1, uint64(time.Now().Add(time.Hour).Unix()), 1)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	loaded, ok := solver.GetIntent(intent.ID)
	if !ok {
		t.Fatalf("intent not found")
	}
	if loaded.Status != IntentStatusOpen {
		t.Fatalf("expected open status")
	}

	open := solver.GetOpenIntents()
	if len(open) != 1 {
		t.Fatalf("expected 1 open intent")
	}
}

func TestRegisterSolverAndSolve(t *testing.T) {
	solver := NewIntentSolver()
	intent, _ := solver.SubmitIntent([]byte("user12345678"), IntentTypeSwap, []byte("in"), []byte("out"), 0.1, uint64(time.Now().Add(time.Hour).Unix()), 1)

	if _, err := solver.SolveIntent(intent.ID, "s1"); err == nil {
		t.Fatalf("expected solver not found")
	}

	solver.RegisterSolver("s1", []byte("solver"))
	result, err := solver.SolveIntent(intent.ID, "s1")
	if err != nil {
		t.Fatalf("solve failed: %v", err)
	}
	if result.Status != IntentStatusSolved {
		t.Fatalf("expected solved status")
	}

	if err := solver.FillIntent(intent.ID); err != nil {
		t.Fatalf("fill failed: %v", err)
	}
	filled, _ := solver.GetIntent(intent.ID)
	if filled.Status != IntentStatusFilled {
		t.Fatalf("expected filled status")
	}
}

func TestSolveInvalidStates(t *testing.T) {
	solver := NewIntentSolver()
	if _, err := solver.SolveIntent("missing", "s1"); err == nil {
		t.Fatalf("expected missing intent error")
	}
}

func TestFillErrors(t *testing.T) {
	solver := NewIntentSolver()
	if err := solver.FillIntent("missing"); err == nil {
		t.Fatalf("expected missing intent")
	}

	intent, _ := solver.SubmitIntent([]byte("user12345678"), IntentTypeSwap, []byte("in"), []byte("out"), 0.1, uint64(time.Now().Add(time.Hour).Unix()), 1)
	if err := solver.FillIntent(intent.ID); err == nil {
		t.Fatalf("expected not solved error")
	}
}

func TestHandlersAndCleanup(t *testing.T) {
	solver := NewIntentSolver()
	intent, _ := solver.SubmitIntent([]byte("user12345678"), IntentTypeSwap, []byte("in"), []byte("out"), 0.1, uint64(time.Now().Add(time.Hour).Unix()), 1)
	solver.RegisterSolver("s1", []byte("solver"))
	solver.RegisterHandler(IntentTypeSwap, func(i *UserIntent) (*UserIntent, error) {
		i.Output = []byte("out2")
		return i, nil
	})

	result, err := solver.SolveIntent(intent.ID, "s1")
	if err != nil {
		t.Fatalf("solve failed: %v", err)
	}
	if string(result.Output) != "out2" {
		t.Fatalf("handler not applied")
	}

	expired, _ := solver.SubmitIntent([]byte("user12345678"), IntentTypeSwap, []byte("in"), []byte("out"), 0.1, uint64(time.Now().Add(-time.Hour).Unix()), 1)
	if solver.CleanupExpired() == 0 {
		t.Fatalf("expected cleanup to expire intent")
	}
	loaded, _ := solver.GetIntent(expired.ID)
	if loaded.Status != IntentStatusExpired {
		t.Fatalf("expected expired status")
	}
}
