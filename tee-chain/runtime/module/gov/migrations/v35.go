package migrations

import (
	"errors"
	"fmt"
	"strings"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// errUndecodableRecord 表示存量字节既不是新编码也不是任何已知的旧编码。
// 迁移遇到时跳过并记录，不中断整批迁移（与 V34 行为一致）。
var errUndecodableRecord = errors.New("undecodable record")

// V35 重跑 V34 的数据迁移（充值历史 / 用户结算历史补写 BlockHeight）。
//
// 背景：34 升级交易执行后，FinalizeTx 在区块末尾触发的 V34 迁移可能因为
// 节点二进制尚未升级（NativeVersion 仍是 33）而被路由到 WASM 路径。
// WASM 运行时的 storage_scan_prefix 使用固定 64KB 共享缓冲区，存量记录
// 较多时 ListByPrefix 的结果会越界，迁移报
// `wasm execute: wasm error: unreachable` 失败；此时链上 runtime_version
// 已经推进到 34、app.state.RuntimeVersion 也被推进为 34，不会再触发 V34。
// 因此新增 V35 作为恢复版本：三个节点都升级到同一二进制后执行 34→35 升级，
// 迁移在原生路径下重跑。
//
// 与 V34 的区别：不再用 ListByPrefix 一次性读出整个前缀，而是按列表的
// (id, k2) 结构逐条点查/写回，单次存储交互的数据量恒定，因此即使本次升级
// 仍被路由到 WASM 路径也不会再因缓冲区上限失败。
//
// 幂等：已迁移记录解码恰好消费全部字节即跳过，重复执行不会抹掉新记录
// 已写入的 BlockHeight。
func V35(api store.ContractApi) error {
	fmt.Printf("[V35] rerun V34 migration via point lookups\n")
	if err := migrateRechargeHistoryBlockHeightPaged(api); err != nil {
		return err
	}
	return migrateUserSettlementHistoryBlockHeightPaged(api)
}

// rechargeHistoryList 返回与 gateway.rechargeHistory 相同前缀/索引布局的列表句柄。
func rechargeHistoryList() *store.StoreList2D[store.UniAddr, uint32, gateway.RechargeRecord] {
	return store.NewStoreList2D[store.UniAddr, uint32, gateway.RechargeRecord](
		"gateway",
		"recharge_history_k1_to_id_",
		"recharge_history_k1_length_",
		"recharge_history_k2_next_",
		"recharge_history_store_",
	)
}

// userSettlementHistoryList 返回与 gateway.userSettlementHistory 相同前缀/索引布局的列表句柄。
func userSettlementHistoryList() *store.StoreList2D[store.UniAddr, uint32, gateway.UserSettlementRecord] {
	return store.NewStoreList2D[store.UniAddr, uint32, gateway.UserSettlementRecord](
		"gateway",
		"user_settlement_history_k1_to_id_",
		"user_settlement_history_k1_length_",
		"user_settlement_history_k2_next_",
		"user_settlement_history_store_",
	)
}

func migrateRechargeHistoryBlockHeightPaged(api store.ContractApi) error {
	return migrateList2DRecords(api, rechargeHistoryList(), "recharge", upgradeRechargeRecord)
}

func migrateUserSettlementHistoryBlockHeightPaged(api store.ContractApi) error {
	return migrateList2DRecords(api, userSettlementHistoryList(), "user settlement", upgradeUserSettlementRecord)
}

// migrateList2DRecords 按 (外层 id, 内层 k2) 逐条点查并升级。
//
// 外层 id 从 0 到 k1_length 连续分配（见 StoreList2D.Insert），内层 k2 的
// 上界由 k2_next 映射给出；Clear 留下的空洞（Get 返回 nil）直接跳过。
// upgrade 返回 (新编码, 是否需要写回, error)：
//   - 已是新编码 → (nil, false, nil)
//   - 需要重写   → (bytes, true, nil)
//   - 无法识别   → (nil, false, errUndecodableRecord)
func migrateList2DRecords[V any](
	api store.ContractApi,
	list *store.StoreList2D[store.UniAddr, uint32, V],
	label string,
	upgrade func(raw []byte) ([]byte, bool, error),
) error {
	txn := api.GetTxn()

	ids, err := (&store.StoreValue[uint32]{Namespace: list.Namespace, Key: list.PrefixK1Length}).GetOrDefault(txn, 0)
	if err != nil {
		return fmt.Errorf("%s: read k1 length: %w", label, err)
	}

	// k2_next 映射与 StoreList2D 内部使用同一种编码（Ix=uint32 → 十进制字符串）。
	nextK2 := &store.StoreMapping[uint32, uint32]{Namespace: list.Namespace, KeyPrefix: list.PrefixK2NextID}

	var scanned, migrated, skipped int
	for id := uint32(0); id < ids; id++ {
		next, err := nextK2.GetOrDefault(txn, id, 0)
		if err != nil {
			return fmt.Errorf("%s: read k2 next id=%d: %w", label, id, err)
		}
		for k2 := uint32(0); k2 < next; k2++ {
			key := list.StorageKey(id, k2)
			raw, err := txn.Get(key)
			if err != nil {
				if isStoreKeyNotFound(err) {
					continue // Clear 留下的空洞
				}
				return fmt.Errorf("%s: get id=%d k2=%d: %w", label, id, k2, err)
			}
			if len(raw) == 0 {
				continue
			}

			scanned++
			out, changed, err := upgrade(raw)
			if err != nil {
				if errors.Is(err, errUndecodableRecord) {
					fmt.Printf("[V35] %s: skip undecodable id=%d k2=%d: %v\n", label, id, k2, err)
					skipped++
					continue
				}
				return fmt.Errorf("%s: encode id=%d k2=%d: %w", label, id, k2, err)
			}
			if !changed {
				continue
			}
			if err := txn.Set(key, out); err != nil {
				return fmt.Errorf("%s: set id=%d k2=%d: %w", label, id, k2, err)
			}
			migrated++
		}
	}

	fmt.Printf("[V35] %s: ids=%d scanned=%d migrated=%d skipped=%d\n", label, ids, scanned, migrated, skipped)
	return nil
}

// isStoreKeyNotFound 判断底层存储的 "key 不存在" 错误。
// 原生 pebble 返回 "pebble: not found"（与 store.StoreMapping 内部判断一致）；
// WASM/内存实现直接返回 (nil, nil)。
func isStoreKeyNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// upgradeRechargeRecord 把一条充值记录升级为含 BlockHeight 的新编码。
// 兼容两种旧编码：V33（含 ChainID/Nonce）与 V1（仅 User/Amount/Time）。
func upgradeRechargeRecord(raw []byte) ([]byte, bool, error) {
	var cur gateway.RechargeRecord
	if n, err := cur.Decode(raw); err == nil && n == len(raw) {
		return nil, false, nil
	}

	var v33 gateway.RechargeRecordV33
	n33, err33 := v33.Decode(raw)
	if err33 == nil && n33 == len(raw) {
		rec := gateway.RechargeRecord{
			User:        v33.User,
			Amount:      v33.Amount,
			Time:        v33.Time,
			ChainID:     v33.ChainID,
			Nonce:       v33.Nonce,
			BlockHeight: 0, // 旧记录无法反推写入高度
		}
		out, err := store.ScaleEncode(rec)
		if err != nil {
			return nil, false, err
		}
		return out, true, nil
	}

	// V33 之前（V1：仅 User/Amount/Time）的残留记录：补 0 值。
	// 正常情况下 V33 迁移已处理，这里兜底避免旧编码继续留在链上。
	var v1 gateway.RechargeRecordV1
	n1, err1 := v1.Decode(raw)
	if err1 == nil && n1 == len(raw) {
		rec := gateway.RechargeRecord{
			User:   v1.User,
			Amount: v1.Amount,
			Time:   v1.Time,
		}
		out, err := store.ScaleEncode(rec)
		if err != nil {
			return nil, false, err
		}
		return out, true, nil
	}

	return nil, false, fmt.Errorf("%w (v33: %v, v1: %v)", errUndecodableRecord, err33, err1)
}

// upgradeUserSettlementRecord 把一条用户结算记录升级为含 BlockHeight 的新编码。
func upgradeUserSettlementRecord(raw []byte) ([]byte, bool, error) {
	var cur gateway.UserSettlementRecord
	if n, err := cur.Decode(raw); err == nil && n == len(raw) {
		return nil, false, nil
	}

	var v33 gateway.UserSettlementRecordV33
	n, decodeErr := v33.Decode(raw)
	if decodeErr != nil || n != len(raw) {
		return nil, false, fmt.Errorf("%w (v33: %v)", errUndecodableRecord, decodeErr)
	}

	rec := gateway.UserSettlementRecord{
		ProviderID:  v33.ProviderID,
		ModelID:     v33.ModelID,
		TokenIn:     v33.TokenIn,
		TokenOut:    v33.TokenOut,
		CachedToken: v33.CachedToken,
		Cost:        v33.Cost,
		Count:       v33.Count,
		Time:        v33.Time,
		BlockHeight: 0, // 旧记录无法反推写入高度
	}
	out, err := store.ScaleEncode(rec)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}
