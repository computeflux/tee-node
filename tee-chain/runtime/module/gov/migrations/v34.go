package migrations

import (
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// V34 为充值历史与用户结算历史补写侧链区块高度字段（BlockHeight）。
//
// 迁移背景：RechargeRecord 与 UserSettlementRecord 在末尾追加了
// BlockHeight(uint64)，用于 dapp 前端"区块链存证查看按钮"定位侧链区块。
// 旧记录的字节流短 8 字节，按新编码解码会 EOF，因此必须原地重写全部记录。
//
// 旧记录无法反推写入时的区块高度（链上未保存），故迁移后旧记录
// BlockHeight 均为 0（语义：V34 之前的旧记录，高度未知），
// 新充值/结算由 Recharge / SettleUsageWithRoute 写入真实高度。
//
// 注意：这里不能用 rechargeHistory.List/ListRaw —— store key 是 (内层id, 序号)
// 的二进制编码，可能包含 '_'，而 StoreMapping.List/ListRaw 会跳过含 '_' 的 suffix。
// 因此按完整前缀扫描并按原 key 写回（与 V33 相同策略）。
func V34(api store.ContractApi) error {
	if err := migrateRechargeHistoryBlockHeight(api); err != nil {
		return err
	}
	return migrateUserSettlementHistoryBlockHeight(api)
}

// migrateRechargeHistoryBlockHeight 将 recharge_history_store_ 前缀下的
// V33 编码记录（User/Amount/Time/ChainID/Nonce）原地重写为含 BlockHeight 的新编码。
func migrateRechargeHistoryBlockHeight(api store.ContractApi) error {
	txn := api.GetTxn()
	prefix := store.ComboNamespaceKey("gateway", "recharge_history_store_")

	keys, vals, err := txn.ListByPrefix(prefix)
	if err != nil {
		return err
	}

	fmt.Printf("[V34] recharge history total: %d\n", len(vals))

	migrated := 0
	for i, key := range keys {
		raw := vals[i]

		// 已是新编码（迁移重复执行）：V34 解码恰好消费全部字节，无需重写。
		var cur gateway.RechargeRecord
		if n, err := cur.Decode(raw); err == nil && n == len(raw) {
			continue
		}

		// 旧编码：按 V33 解码。仅当恰好消费完所有字节时才认定是旧记录，
		// 避免 ScaleDecode 忽略尾部字节、在重复迁移时把 BlockHeight 抹成 0。
		var old gateway.RechargeRecordV33
		n, err := old.Decode(raw)
		if err != nil || n != len(raw) {
			fmt.Printf("[V34] skip undecodable recharge record key=%x: %v\n", key, err)
			continue
		}

		rec := gateway.RechargeRecord{
			User:        old.User,
			Amount:      old.Amount,
			Time:        old.Time,
			ChainID:     old.ChainID,
			Nonce:       old.Nonce,
			BlockHeight: 0, // 旧记录无法反推写入高度
		}
		bt, err := store.ScaleEncode(rec)
		if err != nil {
			return fmt.Errorf("encode recharge record key=%x: %w", key, err)
		}
		if err := txn.Set(key, bt); err != nil {
			return fmt.Errorf("set recharge record key=%x: %w", key, err)
		}
		migrated++
	}

	fmt.Printf("[V34] recharge history migrated: %d / %d\n", migrated, len(vals))
	return nil
}

// migrateUserSettlementHistoryBlockHeight 将 user_settlement_history_store_ 前缀下的
// 旧编码记录（无 BlockHeight）原地重写为含 BlockHeight 的新编码。
func migrateUserSettlementHistoryBlockHeight(api store.ContractApi) error {
	txn := api.GetTxn()
	prefix := store.ComboNamespaceKey("gateway", "user_settlement_history_store_")

	keys, vals, err := txn.ListByPrefix(prefix)
	if err != nil {
		return err
	}

	fmt.Printf("[V34] user settlement history total: %d\n", len(vals))

	migrated := 0
	for i, key := range keys {
		raw := vals[i]

		// 已是新编码（迁移重复执行）：V34 解码恰好消费全部字节，无需重写。
		var cur gateway.UserSettlementRecord
		if n, err := cur.Decode(raw); err == nil && n == len(raw) {
			continue
		}

		// 旧编码：按 V33 及之前的编码解码。仅当恰好消费完所有字节时才认定是旧记录。
		var old gateway.UserSettlementRecordV33
		n, err := old.Decode(raw)
		if err != nil || n != len(raw) {
			fmt.Printf("[V34] skip undecodable user settlement record key=%x: %v\n", key, err)
			continue
		}

		rec := gateway.UserSettlementRecord{
			ProviderID:  old.ProviderID,
			ModelID:     old.ModelID,
			TokenIn:     old.TokenIn,
			TokenOut:    old.TokenOut,
			CachedToken: old.CachedToken,
			Cost:        old.Cost,
			Count:       old.Count,
			Time:        old.Time,
			BlockHeight: 0, // 旧记录无法反推写入高度
		}
		bt, err := store.ScaleEncode(rec)
		if err != nil {
			return fmt.Errorf("encode user settlement record key=%x: %w", key, err)
		}
		if err := txn.Set(key, bt); err != nil {
			return fmt.Errorf("set user settlement record key=%x: %w", key, err)
		}
		migrated++
	}

	fmt.Printf("[V34] user settlement history migrated: %d / %d\n", migrated, len(vals))
	return nil
}
