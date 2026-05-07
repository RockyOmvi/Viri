package appchain

import "testing"

func TestCreateAppChain(t *testing.T) {
	mgr := NewAppChainManager()
	config := AppChainConfig{
		ChainID:       "c1",
		Name:          "chain",
		Type:          AppChainTypeGaming,
		Owner:         []byte("owner"),
		MaxValidators: 2,
		Validators: []ValidatorConfig{
			{Address: []byte("v1")},
		},
	}

	chain, err := mgr.CreateAppChain(config)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if chain.Status != AppChainStatusActive {
		t.Fatalf("expected active")
	}
	if mgr.ChainCount() != 1 {
		t.Fatalf("expected count 1")
	}

	if _, err := mgr.CreateAppChain(config); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestCreateAppChainValidatorLimit(t *testing.T) {
	mgr := NewAppChainManager()
	config := AppChainConfig{
		ChainID:       "c1",
		Name:          "chain",
		Owner:         []byte("owner"),
		MaxValidators: 1,
		Validators: []ValidatorConfig{
			{Address: []byte("v1")},
			{Address: []byte("v2")},
		},
	}
	if _, err := mgr.CreateAppChain(config); err == nil {
		t.Fatalf("expected validator limit error")
	}
}

func TestValidatorsLifecycle(t *testing.T) {
	mgr := NewAppChainManager()
	config := AppChainConfig{ChainID: "c1", Owner: []byte("o"), MaxValidators: 1}
	_, _ = mgr.CreateAppChain(config)

	if err := mgr.AddValidator("missing", ValidatorConfig{}); err == nil {
		t.Fatalf("expected missing chain error")
	}

	if err := mgr.AddValidator("c1", ValidatorConfig{Address: []byte("v1")}); err != nil {
		t.Fatalf("add validator failed: %v", err)
	}
	if err := mgr.AddValidator("c1", ValidatorConfig{Address: []byte("v2")}); err == nil {
		t.Fatalf("expected max validators error")
	}

	if err := mgr.RemoveValidator("c1", []byte("missing")); err == nil {
		t.Fatalf("expected validator not found")
	}
	if err := mgr.RemoveValidator("c1", []byte("v1")); err != nil {
		t.Fatalf("remove validator failed: %v", err)
	}
}

func TestPauseResume(t *testing.T) {
	mgr := NewAppChainManager()
	config := AppChainConfig{ChainID: "c1", Owner: []byte("o"), MaxValidators: 1}
	_, _ = mgr.CreateAppChain(config)

	if err := mgr.PauseChain("missing"); err == nil {
		t.Fatalf("expected missing chain error")
	}
	if err := mgr.PauseChain("c1"); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	if err := mgr.ResumeChain("c1"); err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if err := mgr.ResumeChain("c1"); err == nil {
		t.Fatalf("expected not paused error")
	}
}

func TestQueries(t *testing.T) {
	mgr := NewAppChainManager()
	config := AppChainConfig{ChainID: "c1", Owner: []byte("o"), MaxValidators: 1}
	_, _ = mgr.CreateAppChain(config)

	if _, ok := mgr.GetAppChain("missing"); ok {
		t.Fatalf("unexpected chain")
	}

	if len(mgr.GetOwnerChains([]byte("o"))) != 1 {
		t.Fatalf("expected owner chain")
	}
	if len(mgr.GetOwnerChains([]byte("missing"))) != 0 {
		t.Fatalf("expected empty owner chains")
	}

	if len(mgr.GetActiveChains()) != 1 {
		t.Fatalf("expected active chain")
	}
}
