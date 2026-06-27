package auth

import (
	teechain "github.com/computeflux/tee-provider/tee-chain"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	cauth "github.com/computeflux/tee-provider/tee-chain/runtime/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── Auth 合约桥接（从 tee-chain/contract.go 拆分出来）─────────

// runAuthQuery 构造 AuthQuery 并执行 fn，统一处理合约 runtime 的创建与回滚。
func runAuthQuery[T any](chain *teechain.TeeChain, fn func(cauth.AuthQuery) (T, error)) (T, error) {
	return teechain.WithRuntime(chain, func(runtime *contracts.Runtime) (T, error) {
		return fn(cauth.AuthQuery{Auth: *cauth.NewAuth(runtime)})
	})
}

// runAuthSecret 构造 AuthSecret 并执行 fn，统一处理合约 runtime 的创建与回滚。
func runAuthSecret[T any](chain *teechain.TeeChain, fn func(cauth.AuthSecret) (T, error)) (T, error) {
	return teechain.WithRuntime(chain, func(runtime *contracts.Runtime) (T, error) {
		return fn(cauth.AuthSecret{Auth: *cauth.NewAuth(runtime)})
	})
}

// AuthGetAuthUser 查询 Web3Auth 用户信息。
func AuthGetAuthUser(chain *teechain.TeeChain, ref string) (*cauth.AuthUserRecord, error) {
	return runAuthQuery(chain, func(q cauth.AuthQuery) (*cauth.AuthUserRecord, error) {
		return q.GetAuthUser(ref)
	})
}

// AuthStoreAuthUser 将 Web3Auth 用户信息提交到链上合约。
func AuthStoreAuthUser(chain *teechain.TeeChain, ref string, ciphertext []byte) error {
	refBytes, _ := store.ScaleEncode(ref)
	cipherBytes, _ := store.ScaleEncode(ciphertext)
	nonceBytes, _ := store.ScaleEncode([]byte{})
	return chain.SubmitContract("auth", store.MethodToSelectorBytes("store_auth_user"), [][]byte{refBytes, cipherBytes, nonceBytes})
}

// AuthGetPrivateKey 查询用户加密私钥（跨节点同步）。
func AuthGetPrivateKey(chain *teechain.TeeChain, ref string) (*cauth.PrivateKeyRecord, error) {
	return runAuthQuery(chain, func(q cauth.AuthQuery) (*cauth.PrivateKeyRecord, error) {
		return q.GetPrivateKey(ref)
	})
}

// AuthStorePrivateKey 将用户加密私钥提交到链上合约（跨节点同步）。
func AuthStorePrivateKey(chain *teechain.TeeChain, ref string, ciphertext []byte) error {
	refBytes, _ := store.ScaleEncode(ref)
	cipherBytes, _ := store.ScaleEncode(ciphertext)
	nonceBytes, _ := store.ScaleEncode([]byte{})
	return chain.SubmitContract("auth", store.MethodToSelectorBytes("store_private_key"), [][]byte{refBytes, cipherBytes, nonceBytes})
}

// AuthGetConfig 查询认证服务配置（TEE 内部，不对外）。
func AuthGetConfig(chain *teechain.TeeChain) (*cauth.AuthConfig, error) {
	return runAuthSecret(chain, func(s cauth.AuthSecret) (*cauth.AuthConfig, error) {
		return s.GetConfig()
	})
}

// AuthGetPubConfig 查询认证服务配置（对外，已脱敏）。
func AuthGetPubConfig(chain *teechain.TeeChain) (*cauth.AuthConfig, error) {
	return runAuthQuery(chain, func(q cauth.AuthQuery) (*cauth.AuthConfig, error) {
		return q.GetConfig()
	})
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
