package teechain

import (
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

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

	runtime := contracts.NewRuntimeWithVersion(height, time.Now(), txn, address, chaintypes.UniAddr{
		V: app.dkg.SystemPubKey().Byte(),
	}, version)

	err = contracts.Mutation(version, call, runtime)
	return err
}
