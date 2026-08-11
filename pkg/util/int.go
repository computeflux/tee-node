package util

import (
	"encoding/binary"
)

func Int64ToBytes(time int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(time))
	return b
}

// BytesToInt64 将 8 字节大端序字节串解析为 int64。
// 输入不足 8 字节时返回 0，避免 binary.BigEndian.Uint64 在短切片上 panic
// （例如从 DB 读到的损坏/空值）。
func BytesToInt64(b []byte) int64 {
	if len(b) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b))
}
