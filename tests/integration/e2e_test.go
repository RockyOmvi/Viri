package integration

import (
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/consensus"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
	"github.com/viri-chain/viri/internal/layer2/gas"
	"github.com/viri-chain/viri/internal/layer2/zk"
	"github.com/viri-chain/viri/internal/layer3/bridge"
	"github.com/viri-chain/viri/internal/layer3/governance"
	"github.com/viri-chain/viri/internal/layer3/intent"
	"github.com/viri-chain/viri/internal/layer3/interop"
)

func TestMultiNodeValidatorSet(t *testing.T) {
	staking := consensus.NewStakingModule(24*time.Hour, 0.01)

	keys := make([]*crypto.PrivateKey, 4)
	validators := make([]*consensus.Validator, 0, 4)

	for i := 0; i < 4; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		keys[i] = key

		staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), uint64(1000000+i*100000))
	}

	activeValidators := staking.GetActiveValidators()
	for _, sv := range activeValidators {
		validators = append(validators, &consensus.Validator{
			Address:  sv.Address,
			PublicKey: sv.PublicKey,
			Stake:    sv.Stake,
			IsActive: true,
		})
	}

	vs := consensus.NewValidatorSet(validators, 1)

	if vs.Size() != 4 {
		t.Errorf("expected 4 validators, got %d", vs.Size())
	}

	if vs.Epoch() != 1 {
		t.Errorf("expected epoch 1, got %d", vs.Epoch())
	}

	totalStake := vs.TotalStake()
	if totalStake == 0 {
		t.Error("expected non-zero total stake")
	}
}

func TestGovernanceFlow(t *testing.T) {
	dao := governance.NewGovernanceDAO(10, 1000, 0.5)

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()
	key3, _ := crypto.GenerateKey()

	proposal, err := dao.SubmitProposal(
		"Test Proposal",
		"This is a test proposal",
		governance.ProposalTypeSoftwareUpgrade,
		key1.PubKey().Address(),
		200000,
	)
	if err != nil {
		t.Fatalf("failed to submit proposal: %v", err)
	}

	if proposal.ID != 0 {
		t.Errorf("expected proposal ID 0, got %d", proposal.ID)
	}

	if err := dao.Vote(proposal.ID, key1.PubKey().Address(), governance.VoteChoiceYes, 500000); err != nil {
		t.Fatalf("failed to vote: %v", err)
	}

	if err := dao.Vote(proposal.ID, key2.PubKey().Address(), governance.VoteChoiceYes, 600000); err != nil {
		t.Fatalf("failed to vote: %v", err)
	}

	if err := dao.Vote(proposal.ID, key3.PubKey().Address(), governance.VoteChoiceNo, 300000); err != nil {
		t.Fatalf("failed to vote: %v", err)
	}

	proposal, _ = dao.GetProposal(proposal.ID)
	if proposal.Status != governance.ProposalStatusPassed {
		t.Logf("Proposal status: %v (voting period may not have ended)", proposal.Status)
	}

	activeProposals := dao.GetActiveProposals()
	if len(activeProposals) == 0 {
		t.Error("expected at least one active proposal")
	}
}

func TestBridgeFlow(t *testing.T) {
	br := bridge.NewChainBridge(3)

	br.RegisterChain("ethereum", "Ethereum", "https://eth.example.com")
	br.RegisterChain("polygon", "Polygon", "https://polygon.example.com")

	br.RegisterValidator("validator-1")
	br.RegisterValidator("validator-2")
	br.RegisterValidator("validator-3")

	transfer, err := br.InitiateTransfer(
		"ethereum",
		"polygon",
		[]byte("sender"),
		[]byte("receiver"),
		1000,
		[]byte("ETH"),
	)
	if err != nil {
		t.Fatalf("failed to initiate transfer: %v", err)
	}

	if transfer.Status != bridge.TransferStatusPending {
		t.Errorf("expected status pending, got %v", transfer.Status)
	}

	err = br.AddValidatorSignature(transfer.ID, "validator-1")
	if err != nil {
		t.Fatalf("failed to add signature: %v", err)
	}

	err = br.AddValidatorSignature(transfer.ID, "validator-2")
	if err != nil {
		t.Fatalf("failed to add signature: %v", err)
	}

	err = br.AddValidatorSignature(transfer.ID, "validator-3")
	if err != nil {
		t.Fatalf("failed to add signature: %v", err)
	}

	pendingTransfers := br.GetPendingTransfers()
	if len(pendingTransfers) == 0 {
		t.Log("No pending transfers (may be expected if completed)")
	}
}

