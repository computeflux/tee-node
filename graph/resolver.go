//go:generate go run github.com/99designs/gqlgen generate

package graph

import (
	"fmt"
	"time"

	authsvc "github.com/computeflux/tee-provider/pkg/services/auth"
	"github.com/computeflux/tee-provider/pkg/services/wallet"
	"github.com/computeflux/tee-provider/pkg/store"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

// Resolver 作为 GraphQL 的依赖注入容器
type Resolver struct {
	TeeChain *teechain.TeeChain

	// Web3Auth 服务
	WalletSvc *wallet.WalletService
	EmailSvc  authsvc.EmailService
	OAuthSvc  *authsvc.OAuthService
	OTPSvc    *authsvc.OTPService
}

// ResolveSignerEmail 从 token 解析出用户邮箱
// 支持 API Key 和短期签名 token
func (r *Resolver) ResolveSignerEmail(token string) (string, error) {
	// 先尝试作为签名 token 解析
	if len(token) > 4 && token[:4] == "sig_" {
		session, err := store.GetSignSession(token)
		if err != nil || session == nil {
			return "", fmt.Errorf("invalid sign token")
		}
		if time.Now().Unix() > session.ExpiresAt {
			store.DeleteSignSession(token)
			return "", fmt.Errorf("sign token expired")
		}
		return session.Email, nil
	}

	// 尝试作为 API Key 解析
	tk := store.FindGatewayTokenByKey(token)
	if tk != nil && tk.IsActive {
		return tk.Owner, nil
	}

	return "", fmt.Errorf("unauthorized")
}
