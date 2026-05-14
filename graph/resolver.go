//go:generate go run github.com/99designs/gqlgen generate

package graph

import (
	"github.com/computeflux/tee-provider/pkg/api/auth"
	authsvc "github.com/computeflux/tee-provider/services/auth"
	"github.com/computeflux/tee-provider/services/wallet"
	sidechain "github.com/computeflux/tee-provider/side-chain"
)

// Resolver 作为 GraphQL 的依赖注入容器
type Resolver struct {
	SideChain *sidechain.SideChain

	// Web3Auth 服务
	AuthCfg   *auth.AuthConfig
	WalletSvc *wallet.WalletService
	JWTSvc    *authsvc.JWTService
	EmailSvc  authsvc.EmailService
	OAuthSvc  *authsvc.OAuthService
	OTPSvc    *authsvc.OTPService
}
