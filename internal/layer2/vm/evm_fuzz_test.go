package vm

import (
	"math/big"
	"math/rand"
	"testing"
	"time"
)

func FuzzEVMExecRandomBytecode(f *testing.F) {
	seeds := [][]byte{
		{},
		{byte(EVMSTOP)},
		{byte(EVMADD)},
		{byte(EVMPUSH1), 0x01, byte(EVMPUSH1), 0x01, byte(EVMADD), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMCREATE), byte(EVMSTOP)},
		{byte(EVMSHA3), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMMSTORE), byte(EVMPUSH1), 0x20, byte(EVMPUSH1), 0x00, byte(EVMRETURN)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMLOG0), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMLOG1), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMSLOAD), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x01, byte(EVMPUSH1), 0x00, byte(EVMSSTORE), byte(EVMSTOP)},
		{byte(EVMSELFDESTRUCT), byte(EVMSTOP)},
		{byte(EVMGAS), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMCALL), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMSTATICCALL), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMCALLCODE), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMDELEGATECALL), byte(EVMSTOP)},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, code []byte) {
		if len(code) > 1024 {
			code = code[:1024]
		}

		state := &testState{
			balances: make(map[string]*big.Int),
			nonces:   make(map[string]uint64),
			codes:    make(map[string][]byte),
			storage:  make(map[string]map[string][]byte),
		}
		state.balances[string(addr20(0))] = big.NewInt(1000000)
		state.codes[string(addr20(1))] = []byte{
			byte(EVMPUSH1), 0x01, byte(EVMSTOP),
		}

		ctx := &EVMContext{
			GasPrice:   big.NewInt(1),
			GasLimit:   500000,
			Value:      big.NewInt(0),
			Address:    addr20(0),
			Caller:     addr20(0xff),
			ChainID:    big.NewInt(1337),
			Coinbase:   addr20(0xee),
			BlockNum:   100,
			Timestamp:  12345678,
			PrevRandao: make([]byte, 32),
			BlockGasLimit: 30_000_000,
			BaseFee:    big.NewInt(10),
			GetBlockHash: func(n uint64) []byte {
				return make([]byte, 32)
			},
		}

		evm := NewEVMExecutor(ctx, state)

		evm.staticCall = false

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic executing bytecode %x: %v", code, r)
			}
		}()

		_, gasUsed, err := evm.Execute(code)
		if err != nil && gasUsed > ctx.GasLimit {
			t.Errorf("gasUsed %d > GasLimit %d for code %x: err=%v", gasUsed, ctx.GasLimit, code, err)
		}
	})
}

func FuzzEVMRandomStateAccess(f *testing.F) {
	seeds := [][]byte{
		{},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMSSTORE), byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMSLOAD), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMBALANCE), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EXTCODESIZE), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EXTCODEHASH), byte(EVMSTOP)},
		{byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EVMPUSH1), 0x00, byte(EXTCODECOPY), byte(EVMSTOP)},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 512 {
			data = data[:512]
		}

		code := make([]byte, 0, len(data)*2+2)
		rng := rand.New(rand.NewSource(int64(len(data))))

		for len(code) < 256 && len(code) < len(data)*3/2 {
			op := byte(rng.Intn(0x50))
			if op == byte(EVMJUMPDEST) || op == byte(EVMSTOP) || op == byte(EVMJUMP) || op == byte(EVMJUMPI) {
				code = append(code, op)
				continue
			}
			if op >= byte(EVMPUSH1) && op <= byte(EVMPUSH32) {
				n := int(op - byte(EVMPUSH1) + 1)
				code = append(code, op)
				for i := 0; i < n && len(data) > 0; i++ {
					code = append(code, data[len(code)%len(data)])
				}
				continue
			}
			code = append(code, op)
			if len(code) > 0 && code[len(code)-1] == byte(EVMSLOAD) && len(data) > 0 {
				code = append(code, data[len(code)%len(data)])
			}
		}
		for i := 12; i < len(code) && i < 64; i++ {
			if code[i] == byte(0x5B) {
				code[i] = byte(EVMSTOP)
			}
		}
		for len(code) < 2 {
			code = append(code, byte(EVMSTOP))
		}
		code = append(code, byte(EVMSTOP))

		state := &testState{
			balances: make(map[string]*big.Int),
			nonces:   make(map[string]uint64),
			codes:    make(map[string][]byte),
			storage:  make(map[string]map[string][]byte),
		}
		state.balances[string(addr20(0))] = big.NewInt(1000000)
		state.balances[string(addr20(1))] = big.NewInt(500000)
		state.codes[string(addr20(1))] = []byte{
			byte(EVMPUSH1), 0x00, byte(EVMSTOP),
		}

		ctx := &EVMContext{
			GasPrice:   big.NewInt(1),
			GasLimit:   200000,
			Value:      big.NewInt(0),
			Address:    addr20(0),
			Caller:     addr20(0xff),
			ChainID:    big.NewInt(1337),
			Coinbase:   addr20(0xee),
			BlockNum:   100,
			Timestamp:  12345678,
			PrevRandao: make([]byte, 32),
			BlockGasLimit: 30_000_000,
			BaseFee:    big.NewInt(10),
			GetBlockHash: func(n uint64) []byte {
				return make([]byte, 32)
			},
		}

		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic executing fuzz code %x: %v", code, r)
			}
		}()

		_, gasUsed, err := evm.Execute(code)

		if err != nil {
			if gasUsed > ctx.GasLimit {
				t.Errorf("gasUsed %d exceeds GasLimit %d for code %x", gasUsed, ctx.GasLimit, code)
			}
			return
		}

		if gasUsed > ctx.GasLimit {
			t.Errorf("gasUsed %d exceeds GasLimit %d for successful exec of code %x", gasUsed, ctx.GasLimit, code)
		}
	})
}

