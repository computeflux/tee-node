package teechain

import (
	"fmt"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	"github.com/computeflux/tee-provider/tee-chain/contracts/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/gateway"
)

// ── Gateway 非 codec 查询接口（供 HTTP 层直接调用）──────

func (app *TeeChain) contractRuntime() (*contracts.Runtime, error) {
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
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.LookupAPIKey(keyHash)
}

// GatewayListActiveModels 查询链上活跃模型列表。
func (app *TeeChain) GatewayListActiveModels(offset uint64, limit uint32) ([]gateway.ModelInfo, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.ListActiveModels(offset, limit)
}

// GatewayListActiveProviders 查询链上活跃 Provider 列表。
func (app *TeeChain) GatewayListActiveProviders(offset uint64, limit uint32) ([]gateway.ProviderInfo, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.ListActiveProviders(offset, limit)
}

// GatewayGetProvider 查询单个 Provider。
func (app *TeeChain) GatewayGetProvider(id uint64) (*gateway.ProviderInfo, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetProvider(id)
}

// GatewayGetModel 查询单个 Model。
func (app *TeeChain) GatewayGetModel(id uint64) (*gateway.ModelInfo, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetModel(id)
}

// GatewayGetModelByName 按名称查询活跃模型（走 string exact 索引，O(1) 查询）。
func (app *TeeChain) GatewayGetModelByName(pid uint64, name string) (*gateway.ModelInfoWithID, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetModelByName(pid, name)
}

// GatewaySecretGetModel 查询单个 Model（TEE 内部，不脱敏）。
func (app *TeeChain) GatewaySecretGetModel(id uint64) (*gateway.ModelInfo, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewaySecret{Gateway: *gateway.NewGateway(runtime)}.GetModel(id)
}

// GatewaySecretGetModelByName 按名称查询模型（TEE 内部，不脱敏）。
func (app *TeeChain) GatewaySecretGetModelByName(pid uint64, name string) (*gateway.ModelInfoWithID, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewaySecret{Gateway: *gateway.NewGateway(runtime)}.GetModelByName(pid, name)
}

// GatewaySecretListActiveModels 查询活跃模型列表（TEE 内部，不脱敏）。
func (app *TeeChain) GatewaySecretListActiveModels(offset uint64, limit uint32) ([]gateway.ModelInfo, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewaySecret{Gateway: *gateway.NewGateway(runtime)}.ListActiveModels(offset, limit)
}

// GatewayGetProviderByName 按名称查询 Provider（走 string exact 索引，O(1) 查询）。
func (app *TeeChain) GatewayGetProviderByName(name string) (*gateway.ProviderInfoWithID, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetProviderByName(name)
}

// GatewayGetCredit 查询用户剩余积分（credits - credit_useds）
func (app *TeeChain) GatewayGetCredit(user model.UniAddr) (int64, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return 0, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetCredit(user)
}

// GatewayGetRechargeHistory 查询用户充值历史。
func (app *TeeChain) GatewayGetRechargeHistory(user model.UniAddr, offset uint64, limit uint32) ([]gateway.RechargeRecord, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetRechargeHistory(user, offset, limit)
}

// ── Auth 合约桥接 ──────────────────────────────────────────

// AuthGetAuthUser 查询 Web3Auth 用户信息。
func (app *TeeChain) AuthGetAuthUser(ref string) (*auth.AuthUserRecord, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return auth.AuthQuery{Auth: *auth.NewAuth(runtime)}.GetAuthUser(ref)
}

// AuthGetPrivateKey 查询用户私钥。
func (app *TeeChain) AuthGetPrivateKey(ref string) (*auth.PrivateKeyRecord, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return auth.AuthQuery{Auth: *auth.NewAuth(runtime)}.GetPrivateKey(ref)
}

// AuthStoreAuthUser 将 Web3Auth 用户信息提交到链上合约。
func (app *TeeChain) AuthStoreAuthUser(ref string, ciphertext []byte) error {
	refBytes, _ := codec.Encode(ref)
	cipherBytes, _ := codec.Encode(ciphertext)
	nonceBytes, _ := codec.Encode([]byte{})
	return app.SubmitContract("auth", model.MethodToSelectorBytes("store_auth_user"), [][]byte{refBytes, cipherBytes, nonceBytes})
}

// AuthGetConfig 查询认证服务配置（TEE 内部，不对外）
func (app *TeeChain) AuthGetConfig() (*auth.AuthConfig, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return auth.AuthSecret{Auth: *auth.NewAuth(runtime)}.GetConfig()
}

// AuthGetConfig 查询认证服务配置（TEE 内部，不对外）
func (app *TeeChain) AuthGetPubConfig() (*auth.AuthConfig, error) {
	runtime, err := app.contractRuntime()
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return auth.AuthQuery{Auth: *auth.NewAuth(runtime)}.GetConfig()
}

// AuthSetConfig 设置认证服务配置（链上存储）。
func (app *TeeChain) AuthSetConfig(
	smtpHost string, smtpPort uint32,
	smtpUser, smtpPwd, smtpFrom, smtpFromName string, mockEmail bool,
	googleID, googleSecret, googleRedirect string,
	githubID, githubSecret, githubRedirect string,
	twitterID, twitterSecret, twitterRedirect string,
) error {
	hostBytes, _ := codec.Encode(smtpHost)
	portBytes, _ := codec.Encode(smtpPort)
	userBytes, _ := codec.Encode(smtpUser)
	pwdBytes, _ := codec.Encode(smtpPwd)
	fromBytes, _ := codec.Encode(smtpFrom)
	fromNameBytes, _ := codec.Encode(smtpFromName)
	mockBytes, _ := codec.Encode(mockEmail)
	gIDBytes, _ := codec.Encode(googleID)
	gSecBytes, _ := codec.Encode(googleSecret)
	gRedBytes, _ := codec.Encode(googleRedirect)
	ghIDBytes, _ := codec.Encode(githubID)
	ghSecBytes, _ := codec.Encode(githubSecret)
	ghRedBytes, _ := codec.Encode(githubRedirect)
	twIDBytes, _ := codec.Encode(twitterID)
	twSecBytes, _ := codec.Encode(twitterSecret)
	twRedBytes, _ := codec.Encode(twitterRedirect)

	return app.SubmitContract("auth", model.MethodToSelectorBytes("set_config"), [][]byte{
		hostBytes, portBytes, userBytes, pwdBytes,
		fromBytes, fromNameBytes, mockBytes,
		gIDBytes, gSecBytes, gRedBytes,
		ghIDBytes, ghSecBytes, ghRedBytes,
		twIDBytes, twSecBytes, twRedBytes,
	})
}

func (app *TeeChain) AuthStorePrivateKey(ref string, ciphertext []byte) error {
	refBytes, _ := codec.Encode(ref)
	cipherBytes, _ := codec.Encode(ciphertext)
	nonceBytes, _ := codec.Encode([]byte{})
	return app.SubmitContract("auth", model.MethodToSelectorBytes("store_private_key"), [][]byte{refBytes, cipherBytes, nonceBytes})
}

func (app *TeeChain) ContractMutation(caller []byte, callerType uint32, txn *model.Txn, t time.Time, height int64, call *model.ContractCall) error {
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

	runtime := contracts.NewRuntime(height, t, txn, address, model.UniAddr{
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
