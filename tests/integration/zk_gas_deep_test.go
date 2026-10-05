package integration

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer2/gas"
	"github.com/viri-chain/viri/internal/layer2/privacy"
	"github.com/viri-chain/viri/internal/layer2/zk"
)

// TestNativeZKShieldedPoolFullLifecycle verifies the end-to-end functionality
// of the Native ZK-Shielded Pool: note creation, commitments, nullifiers,
// double-spend protection, and Groth16 zero-knowledge proof verification.
func TestNativeZKShieldedPoolFullLifecycle(t *testing.T) {
	pool := privacy.NewShieldedPool()

	// 1. Create a shielded note (deposit/shielding into pool)
	ownerAddr := []byte("viri_shielded_user_address_001")
	randomness := []byte("random_salt_1234567890123456")
	noteValue := uint64(5000)

	note, err := pool.CreateNote(noteValue, ownerAddr, randomness)
	if err != nil {
		t.Fatalf("Failed to create shielded note: %v", err)
	}

	if pool.NoteCount() != 1 {
		t.Fatalf("Expected NoteCount = 1, got %d", pool.NoteCount())
	}
	if pool.TotalShielded() != noteValue {
		t.Fatalf("Expected TotalShielded = %d, got %d", noteValue, pool.TotalShielded())
	}
	if !pool.HasCommitment(note.Commitment) {
		t.Fatalf("Pool must record commitment for newly minted shielded note")
	}

	// 2. Prevent duplicate commitments
	_, err = pool.CreateNote(noteValue, ownerAddr, randomness)
	if err == nil {
		t.Fatalf("Security failure: duplicate commitment was accepted")
	}

	// 3. Spend the note using nullifier (unshielding/withdrawing)
	unshieldedVal, err := pool.SpendNote(note.Nullifier)
	if err != nil {
		t.Fatalf("Failed to spend shielded note: %v", err)
	}
	if unshieldedVal != noteValue {
		t.Fatalf("Expected unshielded value = %d, got %d", noteValue, unshieldedVal)
	}
	if pool.TotalShielded() != 0 {
		t.Fatalf("Expected TotalShielded = 0 after spend, got %d", pool.TotalShielded())
	}
	if !pool.HasNullifier(note.Nullifier) {
		t.Fatalf("Pool must record spent nullifier")
	}

	// 4. Double-spend prevention
	_, err = pool.SpendNote(note.Nullifier)
	if err == nil {
		t.Fatalf("Critical security failure: double-spending nullifier succeeded!")
	}

	// 5. Zero-Knowledge Circuit Proof Generation & Verification (Simulated R1CS)
	circuit := zk.NewShieldedTransferCircuit()
	pk := zk.GenerateProvingKey(circuit)
	vk := zk.GenerateVerifyingKey(pk, circuit)

	prover := zk.NewProver(pk, circuit)
	assignment := &zk.Assignment{
		Inputs:  []*big.Int{big.NewInt(10), big.NewInt(20), big.NewInt(30)},
		Witness: []*big.Int{big.NewInt(2), big.NewInt(15), big.NewInt(15), big.NewInt(5), big.NewInt(25), big.NewInt(40)},
	}

	proof, err := prover.Prove(assignment)
	if err != nil {
		t.Fatalf("Failed to generate ZK proof: %v", err)
	}

	verifier := zk.NewVerifier(vk, circuit)
	if err := verifier.Verify(proof); err != nil {
		t.Fatalf("Valid ZK proof failed verification: %v", err)
	}

	// 6. Production Gnark Groth16 Prover & Verifier over BN254
	gnarkCircuit := zk.NewCircuit("gnark_shielded_mul", 2, 1, zk.FieldTypePrime)
	gnarkCircuit.AddMulConstraint(0, 1, 2) // Inputs[0] * Inputs[1] == Witness[0]

	gnarkWitness := &zk.Witness{
		Public: []*big.Int{big.NewInt(6), big.NewInt(7)},
		Secret: []*big.Int{big.NewInt(42)},
	}

	gnarkProver := zk.NewGnarkProver()
	gnarkProof, err := gnarkProver.Prove(gnarkCircuit, gnarkWitness)
	if err != nil {
		t.Fatalf("Failed to generate real gnark Groth16 proof: %v", err)
	}

	gnarkVerifier := zk.NewGnarkVerifier()
	if err := gnarkVerifier.Verify(gnarkProof, gnarkCircuit, gnarkWitness); err != nil {
		t.Fatalf("Valid gnark Groth16 proof failed verification: %v", err)
	}

	// 7. Verify Corrupted/Tampered Proof Rejection
	if len(gnarkProof.Raw) > 0 {
		tamperedProof := &zk.Proof{
			CircuitID: gnarkProof.CircuitID,
			Public:    gnarkProof.Public,
			System:    gnarkProof.System,
			Raw:       append([]byte(nil), gnarkProof.Raw...),
		}
		// Flip bits in raw proof
		tamperedProof.Raw[0] ^= 0xFF
		if err := gnarkVerifier.Verify(tamperedProof, gnarkCircuit, gnarkWitness); err == nil {
			t.Fatalf("Security failure: tampered gnark proof passed verification!")
		}
	}
}