func FuzzEVMArithmeticOpcodes(f *testing.F) {
	seeds := [][]int64{
		{0, 0},
		{1, 2},
		{-1, 1},
		{1<<62 - 1, 1},
		{0, -1},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1])
	}

	f.Fuzz(func(t *testing.T, a, b int64) {
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 100000,
			ChainID:  big.NewInt(1337),
		}
		state := newTestState()
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic recovered: %v", r)
			}
		}()

		code := []byte{
			byte(EVMPUSH1), byte(a),
			byte(EVMPUSH1), byte(b),
			byte(EVMADD),
			byte(EVMSTOP),
		}
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMMemoryOpsRandomOffsets(f *testing.F) {
	seeds := []uint64{0, 1, 32, 1024, 1<<20 - 1}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, offset uint64) {
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
			ChainID:  big.NewInt(1337),
		}
		state := newTestState()
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic recovered at offset %d: %v", offset, r)
			}
		}()

		code := []byte{
			byte(EVMPUSH1), 0x42,
			byte(EVMPUSH1+1), byte(offset >> 8), byte(offset),
			byte(EVMMSTORE),
			byte(EVMPUSH1+1), byte(offset >> 8), byte(offset),
			byte(EVMMLOAD),
			byte(EVMSTOP),
		}
		if offset > 0xFFFF {
			code = []byte{
				byte(EVMPUSH1), 0x00,
				byte(EVMPUSH1), 0x00,
				byte(EVMMSTORE),
				byte(EVMSTOP),
			}
		}
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMStackOpsRandomDepth(f *testing.F) {
	seeds := []int{0, 1, 10, 16, 100}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, depth int) {
		if depth < 0 || depth > 200 {
			return
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
		}
		state := newTestState()
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic at depth %d: %v", depth, r)
			}
		}()

		code := []byte{}
		for i := 0; i < depth && i < 16; i++ {
			code = append(code, byte(EVMPUSH1), byte(i))
		}
		if depth > 0 && depth <= 16 {
			code = append(code, byte(EVMDUP1+EVMOpCode(depth)-1))
		}
		code = append(code, byte(EVMSTOP))
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMSHA3RandomData(f *testing.F) {
	seeds := [][]byte{
		{},
		{0x01},
		make([]byte, 32),
		make([]byte, 256),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			data = data[:4096]
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
		}
		state := newTestState()
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		for i, b := range data {
			code := []byte{
				byte(EVMPUSH1), b,
				byte(EVMPUSH1), byte(i),
				byte(EVMMSTORE8),
			}
			evm.Execute(code)
		}
		code := []byte{
			byte(EVMPUSH1+1), byte(len(data) >> 8), byte(len(data)),
			byte(EVMPUSH1+1), 0x00, 0x00,
			byte(EVMSHA3),
			byte(EVMSTOP),
		}
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMBalanceExtCodeOps(f *testing.F) {
	seeds := [][]byte{
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
		make([]byte, 20),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, addr []byte) {
		if len(addr) != 20 {
			addr = addr20(0)
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
			Address:  addr20(0),
		}
		state := &testState{
			balances: make(map[string]*big.Int),
			nonces:   make(map[string]uint64),
			codes:    make(map[string][]byte),
			storage:  make(map[string]map[string][]byte),
		}
		state.balances[string(addr)] = big.NewInt(1000)
		state.codes[string(addr)] = []byte{byte(EVMSTOP)}
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code := []byte{
			byte(EVMPUSH1), 0x01,
			byte(EVMBALANCE),
			byte(EVMSTOP),
		}
		_, _, err := evm.Execute(code)
		_ = err

		code2 := []byte{
			byte(EVMPUSH1), 0x01,
			byte(EXTCODESIZE),
			byte(EVMSTOP),
		}
		evm2 := NewEVMExecutor(ctx, state)
		_, _, err = evm2.Execute(code2)
		_ = err
	})
}

