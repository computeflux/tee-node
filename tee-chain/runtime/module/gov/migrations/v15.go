package migrations

import (
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// V15 将旧版 apiKeys 数据迁移到 APIKeysV2 格式。
func V15(api store.ContractApi) error {
	txn := api.GetTxn()
	base := gateway.NewGateway(api)

	keys, vals, err := base.APIKeys.Primary.ListRaw(txn)
	if err != nil {
		return err
	}

	fmt.Printf("[V15] total entries: %d\n", len(vals))

	migrated := 0
	for i, raw := range vals {
		var old gateway.APIKeyInfo
		if err := store.ScaleDecode(&old, raw); err != nil {
			if migrated == 0 {
				fmt.Printf("[V15] first decode failed: len=%d err=%v\n", len(raw), err)
			}
			continue
		}

		v2 := gateway.APIKeyInfoV2{
			OwnerKey:  old.OwnerKey,
			Prefix:    "",
			Owner:     old.Owner,
			KeyHash:   old.KeyHash,
			RateLimit: old.RateLimit,
			IsActive:  old.IsActive,
			CreatedAt: old.CreatedAt,
		}
		if err := base.APIKeysV2.Put(txn, keys[i], v2); err != nil {
			return fmt.Errorf("put v2: %w", err)
		}
		migrated++
	}

	fmt.Printf("[V15] migrated: %d / %d\n", migrated, len(vals))
	return nil
}
