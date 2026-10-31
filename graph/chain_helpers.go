package graph

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cockroachdb/pebble"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
)

// normalizeHexHash 把前端传入的哈希规范化为裸 hex 字节（兼容 0x 前缀）。
// 供 block_detail 等按哈希查询的 resolver 使用。
func normalizeHexHash(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.EqualFold(s[:2], "0x") {
		s = s[2:]
	}
	if s == "" {
		return nil, fmt.Errorf("empty hash")
	}
	return hex.DecodeString(s)
}

// decodeTxBox 把原始交易字节解码为 model.Tx。
func decodeTxBox(txBytes []byte) (*model.Tx, error) {
	if len(txBytes) == 0 {
		return nil, nil
	}
	txBox := new(model.Tx)
	if err := protoio.Unmarshal(txBytes, txBox); err != nil {
		return nil, err
	}
	return txBox, nil
}

// decodeSysCall 把 call 字节解码为 model.SysCall。
func decodeSysCall(callBytes []byte) (*model.SysCall, error) {
	if len(callBytes) == 0 {
		return nil, nil
	}
	call := new(model.SysCall)
	if err := protoio.Unmarshal(callBytes, call); err != nil {
		return nil, err
	}
	return call, nil
}

// intPtr 把 int64 转为 *int（0 也返回指针，便于前端区分「无」与「0」）。
func intPtr(v int64) *int {
	n := int(v)
	return &n
}

// strPtr 返回字符串指针。
func strPtr(s string) *string {
	return &s
}

// prefixUpperBound 计算前缀扫描的排他上界（末字节进位）。
// 供 pebble 前缀迭代使用。
func prefixUpperBound(prefix []byte) []byte {
	b := make([]byte, len(prefix))
	copy(b, prefix)
	for i := len(b) - 1; i >= 0; i-- {
		b[i]++
		if b[i] != 0 {
			return b[:i+1]
		}
	}
	return append(prefix, 0)
}

// displayCallerAddr 根据 callerType 把交易发起方字节回显为用户可识别的地址。
//
//   - CallerType=2（EVM）：caller 为 20 字节以太坊地址 / 33 字节压缩公钥 /
//     65 字节未压缩公钥，统一派生为 0x 前缀 40 hex 地址（EVM 用户可识别）。
//   - 其它（TEE / Polkadot sr25519 / ed25519）：按 SS58 回显（向后兼容）。
//
// EVM 派生失败时回退 SS58，避免丢字段。
func displayCallerAddr(caller []byte, callerType uint32) string {
	if callerType == 2 {
		if addr := evmCallerToAddress(caller); addr != "" {
			return addr
		}
	}
	return crypto.PubKeyFromByte(caller).SS58()
}

// evmCallerToAddress 把 EVM caller 字节派生为 0x 前缀 20 字节地址。
//
//   - 20 字节：直接是以太坊地址
//   - 33 字节：压缩 secp256k1 公钥（0x02/0x03 前缀）→ 解压 → keccak256[12:]
//   - 65 字节：未压缩 secp256k1 公钥（0x04 前缀）→ keccak256[12:]
//
// 无法识别的长度或前缀返回空串（调用方回退 SS58）。
func evmCallerToAddress(caller []byte) string {
	var addr [20]byte
	switch len(caller) {
	case 20:
		copy(addr[:], caller)
	case 33:
		if caller[0] != 0x02 && caller[0] != 0x03 {
			return ""
		}
		pk, err := ethcrypto.DecompressPubkey(caller)
		if err != nil {
			return ""
		}
		copy(addr[:], ethcrypto.PubkeyToAddress(*pk).Bytes())
	case 65:
		if caller[0] != 0x04 {
			return ""
		}
		h := ethcrypto.Keccak256(caller[1:])
		copy(addr[:], h[12:])
	default:
		return ""
	}
	return "0x" + hex.EncodeToString(addr[:])
}

// countAddressKeys 统计曾发起过交易的地址数。
//
// 每个发起过交易的地址在 SECRET_DB 中会有一条 `call_nonce:{type}:{hex}` 的
// nonce 记录（FinalizeTx 提交时写入），因此按 `call_nonce:` 前缀计数即得
// 去重后的地址数。DB 未就绪时返回 0（前端兜底展示）。
func countAddressKeys() int {
	db := dbstore.SECRET_DB
	if db == nil {
		return 0
	}
	prefix := []byte("call_nonce:")
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return 0
	}
	defer iter.Close()
	n := 0
	for iter.First(); iter.Valid(); iter.Next() {
		n++
	}
	return n
}
