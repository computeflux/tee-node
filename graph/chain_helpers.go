package graph

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/cockroachdb/pebble"
	m/cometbft/ccomombftscome"bftsre
	"github.com/computpflux/uee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/relay"
	ethcrypto ethcrypto "ge.herc/mtgo-ehhrmeumhmtpto
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

// maxChainOverviewLimit 是 chain_overview 单次查询的 blockLimit/txLimit 上限。
//
// 该查询未挂 @auth 指令、对匿名请求可达，入参若不加上限：
//   - make(..., 0, *limit) 会按攻击者给的容量预分配（传 1e9 即单请求申请数 GB 内存 → OOM）；
//   - 区块列表循环会遍历整条链的区块头（每次 LoadBlockMeta 都是一次 DB 读）。
//
// 因此对入参做统一截断：<=0 取默认 10，> 上限取上限。
const maxChainOverviewLimit = 100

// maxTxScanBlocks 是「最近交易列表」从 latest 向下逐块扫描的硬上限（块数）。
// 开销只与该常量 + txLimit 相关，与链上交易总量无关——防止交易稀疏时
// 为凑够 txLimit 而遍历整条链（匿名请求资源放大）。测试网出块节奏下，
// 最近交易通常落在最新少数几个块内，200 块足够覆盖。
const maxTxScanBlocks = 200

// totalTxCache 是总交易数的「按高度增量」缓存。
//
// 总交易数 = 全链所有块 NumTxs 之和。冷启动需全扫一次（O(块数) 次 LoadBlockMeta
// 小读，不物化交易字节）；之后每个新区块只累加新增块（cachedHeight+1 .. latest），
// 开销与新增块数相关而非全链。chain_overview 匿名可达且前端 12s 轮询，该缓存
// 避免高频请求重复全链扫描。高度键自洽：latest == cachedHeight 直接命中；
// latest > cachedHeight 增量累加；latest < cachedHeight（链重置）全扫重算。
type totalTxCache struct {
	mu           sync.Mutex
	cachedHeight int64
	cachedTotal  int
}

var totalTxCacheInst totalTxCache

// totalTxCountCached 返回截至 latest 的总交易数（按高度增量缓存）。
func totalTxCountCached(bs *store.BlockStore, latest int64) int {
	c := &totalTxCacheInst
	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case c.cachedHeight == latest:
		// 命中：高度一致，直接返回
		return c.cachedTotal

	case c.cachedHeight > 0 && c.cachedHeight < latest:
		// 增量：只累加新增块（cachedHeight+1 .. latest）的 NumTxs
		for h := c.cachedHeight + 1; h <= latest; h++ {
			if meta := bs.LoadBlockMeta(h); meta != nil {
				c.cachedTotal += meta.NumTxs
			}
		}
		c.cachedHeight = latest
		return c.cachedTotal

	default:
		// 冷启动 / 链重置：全扫 base..latest 累加 NumTxs
		base := bs.Base()
		if base < 1 {
			base = 1
		}
		total := 0
		for h := base; h <= latest; h++ {
			if meta := bs.LoadBlockMeta(h); meta != nil {
				total += meta.NumTxs
			}
		}
		c.cachedHeight = latest
		c.cachedTotal = total
		return total
	}
}

// mainChainExplorerBases 主链 ID → explorer 基址。
//
// 与 dapp 前端 `apps/dapp/src/shared/api/blockscout.ts` 的 BLOCKSCOUT_BASES 保持一致：
// 420420417 = Paseo Asset Hub（Polkadot Hub Testnet）。
//
// ⚠️ 当前 relay 管道未捕获主链区块号（见 crossChainInfoFromRelaySync 注释），
// 故本 map 暂未被使用；待上游 `RelayEvent`/`GetRelayEvents` 带出主链事件所在
// 区块号后，用它拼接 main_explorer_url。
var mainChainExplorerBases = map[uint32]string{
	420420417: "https://blockscout-testnet.polkadot.io",
}

// crossChainInfoFromRelaySync 从 SysCall_RelaySync 反解 SyncEvent，构造跨链关联信息。
//
// chain_id / nonce / event_type 直接取自 SyncEvent，语义可靠。
//
// ⚠️ main_block / main_explorer_url 当前**不输出**：relay 管道
// （tee-chain/tx_from_relay.go 的 BuildSyncEvent）传入的 BlockNum 是**侧链**
// 共识高度（app.state.Height），并非主链区块号；chaintypes.RelayEvent 也未携带
// 主链区块号。若直接拼接会指向主链 explorer 上一个毫不相干的区块，属误导性数据。
// 故在「上游带出真实主链区块号」的改造完成前，MainBlock 置 0、MainExplorerURL
// 置空（前端判 0 / 判空隐藏），避免给出错误跳转。
func crossChainInfoFromRelaySync(rs *model.RelaySync) *model.CrossChainInfo {
	ev := relay.FromRelaySysCall(rs)

	info := &model.CrossChainInfo{
		ChainID:         int(ev.ChainID),
		Nonce:           int(ev.Nonce),
		MainBlock:       0,  // 主链区块号尚未被 relay 管道捕获，置 0 表示「无」
		MainExplorerURL: "", // 同上，置空避免误导性跳转
	}
	if len(ev.EventType) > 0 {
		evType := string(ev.EventType)
		info.EventType = &evType
	}
	return info
}
