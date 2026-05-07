package interop

import "testing"

func BenchmarkSendPacket(b *testing.B) {
	proto := NewInteropProtocol()
	ch, _ := proto.CreateChannel("pa", "pb", "a", "b", "1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = proto.SendPacket(ch.ID, PacketTypeTransfer, []byte("data"), 100)
	}
}
