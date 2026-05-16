package vm

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type WASMContext struct {
	Caller   []byte
	Address  []byte
	Value    *big.Int
	GasLimit uint64
	Data     []byte
}

type WASMExecutor struct {
	ctx   *WASMContext
	state EVMState
}

func NewWASMExecutor(ctx *WASMContext, state EVMState) *WASMExecutor {
	return &WASMExecutor{ctx: ctx, state: state}
}

func (w *WASMExecutor) Execute(code []byte) ([]byte, uint64, error) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	envBuilder := runtime.NewHostModuleBuilder("env")

	var gasCounter uint64

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, offset uint32, length uint32) uint32 {
			data, ok := m.Memory().Read(offset, length)
			if !ok {
				return 0
			}
			result := w.state.GetCode(data)
			if len(result) > 0 {
				if ok := m.Memory().Write(offset, result); !ok {
					return 0
				}
			}
			gasCounter += 20
			return uint32(len(result))
		}).
		Export("getCode")

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrOffset uint32, addrLen uint32, keyOffset uint32, keyLen uint32, outOffset uint32) uint32 {
			addr, ok := m.Memory().Read(addrOffset, addrLen)
			if !ok {
				return 0
			}
			key, ok := m.Memory().Read(keyOffset, keyLen)
			if !ok {
				return 0
			}
			val := w.state.GetStorage(addr, key)
			if len(val) > 0 {
				if ok := m.Memory().Write(outOffset, val); !ok {
					return 0
				}
			}
			gasCounter += 100
			return uint32(len(val))
		}).
		Export("getStorage")

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrOffset uint32, addrLen uint32, keyOffset uint32, keyLen uint32, valOffset uint32, valLen uint32) {
			addr, ok := m.Memory().Read(addrOffset, addrLen)
			if !ok {
				return
			}
			key, ok := m.Memory().Read(keyOffset, keyLen)
			if !ok {
				return
			}
			val, ok := m.Memory().Read(valOffset, valLen)
			if !ok {
				return
			}
			w.state.SetStorage(addr, key, val)
			gasCounter += 20000
		}).
		Export("setStorage")

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrOffset uint32, addrLen uint32, outOffset uint32) uint32 {
			addr, ok := m.Memory().Read(addrOffset, addrLen)
			if !ok {
				return 0
			}
			balance := w.state.GetBalance(addr)
			if balance == nil {
				balance = big.NewInt(0)
			}
			balBytes := balance.Bytes()
			if ok := m.Memory().Write(outOffset, balBytes); !ok {
				return 0
			}
			gasCounter += 20
			return uint32(len(balBytes))
		}).
		Export("getBalance")

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, nonceOutOffset uint32) {
			nonce := w.state.GetNonce(w.ctx.Address)
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], nonce)
			m.Memory().Write(nonceOutOffset, buf[:])
			gasCounter += 5
		}).
		Export("getNonce")

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, fromOff uint32, fromLen uint32, toOff uint32, toLen uint32, valueLow uint64, valueHigh uint64) uint32 {
			from, ok := m.Memory().Read(fromOff, fromLen)
			if !ok {
				return 0
			}
			to, ok := m.Memory().Read(toOff, toLen)
			if !ok {
				return 0
			}
			val := new(big.Int).SetUint64(valueLow)
			if valueHigh > 0 {
				high := new(big.Int).Lsh(big.NewInt(int64(valueHigh)), 64)
				val.Add(val, high)
			}
			w.state.Transfer(from, to, val)
			gasCounter += 900
			return 1
		}).
		Export("transfer")

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrOffset uint32, addrLen uint32) uint32 {
			addr, ok := m.Memory().Read(addrOffset, addrLen)
			if !ok {
				return 0
			}
			w.state.CreateAccount(addr)
			gasCounter += 25000
			return 1
		}).
		Export("createAccount")

	envBuilder.NewFunctionBuilder().
		WithFunc(func() int32 {
			return int32(w.ctx.GasLimit) - int32(gasCounter)
		}).
		Export("gasLeft")

	envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, logOff uint32, logLen uint32, topicCount uint32) {
			data, ok := m.Memory().Read(logOff, logLen)
			if !ok {
				return
			}
			topics := make([][]byte, topicCount)
			w.state.AddLog(w.ctx.Address, topics, data)
			gasCounter += 375
		}).
		Export("emitLog")

	if _, err := envBuilder.Instantiate(ctx); err != nil {
		return nil, 0, fmt.Errorf("wasm env instantiate: %w", err)
	}

	module, err := runtime.InstantiateWithConfig(ctx, code,
		wazero.NewModuleConfig().WithName("contract").WithStartFunctions())
	if err != nil {
		return nil, 0, fmt.Errorf("wasm instantiate: %w", err)
	}
	defer module.Close(ctx)

	inputLen := uint32(len(w.ctx.Data))
	if inputLen > 0 {
		if !module.Memory().Write(0, w.ctx.Data) {
			return nil, gasCounter, fmt.Errorf("wasm memory write failed")
		}
	}

	execute := module.ExportedFunction("execute")
	if execute == nil {
		execute = module.ExportedFunction("main")
	}
	if execute == nil {
		return nil, gasCounter, fmt.Errorf("wasm: no execute or main export")
	}

	gasCounter += 10
	results, err := execute.Call(ctx, uint64(inputLen))
	if err != nil {
		return nil, gasCounter, fmt.Errorf("wasm execute: %w", err)
	}

	var retData []byte
	if len(results) > 0 {
		retOffset := uint32(results[0] >> 32)
		retLen := uint32(results[0])
		if retLen > 0 {
			data, ok := module.Memory().Read(retOffset, retLen)
			if ok {
				retData = data
			}
		}
	}

	return retData, gasCounter, nil
}
