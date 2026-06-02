package store

import "github.com/computeflux/tee-provider/pkg/model"

// Txn 是 pkg/model.Txn 的本地别名，供 store 包直接使用。
type Txn = model.Txn

// UniAddr 是 pkg/model.UniAddr 的本地别名，供 store 包直接使用。
type UniAddr = model.UniAddr

// ComboNamespaceKey 拼接 namespace 与 key，返回存储用的字节切片。
func ComboNamespaceKey(namespace, key string) []byte {
	return []byte(namespace + "_" + key)
}

// keyUpperBound 返回覆盖给定前缀所有可能后缀的上界 key。
func keyUpperBound(b []byte) []byte {
	end := make([]byte, len(b))
	copy(end, b)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	// all bytes are 0xFF; return a byte slice that is larger than any prefix
	return append(b, 0x00)
}
