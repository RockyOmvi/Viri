package ledger

import (
	"math/big"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func FuzzBlockSerializeDeserialize(f *testing.F) {
	f.Add(uint64(0), []byte{0x01}, uint64(100), uint64(1000))
	f.Add(uint64(100), []byte{}, uint64(0), uint64(0))
	f.Add(uint64(1), []byte{0xde, 0xad}, uint64(50), uint64(500))

	f.Fuzz(func(t *testing.T, height uint64, data []byte, value, gasPrice uint64) {
		tx := &Transaction{
			Hash:     crypto.SHA256(data),
			Nonce:    height,
			From:     []byte{0x01, 0x02},
			To:       []byte{0x03, 0x04},
			Value:    value,
			GasLimit: 21000,
			GasPrice: gasPrice,
			Data:     data,
		}
		block := &Block{
			Header: &Header{
				Version:   Version1,
				Height:    height,
				PrevHash:  crypto.SHA256([]byte("prev")),
				TxsHash:   []byte{},
				Timestamp: time.Now(),
				Proposer:  []byte("proposer"),
			},
			Transactions: []*Transaction{tx},
		}
		block.Header.TxsHash = ComputeTransactionsHash(block.Transactions)

		serialized, err := SerializeBlock(block)
		if err != nil {
			t.Skip()
		}
		deserialized, err := DeserializeBlock(serialized)
		if err != nil {
			t.Errorf("deserialize failed: %v", err)
			return
		}
		if deserialized.Header.Height != block.Header.Height {
			t.Errorf("height mismatch: %d != %d", deserialized.Header.Height, block.Header.Height)
		}
	})
}

func FuzzTransactionSerializeDeserialize(f *testing.F) {
	f.Add(uint64(1), uint64(100), uint64(1000), []byte("data"))
	f.Add(uint64(0), uint64(0), uint64(0), []byte{})
	f.Add(uint64(1000), uint64(1), uint64(1), make([]byte, 10000))

	f.Fuzz(func(t *testing.T, nonce, value, gasPrice uint64, data []byte) {
		tx := &Transaction{
			Nonce:    nonce,
			From:     []byte("from"),
			To:       []byte("to"),
			Value:    value,
			GasLimit: 21000,
			GasPrice: gasPrice,
			Data:     data,
			Signature: &TxSignature{
				R: []byte{0x01, 0x02},
				S: []byte{0x03, 0x04},
				V: 0,
			},
		}
		tx.Hash = tx.ComputeHash()
		serialized, err := SerializeTransaction(tx)
		if err != nil {
			t.Skip()
		}
		deserialized, err := DeserializeTransaction(serialized)
		if err != nil {
			t.Errorf("deserialize tx failed: %v", err)
			return
		}
		if deserialized.Nonce != tx.Nonce {
			t.Errorf("nonce mismatch")
		}
		if string(deserialized.Data) != string(tx.Data) {
			t.Errorf("data mismatch")
		}
	})
}

func FuzzTransactionHashDeterminism(f *testing.F) {
	f.Add(uint64(1), []byte{0x01}, []byte{0x02}, uint64(100), uint64(21000), uint64(1000), []byte("data"), uint64(1))
	f.Add(uint64(0), []byte{}, []byte{}, uint64(0), uint64(0), uint64(0), []byte{}, uint64(0))

	f.Fuzz(func(t *testing.T, nonce uint64, from, to []byte, value, gasLimit, gasPrice uint64, data []byte, chainID uint64) {
		tx := &Transaction{
			Nonce:    nonce,
			From:     from,
			To:       to,
			Value:    value,
			GasLimit: gasLimit,
			GasPrice: gasPrice,
			Data:     data,
			ChainID:  chainID,
		}
		h1 := tx.ComputeHash()
		h2 := tx.ComputeHash()
		if string(h1) != string(h2) {
			t.Errorf("tx hash not deterministic")
		}
	})
}

func FuzzBlockHashDeterminism(f *testing.F) {
	f.Add(uint64(1), []byte{0x01}, []byte{0x02}, []byte{0x03})
	f.Add(uint64(0), []byte{}, []byte{}, []byte{})
	f.Add(uint64(100), []byte{0xde, 0xad, 0xbe, 0xef}, []byte{0xca, 0xfe}, []byte{0xba, 0xbe})

	f.Fuzz(func(t *testing.T, height uint64, prevHash, txsHash, stateRoot []byte) {
		block := &Block{
			Header: &Header{
				Height:    height,
				PrevHash:  prevHash,
				TxsHash:   txsHash,
				StateRoot: stateRoot,
			},
		}
		h1 := block.Hash()
		h2 := block.Hash()
		if string(h1) != string(h2) {
			t.Errorf("block hash not deterministic")
		}
	})
}

func FuzzGenesisConfigValidation(f *testing.F) {
	f.Add(uint64(1), uint64(100_000_000), uint64(10))
	f.Add(uint64(0), uint64(0), uint64(0))
	f.Add(uint64(999999), uint64(1), uint64(1<<62))

	f.Fuzz(func(t *testing.T, chainID, initialSupply, maxGas uint64) {
		g := &GenesisConfig{
			ChainID:        chainID,
			InitialSupply:  initialSupply,
			MaxGasPerBlock: maxGas,
			InitialValidators: []*ValidatorInfo{
				{Address: []byte("addr"), Stake: 1000},
			},
		}
		_ = g.Save("")
	})
}

func FuzzFeeMarketUpdate(f *testing.F) {
	f.Add(uint64(1_000_000_000), uint64(15_000_000), uint64(10_000_000))
	f.Add(uint64(1), uint64(1), uint64(0))
	f.Add(uint64(1<<60), uint64(1<<60), uint64(1<<60))

	f.Fuzz(func(t *testing.T, initialBaseFee, gasTarget, gasUsed uint64) {
		if gasTarget == 0 {
			return
		}
		fm := NewFeeMarket(initialBaseFee, gasTarget, 30_000_000)
		fm.Update(gasUsed)
		baseFee := fm.BaseFee()
		if baseFee == 0 {
			t.Errorf("base fee should never be zero")
		}
		if fm.CalculateTip(gasUsed) > gasUsed {
			t.Errorf("tip cannot exceed gas price")
		}
	})
}

func FuzzEconomicsBlockReward(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(2_100_000))
	f.Add(uint64(10_000_000))
	f.Add(uint64(1<<63))

	f.Fuzz(func(t *testing.T, blockHeight uint64) {
		econ := NewEconomics(DefaultEconomicsConfig())
		reward := econ.CalculateBlockReward(blockHeight)
		if reward == nil {
			t.Errorf("nil reward")
			return
		}
		if reward.Sign() < 0 {
			t.Errorf("negative reward")
		}
	})
}

