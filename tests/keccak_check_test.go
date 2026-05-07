package tests

import (
	"fmt"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestKeccakCheck(t *testing.T) {
	input := make([]byte, 32)
	input[31] = 0x2A
	hash := crypto.Keccak256(input)
	fmt.Printf("Input: %x\n", input)
	fmt.Printf("Hash:  %x\n", hash)
	fmt.Printf("First byte: %02x\n", hash[0])
}
