package intent

import "testing"

func TestRegisterHandler(t *testing.T) {
	solver := NewIntentSolver()
	solver.RegisterHandler(IntentTypeSwap, func(intent *UserIntent) (*UserIntent, error) {
		return intent, nil
	})
	if len(solver.handlers) != 1 {
		t.Fatalf("handler not registered")
	}
}