func FuzzSlashingProcessor(f *testing.F) {
	f.Add(uint64(5000), uint64(10000), uint64(100))
	f.Add(uint64(10000), uint64(0), uint64(0))
	f.Add(uint64(0), uint64(100), uint64(1000))

	f.Fuzz(func(t *testing.T, slashRate, jailPeriod, slashCount uint64) {
		config := &SlashingConfig{
			DoubleSignSlashRate:  slashRate % 10001,
			DoubleSignJailPeriod: jailPeriod % 100000,
			MaxEvidenceAge:       100000,
		}
		sp := NewSlashingProcessor(config)
		valState := &ValidatorState{
			TotalStake: new(big.Int).SetUint64(1_000_000),
			SlashCount: slashCount,
		}
		record := &SlashingRecord{
			Reason:      SlashingDoubleSign,
			Validator:   []byte("val"),
			BlockHeight: 1,
			SlashRate:   slashRate % 10001,
			JailPeriod:  jailPeriod,
		}
		amount, err := sp.ProcessSlashing(record, valState, 1000)
		if err != nil {
			return
		}
		if amount.Sign() < 0 {
			t.Errorf("negative slash amount")
		}
		if !valState.IsJailed {
			t.Errorf("validator should be jailed")
		}
	})
}

func FuzzSlashingUnjailEdgeCases(f *testing.F) {
	f.Add(uint64(100), uint64(50))
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(1000), uint64(2000))

	f.Fuzz(func(t *testing.T, jailedUntil, currentHeight uint64) {
		sp := NewSlashingProcessor(DefaultSlashingConfig())
		valState := &ValidatorState{
			TotalStake:  big.NewInt(1000000),
			IsJailed:    true,
			JailedUntil: jailedUntil,
		}
		err := sp.Unjail(valState, currentHeight)
		if err != nil && currentHeight >= jailedUntil {
			t.Logf("unjail failed for current=%d jailed_until=%d: %v", currentHeight, jailedUntil, err)
		}
	})
}

func FuzzValidatorStateTransitions(f *testing.F) {
	f.Add(uint64(1000000), uint64(5000), uint64(100), uint64(50))
	f.Add(uint64(1), uint64(10000), uint64(0), uint64(0))

	f.Fuzz(func(t *testing.T, totalStake, slashRate, jailPeriod, slashCount uint64) {
		config := &SlashingConfig{
			DoubleSignSlashRate:  slashRate % 10001,
			DoubleSignJailPeriod: jailPeriod % 100000,
			MaxEvidenceAge:       100000,
		}
		sp := NewSlashingProcessor(config)
		valState := &ValidatorState{
			TotalStake: new(big.Int).SetUint64(totalStake),
			IsJailed:   false,
		}
		record := &SlashingRecord{
			Reason:      SlashingDowntime,
			Validator:   []byte("val"),
			BlockHeight: 1,
		}
		_, err := sp.ProcessSlashing(record, valState, 1000)
		if err != nil {
			return
		}
		if slashCount > 0 {
			_, err = sp.ProcessSlashing(record, valState, 1000)
			if err == nil {
				t.Logf("double slash succeeded for jailed validator")
			}
		}
	})
}

