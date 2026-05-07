package p2p

import (
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func newTestPubKey() *crypto.PublicKey {
	priv, _ := crypto.GenerateKey()
	return priv.PubKey()
}

func TestHandshakeEncodeDecode(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Agent = "viri-test/0.1.0"

	data, err := hs.Encode()
	if err != nil {
		t.Fatalf("Failed to encode handshake: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Encoded handshake is empty")
	}

	decoded, err := DecodeHandshakeMessage(data)
	if err != nil {
		t.Fatalf("Failed to decode handshake: %v", err)
	}

	if decoded.Version != hs.Version {
		t.Errorf("Version mismatch: expected %d, got %d", hs.Version, decoded.Version)
	}

	if decoded.Magic != hs.Magic {
		t.Errorf("Magic mismatch: expected 0x%08x, got 0x%08x", hs.Magic, decoded.Magic)
	}

	if decoded.ChainID != hs.ChainID {
		t.Errorf("ChainID mismatch: expected %d, got %d", hs.ChainID, decoded.ChainID)
	}

	if decoded.Agent != hs.Agent {
		t.Errorf("Agent mismatch: expected %s, got %s", hs.Agent, decoded.Agent)
	}

	if decoded.Height != hs.Height {
		t.Errorf("Height mismatch: expected %d, got %d", hs.Height, decoded.Height)
	}

	if len(decoded.PubKey) == 0 {
		t.Error("Decoded PubKey is empty")
	}
}

func TestHandshakeValidate(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Agent = "viri/0.1.0"

	if err := hs.Validate(1337); err != nil {
		t.Errorf("Valid handshake should pass: %v", err)
	}
}

func TestHandshakeInvalidMagic(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Magic = 0xDEADBEEF

	if err := hs.Validate(1337); err == nil {
		t.Error("Invalid magic should fail validation")
	}
}

func TestHandshakeInvalidVersion(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Version = 999

	if err := hs.Validate(1337); err == nil {
		t.Error("Invalid version should fail validation")
	}
}

func TestHandshakeChainIDMismatch(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())

	if err := hs.Validate(42069); err == nil {
		t.Error("Chain ID mismatch should fail validation")
	}
}

func TestHandshakeOldTimestamp(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Timestamp = time.Now().Add(-2 * time.Minute).Unix()

	if err := hs.Validate(1337); err == nil {
		t.Error("Old timestamp should fail validation")
	}
}

func TestHandshakeEmptyAgent(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Agent = ""

	if err := hs.Validate(1337); err == nil {
		t.Error("Empty agent should fail validation")
	}
}

func TestHandshakeLongAgent(t *testing.T) {
	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Agent = "a" + string(make([]byte, 200))

	if err := hs.Validate(1337); err == nil {
		t.Error("Too long agent should fail validation")
	}
}

func TestDecodeHandshakeTooShort(t *testing.T) {
	_, err := DecodeHandshakeMessage([]byte{0x01, 0x02})
	if err == nil {
		t.Error("Too short data should fail decoding")
	}
}