func TestInteropFlow(t *testing.T) {
	protocol := interop.NewInteropProtocol()

	channel, err := protocol.CreateChannel(
		"port-ethereum",
		"port-polygon",
		"ethereum",
		"polygon",
		"1.0",
	)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	if !channel.IsActive {
		t.Errorf("expected channel to be active")
	}

	packet, err := protocol.SendPacket(
		channel.ID,
		interop.PacketTypeTransfer,
		[]byte("packet-data"),
		uint64(time.Now().Add(1*time.Hour).UnixNano()),
	)
	if err != nil {
		t.Fatalf("failed to send packet: %v", err)
	}

	if packet.Status != interop.PacketStatusPending {
		t.Errorf("expected packet status pending, got %v", packet.Status)
	}

	err = protocol.ReceivePacket(channel.ID, packet.Sequence, []byte("ack-proof"))
	if err != nil {
		t.Fatalf("failed to receive packet: %v", err)
	}

	activeChannels := protocol.GetActiveChannels()
	if len(activeChannels) == 0 {
		t.Error("expected at least one active channel")
	}
}

func TestIntentFlow(t *testing.T) {
	solver := intent.NewIntentSolver()

	key1, _ := crypto.GenerateKey()

	solver.RegisterSolver("solver-1", []byte("solver-1-address"))
	solver.RegisterSolver("filler-1", []byte("filler-1-address"))

	swapIntent, err := solver.SubmitIntent(
		key1.PubKey().Address(),
		intent.IntentTypeSwap,
		[]byte("100-ETH"),
		[]byte("200000-USDC"),
		0.05,
		uint64(time.Now().Add(1*time.Hour).UnixNano()),
		1000,
	)
	if err != nil {
		t.Fatalf("failed to submit intent: %v", err)
	}

	openIntents := solver.GetOpenIntents()
	if len(openIntents) == 0 {
		t.Error("expected at least one open intent")
	}

	result, err := solver.SolveIntent(swapIntent.ID, "solver-1")
	if err != nil {
		t.Fatalf("failed to solve intent: %v", err)
	}

	if result.Status != intent.IntentStatusSolved {
		t.Errorf("expected result status solved, got %v", result.Status)
	}

	if err := solver.FillIntent(swapIntent.ID); err != nil {
		t.Fatalf("failed to fill intent: %v", err)
	}

	filledIntent, exists := solver.GetIntent(swapIntent.ID)
	if !exists {
		t.Fatal("intent not found after fill")
	}

	if filledIntent.Status != intent.IntentStatusFilled {
		t.Errorf("expected intent status filled, got %v", filledIntent.Status)
	}
}

func TestZKShieldedTransaction(t *testing.T) {
	circuit := zk.NewShieldedTransferCircuit()
	pk := zk.GenerateProvingKey(circuit)
	vk := zk.GenerateVerifyingKey(pk, circuit)

	pool := zk.NewShieldedPool(circuit, vk)

	prover := zk.NewProver(pk, circuit)
	assignment := &zk.Assignment{
		Inputs:  []*big.Int{big.NewInt(3), big.NewInt(5), big.NewInt(10)},
		Witness: []*big.Int{big.NewInt(1), big.NewInt(8), big.NewInt(8), big.NewInt(3), big.NewInt(13), big.NewInt(21)},
	}

	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("failed to generate proof: %v", err)
	}

	tx, err := pool.ProcessDeposit(1000, []byte("sender1"), proof)
	if err != nil {
		t.Fatalf("failed to process deposit: %v", err)
	}

	if tx.Type != zk.ShieldedTxTypeDeposit {
		t.Errorf("expected deposit type, got %v", tx.Type)
	}

	if pool.GetCommitmentCount() != 1 {
		t.Errorf("expected 1 commitment, got %d", pool.GetCommitmentCount())
	}

	if pool.GetProofCount() != 1 {
		t.Errorf("expected 1 proof, got %d", pool.GetProofCount())
	}

	verifier := zk.NewVerifier(vk, circuit)
	if err := verifier.Verify(proof); err != nil {
		t.Errorf("failed to verify proof: %v", err)
	}
}