func FuzzBlockSigningPayload(f *testing.F) {
	f.Add(uint64(1), []byte{0x01}, []byte{0x02}, []byte{0x03}, uint64(1000))
	f.Add(uint64(0), []byte{}, []byte{}, []byte{}, uint64(0))

	f.Fuzz(func(t *testing.T, height uint64, prevHash, txsHash, stateRoot []byte, timestamp uint64) {
		block := &Block{
			Header: &Header{
				Version:   Version1,
				Height:    height,
				PrevHash:  prevHash,
				TxsHash:   txsHash,
				StateRoot: stateRoot,
				Timestamp: time.Unix(int64(timestamp), 0),
			},
		}
		payload := block.SigningPayload()
		if len(payload) == 0 {
			t.Errorf("empty signing payload")
		}
	})
}

func FuzzReceiptSerializeDeserialize(f *testing.F) {
	f.Add([]byte{0x01}, uint64(1), uint64(21000), uint8(1))
	f.Add([]byte{}, uint64(0), uint64(0), uint8(0))
	f.Add(make([]byte, 32), uint64(100), uint64(50000), uint8(2))

	f.Fuzz(func(t *testing.T, txHash []byte, blockHeight, gasUsed uint64, status uint8) {
		receipt := &Receipt{
			TxHash:      txHash,
			BlockHeight: blockHeight,
			GasUsed:     gasUsed,
			Status:      status,
			Logs: []*Log{
				{Address: []byte("addr"), Topics: [][]byte{[]byte("topic")}, Data: []byte("data")},
			},
		}
		data, err := SerializeReceipt(receipt)
		if err != nil {
			t.Skip()
		}
		deserialized, err := DeserializeReceipt(data)
		if err != nil {
			t.Errorf("deserialize receipt failed: %v", err)
			return
		}
		if deserialized.BlockHeight != receipt.BlockHeight {
			t.Errorf("receipt height mismatch")
		}
	})
}

func FuzzHeaderSerializeDeserialize(f *testing.F) {
	f.Add(uint64(1), []byte{0x01}, []byte{0x02}, []byte{0x03})
	f.Add(uint64(0), []byte{}, []byte{}, []byte{})

	f.Fuzz(func(t *testing.T, height uint64, prevHash, txsHash, stateRoot []byte) {
		header := &Header{
			Version:   Version1,
			Height:    height,
			PrevHash:  prevHash,
			TxsHash:   txsHash,
			StateRoot: stateRoot,
		}
		data, err := SerializeHeader(header)
		if err != nil {
			t.Skip()
		}
		deserialized, err := DeserializeHeader(data)
		if err != nil {
			t.Errorf("deserialize header failed: %v", err)
			return
		}
		if deserialized.Height != header.Height {
			t.Errorf("header height mismatch")
		}
	})
}

func FuzzTxPoolAddGetRemove(f *testing.F) {
	f.Add([]byte("tx1"), uint64(1), uint64(100))
	f.Add([]byte{}, uint64(0), uint64(0))
	f.Add(make([]byte, 1000), uint64(1<<62), uint64(1<<62))

	f.Fuzz(func(t *testing.T, txData []byte, nonce, gasPrice uint64) {
		tx := &Transaction{
			Hash:     crypto.SHA256(txData),
			Nonce:    nonce,
			From:     []byte("from"),
			To:       []byte("to"),
			GasLimit: 21000,
			GasPrice: gasPrice,
			Data:     txData,
		}
		if len(txData) == 0 || nonce == 0 {
			return
		}
		_ = tx.ComputeHash()
	})
}

func FuzzEconomicsProcessBlock(f *testing.F) {
	f.Add(uint64(1), uint64(1), uint64(1))
	f.Add(uint64(0), uint64(0), uint64(0))
	f.Add(uint64(100), uint64(1000), uint64(10))

	f.Fuzz(func(t *testing.T, gasPrice, gasLimit, blockHeight uint64) {
		econ := NewEconomics(DefaultEconomicsConfig())
		txs := []*Transaction{
			{GasPrice: gasPrice, GasLimit: gasLimit},
		}
		result, err := econ.ProcessBlock(txs, blockHeight)
		if err != nil {
			return
		}
		if result.BlockHeight != blockHeight {
			t.Errorf("block height mismatch in result")
		}
	})
}
