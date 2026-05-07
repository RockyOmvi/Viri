package contracts

import "testing"

func BenchmarkDeployContract(b *testing.B) {
	cm := NewContractManager()
	owner := []byte("owner")
	code := []byte{0x00, 0x01}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.Deploy(owner, code, uint64(i))
	}
}
