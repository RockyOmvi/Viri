package appchain

import (
	"strconv"
	"testing"
)

func BenchmarkCreateAppChain(b *testing.B) {
	mgr := NewAppChainManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.CreateAppChain(AppChainConfig{
			ChainID:       "c" + strconv.Itoa(i),
			Owner:         []byte("o"),
			MaxValidators: 1,
		})
	}
}
