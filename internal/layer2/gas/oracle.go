package gas

import (
	"encoding/json"
	"fmt"
	"sync"
)

type GasOracle struct {
	mu sync.Mutex

	baseFee       uint64
	minBaseFee    uint64
	maxBaseFee    uint64

	targetGasPerBlock uint64
	maxGasPerBlock    uint64

	baseFeeChangeDenominator uint64

	recentBlocks []BlockGasInfo
	maxHistory   int

	priorityFeePercentiles []uint64
	recentPriorityFees     []uint64
}

type BlockGasInfo struct {
	BlockNumber    uint64
	GasUsed        uint64
	GasLimit       uint64
	BaseFee        uint64
	Timestamp      uint64
	PriorityFees   []uint64
}

type GasEstimate struct {
	BaseFee       uint64 `json:"base_fee"`
	PriorityFee   uint64 `json:"priority_fee"`
	TotalEstimate uint64 `json:"total_estimate"`
	MaxFee        uint64 `json:"max_fee"`
}

type GasConfig struct {
	InitialBaseFee       uint64
	MinBaseFee           uint64
	MaxBaseFee           uint64
	TargetGasPerBlock    uint64
	MaxGasPerBlock       uint64
	BaseFeeChangeDenom   uint64
	MaxHistory           int
}

func DefaultGasConfig() GasConfig {
	return GasConfig{
		InitialBaseFee:       1_000_000_000,
		MinBaseFee:           100_000_000,
		MaxBaseFee:           100_000_000_000,
		TargetGasPerBlock:    15_000_000,
		MaxGasPerBlock:       30_000_000,
		BaseFeeChangeDenom:   8,
		MaxHistory:           64,
	}
}

func NewGasOracle(config GasConfig) *GasOracle {
	if config.InitialBaseFee == 0 {
		config.InitialBaseFee = 1_000_000_000
	}
	if config.MinBaseFee == 0 {
		config.MinBaseFee = 100_000_000
	}
	if config.MaxBaseFee == 0 {
		config.MaxBaseFee = 100_000_000_000
	}
	if config.TargetGasPerBlock == 0 {
		config.TargetGasPerBlock = 15_000_000
	}
	if config.MaxGasPerBlock == 0 {
		config.MaxGasPerBlock = 30_000_000
	}
	if config.BaseFeeChangeDenom == 0 {
		config.BaseFeeChangeDenom = 8
	}
	if config.MaxHistory == 0 {
		config.MaxHistory = 64
	}

	return &GasOracle{
		baseFee:                  config.InitialBaseFee,
		minBaseFee:               config.MinBaseFee,
		maxBaseFee:               config.MaxBaseFee,
		targetGasPerBlock:        config.TargetGasPerBlock,
		maxGasPerBlock:           config.MaxGasPerBlock,
		baseFeeChangeDenominator: config.BaseFeeChangeDenom,
		maxHistory:               config.MaxHistory,
		priorityFeePercentiles:   []uint64{10, 25, 50, 75, 90},
	}
}

func (g *GasOracle) GetBaseFee() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.baseFee
}

func (g *GasOracle) EstimateGas(priorityLevels []uint64) GasEstimate {
	g.mu.Lock()
	defer g.mu.Unlock()

	baseFee := g.baseFee

	priorityFee := g.estimatePriorityFee(priorityLevels)

	totalEstimate := baseFee + priorityFee

	maxFee := baseFee * 2
	if maxFee > g.maxBaseFee {
		maxFee = g.maxBaseFee
	}

	return GasEstimate{
		BaseFee:       baseFee,
		PriorityFee:   priorityFee,
		TotalEstimate: totalEstimate,
		MaxFee:        maxFee,
	}
}

func (g *GasOracle) ProcessBlock(blockInfo BlockGasInfo) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if blockInfo.GasLimit > g.maxGasPerBlock {
		return fmt.Errorf("block gas limit %d exceeds max %d", blockInfo.GasLimit, g.maxGasPerBlock)
	}

	g.adjustBaseFee(blockInfo.GasUsed, blockInfo.GasLimit)

	if len(blockInfo.PriorityFees) > 0 {
		for _, fee := range blockInfo.PriorityFees {
			g.recentPriorityFees = append(g.recentPriorityFees, fee)
		}

		if len(g.recentPriorityFees) > 1024 {
			g.recentPriorityFees = g.recentPriorityFees[len(g.recentPriorityFees)-512:]
		}
	}

	g.recentBlocks = append(g.recentBlocks, blockInfo)
	if len(g.recentBlocks) > g.maxHistory {
		g.recentBlocks = g.recentBlocks[len(g.recentBlocks)-g.maxHistory:]
	}

	return nil
}

