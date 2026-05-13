package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/api/auth"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/services/wallet"
)

// ── Mutation resolvers ───────────────────────────────────

func (r *mutationResolver) RequestOTP(ctx context.Context, email, clientID string) (*auth.AuthResponse, error) {
	otp, err := r.OTPSvc.Generate(email, clientID)
	if err != nil {
		return &auth.AuthResponse{Success: false, Error: err.Error()}, nil
	}
	if err := r.EmailSvc.SendOTP(email, otp); err != nil {
		return &auth.AuthResponse{Success: false, Error: "failed to send email"}, nil
	}
	return &auth.AuthResponse{Success: true}, nil
}

func (r *mutationResolver) VerifyOTP(ctx context.Context, email, otp, clientID string) (*auth.AuthResponse, error) {
	if err := r.OTPSvc.Verify(email, otp); err != nil {
		return &auth.AuthResponse{Success: false, Error: err.Error()}, nil
	}

	provider := "email_passwordless"
	address := r.WalletSvc.DeriveAddress(email)
	userInfo := buildUserInfo(email, provider)
	token, err := r.JWTSvc.IssueToken(email, address, provider)
	if err != nil {
		return &auth.AuthResponse{Success: false, Error: "failed to issue token"}, nil
	}

	// 持久化
	model.PutAuthUser(&model.AuthUser{
		Email: email, Name: userInfo.Name, Address: address,
		AuthProvider: provider, LastLogin: time.Now().Unix(),
	})
	model.PutAuthSession(&model.AuthSession{
		Token: token, Email: email, Address: address,
		AuthProvider: provider, CreatedAt: time.Now().Unix(),
	})

	return &auth.AuthResponse{
		Success: true, UserInfo: userInfo, Token: token, Address: address,
	}, nil
}

func (r *mutationResolver) SocialLogin(ctx context.Context, provider string,
	idToken *string, code *string, codeVerifier *string) (*auth.AuthResponse, error) {

	var oauthUser *auth.OAuthUserInfo
	var err error

	switch provider {
	case "google":
		if idToken != nil && *idToken != "" {
			oauthUser, err = r.OAuthSvc.VerifyGoogleIDToken(*idToken)
		} else if code != nil && *code != "" {
			oauthUser, err = r.OAuthSvc.ExchangeGoogleCode(*code)
		}
	case "github":
		if code != nil && *code != "" {
			oauthUser, err = r.OAuthSvc.ExchangeGitHubCode(*code)
		}
	case "twitter", "x":
		v := ""
		if codeVerifier != nil {
			v = *codeVerifier
		}
		c := ""
		if code != nil {
			c = *code
		}
		oauthUser, err = r.OAuthSvc.ExchangeXCode(c, v)
	default:
		return &auth.AuthResponse{Success: false, Error: "unsupported provider: " + provider}, nil
	}

	if err != nil {
		return &auth.AuthResponse{Success: false, Error: err.Error()}, nil
	}

	address := r.WalletSvc.DeriveAddress(oauthUser.Email)
	token, _ := r.JWTSvc.IssueToken(oauthUser.Email, address, provider)
	userInfo := &auth.UserInfo{
		Email:        oauthUser.Email,
		Name:         oauthUser.Name,
		ProfileImage: oauthUser.AvatarURL,
		TypeOfLogin:  provider,
		Verifier:     provider,
		VerifierID:   oauthUser.ProviderID,
	}

	// 持久化
	model.PutAuthUser(&model.AuthUser{
		Email: oauthUser.Email, Name: oauthUser.Name, Address: address,
		AuthProvider: provider, LastLogin: time.Now().Unix(),
	})

	return &auth.AuthResponse{
		Success: true, UserInfo: userInfo, Token: token, Address: address,
	}, nil
}

func (r *mutationResolver) SignTransaction(ctx context.Context, tokenStr string, tx auth.TxInput) (*auth.WalletResult, error) {
	claims, err := r.JWTSvc.ParseToken(tokenStr)
	if err != nil {
		return &auth.WalletResult{Status: "error", Error: "unauthorized"}, nil
	}
	raw, err := r.WalletSvc.SignTx(claims.Email, &wallet.TxRequest{
		From:         tx.From,
		To:           tx.To,
		Value:        tx.Value,
		Data:         tx.Data,
		Gas:          tx.Gas,
		GasPrice:     tx.GasPrice,
		Nonce:        tx.Nonce,
		ChainID:      tx.ChainID,
		PasswordHash: tx.PasswordHash,
	})
	if err != nil {
		return &auth.WalletResult{Status: "error", Error: err.Error()}, nil
	}
	return &auth.WalletResult{Status: "ok", Result: raw}, nil
}