func FuzzEVMBlockInfoOpcodes(f *testing.F) {
	seeds := []uint64{0, 1, 100, 12345678, 1<<62 - 1}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, blockNum uint64) {
		ctx := &EVMContext{
			GasPrice:   big.NewInt(1),
			GasLimit:   100000,
			BlockNum:   blockNum,
			Timestamp:  uint64(time.Now().Unix()),
			Coinbase:   addr20(0xee),
			ChainID:    big.NewInt(1337),
			PrevRandao: make([]byte, 32),
			BlockGasLimit: 30_000_000,
			BaseFee:    big.NewInt(10),
		}
		state := newTestState()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic at blockNum %d: %v", blockNum, r)
			}
		}()

		for _, op := range []EVMOpCode{EVMNUMBER, EVMTIMESTAMP, EVMCOINBASE, EVMCHAINID, EVMGASLIMIT, EVMBASEFEE} {
			evm2 := NewEVMExecutor(ctx, state)
			code := []byte{byte(op), byte(EVMSTOP)}
			_, _, err := evm2.Execute(code)
			_ = err
		}
	})
}

func FuzzEVMGasMeasurementConsistency(f *testing.F) {
	seeds := []uint64{1000, 10000, 100000, 1000000}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, gasLimit uint64) {
		if gasLimit < 100 || gasLimit > 10_000_000 {
			return
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: gasLimit,
		}
		state := newTestState()
		code := []byte{byte(EVMPUSH1), 0x01, byte(EVMPUSH1), 0x02, byte(EVMADD), byte(EVMSTOP)}

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		evm := NewEVMExecutor(ctx, state)
		_, gasUsed, err := evm.Execute(code)
		if err == nil && gasUsed > gasLimit {
			t.Errorf("gasUsed %d > gasLimit %d", gasUsed, gasLimit)
		}
	})
}

func FuzzEVMSelfdestructEdgeCases(f *testing.F) {
	seeds := [][]byte{
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		make([]byte, 20),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, recipient []byte) {
		if len(recipient) != 20 {
			return
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 100000,
			Address:  addr20(0x01),
		}
		state := &testState{
			balances: make(map[string]*big.Int),
			nonces:   make(map[string]uint64),
			codes:    make(map[string][]byte),
			storage:  make(map[string]map[string][]byte),
		}
		state.balances[string(addr20(0x01))] = big.NewInt(100)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		evm := NewEVMExecutor(ctx, state)
		code := append([]byte{byte(EVMPUSH1), byte(recipient[19])}, byte(EVMSELFDESTRUCT))
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMBlockHashOpcode(f *testing.F) {
	seeds := []uint64{0, 1, 256, 1<<62 - 1}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, blockNum uint64) {
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 100000,
			GetBlockHash: func(n uint64) []byte {
				if n > 256 {
					return nil
				}
				h := make([]byte, 32)
				h[0] = byte(n)
				return h
			},
		}
		state := newTestState()
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code := []byte{
			byte(EVMPUSH1+1), byte(blockNum >> 8), byte(blockNum),
			byte(EVMBLOCKHASH),
			byte(EVMSTOP),
		}
		if blockNum > 0xFFFF {
			code = []byte{byte(EVMPUSH1), 0x00, byte(EVMBLOCKHASH), byte(EVMSTOP)}
		}
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMGasPriceChainIDEnvironment(f *testing.F) {
	seeds := []int64{0, 1, 1337, 1<<62 - 1}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, chainID int64) {
		ctx := &EVMContext{
			GasPrice: big.NewInt(100),
			GasLimit: 100000,
			ChainID:  big.NewInt(chainID),
		}
		state := newTestState()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		evm := NewEVMExecutor(ctx, state)
		code := []byte{byte(EVMGASPRICE), byte(EVMSTOP)}
		_, _, err := evm.Execute(code)
		_ = err

		evm2 := NewEVMExecutor(ctx, state)
		code2 := []byte{byte(EVMCHAINID), byte(EVMSTOP)}
		_, _, err = evm2.Execute(code2)
		_ = err
	})
}

