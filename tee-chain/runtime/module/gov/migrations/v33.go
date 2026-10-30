package migrations

import (
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// V33 为充值历史补写跨链凭据字段（ChainID / Nonce）。
//
// 迁移背景：RechargeRecord 在 User/Amount/Time 之后追加了 ChainID(uint32) 与
// Nonce(uint64)，用于把本链充值记录与主链 Recharge 事件（链 ID + 事件序号）关联。
// 旧记录的字节流短 12 字节，按新编码解码会 EOF，因此必须原地重写全部记录。
//
// 旧记录无法反推来源链：recharge_nonces 只保存已使用 nonce 的集合，
// 与具体历史条目没有对应关系。故迁移后旧记录 ChainID/Nonce 均为 0
// （语义：非跨链来源或链信息未知），新充值由 Recharge 写入真实值。
//
// 注意：这里不能用 rechargeHistory.List/ListRaw —— store key 是 (内层id, 序号)
// 的二进制编码，可能包含 '_'，而 StoreMapping.List/ListRaw 会跳过含 '_' 的 suffix。
// 因此按完整前缀扫描并按原 key 写回。
func V33(api store.ContractApi) error {
	txn := api.GetTxn()
	prefix := store.ComboNamespaceKey("gateway", "recharge_history_store_")

	keys, vals, err := txn.ListByPrefix(prefix)
	if err != nil {
		return err
	}

	fmt.Printf("[V33] recharge history total: %d\n", len(vals))

	migrated := 0
	for i, key := range keys {
		raw := vals[i]

		// 已是新编码（迁移重复执行）：V2 解码恰好消费全部字节，无需重写。
		var cur gateway.RechargeRecord
		if n, err := cur.Decode(raw); err == nil && n == len(raw) {
			continue
		}

		// 旧编码：按 V1 解码。仅当恰好消费完所有字节时才认定是旧记录，
		// 避免 ScaleDecode 忽略尾部字节、在重复迁移时把 ChainID/Nonce 抹成 0。
		var old gateway.RechargeRecordV1
		n, err := old.Decode(raw)
		if err != nil || n != len(raw) {
			fmt.Printf("[V33] skip undecodable record key=%x: %v\n", key, err)
			continue
		}

		rec := gateway.RechargeRecord{
			User:   old.User,
			Amount: old.Amount,
			Time:   old.Time,
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

	fmt.Printf("[V33] migrated: %d / %d\n", migrated, len(vals))
	return nil
}
