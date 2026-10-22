package graph

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/computeflux/tee-provider/pkg/types"
)

// ── GET /auth/whoami（服务间 token 内省）────────────────────────────
//
// 下游服务（actives 消息中心等）用它把「用户已有的登录会话 token」换成钱包地址，
// 从而**不需要用户再单独登录/签名**（注入钱包签名会弹窗，体验不可接受）。
//
// 请求:  Authorization: Bearer auth_tee_…（也接受裸 token，与 /gql 一致）
// 成功:  200 {"wallet":"0x…","userId":"<owner>"}
// 失败:  401 {"error":"invalid token"}（不存在 / 停用 / 过期）
//
// 身份与 /gql 的 AuthCheck 同源：AuthMiddleware 已把 token 解析进 context，
// 这里直接取用；未经过中间件时（如单测）回退到 initToken。
func handleWhoami(w http.ResponseWriter, r *http.Request) {
	token := GatewayTokenFromContext(r.Context())
	if token == nil {
		token = initToken(r)
	}

	wallet := walletFromGatewayToken(token)
	// 只有 EVM 地址（T=2, 20 字节）才能给下游当钱包身份用；
	// 其它链类型或空地址视为无效（避免下游拿到不可用的身份）。
	if token == nil || !token.IsActive || token.IsExpired() || wallet == "" {
		writeWhoamiJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	writeWhoamiJSON(w, http.StatusOK, struct {
		Wallet string `json:"wallet"`
		UserID string `json:"userId"`
	}{Wallet: wallet, UserID: token.Owner})
}

func writeWhoamiJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// walletFromGatewayToken 把 token 记录的链上地址转成 EVM 0x 地址。
// 登录时 createUserAPIKey 用 UniAddr{T: 2, V: 20 字节} 存 EVM 地址（见 auth_helpers.go）。
func walletFromGatewayToken(token *types.GatewayTokenInfo) string {
	if token == nil {
		return ""
	}
	if token.Addr.T != 2 || len(token.Addr.V) != 20 {
		return ""
	}
	return "0x" + hex.EncodeToString(token.Addr.V)
}
