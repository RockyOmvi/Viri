package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
	"github.com/viri-chain/viri/internal/layer2/agents"
	"github.com/viri-chain/viri/internal/layer2/execution"
	"github.com/viri-chain/viri/internal/layer2/gas"
	"github.com/viri-chain/viri/internal/layer2/mev"
	"github.com/viri-chain/viri/internal/layer2/privacy"
	"github.com/viri-chain/viri/internal/layer2/rollups"
)

type chainBlockProducer struct {
	mu              sync.Mutex
	blockchain      *ledger.PersistentBlockchain
	key             *crypto.PrivateKey
	validator       []byte
	pendingBlocks   map[uint64]*ledger.Block
	pendingReceipts map[uint64][]*ledger.Receipt
	execEngine      *execution.ExecutionEngine
	stateMgr        *state.StateManager
	gasOracle       *gas.GasOracle
	mevState        *mev.MEVState
	shieldedPool    *privacy.ShieldedPool
	rollupChain     *rollups.RollupChain
	agentMgr        *agents.AgentManager
}

func newChainBlockProducer(bc *ledger.PersistentBlockchain, key *crypto.PrivateKey, exec *execution.ExecutionEngine, sm *state.StateManager, gobj *gas.GasOracle, mevObj *mev.MEVState, sp *privacy.ShieldedPool, rc *rollups.RollupChain, am *agents.AgentManager) *chainBlockProducer {
	return &chainBlockProducer{
		blockchain:      bc,
		key:             key,
		validator:       key.PubKey().Address(),
		execEngine:      exec,
		stateMgr:        sm,
		gasOracle:       gobj,
		mevState:        mevObj,
		shieldedPool:    sp,
		rollupChain:     rc,
		agentMgr:        am,
		pendingBlocks:   make(map[uint64]*ledger.Block),
		pendingReceipts: make(map[uint64][]*ledger.Receipt),
	}
}

// stateToExecAccount converts a state.Account to an execution.AccountState for the L2 engine.
func stateToExecAccount(acct *state.Account) *execution.AccountState {
	storage := make(map[string][]byte)
	if acct.Storage != nil {
		storage = make(map[string][]byte, len(acct.Storage))
		for k, v := range acct.Storage {
			storage[k] = v
		}
	}
	return &execution.AccountState{
		Address: acct.Address,
		Balance: new(big.Int).Set(acct.Balance),
		Nonce:   acct.Nonce,
		Code:    acct.Code,
		Storage: storage,
	}
}

// execToStateAccount applies execution results back to the state layer.
func execToStateAccount(src *execution.AccountState, dst *state.Account) {
	dst.Balance = new(big.Int).Set(src.Balance)
	dst.Nonce = src.Nonce
	if len(src.Code) > 0 {
		dst.Code = src.Code
		dst.Type = state.AccountTypeContract
	}
	if src.Storage != nil {
		if dst.Storage == nil {
			dst.Storage = make(map[string][]byte, len(src.Storage))
		}
		for k, v := range src.Storage {
			dst.Storage[k] = v
		}
	}
}

// executeBlockTxs runs L2 execution on the block's transactions and returns receipts.
func (p *chainBlockProducer) executeBlockTxs(block *ledger.Block, height uint64) []*ledger.Receipt {
	if p.execEngine == nil || p.stateMgr == nil || len(block.Transactions) == 0 {
		return nil
	}

	getAccount := func(addr []byte) (*execution.AccountState, error) {
		acct, err := p.stateMgr.GetAccount(addr)
		if err != nil {
			return &execution.AccountState{
				Address: addr,
				Balance: big.NewInt(0),
				Nonce:   0,
				Storage: make(map[string][]byte),
			}, nil
		}
		return stateToExecAccount(acct), nil
	}

	setAccount := func(addr []byte, execAcct *execution.AccountState) error {
		acct, err := p.stateMgr.GetAccount(addr)
		if err != nil {
			acct = state.NewAccount(addr, state.AccountTypeNormal)
		}
		execToStateAccount(execAcct, acct)
		return p.stateMgr.SetAccount(acct)
	}

	results, _, err := p.execEngine.ExecuteBlock(block.Transactions, height, getAccount, setAccount)
	if err != nil {
		fmt.Printf("[WARN] L2 execution partial failure at block %d: %v\n", height, err)
	}

	var receipts []*ledger.Receipt
	var receiptHashes [][]byte
	for i, res := range results {
		receipt := &ledger.Receipt{
			TxHash:      block.Transactions[i].Hash,
			BlockHeight: height,
			GasUsed:     res.GasUsed,
			Status:      res.Status,
			Logs:        res.Logs,
		}
		receipts = append(receipts, receipt)
		data, _ := ledger.SerializeReceipt(receipt)
		receiptHashes = append(receiptHashes, crypto.SHA256(data))
	}

	if len(receiptHashes) > 0 {
		tree, err := crypto.NewMerkleTree(receiptHashes)
		if err == nil {
			block.Header.ReceiptsRoot = tree.RootHash
		}
	}

	if p.stateMgr != nil {
		if err := p.stateMgr.Commit(height); err == nil {
			block.Header.StateRoot = p.stateMgr.StateRoot()
		}
	}

	return receipts
}

