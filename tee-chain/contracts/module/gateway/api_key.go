package gateway

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// ── API Key ──────────────────────────────────────────────

// RegisterAPIKey 注册 API Key 哈希（原始 key 绝不上链）
func (g GatewayMutation) RegisterAPIKey(keyHash []byte, rateLimit uint32) error {
	if len(keyHash) != 32 {
		return fmt.Errorf("keyHash must be 32 bytes")
	}
	owner := g.api.GetCaller()
	hexKey := hex.EncodeToString(keyHash)

	if existing, _ := g.apiKeys.Primary.Get(g.api.GetTxn(), hexKey); existing != nil {
		return fmt.Errorf("api key already registered")
	}

	return g.apiKeys.Put(g.api.GetTxn(), hexKey, APIKeyInfo{
		OwnerKey:  uniAddrKey(owner),
		Owner:     owner,
		KeyHash:   keyHash,
		RateLimit: rateLimit,
		IsActive:  true,
		CreatedAt: uint64(g.api.GetBlockTime()),
	})
}

// RevokeAPIKey 吊销 API Key
func (g GatewayMutation) RevokeAPIKey(keyHash []byte) error {
	if len(keyHash) != 32 {
		return fmt.Errorf("keyHash must be 32 bytes")
	}
	owner := g.api.GetCaller()
	hexKey := hex.EncodeToString(keyHash)

	info, err := g.apiKeys.Primary.Get(g.api.GetTxn(), hexKey)
	if err != nil || info == nil {
		return ErrAPIKeyNotFound
	}
	if !bytes.Equal(info.Owner.V, owner.V) || info.Owner.T != owner.T {
		return ErrNotOwner
	}
	return g.apiKeys.Update(g.api.GetTxn(), hexKey, func(cur *APIKeyInfo) {
		cur.IsActive = false
	})
}

// UpdateAPIKey 更新 API Key 配置
func (g GatewayMutation) UpdateAPIKey(keyHash []byte, rateLimit uint32) error {
	if len(keyHash) != 32 {
		return fmt.Errorf("keyHash must be 32 bytes")
	}
	owner := g.api.GetCaller()
	hexKey := hex.EncodeToString(keyHash)

	info, err := g.apiKeys.Primary.Get(g.api.GetTxn(), hexKey)
	if err != nil || info == nil {
		return ErrAPIKeyNotFound
	}
	if !bytes.Equal(info.Owner.V, owner.V) || info.Owner.T != owner.T {
		return ErrNotOwner
	}
	return g.apiKeys.Update(g.api.GetTxn(), hexKey, func(cur *APIKeyInfo) {
		cur.RateLimit = rateLimit
	})
}

// LookupAPIKey TEE 提供方根据 hash 查找 API Key 对应的用户
func (g GatewayQuery) LookupAPIKey(keyHash []byte) (*APIKeyInfo, error) {
	if len(keyHash) != 32 {
		return nil, ErrAPIKeyNotFound
	}
	hexKey := hex.EncodeToString(keyHash)
	return g.apiKeys.Primary.Get(g.api.GetTxn(), hexKey)
}

// GetAPIKeysByOwner 返回某个用户注册的所有 API Key（使用 OwnerKey exact 索引，O(1) 查询）
func (g GatewayQuery) GetAPIKeysByOwner(owner store.UniAddr) ([]APIKeyInfo, error) {
	keys, err := g.apiKeys.Query(g.api.GetTxn(), store.QueryCond{Field: "OwnerKey", Val: uniAddrKey(owner)})
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyInfo, 0, len(keys))
	for _, k := range keys {
		info, err := g.apiKeys.Primary.Get(g.api.GetTxn(), k)
		if err != nil || info == nil {
			continue
		}
		out = append(out, *info)
	}
	return out, nil
}
