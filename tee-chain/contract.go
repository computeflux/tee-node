package teechain

import (
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	"github.com/computeflux/tee-provider/tee-chain/contracts/gateway"
)

// ── Gateway 非 codec 查询接口（供 HTTP 层直接调用）──────

func (app *TeeChain) gatewayRuntime() (*contracts.Runtime, error) {
	if model.DBINS == nil {
		return nil, fmt.Errorf("DBINS is nil")
	}
	txn := model.DBINS.NewTransaction()
	// NOTE: caller does NOT call txn.Commit() for read-only queries.
	// The caller must defer txn.Rollback().

	var sudo model.UniAddr
	if app.dkg != nil && app.dkg.SystemPubKey() != nil {
		sudo = model.UniAddr{V: app.dkg.SystemPubKey().Byte()}
	}
	return contracts.NewRuntime(app.state.Height, time.Now(), txn, model.UniAddr{}, sudo), nil
}

// GatewayLookupAPIKey 根据 keyHash 查询链上 API Key 信息。
func (app *TeeChain) GatewayLookupAPIKey(keyHash []byte) (*gateway.APIKeyInfo, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.LookupAPIKey(keyHash)
}

// GatewayListActiveModels 查询链上活跃模型列表。
func (app *TeeChain) GatewayListActiveModels(offset uint64, limit uint32) ([]gateway.ModelInfo, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.ListActiveModels(offset, limit)
}

// GatewayListActiveProviders 查询链上活跃 Provider 列表。
func (app *TeeChain) GatewayListActiveProviders(offset uint64, limit uint32) ([]gateway.ProviderInfo, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.ListActiveProviders(offset, limit)
}

// GatewayGetProvider 查询单个 Provider。
func (app *TeeChain) GatewayGetProvider(id uint64) (*gateway.ProviderInfo, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetProvider(id)
}

// GatewayGetModel 查询单个 Model。
func (app *TeeChain) GatewayGetModel(id uint64) (*gateway.ModelInfo, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetModel(id)
}

// GatewayGetModelByName 按名称查询活跃模型（走 string exact 索引，O(1) 查询）。
func (app *TeeChain) GatewayGetModelByName(pid uint64, name string) (*gateway.DataWithID[gateway.ModelInfo], error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetModelByName(pid, name)
}

// GatewayGetProviderByName 按名称查询 Provider（走 string exact 索引，O(1) 查询）。
func (app *TeeChain) GatewayGetProviderByName(name string) (*gateway.DataWithID[gateway.ProviderInfo], error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetProviderByName(name)
}

// GatewayGetEncryptedAPIKey 查询加密 API Key。
func (app *TeeChain) GatewayGetEncryptedAPIKey(ref string) (*gateway.EncryptedAPIKey, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetEncryptedAPIKey(ref)
}

// GatewayGetRechargeHistory 查询用户充值历史。
func (app *TeeChain) GatewayGetRechargeHistory(user model.UniAddr, offset uint64, limit uint32) ([]gateway.RechargeRecord, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetRechargeHistory(user, offset, limit)
}

// GatewayGetSettlementHistory 查询 Provider 结算历史。
func (app *TeeChain) GatewayGetSettlementHistory(providerID uint64, offset uint64, limit uint32) ([]gateway.SettlementRecord, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetSettlementHistory(providerID, offset, limit)
}

// GatewayGetUsageHistory 查询用户在某 Provider+Model 上的用量历史（按日聚合）。
func (app *TeeChain) GatewayGetUsageHistory(user model.UniAddr, providerID uint64, modelID uint64, startDay string, endDay string) ([]gateway.DailyUsage, error) {
	runtime, err := app.gatewayRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetUsageHistory(user, providerID, modelID, startDay, endDay)
}

func (app *TeeChain) ContractMutation(caller []byte, callerType uint32, txn *model.Txn, height int64, call *model.ContractCall) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ContractMutation recovered from panic: %v", r)
		}
	}()

	address := model.UniAddr{
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
	runtime := contracts.NewRuntime(height, time.Now(), txn, address, model.UniAddr{
		V: pub.ToBytes(),
	})

	err = contracts.Mutation(call, runtime)
	return err
}

// ContractQuery 只读合约查询。contract=dao 时 method 为查询方法名，args 为字符串参数列表（与 ExecuteQuery 约定一致）。
func (app *TeeChain) ContractQuery(caller []byte, callerType uint32, contract string, method string, args [][]byte) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			data = nil
			err = fmt.Errorf("ContractMutation recovered from panic: %v", r)
		}
	}()

	height := app.state.Height
	txn := model.DBINS.NewTransaction()
	defer txn.Rollback()

	address := model.UniAddr{
		T: callerType,
		V: caller,
	}

	runtime := contracts.NewRuntime(height, time.Now(), txn, address, model.UniAddr{
		V: app.dkg.SystemPubKey().Byte(),
	})

	data, err = contracts.Query(&model.ContractCall{
		Name:   []byte(contract),
		Method: model.MethodToSelectorBytes(method),
		Args:   args,
	}, runtime)

	return data, err
}

func (app *TeeChain) ContractDryRun(caller []byte, callerType uint32, call *model.ContractCall) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ContractMutation recovered from panic: %v", r)
		}
	}()

	height := app.state.Height
	txn := model.DBINS.NewTransaction()
	defer txn.Rollback()

	address := model.UniAddr{
		T: callerType,
		V: caller,
	}

	runtime := contracts.NewRuntime(height, time.Now(), txn, address, model.UniAddr{
		V: app.dkg.SystemPubKey().Byte(),
	})

	err = contracts.Mutation(call, runtime)
	return err
}
