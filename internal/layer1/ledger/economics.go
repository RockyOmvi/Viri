package ledger

import (
	"fmt"
	"math/big"
	"sync"
)

type EconomicsConfig struct {
	BlockReward          *big.Int
	UncleReward          *big.Int
	MaxSupply            *big.Int
	InitialSupply        *big.Int
	HalvingInterval      uint64
	BaseFeeTarget        *big.Int
	BaseFeeMaxChangeRate *big.Int
	GasTarget            uint64
	GasLimitBound        uint64
	ValidatorShare       *big.Int
	DeveloperShare       *big.Int
	BurnShare            *big.Int
}

func DefaultEconomicsConfig() *EconomicsConfig {
	blockReward := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	maxSupply := new(big.Int).Exp(big.NewInt(10), big.NewInt(26), nil)
	initialSupply := new(big.Int).Exp(big.NewInt(10), big.NewInt(25), nil)

	return &EconomicsConfig{
		BlockReward:          blockReward,
		UncleReward:          new(big.Int).Div(blockReward, big.NewInt(32)),
		MaxSupply:            maxSupply,
		InitialSupply:        initialSupply,
		HalvingInterval:      2_100_000,
		BaseFeeTarget:        new(big.Int).SetUint64(1_000_000_000),
		BaseFeeMaxChangeRate: new(big.Int).SetUint64(125),
		GasTarget:            15_000_000,
		GasLimitBound:        1024,
		ValidatorShare:       big.NewInt(80),
		DeveloperShare:       big.NewInt(10),
		BurnShare:            big.NewInt(10),
	}
}

type Economics struct {
	mu             sync.RWMutex
	config         *EconomicsConfig
	totalSupply    *big.Int
	circulatingSupply *big.Int
	burned         *big.Int
	totalFees      *big.Int
	blockHeight    uint64
}

func NewEconomics(config *EconomicsConfig) *Economics {
	if config == nil {
		config = DefaultEconomicsConfig()
	}

	return &Economics{
		config:          config,
		totalSupply:     new(big.Int).Set(config.MaxSupply),
		circulatingSupply: new(big.Int).Set(config.InitialSupply),
		burned:          big.NewInt(0),
		totalFees:       big.NewInt(0),
	}
}

func (e *Economics) CalculateBlockReward(blockHeight uint64) *big.Int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	epochs := blockHeight / e.config.HalvingInterval
	if epochs >= 64 {
		return big.NewInt(0)
	}

	reward := new(big.Int).Set(e.config.BlockReward)
	for i := uint64(0); i < epochs; i++ {
		reward.Div(reward, big.NewInt(2))
	}

	return reward
}

func (e *Economics) CalculateFees(txs []*Transaction) (*big.Int, *big.Int, *big.Int) {
	totalGasUsed := uint64(0)
	totalGasPrice := uint64(0)

	for _, tx := range txs {
		totalGasUsed += tx.GasLimit
		totalGasPrice += tx.GasPrice * tx.GasLimit
	}

	feeAmount := new(big.Int).SetUint64(totalGasPrice)
	validatorShare := new(big.Int).Div(
		new(big.Int).Mul(feeAmount, e.config.ValidatorShare),
		big.NewInt(100),
	)
	burnShare := new(big.Int).Div(
		new(big.Int).Mul(feeAmount, e.config.BurnShare),
		big.NewInt(100),
	)

	return feeAmount, validatorShare, burnShare
}

func (e *Economics) ProcessBlock(txs []*Transaction, blockHeight uint64) (*BlockEconomics, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	reward := e.calculateBlockRewardLocked(blockHeight)
	fees, validatorFees, burnFees := e.calculateFeesLocked(txs)

	totalIssuance := new(big.Int).Add(reward, fees)
	newCirculating := new(big.Int).Add(e.circulatingSupply, totalIssuance)

	if newCirculating.Cmp(e.totalSupply) > 0 {
		return nil, fmt.Errorf("would exceed max supply")
	}

	e.circulatingSupply = newCirculating
	e.burned = new(big.Int).Add(e.burned, burnFees)
	e.totalFees = new(big.Int).Add(e.totalFees, fees)
	e.blockHeight = blockHeight

	return &BlockEconomics{
		BlockHeight:    blockHeight,
		BlockReward:    reward,
		TotalFees:      fees,
		ValidatorFees:  validatorFees,
		BurnedFees:     burnFees,
		NetIssuance:    totalIssuance,
		CirculatingSupply: new(big.Int).Set(e.circulatingSupply),
	}, nil
}

func (e *Economics) CirculatingSupply() *big.Int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return new(big.Int).Set(e.circulatingSupply)
}

func (e *Economics) Burned() *big.Int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return new(big.Int).Set(e.burned)
}

func (e *Economics) TotalFees() *big.Int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return new(big.Int).Set(e.totalFees)
}

func (e *Economics) InflationRate(blockHeight uint64) *big.Float {
	reward := e.CalculateBlockReward(blockHeight)

	yearlyReward := new(big.Int).Mul(reward, big.NewInt(365*24*60*60))

	return new(big.Float).Quo(
		new(big.Float).SetInt(yearlyReward),
		new(big.Float).SetInt(e.CirculatingSupply()),
	)
}

func (e *Economics) calculateBlockRewardLocked(blockHeight uint64) *big.Int {
	epochs := blockHeight / e.config.HalvingInterval
	if epochs >= 64 {
		return big.NewInt(0)
	}

	reward := new(big.Int).Set(e.config.BlockReward)
	for i := uint64(0); i < epochs; i++ {
		reward.Div(reward, big.NewInt(2))
	}

	return reward
}

func (e *Economics) calculateFeesLocked(txs []*Transaction) (*big.Int, *big.Int, *big.Int) {
	totalGasPrice := uint64(0)
	for _, tx := range txs {
		totalGasPrice += tx.GasPrice * tx.GasLimit
	}

	feeAmount := new(big.Int).SetUint64(totalGasPrice)
	validatorFees := new(big.Int).Div(
		new(big.Int).Mul(feeAmount, e.config.ValidatorShare),
		big.NewInt(100),
	)
	burnFees := new(big.Int).Div(
		new(big.Int).Mul(feeAmount, e.config.BurnShare),
		big.NewInt(100),
	)

	return feeAmount, validatorFees, burnFees
}

type BlockEconomics struct {
	BlockHeight       uint64
	BlockReward       *big.Int
	TotalFees         *big.Int
	ValidatorFees     *big.Int
	BurnedFees        *big.Int
	NetIssuance       *big.Int
	CirculatingSupply *big.Int
}