func (p *chainBlockProducer) CreateBlock(proposer []byte, height uint64) ([]byte, []byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	txs := p.blockchain.TxPool().GetPending()
	if len(txs) == 0 {
		txs = []*ledger.Transaction{}
	}

	// Order transactions via MEV for optimal block construction
	if p.mevState != nil && len(txs) > 0 {
		stdTxs := make([]*mev.StandardTx, 0, len(txs))
		for _, tx := range txs {
			stdTxs = append(stdTxs, &mev.StandardTx{
				ID:        tx.Hash,
				Sender:    tx.From,
				To:        tx.To,
				Data:      tx.Data,
				Nonce:     tx.Nonce,
				GasTipCap: new(big.Int).SetUint64(tx.GasPrice),
				GasFeeCap: new(big.Int).SetUint64(tx.GasPrice),
				GasLimit:  tx.GasLimit,
				Value:     new(big.Int).SetUint64(tx.Value),
				Timestamp: uint64(time.Now().Unix()),
			})
		}
		stdTxs = p.mevState.OrderTransactions(stdTxs)
		ordered := make([]*ledger.Transaction, 0, len(stdTxs))
		for _, stx := range stdTxs {
			for _, tx := range txs {
				if bytes.Equal(tx.Hash, stx.ID) {
					ordered = append(ordered, tx)
					break
				}
			}
		}
		txs = ordered
	}

	prevHash := p.blockchain.TipHash()

	block, err := ledger.NewBlock(height, prevHash, txs, proposer, p.key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create block: %w", err)
	}

	// Execute L2 transactions during proposal so all validators agree on state
	receipts := p.executeBlockTxs(block, height)

	// Re-sign after execution modified headers
	sig, err := p.key.Sign(block.SigningPayload())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign block: %w", err)
	}
	block.Header.Signature = sig.Bytes()

	p.pendingBlocks[height] = block
	p.pendingReceipts[height] = receipts

	blockData, err := json.Marshal(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to serialize block: %w", err)
	}
	return blockData, block.Hash(), nil
}

func (p *chainBlockProducer) ValidateBlock(blockData []byte, blockHash []byte, height uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If we already have this block from our own proposal, just validate hash
	if pb, ok := p.pendingBlocks[height]; ok && pb.Header.Height == height {
		if bytes.Equal(pb.Hash(), blockHash) {
			return nil
		}
		// Different block proposed — clear ours and accept theirs
		delete(p.pendingBlocks, height)
		delete(p.pendingReceipts, height)
	}

	var block ledger.Block
	if err := json.Unmarshal(blockData, &block); err != nil {
		return fmt.Errorf("failed to decode block: %w", err)
	}

	if block.Header.Height != height {
		return fmt.Errorf("block height mismatch: expected %d, got %d", height, block.Header.Height)
	}

	if !block.Verify() {
		return fmt.Errorf("block verification failed")
	}

	// Store the validated block for CommitBlock
	p.pendingBlocks[height] = &block
	return nil
}

func (p *chainBlockProducer) GetBlockData(height uint64) ([]byte, error) {
	block, err := p.blockchain.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return json.Marshal(block)
}

func (p *chainBlockProducer) RotateKey() error {
	return nil
}

func (p *chainBlockProducer) CommitBlock(blockHash []byte, height uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	block := p.pendingBlocks[height]
	if block == nil {
		return fmt.Errorf("no pending block for height %d", height)
	}

	receipts := p.pendingReceipts[height]
	delete(p.pendingBlocks, height)
	delete(p.pendingReceipts, height)

	// Set the consensus hash so Block.Hash() returns the agreed-upon hash
	block.ConsensusHash = blockHash

	// Execute L2 transactions for local state tracking
	if p.execEngine != nil && len(block.Transactions) > 0 && len(receipts) == 0 {
		localReceipts := p.executeBlockTxs(block, height)
		if receipts == nil {
			receipts = localReceipts
		}
	}

	// Save receipts
	if p.blockchain != nil && len(receipts) > 0 {
		if err := p.blockchain.SaveReceipts(receipts); err != nil {
			fmt.Printf("[WARN] Failed to save receipts for block %d: %v\n", height, err)
		}
	}

	if err := p.blockchain.AddBlock(block); err != nil {
		return fmt.Errorf("failed to add block: %w", err)
	}

	// Remove confirmed transactions from the pool to avoid stale re-inclusion
	if len(block.Transactions) > 0 {
		p.blockchain.TxPool().RemoveConfirmed(block.Transactions)
	}

	// Clean up old pending blocks
	for h := range p.pendingBlocks {
		if h < height {
			delete(p.pendingBlocks, h)
			delete(p.pendingReceipts, h)
		}
	}

	// Submit rollup batch for this block
	if p.rollupChain != nil {
		blockData, err := json.Marshal(block)
		if err == nil {
			p.rollupChain.SubmitBatch(blockData, p.validator, uint64(time.Now().Unix()))
		}
	}

	// Process committed block through gas oracle
	if p.gasOracle != nil {
		totalGas := uint64(0)
		for _, r := range receipts {
			totalGas += r.GasUsed
		}
		_ = p.gasOracle.ProcessBlock(gas.BlockGasInfo{
			BlockNumber: height,
			GasUsed:     totalGas,
			GasLimit:    30_000_000,
			BaseFee:     p.gasOracle.GetBaseFee(),
			Timestamp:   uint64(time.Now().Unix()),
		})
	}

	return nil
}

func (p *chainBlockProducer) GetBlockHash(height uint64) ([]byte, error) {
	block, err := p.blockchain.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return block.Hash(), nil
}

func (p *chainBlockProducer) GetChainHeight() uint64 {
	return p.blockchain.Height()
}

func (p *chainBlockProducer) Sign(data []byte) (*crypto.Signature, error) {
	return p.key.Sign(data)
}

func (p *chainBlockProducer) VerifySign(pubKey []byte, data []byte, sig *crypto.Signature) bool {
	pub, err := crypto.PubKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	return pub.Verify(data, sig)
}

func (p *chainBlockProducer) GetValidatorAddress() []byte {
	return p.validator
}

func (p *chainBlockProducer) GetValidatorPublicKey() []byte {
	return p.key.PubKey().Bytes()
}
