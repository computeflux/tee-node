package graph

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
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
// addrHex 为 Ethereum 地址 hex 字符串（如 "0xabc..."），用于结算时构造 UniAddr。
func createUserAPIKey(owner string, addrHex string) string {
	apiKey := generateAPIKey()
	nextID := model.GetNextID("gateway_token")

	// 构造 UniAddr：以太坊地址 callerType=2（EVM），去掉 0x 前缀
	addr := model.UniAddr{T: 2, V: []byte{}}
	if addrHex != "" && addrHex != "0x" {
		addrHex = strings.TrimPrefix(addrHex, "0x")
		if b, err := hex.DecodeString(addrHex); err == nil && len(b) == 20 {
			addr.V = b
		}
	}

	token := &model.GatewayTokenInfo{
		Owner:     owner,
		Addr:      addr,
		Key:       apiKey,
		RateLimit: 60,
		IsActive:  true,
		CreatedAt: uint64(time.Now().Unix()),
		ExpiresAt: uint64(time.Now().Add(24 * time.Hour).Unix()), // 默认1天过期
	}
	model.SetGatewayToken(nextID, token)
	model.SetGatewayTokenByKey(token) // 同时写入 key 索引，供 O(1) 查询
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
	tk, _ := model.GetGatewayTokenByKey(token)
	if tk != nil {
		if tk.IsExpired() {
			return "", fmt.Errorf("token expired")
		}
		if tk.IsActive {
			return tk.Owner, nil
		}
	}

	return "", fmt.Errorf("unauthorized")
}