func (r *mutationResolver) SignMessage(ctx context.Context, tokenStr, message, from string) (*auth.WalletResult, error) {
	claims, err := r.JWTSvc.ParseToken(tokenStr)
	if err != nil {
		return &auth.WalletResult{Status: "error", Error: "unauthorized"}, nil
	}
	sig, err := r.WalletSvc.SignMessage(claims.Email, message)
	if err != nil {
		return &auth.WalletResult{Status: "error", Error: err.Error()}, nil
	}
	return &auth.WalletResult{Status: "ok", Result: sig}, nil
}

func (r *mutationResolver) SetPassword(ctx context.Context, tokenStr, passwordHash string) (*auth.WalletResult, error) {
	claims, err := r.JWTSvc.ParseToken(tokenStr)
	if err != nil {
		return &auth.WalletResult{Status: "error", Error: "unauthorized"}, nil
	}
	if err := r.WalletSvc.EncryptPrivateKey(claims.Email, passwordHash); err != nil {
		return &auth.WalletResult{Status: "error", Error: err.Error()}, nil
	}
	return &auth.WalletResult{Status: "ok"}, nil
}

// ── Recovery ─────────────────────────────────────────────

func (r *mutationResolver) RequestRecovery(ctx context.Context, recoveryEmail string) (*auth.RecoveryResult, error) {
	rec, err := model.FindRecoveryByEmail(recoveryEmail)
	if err != nil || rec == nil {
		return &auth.RecoveryResult{Status: "error", Error: "no recovery config for this email"}, nil
	}

	otp := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	model.PutRecoveryOTP(rec.UserEmail, otp)
	return &auth.RecoveryResult{Status: "ok"}, nil
}

func (r *mutationResolver) VerifyRecovery(ctx context.Context, recoveryEmail, otp string) (*auth.RecoveryResult, error) {
	rec, err := model.FindRecoveryByEmail(recoveryEmail)
	if err != nil || rec == nil {
		return &auth.RecoveryResult{Status: "error", Error: "not found"}, nil
	}

	stored, _ := model.GetRecoveryOTP(rec.UserEmail)
	if stored != otp {
		return &auth.RecoveryResult{Status: "error", Error: "invalid OTP"}, nil
	}
	model.DeleteRecoveryOTP(rec.UserEmail)

	token, err := r.JWTSvc.IssueRecoveryToken(rec.UserEmail)
	if err != nil {
		return &auth.RecoveryResult{Status: "error", Error: "token issue failed"}, nil
	}
	return &auth.RecoveryResult{Status: "ok", RecoveryToken: token}, nil
}

func (r *mutationResolver) CompleteRecovery(ctx context.Context, recoveryToken, passwordHash string) (*auth.RecoveryResult, error) {
	claims, err := r.JWTSvc.ParseToken(recoveryToken)
	if err != nil {
		return &auth.RecoveryResult{Status: "error", Error: "invalid token"}, nil
	}

	if err := r.WalletSvc.EncryptPrivateKey(claims.Email, passwordHash); err != nil {
		return &auth.RecoveryResult{Status: "error", Error: err.Error()}, nil
	}

	address := r.WalletSvc.DeriveAddress(claims.Email)
	token, _ := r.JWTSvc.IssueToken(claims.Email, address, "recovery")
	return &auth.RecoveryResult{
		Status: "ok", Token: token, Address: address, Email: claims.Email,
	}, nil
}

// ── Query resolvers ──────────────────────────────────────

func (r *queryResolver) OAuthURL(ctx context.Context, provider string) (*auth.OAuthURL, error) {
	switch provider {
	case "google":
		return &auth.OAuthURL{AuthUrl: r.OAuthSvc.GoogleAuthURL("")}, nil
	case "github":
		return &auth.OAuthURL{AuthUrl: r.OAuthSvc.GitHubAuthURL("")}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (r *queryResolver) AuthHealth(ctx context.Context) (string, error) {
	return "ok", nil
}

// ── helpers ──────────────────────────────────────────────

func buildUserInfo(email, provider string) *auth.UserInfo {
	name := email
	if i := strings.Index(email, "@"); i > 0 {
		name = email[:i]
	}
	return &auth.UserInfo{
		Email:        email,
		Name:         name,
		TypeOfLogin:  provider,
		Verifier:     provider,
		VerifierID:   email,
	}
}
