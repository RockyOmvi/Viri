package interop

import "testing"

func TestCreateChannel(t *testing.T) {
	proto := NewInteropProtocol()
	ch, err := proto.CreateChannel("pa", "pb", "a", "b", "1")
	if err != nil {
		t.Fatalf("create channel failed: %v", err)
	}
	if !ch.IsActive {
		t.Fatalf("expected active channel")
	}

	if _, err := proto.CreateChannel("pa", "pb", "a", "b", "1"); err == nil {
		t.Fatalf("expected duplicate channel error")
	}
}

func TestSendReceivePacket(t *testing.T) {
	proto := NewInteropProtocol()
	ch, _ := proto.CreateChannel("pa", "pb", "a", "b", "1")

	packet, err := proto.SendPacket(ch.ID, PacketTypeTransfer, []byte("data"), 100)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if packet.Status != PacketStatusPending {
		t.Fatalf("expected pending")
	}

	if err := proto.ReceivePacket(ch.ID, packet.Sequence, []byte("proof")); err != nil {
		t.Fatalf("receive failed: %v", err)
	}

	if err := proto.ReceivePacket(ch.ID, packet.Sequence, []byte("proof")); err == nil {
		t.Fatalf("expected packet not pending error")
	}
}

func TestReceivePacketErrors(t *testing.T) {
	proto := NewInteropProtocol()
	if err := proto.ReceivePacket("missing", 0, nil); err == nil {
		t.Fatalf("expected missing packet error")
	}
}

func TestHandlers(t *testing.T) {
	proto := NewInteropProtocol()
	ch, _ := proto.CreateChannel("pa", "pb", "a", "b", "1")

	proto.RegisterHandler(ch.PortB, func(packet *IBCPacket) ([]byte, error) {
		return []byte("ok"), nil
	})

	packet, _ := proto.SendPacket(ch.ID, PacketTypeTransfer, []byte("data"), 100)
	if err := proto.ReceivePacket(ch.ID, packet.Sequence, []byte("proof")); err != nil {
		t.Fatalf("receive failed: %v", err)
	}

	loaded, ok := proto.GetChannel(ch.ID)
	if !ok || !loaded.IsActive {
		t.Fatalf("channel should be active")
	}

	active := proto.GetActiveChannels()
	if len(active) != 1 {
		t.Fatalf("expected 1 active channel")
	}
}

func TestSendPacketErrors(t *testing.T) {
	proto := NewInteropProtocol()
	if _, err := proto.SendPacket("missing", PacketTypeTransfer, []byte("data"), 100); err == nil {
		t.Fatalf("expected missing channel error")
	}

	ch, _ := proto.CreateChannel("pa", "pb", "a", "b", "1")
	_ = proto.CloseChannel(ch.ID)
	if _, err := proto.SendPacket(ch.ID, PacketTypeTransfer, []byte("data"), 100); err == nil {
		t.Fatalf("expected inactive channel error")
	}
}

func TestCloseChannel(t *testing.T) {
	proto := NewInteropProtocol()
	if err := proto.CloseChannel("missing"); err == nil {
		t.Fatalf("expected missing channel error")
	}

	ch, _ := proto.CreateChannel("pa", "pb", "a", "b", "1")
	if err := proto.CloseChannel(ch.ID); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	loaded, _ := proto.GetChannel(ch.ID)
	if loaded.IsActive {
		t.Fatalf("expected inactive channel")
	}
}
