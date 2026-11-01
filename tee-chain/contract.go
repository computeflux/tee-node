package teechain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// runtimeUpgradeSelector 是 gov.runtime_upgrade 的方法选择器（mutates=true）。
var runtimeUpgradeSelector = store.MethodToSelectorBytes("runtime_upgrade")

// runtimeUpgradeTargetVersion 若 call 是 gov.runtime_upgrade，返回其目标版本。
//
// PendingRuntimeUpgrade 的 SCALE 编码以 Version(u32, LE) 开头（见
// ext/runtimeupgrade 的 scaleEncodePendingRuntimeUpgrade 与 gov_gen.go 的
// runtime_upgrade 分发），因此直接读 args[0] 前 4 字节即可，无需完整解码。
func runtimeUpgradeTargetVersion(call store.ContractCall) (uint32, bool) {
	if string(call.GetName()) != "gov" {
		return 0, false
	}
	if !bytes.Equal(call.GetMethod(), runtimeUpgradeSelector) {
		return 0, false
	}
	args := call.GetArgs()
	if len(args) == 0 || len(args[0]) < 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(args[0][:4]), true
}

// nativeVersionForCall 选择本次调用使用的运行时版本。
//
// 运行时升级的 WASM 载荷（400KB+）远超 WASM 兼容路径的 256KB 共享缓冲区，
// 经 WASM 执行会被截断，导致链上留下 gunzip unexpected EOF 的残缺 blob。
// 若本二进制恰好就是升级目标版本，直接走原生路径（本二进制实现即目标版本
// 的运行时），保证 WASM 与版本元数据完整落库。
func nativeVersionForCall(version uint32, call store.ContractCall) uint32 {
	if target, ok := runtimeUpgradeTargetVersion(call); ok && target == module.NativeVersion {
		return target
	}
	return version
}

// ContractRuntime 构造一个新的合约运行时上下文，供外部包（gateway、graph 等）使用。
// 调用者负责 defer runtime.GetTxn().Rollback()。
func (app *TeeChain) ContractRuntime() (*contracts.Runtime, error) {
	return app.ContractRuntimeWithCaller(chaintypes.UniAddr{})
}

// ContractRuntimeWithCaller 构造一个指定调用方 caller 的合约运行时上下文。
// 主要用于 GetProviderByOwner 等依赖 caller 身份的查询。
func (app *TeeChain) ContractRuntimeWithCaller(caller chaintypes.UniAddr) (*contracts.Runtime, error) {
	if dbstore.SECRET_DB == nil {
		return nil, fmt.Errorf("SECRET_DB is nil")
	}
	txn := dbstore.SECRET_DB.NewTransaction()
	// NOTE: caller does NOT call txn.Commit() for read-only queries.
	// The caller must defer txn.Rollback().

	var sudo chaintypes.UniAddr
	if app.dkg != nil && app.dkg.SystemPubKey() != nil {
		sudo = chaintypes.UniAddr{V: app.dkg.SystemPubKey().Byte()}
	}

	version, err := GetRuntimeVersionAtHeight(txn, app.state.Height)
	if err != nil {
		txn.Rollback()
		return nil, fmt.Errorf("get runtime version: %w", err)
	}

	return contracts.NewRuntimeWithVersion(app.state.Height, time.Now(), txn, caller, sudo, version), nil
}

// WithRuntime 在只读合约 runtime 上执行 fn，并统一处理创建与回滚。
func WithRuntime[T any](chain *TeeChain, fn func(*contracts.Runtime) (T, error)) (T, error) {
	return WithRuntimeCaller(chain, chaintypes.UniAddr{}, fn)
}

// WithRuntimeCaller 在指定 caller 的只读合约 runtime 上执行 fn，并统一处理创建与回滚。
func WithRuntimeCaller[T any](chain *TeeChain, caller chaintypes.UniAddr, fn func(*contracts.Runtime) (T, error)) (T, error) {
	var zero T
	runtime, err := chain.ContractRuntimeWithCaller(caller)
	if err != nil {
		return zero, err
	}
	defer runtime.GetTxn().Rollback()
	return fn(runtime)
}

// ContractMutation 执行合约状态变更（共识层 ProcessProposal 调用）。
func (app *TeeChain) ContractMutation(caller []byte, callerType uint32, txn *dbstore.Txn, t time.Time, height int64, call store.ContractCall) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ContractMutation recovered from panic: %v", r)
		}
	}()

	address := chaintypes.UniAddr{
		T: callerType,
		V: caller,
	}

	pub, err := app.GetDkgPubkey()
	if err != nil {
		return fmt.Errorf("get dkg pubkey: %w", err)
	}
	if pub == nil {
		return fmt.Errorf("dkg pubkey is nil")
	}

	version, err := GetRuntimeVersionAtHeight(txn, height)
	if err != nil {
		return fmt.Errorf("get runtime version: %w", err)
	}
	version = nativeVersionForCall(version, call)

	runtime := contracts.NewRuntimeWithVersion(height, t, txn, address, chaintypes.UniAddr{
		V: pub.ToBytes(),
	}, version)

	err = contracts.Mutation(version, call, runtime)
	return err
}

// ContractQuery 只读合约查询。
func (app *TeeChain) ContractQuery(caller []byte, callerType uint32, contract string, method string, args [][]byte) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			data = nil
			err = fmt.Errorf("ContractQuery recovered from panic: %v", r)
		}
	}()

	height := app.state.Height
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	address := chaintypes.UniAddr{
		T: callerType,
		V: caller,
	}

	version, err := GetRuntimeVersionAtHeight(txn, height)
	if err != nil {
		return nil, fmt.Errorf("get runtime version: %w", err)
	}

	runtime := contracts.NewRuntimeWithVersion(height, time.Now(), txn, address, chaintypes.UniAddr{
		V: app.dkg.SystemPubKey().Byte(),
	}, version)

	data, err = contracts.Query(version, &model.ContractCall{
		Name:   []byte(contract),
		Method: store.MethodToSelectorBytes(method),
		Args:   args,
	}, runtime)

	return data, err
}

// ContractDryRun 干跑合约执行（不提交）。
func (app *TeeChain) ContractDryRun(caller []byte, callerType uint32, call store.ContractCall) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ContractDryRun recovered from panic: %v", r)
		}
	}()

	height := app.state.Height
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	address := chaintypes.UniAddr{
		T: callerType,
		V: caller,
	}

	version, err := GetRuntimeVersionAtHeight(txn, height)
	if err != nil {
		return fmt.Errorf("get runtime version: %w", err)
	}
	version = nativeVersionForCall(version, call)

	runtime := contracts.NewRuntimeWithVersion(height, time.Now(), txn, address, chaintypes.UniAddr{
		V: app.dkg.SystemPubKey().Byte(),
	}, version)

	err = contracts.Mutation(version, call, runtime)
	return err
}
