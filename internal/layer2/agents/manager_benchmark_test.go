package agents

import (
	"strconv"
	"testing"
)

func BenchmarkRegisterAgent(b *testing.B) {
	am := NewAgentManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := "agent" + strconv.Itoa(i)
		_ = am.Register(id, AgentTypeValidator, []byte("addr"), 1)
	}
}
