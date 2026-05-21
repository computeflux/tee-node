package graph

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/api/gateway"
	"github.com/computeflux/tee-provider/pkg/model"
)

// generateAPIKey generates a new API key in the format tee_<64 hex chars>
func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "tee_" + hex.EncodeToString(b)
}

// createUserAPIKey creates and stores a new API Key for the given user.
func createUserAPIKey(owner string) string {
	apiKey := generateAPIKey()
	nextID := model.GetNextID("gateway_token")
	model.SetGatewayToken(nextID, &model.GatewayTokenInfo{
		Owner:     owner,
		Key:       apiKey,
		RateLimit: 60,
		IsActive:  true,
		CreatedAt: uint64(time.Now().Unix()),
	})
	model.SetNextID("gateway_token", nextID+1)
	gateway.InvalidateCache()
	return apiKey
}

// resolveEmailFromToken resolves user email from an API Key or sig_ token.
func resolveEmailFromToken(token string) (string, error) {
	if len(token) > 4 && token[:4] == "sig_" {
		session, err := model.GetSignSession(token)
		if err != nil || session == nil {
			return "", fmt.Errorf("invalid sign token")
		}
		if time.Now().Unix() > session.ExpiresAt {
			model.DeleteSignSession(token)
			return "", fmt.Errorf("sign token expired")
		}
		return session.Email, nil
	}

	// 优先从合约查询 API Key
	if tk := gateway.LookupAPIKeyViaContract(token); tk != nil && tk.IsActive {
		return tk.Owner, nil
	}

	// Fallback: PebbleDB（Web3Auth 登录生成的临时 key）
	tk := model.FindGatewayTokenByKey(token)
	if tk != nil && tk.IsActive {
		return tk.Owner, nil
	}

	return "", fmt.Errorf("unauthorized")
}
