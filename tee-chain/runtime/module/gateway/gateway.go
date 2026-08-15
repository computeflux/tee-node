package gateway

import (
	"encoding/binary"
	"errors"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
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
	ErrRouteNotFound    = errors.New("route not found")
	ErrAPIKeyNotFound   = errors.New("api key not found")
	ErrNotOwner         = errors.New("not owner")
	ErrDuplicateNonce   = errors.New("duplicate settlement nonce")
)

// Gateway API 网关合约
type Gateway struct {
	api store.ContractApi

	credits               *store.StoreMapping[store.UniAddr, uint64]                      `store:"keyPfx:credits"`
	credit_useds          *store.StoreMapping[store.UniAddr, uint64]                      `store:"keyPfx:credit_useds"`
	models                *store.IndexedList[uint64, ModelInfo]                           `store:"keyPfx:models"`
	providers             *store.IndexedList[uint64, ProviderInfo]                        `store:"keyPfx:providers"`
	providerByOwner       *store.StoreMapping[store.UniAddr, uint64]                      `store:"keyPfx:provider_by_owner"` // owner → providerID（一人一 provider）
	providerCredits       *store.StoreMapping[uint64, uint64]                             `store:"keyPfx:provider_credit"`   // providerID → accumulated credits
	routes                *store.IndexedList[uint64, ModelRoute]                          `store:"keyPfx:routes"`            // 模型路由
	modelByRoute          *store.StoreMapping[uint64, uint64]                             `store:"keyPfx:model_by_route"`    // routeID → modelCount（反向索引）
	modelReports          *store.IndexedList[uint64, ModelReport]                         `store:"keyPfx:model_reports"`     // 用户举报
	modelAudits           *store.IndexedList[uint64, ModelAudit]                          `store:"keyPfx:model_audits"`      // 系统检查记录
	APIKeys               *store.IndexedStore[string, APIKeyInfo]                         `store:"keyPfx:api_key"`           // hex(keyHash) → APIKeyInfo，带 OwnerKey exact 索引
	APIKeysV2             *store.IndexedStore[string, APIKeyInfoV2]                       `store:"keyPfx:v2_api_key"`
	rechargeHistory       *store.StoreList2D[store.UniAddr, uint32, RechargeRecord]       `store:"keyPfx:recharge_history"`        // user → []RechargeRecord
	transferHistory       *store.StoreList2D[store.UniAddr, uint32, TransferRecord]       `store:"keyPfx:transfer_history"`        // user → []TransferRecord
	settlementHistory     *store.StoreList2D[uint64, uint32, ProviderSettlementRecord]    `store:"keyPfx:settlement_history"`      // providerID → []SettlementRecord
	userSettlementHistory *store.StoreList2D[store.UniAddr, uint32, UserSettlementRecord] `store:"keyPfx:user_settlement_history"` // user → []SettlementRecord
	rechargeNonces        *store.StoreMapping[string, bool]                               `store:"keyPfx:recharge_nonces"`         // hex(nonce) → true，充值防重放
	settledNonces         *store.StoreMapping[string, bool]                               `store:"keyPfx:settled_nonces"`          // hex(nonce) → true，防重放
	withdrawNonces        *store.IndexedList[uint64, WithdrawApply]                       `store:"keyPfx:withdraw_nonces"`
	withdrawByUser        *store.StoreList2D[store.UniAddr, uint32, uint64]               `store:"keyPfx:withdraw_by_user"`

	// 连续全失败结算周期数（modelID → 次数）：每个周期无成功且有失败则 +1，有成功则清零；
	// 达到阈值自动下架。独立存储，避免修改 ModelInfo 结构导致存量数据无法解码。
	modelFailPeriods *store.StoreMapping[uint64, uint8] `store:"keyPfx:model_fail_periods"`
}

type GatewayQuery struct{ Gateway }
type GatewayMutation struct{ Gateway }
type GatewaySecret struct{ Gateway }

// ── Init ─────────────────────────────────────────────────
func (g GatewayMutation) Init() error {
	return nil
}
