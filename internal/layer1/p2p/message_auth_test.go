package p2p

import (
	"bytes"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestSignMessageAndVerifySignedMessageRoundtrip(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	msg := NewMessage(MsgBlock, []byte("test block data"))
	chainID := uint64(1337)

	signedMsg, err := SignMessage(msg, privKey, chainID)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	if len(signedMsg.Signature) == 0 {
		t.Error("signature is empty")
	}

	if len(signedMsg.PublicKey) != CompressedPubKeyLen {
		t.Errorf("expected public key length %d, got %d", CompressedPubKeyLen, len(signedMsg.PublicKey))
	}

	err = VerifySignedMessage(signedMsg, chainID, DefaultMaxMessageAge)
	if err != nil {
		t.Errorf("failed to verify signed message: %v", err)
	}
}

func TestSignMessageReplayProtectionOldTimestamp(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	msg := NewMessage(MsgTransaction, []byte("test"))
	chainID := uint64(1337)

	signedMsg, _ := SignMessage(msg, privKey, chainID)

	signedMsg.Timestamp = time.Now().Add(-10 * time.Minute).Unix()

	err := VerifySignedMessage(signedMsg, chainID, 5*time.Minute)
	if err != ErrSignatureExpired {
		t.Errorf("expected ErrSignatureExpired, got %v", err)
	}
}

func TestSignMessageCrossChainReplayProtection(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	msg := NewMessage(MsgBlock, []byte("test"))
	chainID := uint64(1337)

	signedMsg, _ := SignMessage(msg, privKey, chainID)

	err := VerifySignedMessage(signedMsg, 9999, DefaultMaxMessageAge)
	if err != ErrChainIDMismatch {
		t.Errorf("expected ErrChainIDMismatch, got %v", err)
	}
}

func TestSignMessageTamperedMessageDetection(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	msg := NewMessage(MsgBlock, []byte("original data"))
	chainID := uint64(1337)

	signedMsg, _ := SignMessage(msg, privKey, chainID)

	signedMsg.MessageBytes = []byte("tampered data")

	err := VerifySignedMessage(signedMsg, chainID, DefaultMaxMessageAge)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature for tampered message, got %v", err)
	}
}

func TestMessageAuthenticator_SignAndVerify(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	chainID := uint64(1337)

	auth := NewMessageAuthenticator(privKey, chainID, DefaultMaxMessageAge)

	msg := NewMessage(MsgProposal, []byte("consensus proposal"))

	signedMsg, err := auth.Sign(msg)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	err = auth.Verify(signedMsg)
	if err != nil {
		t.Errorf("failed to verify: %v", err)
	}
}

func TestMessageAuthenticator_GetSenderAddress(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	chainID := uint64(1337)

	auth := NewMessageAuthenticator(privKey, chainID, DefaultMaxMessageAge)

	msg := NewMessage(MsgBlock, []byte("test block"))

	signedMsg, _ := auth.Sign(msg)

	addr, err := GetSenderAddress(signedMsg)
	if err != nil {
		t.Fatalf("failed to get sender address: %v", err)
	}

	if len(addr) == 0 {
		t.Error("sender address is empty")
	}

	expectedAddr := privKey.PubKey().Address()
	if !bytes.Equal(addr, expectedAddr) {
		t.Error("sender address does not match expected address")
	}
}

func TestMessageAuthenticator_GetPeerID(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	chainID := uint64(1337)

	auth := NewMessageAuthenticator(privKey, chainID, DefaultMaxMessageAge)

	peerID := auth.GetPeerID()
	if peerID == "" || peerID == "unknown" {
		t.Error("invalid peer ID")
	}
}

func TestSignedMessageEncodeDecodeRoundtrip(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	msg := NewMessage(MsgTransaction, []byte("test tx"))
	chainID := uint64(1337)

	signedMsg, _ := SignMessage(msg, privKey, chainID)

	encoded, err := signedMsg.Encode()
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoded, err := DecodeSignedMessage(encoded)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if decoded.ChainID != chainID {
		t.Errorf("chain ID mismatch: expected %d, got %d", chainID, decoded.ChainID)
	}

	if !bytes.Equal(decoded.MessageBytes, signedMsg.MessageBytes) {
		t.Error("message bytes mismatch after encode/decode")
	}

	if len(decoded.Signature) != len(signedMsg.Signature) {
		t.Error("signature length mismatch after encode/decode")
	}
}

func TestVerifySignedMessageInvalidTimestamp(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	msg := NewMessage(MsgBlock, []byte("test"))
	chainID := uint64(1337)

	signedMsg, _ := SignMessage(msg, privKey, chainID)
	signedMsg.Timestamp = time.Now().Add(10 * time.Second).Unix()

	err := VerifySignedMessage(signedMsg, chainID, DefaultMaxMessageAge)
	if err != ErrInvalidTimestamp {
		t.Errorf("expected ErrInvalidTimestamp, got %v", err)
	}
}

func TestCompressDecompressPubKeyRoundtrip(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	pubKey := privKey.PubKey()

	compressed := pubKey.Compressed()
	if len(compressed) != CompressedPubKeyLen {
		t.Errorf("expected compressed key length %d, got %d", CompressedPubKeyLen, len(compressed))
	}

	decompressed, err := crypto.DecompressPubKey(compressed)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	if pubKey.Hex() != decompressed.Hex() {
		t.Error("decompressed key does not match original")
	}
}
