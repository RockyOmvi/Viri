package accounts

import (
	"fmt"
	"time"
)

// RecoveryConfig controls social recovery settings.
type RecoveryConfig struct {
	Guardians       [][]byte // addresses of trusted recovery guardians
	GuardianThreshold uint8  // minimum guardians required to approve recovery
	TimelockDuration time.Duration // delay before recovery executes
}

// RecoveryRequest tracks a pending wallet recovery.
type RecoveryRequest struct {
	Wallet    []byte // address of the wallet being recovered
	NewKey    []byte // new authorized public key
	NewSigner []byte // new signer address
	Initiated time.Time
	Approvals map[string]bool // guardian address -> approved
	Ready     bool
	Executed  bool
}

// RecoveryManager manages social recovery of smart wallets.
type RecoveryManager struct {
	manager   *AccountManager
	requests  map[string]*RecoveryRequest
	configs   map[string]*RecoveryConfig // wallet address -> config
}

// NewRecoveryManager creates a recovery manager.
func NewRecoveryManager(manager *AccountManager) *RecoveryManager {
	return &RecoveryManager{
		manager:  manager,
		requests: make(map[string]*RecoveryRequest),
		configs:  make(map[string]*RecoveryConfig),
	}
}

// SetRecoveryConfig sets the guardians and threshold for a wallet.
func (rm *RecoveryManager) SetRecoveryConfig(wallet []byte, guardians [][]byte, threshold uint8, timelock time.Duration) {
	if threshold == 0 {
		threshold = uint8(len(guardians)/2 + 1) // default: majority
	}
	if timelock == 0 {
		timelock = 72 * time.Hour // default: 3 days
	}
	rm.configs[string(wallet)] = &RecoveryConfig{
		Guardians:         guardians,
		GuardianThreshold: threshold,
		TimelockDuration:  timelock,
	}
}

// GetRecoveryConfig returns the recovery config for a wallet.
func (rm *RecoveryManager) GetRecoveryConfig(wallet []byte) (*RecoveryConfig, bool) {
	cfg, ok := rm.configs[string(wallet)]
	return cfg, ok
}

// InitiateRecovery starts the recovery process for a wallet.
func (rm *RecoveryManager) InitiateRecovery(wallet []byte, newKey []byte, newSigner []byte) error {
	cfg, ok := rm.configs[string(wallet)]
	if !ok {
		return fmt.Errorf("no recovery config for wallet")
	}
	if len(cfg.Guardians) == 0 {
		return fmt.Errorf("no guardians configured")
	}

	key := string(wallet)
	if _, exists := rm.requests[key]; exists {
		return fmt.Errorf("recovery already in progress")
	}

	rm.requests[key] = &RecoveryRequest{
		Wallet:    wallet,
		NewKey:    newKey,
		NewSigner: newSigner,
		Initiated: time.Now(),
		Approvals: make(map[string]bool),
	}

	return nil
}

// ApproveRecovery allows a guardian to approve a pending recovery.
func (rm *RecoveryManager) ApproveRecovery(wallet []byte, guardian []byte) error {
	key := string(wallet)
	req, exists := rm.requests[key]
	if !exists {
		return fmt.Errorf("no pending recovery for wallet")
	}
	if req.Executed {
		return fmt.Errorf("recovery already executed")
	}
	if req.Ready {
		return fmt.Errorf("recovery already approved by enough guardians")
	}

	cfg, ok := rm.configs[key]
	if !ok {
		return fmt.Errorf("no recovery config")
	}

	// Verify guardian is authorized
	isGuardian := false
	for _, g := range cfg.Guardians {
		if string(g) == string(guardian) {
			isGuardian = true
			break
		}
	}
	if !isGuardian {
		return fmt.Errorf("address is not a recovery guardian")
	}

	req.Approvals[string(guardian)] = true

	// Check if threshold met
	if uint8(len(req.Approvals)) >= cfg.GuardianThreshold {
		req.Ready = true
	}

	return nil
}

// ExecuteRecovery finalizes a recovery after the timelock expires.
func (rm *RecoveryManager) ExecuteRecovery(wallet []byte) error {
	key := string(wallet)
	req, exists := rm.requests[key]
	if !exists {
		return fmt.Errorf("no pending recovery")
	}
	if req.Executed {
		return fmt.Errorf("recovery already executed")
	}
	if !req.Ready {
		return fmt.Errorf("recovery not yet approved by enough guardians")
	}

	cfg, ok := rm.configs[key]
	if !ok {
		return fmt.Errorf("no recovery config")
	}

	// Check timelock
	if time.Since(req.Initiated) < cfg.TimelockDuration {
		remaining := cfg.TimelockDuration - time.Since(req.Initiated)
		return fmt.Errorf("recovery timelock not expired: %v remaining", remaining)
	}

	// Execute recovery: update wallet signers
	acc, exists := rm.manager.GetAccount(wallet)
	if !exists {
		return fmt.Errorf("wallet not found")
	}

	// Replace signers
	if len(req.NewSigner) > 0 {
		acc.Signers = [][]byte{req.NewSigner}
	} else if len(req.NewKey) > 0 {
		acc.Signers = [][]byte{req.NewKey}
	}
	acc.Threshold = 1
	rm.manager.SetAccountDirect(wallet, acc)

	req.Executed = true
	return nil
}

// CancelRecovery cancels a pending recovery.
func (rm *RecoveryManager) CancelRecovery(wallet []byte) error {
	key := string(wallet)
	if _, exists := rm.requests[key]; !exists {
		return fmt.Errorf("no pending recovery")
	}
	delete(rm.requests, key)
	return nil
}

// PendingRecovery returns the pending recovery request for a wallet, if any.
func (rm *RecoveryManager) PendingRecovery(wallet []byte) (*RecoveryRequest, bool) {
	req, ok := rm.requests[string(wallet)]
	return req, ok
}
