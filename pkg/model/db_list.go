package model

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

func AddToList(namespace string, key string, val []byte) error {
	indexBt, err := GetKey(namespace, "index_"+key)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return err
	}

	var index uint64 = 0
	if indexBt != nil {
		index = util.BytesToUint64(indexBt)
	}

	txn := DBINS.NewTransaction()
	if err := txn.SetKey(namespace, "index_"+key, util.Uint64ToBytes(index+1)); err != nil {
		return err
	}
	if err := txn.SetKey(namespace, key+fmt.Sprint(index), val); err != nil {
		return err
	}
	return txn.Commit()
}

// GetList 基于游标分页获取列表。
// cursor: 上一页最后一条记录的键，nil 或空表示从首条开始。
// size: 本页最多返回条数。
// 返回: 本页数据、下一页游标（无更多为 nil）、错误。
func GetList(namespace string, subkey string, cursor []byte, size int) ([][]byte, []byte, error) {
	key := []byte(comboKey(namespace, subkey))
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: key,
		UpperBound: keyUpperBound(key),
	})
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	var list [][]byte
	var nextCursor []byte
	var lastKey []byte

	// 有游标：从游标之后开始
	if len(cursor) > 0 {
		if !iter.SeekGE(cursor) {
			return list, nil, nil
		}
		// 若停在游标键上，跳过该条（游标即上页最后一条）
		if iter.Valid() && bytes.Equal(iter.Key(), cursor) {
			iter.Next()
		}
	} else {
		iter.First()
	}

	for ; iter.Valid() && len(list) < size; iter.Next() {
		v := iter.Value()
		value, err := util.Unseal(v, nil)
		if err != nil {
			return nil, nil, err
		}
		list = append(list, value)
		k := iter.Key()
		lastKey = make([]byte, len(k))
		copy(lastKey, k)
	}

	// 若取满 size 条且循环后 iter 仍有效，说明还有下一条，返回当前页最后一条的键作游标
	if len(list) == size && lastKey != nil && iter.Valid() {
		nextCursor = make([]byte, len(lastKey))
		copy(nextCursor, lastKey)
	}

	return list, nextCursor, nil
}

func DeleteList(namespace string, key string) error {
	return DeletekeysByPrefix(namespace, key)
}
