// package store: StoreValue 提供类似智能合约单值存储的能力。
// 用于存储不依赖 key 的单一值（如配置、状态标记等）。
package store

// StoreValue 类似 Solidity 中的单值存储（如状态变量）。
type StoreValue[V any] struct {
	Namespace string
	Key       string
}

func (s *StoreValue[V]) fullKey() []byte {
	return ComboNamespaceKey(s.Namespace, s.Key)
}

// get 读取 raw 值；不存在时返回 (nil, nil)。
func (s *StoreValue[V]) get(txn StoreTx) ([]byte, error) {
	v, err := txn.Get(s.fullKey())
	if err != nil || len(v) == 0 {
		return nil, nil // 不存在或读取失败均视为无数据
	}
	return v, nil
}

// set 写入 raw 值。
func (s *StoreValue[V]) set(txn StoreTx, value []byte) error {
	return txn.Set(s.fullKey(), value)
}

// Delete 删除该值。
func (s *StoreValue[V]) Delete(txn StoreTx) error {
	return txn.Delete(s.fullKey())
}

// Exists 判断该值是否存在。
func (s *StoreValue[V]) Exists(txn StoreTx) (bool, error) {
	v, err := s.get(txn)
	if err != nil {
		return false, err
	}
	return len(v) > 0, nil
}

// Get 读取并使用 codec.Decode 反序列化；不存在时返回 (nil, nil)。
func (s *StoreValue[V]) Get(txn StoreTx) (*V, error) {
	bt, err := s.get(txn)
	if err != nil {
		return nil, err
	}
	if bt == nil {
		return nil, nil
	}

	val := new(V)
	if err := ScaleDecode(val, bt); err != nil {
		return nil, err
	}
	return val, nil
}

// GetOrDefault 读取；不存在时返回 defaultVal。
func (s *StoreValue[V]) GetOrDefault(txn StoreTx, defaultVal V) (V, error) {
	val, err := s.Get(txn)
	if err != nil {
		var zero V
		return zero, err
	}
	if val == nil {
		return defaultVal, nil
	}
	return *val, nil
}

// Set 使用 codec.Encode 序列化并写入。
func (s *StoreValue[V]) Set(txn StoreTx, v V) error {
	bt, err := ScaleEncode(v)
	if err != nil {
		return err
	}
	return s.set(txn, bt)
}

// GetRaw 读取原始字节；不存在时返回 nil。
func (s *StoreValue[V]) GetRaw(txn StoreTx) ([]byte, error) {
	return s.get(txn)
}

// SetRaw 写入原始字节。
func (s *StoreValue[V]) SetRaw(txn StoreTx, value []byte) error {
	return s.set(txn, value)
}