func (g *GasOracle) adjustBaseFee(gasUsed, gasLimit uint64) {
	if gasLimit == 0 {
		return
	}

	targetGas := gasLimit / 2
	if g.targetGasPerBlock > 0 {
		targetGas = g.targetGasPerBlock
	}

	if gasUsed == targetGas {
		return
	}

	if gasUsed > targetGas {
		gasUsedDelta := gasUsed - targetGas
		baseFeeDelta := (g.baseFee * gasUsedDelta) / targetGas / g.baseFeeChangeDenominator

		g.baseFee += baseFeeDelta
	} else {
		gasUsedDelta := targetGas - gasUsed
		baseFeeDelta := (g.baseFee * gasUsedDelta) / targetGas / g.baseFeeChangeDenominator

		if g.baseFee > baseFeeDelta {
			g.baseFee -= baseFeeDelta
		} else {
			g.baseFee = g.minBaseFee
		}
	}

	if g.baseFee < g.minBaseFee {
		g.baseFee = g.minBaseFee
	}

	if g.baseFee > g.maxBaseFee {
		g.baseFee = g.maxBaseFee
	}
}

func (g *GasOracle) estimatePriorityFee(percentiles []uint64) uint64 {
	if len(g.recentPriorityFees) == 0 {
		return 1_500_000_000
	}

	if len(percentiles) == 0 {
		percentiles = []uint64{50}
	}

	sorted := make([]uint64, len(g.recentPriorityFees))
	copy(sorted, g.recentPriorityFees)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var sum uint64
	for _, p := range percentiles {
		idx := int(float64(len(sorted)-1) * float64(p) / 100.0)
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		sum += sorted[idx]
	}

	return sum / uint64(len(percentiles))
}

func (g *GasOracle) GetNetworkUtilization() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.recentBlocks) == 0 {
		return 0.0
	}

	var totalUsed, totalLimit uint64
	for _, block := range g.recentBlocks {
		totalUsed += block.GasUsed
		totalLimit += block.GasLimit
	}

	if totalLimit == 0 {
		return 0.0
	}

	return float64(totalUsed) / float64(totalLimit)
}

func (g *GasOracle) GetGasPriceHistory() []BlockGasInfo {
	g.mu.Lock()
	defer g.mu.Unlock()

	history := make([]BlockGasInfo, len(g.recentBlocks))
	copy(history, g.recentBlocks)
	return history
}

func (g *GasOracle) GetPercentileFee(percentile uint64) uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.recentPriorityFees) == 0 {
		return 1_500_000_000
	}

	sorted := make([]uint64, len(g.recentPriorityFees))
	copy(sorted, g.recentPriorityFees)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	idx := int(float64(len(sorted)-1) * float64(percentile) / 100.0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return sorted[idx]
}

func (g *GasOracle) GetRecommendedPriorityFee() uint64 {
	return g.GetPercentileFee(50)
}

func (g *GasOracle) GetFastPriorityFee() uint64 {
	return g.GetPercentileFee(75)
}

func (g *GasOracle) GetSlowPriorityFee() uint64 {
	return g.GetPercentileFee(25)
}

func (g *GasOracle) ExportState() ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := map[string]interface{}{
		"base_fee":        g.baseFee,
		"min_base_fee":    g.minBaseFee,
		"max_base_fee":    g.maxBaseFee,
		"target_gas":      g.targetGasPerBlock,
		"max_gas":         g.maxGasPerBlock,
		"recent_blocks":   g.recentBlocks,
		"priority_fees":   g.recentPriorityFees,
	}

	return json.Marshal(state)
}

func (g *GasOracle) ImportState(data []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid state data: %w", err)
	}

	if val, exists := state["base_fee"]; exists {
		if f, ok := val.(float64); ok {
			g.baseFee = uint64(f)
		}
	}

	return nil
}

func (g *GasOracle) GetBaseFeeTrend() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.recentBlocks) < 2 {
		return "stable"
	}

	recentBlocks := g.recentBlocks[len(g.recentBlocks)-10:]

	var increasing, decreasing int
	for i := 1; i < len(recentBlocks); i++ {
		if recentBlocks[i].BaseFee > recentBlocks[i-1].BaseFee {
			increasing++
		} else if recentBlocks[i].BaseFee < recentBlocks[i-1].BaseFee {
			decreasing++
		}
	}

	if increasing > decreasing {
		return "increasing"
	} else if decreasing > increasing {
		return "decreasing"
	}

	return "stable"
}

func (g *GasOracle) CalculateMaxFee(priorityFee uint64, baseFeeMultiplier float64) uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	maxFee := uint64(float64(g.baseFee) * baseFeeMultiplier)
	maxFee += priorityFee

	if maxFee > g.maxBaseFee {
		maxFee = g.maxBaseFee
	}

	return maxFee
}

func (g *GasOracle) ValidateGasParams(gasLimit, gasPrice, maxFeePerGas, maxPriorityFeePerGas uint64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if gasLimit == 0 {
		return fmt.Errorf("gas limit cannot be zero")
	}

	if gasLimit > g.maxGasPerBlock {
		return fmt.Errorf("gas limit %d exceeds max %d", gasLimit, g.maxGasPerBlock)
	}

	if gasPrice < g.minBaseFee {
		return fmt.Errorf("gas price %d below minimum %d", gasPrice, g.minBaseFee)
	}

	if maxFeePerGas < g.baseFee {
		return fmt.Errorf("max fee %d below current base fee %d", maxFeePerGas, g.baseFee)
	}

	return nil
}

func (g *GasOracle) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.baseFee = 1_000_000_000
	g.recentBlocks = nil
	g.recentPriorityFees = nil
}