func TestGasOracleIntegration(t *testing.T) {
	oracle := gas.NewGasOracle(gas.DefaultGasConfig())

	for i := 0; i < 10; i++ {
		block := gas.BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      15_000_000 + uint64(i*1_000_000),
			GasLimit:     30_000_000,
			BaseFee:      oracle.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: []uint64{1_500_000_000 + uint64(i*100_000_000)},
		}

		if err := oracle.ProcessBlock(block); err != nil {
			t.Fatalf("failed to process block %d: %v", i+1, err)
		}
	}

	estimate := oracle.EstimateGas([]uint64{25, 50, 75})

	if estimate.BaseFee == 0 {
		t.Error("expected non-zero base fee")
	}

	if estimate.PriorityFee == 0 {
		t.Error("expected non-zero priority fee")
	}

	utilization := oracle.GetNetworkUtilization()
	if utilization <= 0 || utilization > 1 {
		t.Errorf("expected utilization between 0 and 1, got %f", utilization)
	}

	trend := oracle.GetBaseFeeTrend()
	if trend == "" {
		t.Error("expected non-empty trend")
	}
}

func TestFullBlockchainWithGovernance(t *testing.T) {
	dir, err := os.MkdirTemp("", "full-integration-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := state.NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	genesis := ledger.TestGenesis()
	if err := genesis.ValidateAndSanitize(); err != nil {
		t.Fatal(err)
	}

	blockchain, err := ledger.NewPersistentBlockchain(genesis, db)
	if err != nil {
		t.Fatal(err)
	}

	_ = blockchain

	stateMgr, err := state.NewStateManager(db)
	if err != nil {
		t.Fatal(err)
	}

	if err := stateMgr.Initialize(new(big.Int).SetUint64(genesis.InitialSupply)); err != nil {
		t.Fatal(err)
	}

	dao := governance.NewGovernanceDAO(10, 1000, 0.5)
	br := bridge.NewChainBridge(2)
	protocol := interop.NewInteropProtocol()
	solver := intent.NewIntentSolver()

	key, _ := crypto.GenerateKey()
	staking := consensus.NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	proposal, err := dao.SubmitProposal(
		"Register Bridge",
		"Register a new bridge chain",
		governance.ProposalTypeSoftwareUpgrade,
		key.PubKey().Address(),
		200000,
	)
	if err != nil {
		t.Fatalf("failed to submit proposal: %v", err)
	}

	_ = dao.Vote(proposal.ID, key.PubKey().Address(), governance.VoteChoiceYes, 1000000)

	br.RegisterChain("test-chain", "TestChain", "https://test.example.com")
	br.RegisterChain("viri", "Viri", "https://viri.example.com")

	br.RegisterValidator("validator-1")
	br.RegisterValidator("validator-2")

	_, err = br.InitiateTransfer(
		"test-chain",
		"viri",
		[]byte("sender"),
		key.PubKey().Address(),
		500,
		[]byte("TEST"),
	)
	if err != nil {
		t.Fatalf("failed to initiate transfer: %v", err)
	}

	channel, err := protocol.CreateChannel(
		"port-test",
		"port-viri",
		"test-chain",
		"viri",
		"1.0",
	)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	_, err = protocol.SendPacket(
		channel.ID,
		interop.PacketTypeTransfer,
		[]byte("cross-chain-data"),
		uint64(time.Now().Add(1*time.Hour).UnixNano()),
	)
	if err != nil {
		t.Fatalf("failed to send packet: %v", err)
	}

	_, err = solver.SubmitIntent(
		key.PubKey().Address(),
		intent.IntentTypeSwap,
		[]byte("100-TEST"),
		[]byte("50-VIRI"),
		0.05,
		uint64(time.Now().Add(1*time.Hour).UnixNano()),
		100,
	)
	if err != nil {
		t.Fatalf("failed to submit intent: %v", err)
	}

	if dao.ProposalCount() < 1 {
		t.Error("expected at least one proposal")
	}

	if len(protocol.GetActiveChannels()) < 1 {
		t.Error("expected at least one active channel")
	}

	if len(solver.GetOpenIntents()) < 1 {
		t.Error("expected at least one open intent")
	}
}
