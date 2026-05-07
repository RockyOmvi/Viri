package interop

import "testing"

func TestGetChannelMissing(t *testing.T) {
	proto := NewInteropProtocol()
	if _, ok := proto.GetChannel("missing"); ok {
		t.Fatalf("unexpected channel")
	}
}
