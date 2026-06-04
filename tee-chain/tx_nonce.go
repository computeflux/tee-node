package teechain

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/wetee-dao/ink.go"
)

const callerNoncePrefix = "call_nonce"

func GetNodeNextNonce(node ink.SignerType) uint64 {
	return GetNextCallerNonce(node.Public(), 0)
}

// GetNextCallerNonce 返回下一个可用 nonce（只读，不写 DB）。
// DB 仅在 FinalizeTx 成功提交时通过 txn 更新。
func GetNextCallerNonce(caller []byte, callerType uint32) uint64 {
	key := GetCallerNonceKey(caller, callerType)
	bt, err := dbstore.GetFromByteKey([]byte(key))
	if err != nil || len(bt) == 0 {
		return 1
	}
	return binary.BigEndian.Uint64(bt) + 1
}

func TxnGetCurrentCallerNonce(txn *dbstore.Txn, caller []byte, callerType uint32) (uint64, error) {
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

func TxnSetCallerNonce(txn *dbstore.Txn, caller []byte, callerType uint32, nonce uint64) error {
	key := GetCallerNonceKey(caller, callerType)
	return txn.Set([]byte(key), uint64BigEndian(nonce))
}

func GetCallerNonceKey(caller []byte, callerType uint32) string {
	return fmt.Sprintf("%s:%d:%x", callerNoncePrefix, callerType, caller)
}

func uint64BigEndian(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}