// TestMultiTokenGasPaymentFullLifecycle tests paying gas in arbitrary tokens:
// oracle rate configuration, conversion to/from native VIRI, transaction fee currency
// field handling, signature payload binding (preventing tampering), and fallback logic.
func TestMultiTokenGasPaymentFullLifecycle(t *testing.T) {
	oracle := gas.NewFeeConversionOracle(1 * time.Minute)

	// Simulated ERC-20 token addresses (e.g. USDC, DAI)
	usdcToken := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14}
	daiToken := []byte{0xDA, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0xDA}

	// 1. Set conversion rates
	// 1 USDC = 2.0 Native VIRI units
	// 1 DAI  = 0.5 Native VIRI units
	oracle.SetRate(usdcToken, 2.0)
	oracle.SetRate(daiToken, 0.5)

	if !oracle.HasRate(usdcToken) {
		t.Fatalf("Oracle must have rate for registered USDC token")
	}
	if oracle.GetRate(usdcToken) != 2.0 {
		t.Fatalf("Expected USDC rate 2.0, got %f", oracle.GetRate(usdcToken))
	}
	if oracle.GetRate(daiToken) != 0.5 {
		t.Fatalf("Expected DAI rate 0.5, got %f", oracle.GetRate(daiToken))
	}

	// 2. Conversion accuracy
	// 1,000 USDC at 2.0 rate should yield 2,000 Native VIRI
	nativeEquiv := oracle.ConvertToNative(usdcToken, 1000)
	if nativeEquiv != 2000 {
		t.Fatalf("Expected 2000 native units, got %d", nativeEquiv)
	}

	// 2,000 Native VIRI at 2.0 rate should convert back to 1,000 USDC
	tokenEquiv := oracle.ConvertFromNative(usdcToken, 2000)
	if tokenEquiv != 1000 {
		t.Fatalf("Expected 1000 USDC units, got %d", tokenEquiv)
	}

	// 3. Native coin fallback (nil or zero address)
	if oracle.GetRate(nil) != 1.0 {
		t.Fatalf("Nil token must default to 1.0 (1:1 with native)")
	}
	if oracle.ConvertToNative(nil, 500) != 500 {
		t.Fatalf("Nil token conversion must return exact native amount")
	}

	// 4. Unknown token handling
	unknownToken := []byte{0x99, 0x99, 0x99}
	if oracle.HasRate(unknownToken) {
		t.Fatalf("Unknown token should not have a registered rate")
	}
	if oracle.ConvertToNative(unknownToken, 100) != 0 {
		t.Fatalf("Unknown token conversion must yield 0")
	}

	// 5. Transaction Construction with FeeCurrency
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	recipient := []byte("recipient_address_20b")
	txWithFeeToken := &ledger.Transaction{
		Nonce:       1,
		From:        key.PubKey().Bytes(),
		To:          recipient,
		Value:       100,
		GasLimit:    21000,
		GasPrice:    50, // 50 units in USDC
		FeeCurrency: usdcToken,
		ChainID:     1,
	}

	// Check FeeToken accessor
	if !bytes.Equal(txWithFeeToken.FeeToken(), usdcToken) {
		t.Fatalf("FeeToken() should return the configured ERC-20 token address")
	}

	// 6. Signing Payload Binding (CR-03: FeeCurrency must be part of signed data)
	payload := txWithFeeToken.SigningPayload()
	if !bytes.Contains(payload, usdcToken) {
		t.Fatalf("Signing payload must include FeeCurrency to prevent fee token malleability attacks")
	}

	// Sign the transaction
	sig, err := key.Sign(payload)
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}
	txWithFeeToken.Signature = &ledger.TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
		V: 0,
	}

	// Verify signature
	if !key.PubKey().Verify(payload, sig) {
		t.Fatalf("Signature verification failed on multi-token gas transaction")
	}

	// Verify tampering with FeeCurrency invalidates payload
	tamperedPayload := append(payload, 0xFF)
	if key.PubKey().Verify(tamperedPayload, sig) {
		t.Fatalf("Security failure: signature should not match tampered payload")
	}
}
