package graph

// 本文件实现 tx_detail 的 balance_changes 字段解析（schema 见 chain.graphqls）。
//
// 注意：不要把这些代码放进 chain.resolvers.go —— 该文件由 gqlgen 重新生成，
// 手写逻辑会被清掉（同 address.resolvers.go 的约定）。此文件不受 gqlgen 管理，安全。

import (
	"encoding/hex"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/model"
	cstore "github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// balanceChangeEventType 是合约余额写入点统一发射的事件类型
// （tee-chain/runtime/module/gov/token.go 与 gateway/credit.go 的 emitBalanceChange）。
const balanceChangeEventType = "balance_change"

// balanceChangesFromEvents 从交易结果事件中解析余额变动列表。
//
// 只读解析：过滤 type == "balance_change" 的事件，取 attrs 的
// kind / address / delta 三个属性。address 是链上 UniAddr 的 T:hex(V) 编码
// （store.UniAddr.String()），用 uniAddrDisplay 转成展示地址（0x EVM / SS58）。
//
// 容错约定（旧交易 / 异常事件不报错）：
//   - 无事件 / 无 balance_change 事件 → 返回空列表（非 nil，GraphQL 输出 []）；
//   - 单条事件属性缺失或地址无法解析 → 跳过该条，不影响其余条目。
func balanceChangesFromEvents(events []abci.Event) []*model.BalanceChange {
	out := make([]*model.BalanceChange, 0, len(events))
	for _, ev := range events {
		if ev.Type != balanceChangeEventType {
			continue
		}
		var kind, addrEncoded, delta string
		for _, attr := range ev.Attributes {
			switch attr.Key {
			case "kind":
				kind = attr.Value
			case "address":
				addrEncoded = attr.Value
			case "delta":
				delta = attr.Value
			}
		}
		if kind == "" || addrEncoded == "" || delta == "" {
			continue
		}
		addr, ok := parseUniAddrEncoded(addrEncoded)
		if !ok {
			continue
		}
		display := uniAddrDisplay(addr)
		if display == "" {
			continue
		}
		out = append(out, &model.BalanceChange{
			Kind:    kind,
			Address: display,
			Delta:   delta,
		})
	}
	return out
}

// parseUniAddrEncoded 解析 UniAddr 的 T:hex(V) 编码（store.UniAddr.String() 的逆）。
// 格式非法（无冒号 / T 非数字 / V 非 hex）时返回 ok=false。
func parseUniAddrEncoded(s string) (cstore.UniAddr, bool) {
	idx := strings.IndexByte(s, ':')
	if idx <= 0 {
		return cstore.UniAddr{}, false
	}
	var t uint32
	for _, c := range s[:idx] {
		if c < '0' || c > '9' {
			return cstore.UniAddr{}, false
		}
		t = t*10 + uint32(c-'0')
	}
	v, err := hex.DecodeString(s[idx+1:])
	if err != nil {
		return cstore.UniAddr{}, false
	}
	return cstore.UniAddr{T: t, V: v}, true
}
