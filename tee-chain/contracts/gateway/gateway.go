package gateway

import (
	"encoding/binary"
	"errors"

	"github.com/computeflux/tee-provider/pkg/model"
	"golang.org/x/crypto/blake2b"
)

// uniAddrKey 将 UniAddr 转为确定性 uint64，用于 exact 索引。
func uniAddrKey(a model.UniAddr) uint64 {
	h, _ := blake2b.New256(nil)
	h.Write(a.V)
	var tBytes [4]byte
	binary.BigEndian.PutUint32(tBytes[:], a.T)
	h.Write(tBytes[:])
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}

var (
	ErrLowCredit        = errors.New("low Credit")
	ErrModelNotFound    = errors.New("model not found")
	ErrProviderNotFound = errors.New("provider not found")
	ErrAPIKeyNotFound   = errors.New("api key not found")
	ErrNotOwner         = errors.New("not owner")
	ErrDuplicateNonce   = errors.New("duplicate settlement nonce")
)

// Gateway API 网关合约
type Gateway struct {
	api model.ContractApi

	credits               *model.StoreMapping[model.UniAddr, uint64]                      `store:"keyPfx:v2_credits"`
	credit_useds          *model.StoreMapping[model.UniAddr, uint64]                      `store:"keyPfx:v2_credit_useds"`
	models                *model.IndexedList[uint64, ModelInfo]                           `store:"keyPfx:v2_models"`
	providers             *model.IndexedList[uint64, ProviderInfo]                        `store:"keyPfx:v2_providers"`
	providerByOwner       *model.StoreMapping[model.UniAddr, uint64]                      `store:"keyPfx:v2_provider_by_owner"`       // owner → providerID（一人一 provider）
	providerCredits       *model.StoreMapping[uint64, uint64]                             `store:"keyPfx:v2_provider_credit"`         // providerID → accumulated credits
	apiKeys               *model.IndexedStore[string, APIKeyInfo]                         `store:"keyPfx:v2_api_key"`                 // hex(keyHash) → APIKeyInfo，带 OwnerKey exact 索引
	rechargeHistory       *model.StoreList2D[model.UniAddr, uint32, RechargeRecord]       `store:"keyPfx:v2_recharge_history"`        // user → []RechargeRecord
	settlementHistory     *model.StoreList2D[uint64, uint32, ProviderSettlementRecord]    `store:"keyPfx:v4_settlement_history"`      // providerID → []SettlementRecord
	userSettlementHistory *model.StoreList2D[model.UniAddr, uint32, UserSettlementRecord] `store:"keyPfx:v4_user_settlement_history"` // user → []SettlementRecord
	settledNonces         *model.StoreMapping[string, bool]                               `store:"keyPfx:v5_settled_nonces"`          // hex(nonce) → true，防重放
}

type GatewayQuery struct{ Gateway }
type GatewayMutation struct{ Gateway }
type GatewaySecret struct{ Gateway }

// ── Init ─────────────────────────────────────────────────
func (g GatewayMutation) Init() error {
	return nil
}
