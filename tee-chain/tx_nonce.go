package teechain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/wetee-dao/ink.go"
)

// callerNoncePrefix 是 caller nonce 的 PebbleDB key 前缀。
const callerNoncePrefix = "caller_nonce"

// nonceMu 保护内存中的 nonce 缓存并发访问。
var nonceMu sync.Mutex

// GetCallerNonceKey 生成 caller nonce 的 PebbleDB key。
func GetCallerNonceKey(caller []byte, callerType uint32) string {
	return fmt.Sprintf("%s:%d:%x", callerNoncePrefix, callerType, caller)
}

func GetNodeNextNonce(node ink.SignerType) uint64 {
	return GetNextCallerNonce(node.Public(), 0)
}

// GetNextCallerNonce 返回 caller 的下一个可用 nonce，并原子递增。
// 首次调用时返回 1（nonce 从 1 开始，0 表示未设置）。
func GetNextCallerNonce(caller []byte, callerType uint32) uint64 {
	nonceMu.Lock()
	defer nonceMu.Unlock()

	key := GetCallerNonceKey(caller, callerType)
	bt, err := model.GetKey("", key)
	if err != nil || len(bt) == 0 {
		// 首次使用，从 1 开始
		nextBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(nextBytes, 1)
		_ = model.SetKey("", key, nextBytes)
		return 1
	}

	current := binary.BigEndian.Uint64(bt)
	next := current + 1
	nextBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nextBytes, next)
	_ = model.SetKey("", key, nextBytes)
	return next
}

// GetCurrentCallerNonce 返回 caller 当前的 nonce（不递增）。
func GetCurrentCallerNonce(caller []byte, callerType uint32) uint64 {
	key := GetCallerNonceKey(caller, callerType)
	bt, err := model.GetKey("", key)
	if err != nil || len(bt) == 0 {
		return 0
	}
	return binary.BigEndian.Uint64(bt)
}

// TxnGetCurrentCallerNonce 在事务中读取 caller 当前 nonce。
func TxnGetCurrentCallerNonce(txn *model.Txn, caller []byte, callerType uint32) (uint64, error) {
	key := GetCallerNonceKey(caller, callerType)
	bt, err := txn.Get([]byte(key))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if len(bt) == 0 {
		return 0, nil
	}
	return binary.BigEndian.Uint64(bt), nil
}

// TxnSetCallerNonce 在事务中设置 caller nonce。
func TxnSetCallerNonce(txn *model.Txn, caller []byte, callerType uint32, nonce uint64) error {
	key := GetCallerNonceKey(caller, callerType)
	bt := make([]byte, 8)
	binary.BigEndian.PutUint64(bt, nonce)
	return txn.Set([]byte(key), bt)
}
