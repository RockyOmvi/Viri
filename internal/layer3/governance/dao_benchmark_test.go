package governance

import (
	"testing"
	"time"
)

func BenchmarkSubmitProposal(b *testing.B) {
	dao := NewGovernanceDAO(10*time.Millisecond, 1, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dao.SubmitProposal("t", "d", ProposalTypeText, []byte("p"), 1)
	}
}
