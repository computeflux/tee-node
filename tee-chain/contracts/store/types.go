package store

// StoreTx 抽象化存储操作接口。Native (*model.Txn) 和 WASM 均满足。
type StoreTx interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	ListByPrefix(prefix []byte) ([][]byte, [][]byte, error)
	DeletekeysByPrefix(prefix []byte) error
	Commit() error
	Rollback() error
}

// 不提供 Txn 别名 — 新代码使用 StoreTx 接口，测试文件自行 import model.Txn。

func ComboNamespaceKey(namespace, key string) []byte {
	return []byte(namespace + "_" + key)
}

func keyUpperBound(b []byte) []byte {
	end := make([]byte, len(b))
	copy(end, b)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return append(b, 0x00)
}
