package auth

import (
	teechain "github.com/computeflux/tee-provider/tee-chain"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	cauth "github.com/computeflux/tee-provider/tee-chain/contracts/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// ── Auth 合约桥接（从 tee-chain/contract.go 拆分出来）─────────

// contractRuntime 构造合约运行时上下文（本包内部使用）。
func contractRuntime(chain *teechain.TeeChain) (*contracts.Runtime, error) {
	return chain.ContractRuntime()
}

// AuthGetAuthUser 查询 Web3Auth 用户信息。
func AuthGetAuthUser(chain *teechain.TeeChain, ref string) (*cauth.AuthUserRecord, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cauth.AuthQuery{Auth: *cauth.NewAuth(runtime)}.GetAuthUser(ref)
}

// AuthGetPrivateKey 查询用户私钥。
func AuthGetPrivateKey(chain *teechain.TeeChain, ref string) (*cauth.PrivateKeyRecord, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cauth.AuthQuery{Auth: *cauth.NewAuth(runtime)}.GetPrivateKey(ref)
}

// AuthStoreAuthUser 将 Web3Auth 用户信息提交到链上合约。
func AuthStoreAuthUser(chain *teechain.TeeChain, ref string, ciphertext []byte) error {
	refBytes, _ := store.ScaleEncode(ref)
	cipherBytes, _ := store.ScaleEncode(ciphertext)
	nonceBytes, _ := store.ScaleEncode([]byte{})
	return chain.SubmitContract("auth", store.MethodToSelectorBytes("store_auth_user"), [][]byte{refBytes, cipherBytes, nonceBytes})
}

// AuthGetConfig 查询认证服务配置（TEE 内部，不对外）。
func AuthGetConfig(chain *teechain.TeeChain) (*cauth.AuthConfig, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cauth.AuthSecret{Auth: *cauth.NewAuth(runtime)}.GetConfig()
}

// AuthGetPubConfig 查询认证服务配置（对外，已脱敏）。
func AuthGetPubConfig(chain *teechain.TeeChain) (*cauth.AuthConfig, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cauth.AuthQuery{Auth: *cauth.NewAuth(runtime)}.GetConfig()
}

// AuthSetConfig 设置认证服务配置（链上存储）。
func AuthSetConfig(
	chain *teechain.TeeChain,
	smtpHost string, smtpPort uint32,
	smtpUser, smtpPwd, smtpFrom, smtpFromName string, mockEmail bool,
	googleID, googleSecret, googleRedirect string,
	githubID, githubSecret, githubRedirect string,
	twitterID, twitterSecret, twitterRedirect string,
) error {
	hostBytes, _ := store.ScaleEncode(smtpHost)
	portBytes, _ := store.ScaleEncode(smtpPort)
	userBytes, _ := store.ScaleEncode(smtpUser)
	pwdBytes, _ := store.ScaleEncode(smtpPwd)
	fromBytes, _ := store.ScaleEncode(smtpFrom)
	fromNameBytes, _ := store.ScaleEncode(smtpFromName)
	mockBytes, _ := store.ScaleEncode(mockEmail)
	gIDBytes, _ := store.ScaleEncode(googleID)
	gSecBytes, _ := store.ScaleEncode(googleSecret)
	gRedBytes, _ := store.ScaleEncode(googleRedirect)
	ghIDBytes, _ := store.ScaleEncode(githubID)
	ghSecBytes, _ := store.ScaleEncode(githubSecret)
	ghRedBytes, _ := store.ScaleEncode(githubRedirect)
	twIDBytes, _ := store.ScaleEncode(twitterID)
	twSecBytes, _ := store.ScaleEncode(twitterSecret)
	twRedBytes, _ := store.ScaleEncode(twitterRedirect)

	return chain.SubmitContract("auth", store.MethodToSelectorBytes("set_config"), [][]byte{
		hostBytes, portBytes, userBytes, pwdBytes,
		fromBytes, fromNameBytes, mockBytes,
		gIDBytes, gSecBytes, gRedBytes,
		ghIDBytes, ghSecBytes, ghRedBytes,
		twIDBytes, twSecBytes, twRedBytes,
	})
}

// AuthStorePrivateKey 存储用户私钥到链上合约。
func AuthStorePrivateKey(chain *teechain.TeeChain, ref string, ciphertext []byte) error {
	refBytes, _ := store.ScaleEncode(ref)
	cipherBytes, _ := store.ScaleEncode(ciphertext)
	nonceBytes, _ := store.ScaleEncode([]byte{})
	return chain.SubmitContract("auth", store.MethodToSelectorBytes("store_private_key"), [][]byte{refBytes, cipherBytes, nonceBytes})
}
