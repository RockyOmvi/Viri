package gas

import (
	"math/big"
	"sync"
	"time"
)

// NativeCoinAddress is the sentinel address for native coin gas payments.
var NativeCoinAddress = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

// FeeConversionOracle provides exchange rates from token amounts to native coin.
// In production, this is backed by on-chain price feeds (e.g., Oracles, DEX pools).
// For the testnet, rates can be set via governance.
type FeeConversionOracle struct {
	mu        sync.RWMutex
	rates     map[string]float64 // token address hex -> native coin per token unit
	lastUpdate map[string]time.Time
	updateInterval time.Duration
}

// NewFeeConversionOracle creates a new oracle with the given update interval.
func NewFeeConversionOracle(updateInterval time.Duration) *FeeConversionOracle {
	return &FeeConversionOracle{
		rates:          make(map[string]float64),
		lastUpdate:     make(map[string]time.Time),
		updateInterval: updateInterval,
	}
}

// SetRate sets the conversion rate for a token (native coin per token unit).
func (o *FeeConversionOracle) SetRate(token []byte, rate float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rates[string(token)] = rate
	o.lastUpdate[string(token)] = time.Now()
}

// GetRate returns the conversion rate for a token.
// nil or native address returns 1.0 (1:1 with native).
func (o *FeeConversionOracle) GetRate(token []byte) float64 {
	if len(token) == 0 || isNative(token) {
		return 1.0
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	rate, ok := o.rates[string(token)]
	if !ok {
		return 0 // unknown token: no conversion
	}
	return rate
}

// ConvertToNative converts a token amount to native coin equivalent.
func (o *FeeConversionOracle) ConvertToNative(token []byte, tokenAmount uint64) uint64 {
	rate := o.GetRate(token)
	if rate <= 0 {
		return 0
	}
	if rate == 1.0 {
		return tokenAmount
	}
	native := new(big.Float).SetUint64(tokenAmount)
	native.Mul(native, new(big.Float).SetFloat64(rate))
	result, _ := native.Uint64()
	return result
}

// ConvertFromNative converts native coin amount to token amount.
func (o *FeeConversionOracle) ConvertFromNative(token []byte, nativeAmount uint64) uint64 {
	rate := o.GetRate(token)
	if rate <= 0 {
		return 0
	}
	if rate == 1.0 {
		return nativeAmount
	}
	tokenAmount := new(big.Float).SetUint64(nativeAmount)
	tokenAmount.Quo(tokenAmount, new(big.Float).SetFloat64(rate))
	result, _ := tokenAmount.Uint64()
	return result
}

// HasRate returns true if a rate is configured for the token.
func (o *FeeConversionOracle) HasRate(token []byte) bool {
	if len(token) == 0 || isNative(token) {
		return true
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	_, ok := o.rates[string(token)]
	return ok
}

// KnownTokens returns all tokens with configured rates.
func (o *FeeConversionOracle) KnownTokens() [][]byte {
	o.mu.RLock()
	defer o.mu.RUnlock()
	tokens := make([][]byte, 0, len(o.rates))
	for key := range o.rates {
		tokens = append(tokens, []byte(key))
	}
	return tokens
}

func isNative(token []byte) bool {
	return len(token) == 0 || (len(token) == 20 && bytesEqual(token, NativeCoinAddress))
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DefaultConversionRates returns example rates for common test tokens.
// 1 VIRI = 1000 USDC, 1 VIRI = 0.5 ETH, 1 VIRI = 4 DAI
func DefaultConversionRates() map[string]float64 {
	return map[string]float64{
		"usdc": 1000.0,
		"eth":  0.5,
		"dai":  4.0,
		"wbtc": 0.00005,
	}
}