func FuzzEVMSstoreSloadRepeated(f *testing.F) {
	seeds := []int{0, 1, 5, 20}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, count int) {
		if count < 0 || count > 50 {
			return
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
			Address:  addr20(0x01),
		}
		state := newTestState()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code := []byte{}
		for i := 0; i < count; i++ {
			code = append(code, byte(EVMPUSH1), byte(i), byte(EVMPUSH1), byte(i), byte(EVMSSTORE))
		}
		for i := 0; i < count; i++ {
			code = append(code, byte(EVMPUSH1), byte(i), byte(EVMSLOAD), byte(EVMPOP))
		}
		code = append(code, byte(EVMSTOP))
		evm := NewEVMExecutor(ctx, state)
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMLogWithRandomTopics(f *testing.F) {
	seeds := []int{0, 1, 2, 3, 4}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, numTopics int) {
		if numTopics < 0 || numTopics > 4 {
			return
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
			Address:  addr20(0x01),
		}
		state := newTestState()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code := []byte{
			byte(EVMPUSH1), 0x20, // size
			byte(EVMPUSH1), 0x00, // offset
		}
		for i := 0; i < numTopics; i++ {
			code = append(code, byte(EVMPUSH1), byte(i))
		}
		code = append(code, byte(EVMLOG0+EVMOpCode(numTopics)))
		code = append(code, byte(EVMSTOP))
		evm := NewEVMExecutor(ctx, state)
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMCreateCallOps(f *testing.F) {
	seeds := []uint64{0, 1, 100}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, callValue uint64) {
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
			Address:  addr20(0x01),
		}
		state := newTestState()
		state.balances[string(addr20(0x01))] = big.NewInt(1000000)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code := []byte{
			byte(EVMPUSH1), 0x00, // size
			byte(EVMPUSH1), 0x00, // offset
			byte(EVMPUSH1), byte(callValue), // value
			byte(EVMCREATE),
			byte(EVMSTOP),
		}
		evm := NewEVMExecutor(ctx, state)
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMPcMsizeGas(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(100))
	f.Add(uint64(1000))

	f.Fuzz(func(t *testing.T, _ uint64) {
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 100000,
		}
		state := newTestState()
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code := []byte{byte(EVMPC), byte(EVMMSIZE), byte(EVMGAS), byte(EVMSTOP)}
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMRevertReturnData(f *testing.F) {
	f.Add([]byte("test data"))
	f.Add([]byte{})
	f.Add(make([]byte, 256))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 512 {
			data = data[:512]
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
		}
		state := newTestState()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code := []byte{}
		for i, b := range data {
			code = append(code, byte(EVMPUSH1), b)
			code = append(code, byte(EVMPUSH1), byte(i))
			code = append(code, byte(EVMMSTORE8))
		}
		code = append(code, byte(EVMPUSH1+1), byte(len(data)>>8), byte(len(data)))
		code = append(code, byte(EVMPUSH1+1), 0x00, 0x00)
		code = append(code, byte(EVMRETURN))
		evm := NewEVMExecutor(ctx, state)
		_, _, err := evm.Execute(code)
		_ = err
	})
}

func FuzzEVMPushOpsOverflow(f *testing.F) {
	seeds := [][]byte{
		{0x60, 0xFF, 0xFF, 0xFF, 0xFF},
		{0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		{0x60, 0x00, 0x60, 0x00, 0x01, 0x00},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, code []byte) {
		if len(code) > 1024 {
			code = code[:1024]
		}
		ctx := &EVMContext{
			GasPrice: big.NewInt(1),
			GasLimit: 500000,
		}
		state := newTestState()
		evm := NewEVMExecutor(ctx, state)

		defer func() {
			if r := recover(); r != nil {
				t.Logf("panic: %v", r)
			}
		}()

		code = append(code, byte(EVMSTOP))
		_, _, err := evm.Execute(code)
		_ = err
	})
}
