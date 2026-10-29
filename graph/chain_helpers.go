package graph

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
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
