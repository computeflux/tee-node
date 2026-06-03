package gateway

import (
	"encoding/binary"
	"errors"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"golang.org/x/crypto/blake2b"
)

// uniAddrKey 将 UniAddr 转为确定性 uint64，用于 exact 索引。
func uniAddrKey(a store.UniAddr) uint64 {
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
	api store.ContractApi

	credits               *store.StoreMapping[store.UniAddr, uint64]                      `store:"keyPfx:v9_credits"`
	credit_useds          *store.StoreMapping[store.UniAddr, uint64]                      `store:"keyPfx:v9_credit_useds"`
	models                *store.IndexedList[uint64, ModelInfo]                           `store:"keyPfx:v9_models"`
	providers             *store.IndexedList[uint64, ProviderInfo]                        `store:"keyPfx:v9_providers"`
	providerByOwner       *store.StoreMapping[store.UniAddr, uint64]                      `store:"keyPfx:v9_provider_by_owner"`       // owner → providerID（一人一 provider）
	providerCredits       *store.StoreMapping[uint64, uint64]                             `store:"keyPfx:v9_provider_credit"`         // providerID → accumulated credits
	apiKeys               *store.IndexedStore[string, APIKeyInfo]                         `store:"keyPfx:v9_api_key"`                 // hex(keyHash) → APIKeyInfo，带 OwnerKey exact 索引
	rechargeHistory       *store.StoreList2D[store.UniAddr, uint32, RechargeRecord]       `store:"keyPfx:v9_recharge_history"`        // user → []RechargeRecord
	settlementHistory     *store.StoreList2D[uint64, uint32, ProviderSettlementRecord]    `store:"keyPfx:v9_settlement_history"`      // providerID → []SettlementRecord
	userSettlementHistory *store.StoreList2D[store.UniAddr, uint32, UserSettlementRecord] `store:"keyPfx:v9_user_settlement_history"` // user → []SettlementRecord
	settledNonces         *store.StoreMapping[string, bool]                               `store:"keyPfx:v9_settled_nonces"`          // hex(nonce) → true，防重放
}

type GatewayQuery struct{ Gateway }
type GatewayMutation struct{ Gateway }
type GatewaySecret struct{ Gateway }

// ── Init ─────────────────────────────────────────────────
func (g GatewayMutation) Init() error {
	return nil
}
