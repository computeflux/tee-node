package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/cockroachdb/pebble"
	"github.com/cosmos/gogoproto/proto"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

var DBINS *DB

const (
	dbPath = "./chain_data/wetee"
)

type DB struct {
	*pebble.DB
}

func NewDB() (*DB, error) {
	// Open DB
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return nil, err
	}

	// Create a new DB instance and initialize with DB
	dbInstance := &DB{}
	dbInstance.DB = db

	DBINS = dbInstance

	return DBINS, nil
}

func Set(key string, value []byte) error {
	return SetKey("", key, value)
}

func Get(key string) ([]byte, error) {
	return GetKey("", key)
}

func SetKey(namespace, key string, value []byte) error {
	val, err := util.SealWithProductKey(value, nil)
	if err != nil {
		return err
	}

	return DBINS.Set([]byte(comboKey(namespace, key)), val, pebble.Sync)
}

func SetJson[T any](namespace, key string, val *T) error {
	bt, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return SetKey(namespace, key, bt)
}

func SetCodec[T any](namespace, key string, val T) error {
	bt, err := codec.Encode(val)
	if err != nil {
		return err
	}

	return SetKey(namespace, key, bt)
}

func GetCodec[T any](namespace, key string) (*T, error) {
	bt, err := GetKey(namespace, key)
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

func GetCodecOrDefault[T any](namespace, key string, defaultVal T) (T, error) {
	val, err := GetCodec[T](namespace, key)
	if err != nil {
		var zero T
		return zero, err
	}
	if val == nil {
		return defaultVal, nil
	}
	return *val, nil
}

func GetKey(namespace, key string) ([]byte, error) {
	value, _, err := DBINS.Get([]byte(comboKey(namespace, key)))
	if err != nil {
		return nil, err
	}

	return util.Unseal(value, nil)
}

func GetJson[T any](namespace, key string) (*T, error) {
	v, err := GetKey(namespace, key)
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
	err = json.Unmarshal(v, val)

	return val, err
}

func GetJsonList[T any](namespace, key string) ([]*T, [][]byte, error) {
	rkey := []byte(comboKey(namespace, key))
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: rkey,
		UpperBound: keyUpperBound(rkey),
	})
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	var keys [][]byte
	var list []*T
	for iter.First(); iter.Valid(); iter.Next() {
		v := iter.Value()
		value, err := util.Unseal(v, nil)
		if err != nil {
			return nil, nil, err
		}

		val := new(T)
		err = json.Unmarshal(value, val)
		if err != nil {
			return nil, nil, fmt.Errorf("unmarshal db entry: %w", err)
		}
		keys = append(keys, iter.Key())
		list = append(list, val)
	}

	return list, keys, nil
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
	// all bytes are 0xFF; return a byte slice that is larger than any prefix
	return append(b, 0x00)
}

func GetProtoMessageList[T any](namespace, key string) ([]*T, [][]byte, error) {
	rkey := []byte(comboKey(namespace, key))
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: rkey,
		UpperBound: keyUpperBound(rkey),
	})
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	keys := make([][]byte, 0, 100)
	list := make([]*T, 0, 100)
	for iter.First(); iter.Valid(); iter.Next() {
		v := iter.Value()
		keys = append(keys, *util.DeepCopy(iter.Key()))
		value, err := util.Unseal(v, nil)
		if err != nil {
			return nil, nil, err
		}

		val := new(T)
		err = protoio.ReadMessage(bytes.NewBuffer(value), val)
		if err == nil {
			list = append(list, val)
		}
	}

	return list, keys, nil
}

func GetProtoMessage[T any](namespace, key string) (*T, error) {
	v, err := GetKey(namespace, key)
	if err != nil {
		return nil, err
	}

	if len(v) == 0 {
		return nil, nil
	}

	value, err := util.Unseal(v, nil)
	if err != nil {
		return nil, err
	}

	val := new(T)
	err = protoio.ReadMessage(bytes.NewBuffer(value), val)
	return val, err
}

func SetProtoMessage[T proto.Message](namespace, key string, value T) error {
	buf := new(bytes.Buffer)
	err := protoio.WriteMessage(value, buf)
	if err != nil {
		return err
	}
	return SetKey(namespace, key, buf.Bytes())
}

func DeleteKey(namespace, key string) error {
	return DBINS.Delete([]byte(comboKey(namespace, key)), pebble.Sync)
}

func DeleteByteKey(key []byte) error {
	return DBINS.Delete(key, pebble.Sync)
}

func DeletekeysByPrefix(namespace, key string) error {
	rkey := []byte(comboKey(namespace, key))
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: rkey,
		UpperBound: keyUpperBound(rkey),
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	txn := DBINS.DB.NewBatch()
	for iter.First(); iter.Valid(); iter.Next() {
		txn.Delete(iter.Key(), pebble.Sync)
	}

	return txn.Commit(pebble.Sync)
}

func comboKey(namespace, key string) string {
	return namespace + "_" + key
}

func ComboNamespaceKey(namespace, key string) []byte {
	return []byte(comboKey(namespace, string(key)))
}
