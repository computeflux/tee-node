package types

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"google.golang.org/protobuf/proto"
)

type Txn struct {
	in *pebble.Batch
}

func (db *DB) NewTransaction() *Txn {
	return &Txn{in: db.DB.NewIndexedBatch()}
}

func (txn *Txn) SetKey(namespace, key string, value []byte) error {
	return txn.Set([]byte(comboKey(namespace, key)), value)
}

func (txn *Txn) GetKey(namespace, key string, value proto.Message) ([]byte, error) {
	return txn.Get([]byte(comboKey(namespace, key)))
}

func (txn *Txn) Set(key, value []byte) error {
	// fmt.Println("----------------------txn set ==>  ", string(key))
	val, err := SECRET_DB.Seal(value)
	if err != nil {
		return err
	}

	return txn.in.Set(key, val, pebble.Sync)
}

func (txn *Txn) Get(key []byte) ([]byte, error) {
	v, _, err := txn.in.Get(key)
	if err != nil {
		return nil, err
	}

	return SECRET_DB.Unseal(v)
}

func (txn *Txn) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
	iter, err := txn.in.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})

	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	var keys [][]byte
	var list [][]byte
	for iter.First(); iter.Valid(); iter.Next() {
		// iter.Key() 指向内部缓冲区，需深拷贝后再保存
		keyCopy := copyIterKey(iter.Key())

		// 解密数据（深拷贝 value，避免 iter.Next() 覆盖内部缓冲区）
		v := iter.Value()
		valueCopy := make([]byte, len(v))
		copy(valueCopy, v)
		decrypted, err := SECRET_DB.Unseal(valueCopy)
		if err != nil {
			return nil, nil, err
		}

		list = append(list, decrypted)
		keys = append(keys, keyCopy)
	}

	return keys, list, nil
}

// ListByPrefixLimit 带 offset/limit 的 prefix 扫描，返回 key 和 value 列表。
// offset 为跳过的条目数，limit 为最大返回数（0 表示无限制）。
func (txn *Txn) ListByPrefixLimit(prefix []byte, offset, limit uint64) ([][]byte, [][]byte, error) {
	iter, err := txn.in.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	var keys [][]byte
	var list [][]byte
	var skipped uint64
	for iter.First(); iter.Valid(); iter.Next() {
		if offset > 0 {
			if skipped < offset {
				skipped++
				continue
			}
		}
		if limit > 0 && uint64(len(keys)) >= limit {
			break
		}
		// iter.Key() 指向内部缓冲区，需深拷贝后再保存
		keyCopy := copyIterKey(iter.Key())

		// 解密数据（深拷贝 value，避免 iter.Next() 覆盖内部缓冲区）
		v := iter.Value()
		valueCopy := make([]byte, len(v))
		copy(valueCopy, v)
		decrypted, err := SECRET_DB.Unseal(valueCopy)
		if err != nil {
			return nil, nil, err
		}
		list = append(list, decrypted)
		keys = append(keys, keyCopy)
	}
	return keys, list, nil
}

func (txn *Txn) Delete(key []byte) error {
	return txn.in.Delete(key, pebble.Sync)
}

func (txn *Txn) DeletekeysByPrefix(prefix []byte) error {
	iter, err := txn.in.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if err := txn.Delete(iter.Key()); err != nil {
			return err
		}
	}

	return nil
}

func TxnGetJson[T any](txn *Txn, key []byte) (*T, error) {
	v, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if len(v) == 0 {
		return new(T), nil
	}

	val := new(T)
	err = json.Unmarshal(v, val)

	return val, err
}

func TxnSetJson[T any](txn *Txn, key []byte, val *T) error {
	bt, err := json.Marshal(val)
	if err != nil {
		return err
	}

	// fmt.Println("set", key, string(bt))
	return txn.Set(key, bt)
}

func TxnGetProtoMessageList[T any](txn *Txn, key []byte) (list []*T, err error) {
	iter, err := txn.in.NewIter(&pebble.IterOptions{
		LowerBound: key,
		UpperBound: keyUpperBound(key),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		v := iter.Value()
		value, err := SECRET_DB.Unseal(v)
		if err != nil {
			return nil, err
		}

		val := new(T)
		err = protoio.Unmarshal(value, val)
		if err == nil {
			list = append(list, val)
		}
	}

	return
}

func TxnGetProtoMessage[T any](txn *Txn, key []byte) (*T, error) {
	v, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if len(v) == 0 {
		return nil, nil
	}

	val := new(T)
	err = protoio.Unmarshal(v, val)
	return val, err
}

func TxnSetProtoMessage[T protoio.VTMarshaler](txn *Txn, key []byte, value T) error {
	buf := new(bytes.Buffer)
	err := protoio.WriteMessage(value, buf)
	if err != nil {
		return err
	}
	return txn.Set(key, buf.Bytes())
}

func TxnSetCodec[T any](txn store.StoreTx, namespace, key string, val T) error {
	bt, err := store.ScaleEncode(val)
	if err != nil {
		return err
	}

	return txn.Set([]byte(comboKey(namespace, key)), bt)
}

func TxnGetCodec[T any](txn store.StoreTx, namespace, key string) (*T, error) {
	bt, err := txn.Get([]byte(comboKey(namespace, key)))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	val := new(T)
	err = store.ScaleDecode(val, bt)
	return val, err
}

func (txn *Txn) NewIter(opts *pebble.IterOptions) (*pebble.Iterator, error) {
	return txn.in.NewIter(opts)
}

func (txn *Txn) Rollback() error {
	return txn.in.Close()
}

func (txn *Txn) Commit() error {
	return txn.in.Commit(pebble.Sync)
}

func (txn *Txn) Count() uint32 {
	return txn.in.Count()
}
