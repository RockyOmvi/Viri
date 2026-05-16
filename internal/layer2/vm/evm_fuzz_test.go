package vm

import (
	"math/big"
	"math/rand"
	"testing"
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
