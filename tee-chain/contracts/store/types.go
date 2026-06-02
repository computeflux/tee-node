package store

import "fmt"

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

// Option 类似 Rust Option，用于可空字段的 SCALE 编解码。
type Option[T any] struct {
	isSome bool
	V      T
}

func NewNone[T any]() Option[T] {
	return Option[T]{isSome: false}
}

func NewSome[T any](v T) Option[T] {
	return Option[T]{isSome: true, V: v}
}

func (o Option[T]) IsSome() bool { return o.isSome }
func (o Option[T]) IsNone() bool { return !o.isSome }

func (o Option[T]) UnWrap() (T, error) {
	if !o.isSome {
		var zero T
		return zero, fmt.Errorf("unwrap error: Option is None")
	}
	return o.V, nil
}
