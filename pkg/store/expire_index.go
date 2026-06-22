package store

import (
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
)

const expireIndexPrefix = "expire:"

// ScheduleExpiration 为指定记录注册过期索引。
// namespace: 数据所在 namespace（如 "GATEWAY"）。
// subKey: 数据在该 namespace 下的 key（如 "token_by_key:auth_tee_xxx"）。
// expiresAt: 过期时间（Unix 秒）；<=0 表示永不过期，不注册索引。
// 索引 key 格式：{namespace}_expire:{expiresAt}:{subKey}
// value 直接存储 subKey，避免冗余。
func ScheduleExpiration(namespace, subKey string, expiresAt int64) error {
	if expiresAt <= 0 {
		return nil
	}
	return SECRET_DB.SetKey(namespace, expireIndexKey(expiresAt, subKey), []byte(subKey))
}

// CancelExpiration 删除指定记录的过期索引。
// 由于过期索引 key 中包含 expiresAt，调用方应传入创建时相同的 expiresAt。
// expiresAt <= 0 表示永不过期，无索引可删除，直接返回。
func CancelExpiration(namespace, subKey string, expiresAt int64) error {
	if expiresAt <= 0 {
		return nil
	}
	return SECRET_DB.DeleteKey(namespace, expireIndexKey(expiresAt, subKey))
}

// CleanupExpiredByIndex 基于过期索引扫描并删除已过期记录。
// 只遍历截止到 now 的索引项，时间复杂度 O(已过期数量)。
// deleteFn 接收原始 subKey 并执行删除逻辑；索引项会在 deleteFn 调用后自动删除。
// 为防止单次内存占用过大，按 batchSize 分批处理（默认 1000）。
func CleanupExpiredByIndex(namespace string, deleteFn func(subKey string) error) error {
	return CleanupExpiredByIndexBatch(namespace, deleteFn, 1000)
}

// CleanupExpiredByIndexBatch 支持指定批次的索引清理。
func CleanupExpiredByIndexBatch(namespace string, deleteFn func(subKey string) error, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 1000
	}
	now := time.Now().Unix()
	lower := []byte(comboKey(namespace, expireIndexPrefix))
	upper := []byte(comboKey(namespace, fmt.Sprintf("%s%s:", expireIndexPrefix, formatExpireTimestamp(now))))

	iter, err := SECRET_DB.NewIter(&pebble.IterOptions{
		LowerBound: lower,
		UpperBound: upper,
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	// 先收集再删除，避免在迭代过程中修改迭代器看到的 key 范围
	toDelete := make([]struct {
		indexKey []byte
		subKey   string
	}, 0, batchSize)
	var cleaned int

	for iter.First(); iter.Valid(); iter.Next() {
		raw, err := SECRET_DB.Unseal(iter.Value())
		var subKey string
		if err == nil {
			subKey = string(raw)
		}
		toDelete = append(toDelete, struct {
			indexKey []byte
			subKey   string
		}{indexKey: copyIterKey(iter.Key()), subKey: subKey})

		if len(toDelete) >= batchSize {
			cleaned += flushExpiredBatch(toDelete, deleteFn)
			toDelete = toDelete[:0]
		}
	}

	cleaned += flushExpiredBatch(toDelete, deleteFn)
	if cleaned > 0 {
		fmt.Printf("[cleanup] namespace=%s cleaned=%d\n", namespace, cleaned)
	}
	return nil
}

func flushExpiredBatch(batch []struct {
	indexKey []byte
	subKey   string
}, deleteFn func(subKey string) error) int {
	cleaned := 0
	for _, item := range batch {
		if item.subKey == "" {
			// 索引本身损坏或无法解密，直接删除索引
			if err := SECRET_DB.DeleteByteKey(item.indexKey); err == nil {
				cleaned++
			}
			continue
		}

		// 只有实际数据删除成功后才删除索引，避免永久丢失对数据的清理能力
		if err := deleteFn(item.subKey); err == nil {
			if err := SECRET_DB.DeleteByteKey(item.indexKey); err == nil {
				cleaned++
			} else {
				fmt.Printf("[cleanup] failed to delete index for %s: %v\n", item.subKey, err)
			}
		} else {
			fmt.Printf("[cleanup] failed to delete %s: %v\n", item.subKey, err)
		}
	}
	return cleaned
}

// expireIndexKey 生成过期索引 key。
// 格式：expire:{zero-padded-expiresAt}:{subKey}
// expiresAt 使用 16 位定长零填充，保证按字典序排列等价于按时间排序，且避免未来时间戳位数变化导致范围错误。
func expireIndexKey(expiresAt int64, subKey string) string {
	return fmt.Sprintf("%s%s:%s", expireIndexPrefix, formatExpireTimestamp(expiresAt), subKey)
}

// formatExpireTimestamp 将 Unix 秒时间戳格式化为 16 位定长字符串。
func formatExpireTimestamp(ts int64) string {
	return fmt.Sprintf("%016d", ts)
}
