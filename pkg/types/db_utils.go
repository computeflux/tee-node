package types

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

// copyIterKey 复制迭代器返回的 key。
// Pebble 的 iter.Key() 返回的 slice 指向迭代器内部缓冲区，调用 Next() 后会被覆盖。
// 若需要将 key 保存到循环外部，必须通过此函数深拷贝。
func copyIterKey(k []byte) []byte {
	ck := make([]byte, len(k))
	copy(ck, k)
	return ck
}

func comboKey(namespace, key string) string {
	return namespace + "_" + key
}

func ComboNamespaceKey(namespace, key string) []byte {
	return []byte(comboKey(namespace, string(key)))
}
