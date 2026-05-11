package model

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/cockroachdb/pebble"
	"github.com/cosmos/gogoproto/proto"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
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
	val, err := util.SealWithProductKey(value, nil)
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

	return util.Unseal(v, nil)
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
		// 复制 key（iter.Key() 返回的 slice 指向内部缓冲区）
		key := iter.Key()
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)

		// 解密数据
		v := iter.Value()
		decrypted, err := util.Unseal(v, nil)
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
	iter, err := DBINS.NewIter(&pebble.IterOptions{
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
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: key,
		UpperBound: keyUpperBound(key),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		v := iter.Value()
		value, err := util.Unseal(v, nil)
		if err != nil {
			return nil, err
		}

		val := new(T)
		err = protoio.ReadMessage(bytes.NewBuffer(value), val)
		if err == nil {
			list = append(list, val)
		}
	}

	return
}

func TxnGetProtoMessage[T any](txn *Txn, key []byte) (*T, error) {
	v, err := txn.Get(key)
	if err != nil {
		return nil, err
	}

	if len(v) == 0 {
		return nil, nil
	}

	val := new(T)
	err = protoio.ReadMessage(bytes.NewBuffer(v), val)
	return val, err
}

func TxnSetProtoMessage[T proto.Message](txn *Txn, key []byte, value T) error {
	buf := new(bytes.Buffer)
	err := protoio.WriteMessage(value, buf)
	if err != nil {
		return err
	}
	return txn.Set(key, buf.Bytes())
}

func TxnSetCodec[T any](txn *Txn, namespace, key string, val T) error {
	bt, err := codec.Encode(val)
	if err != nil {
		return err
	}

	return txn.Set([]byte(comboKey(namespace, key)), bt)
}

func TxnGetCodec[T any](txn *Txn, namespace, key string) (*T, error) {
	bt, err := txn.Get([]byte(comboKey(namespace, key)))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	val := new(T)
	err = codec.Decode(bt, val)
	return val, err
}

func (txn *Txn) Rollback() error {
	return txn.in.Close()
}

func (txn *Txn) Commit() error {
	return txn.in.Commit(pebble.Sync)
}
