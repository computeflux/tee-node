package store

// ── 交易级事件校验与资源上限 ──────────────────────────────────────────────
//
// 事件会直接进入 abci.ExecTxResult.Events（参与区块结果持久化与 LastResultsHash
// 计算）并被 CometBFT kv TxIndexer 全量索引。WASM 合约可在一个交易里循环调用
// emit_event（仓库内 wasm 引擎无 fuel/gas 计量），因此必须对事件数量、属性个数、
// key/value 长度做硬上限，并对属性 key 做格式校验，防止：
//   1. 合约可控的资源放大（撑大区块结果与索引库）；
//   2. 恶意合约伪造链上系统事件（relay_synced / contract_call）；
//   3. 恶意合约构造保留 tag（tx.height / tx.hash）导致 TxIndexer 索引写入失败。

// reservedEventTypes 系统管道事件保留名单（tee-chain/tx_finalize.go 发射）。
// 合约事件禁止使用这些名称，防止恶意合约伪造链上系统事件误导索引器/交易详情页。
// "tx" 是 CometBFT kv TxIndexer 的保留前缀（tx.height / tx.hash），
// 合约事件 type 为 "tx" 时属性 key "height"/"hash" 会拼出保留 tag 导致索引报错。
var reservedEventTypes = map[string]bool{
	"relay_synced":  true,
	"contract_call": true,
	"tx":            true,
}

// 事件资源上限（硬限制，超限直接丢弃，不索引）。
const (
	// MaxEventsPerTx 单笔交易事件条数上限。
	MaxEventsPerTx = 64
	// MaxAttrsPerEvent 单事件属性个数上限。
	MaxAttrsPerEvent = 32
	// MaxAttrKeyLen 属性 key 字节长度上限。
	MaxAttrKeyLen = 64
	// MaxAttrValueLen 属性 value 字节长度上限。
	MaxAttrValueLen = 256
	// MaxEventTypeLen 事件 type 字节长度上限。
	MaxEventTypeLen = 64
)

// IsValidEventType 校验交易级事件 type：
//   - 非空、长度 ≤ MaxEventTypeLen
//   - 小写字母开头，仅含小写字母/数字/下划线（统一小写下划线命名）
//   - 不在系统保留名单内（relay_synced / contract_call / tx）
//
// 非法返回 false。供原生 Runtime.EmitEvent 与 WASM emit_event host 函数共用。
func IsValidEventType(eventType string) bool {
	if eventType == "" || len(eventType) > MaxEventTypeLen {
		return false
	}
	if reservedEventTypes[eventType] {
		return false
	}
	for i, c := range eventType {
		if i == 0 {
			if c < 'a' || c > 'z' {
				return false
			}
			continue
		}
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// IsValidEventAttr 校验单个事件属性 key/value：
//   - key 非空、长度 ≤ MaxAttrKeyLen
//   - key 不含 '.'（CometBFT kv TxIndexer 用 event.Type + "." + attr.Key 拼索引 tag，
//     含 '.' 的 key 会产生歧义 tag）
//   - key 不含控制字符（\x00-\x1f）
//   - value 长度 ≤ MaxAttrValueLen
//
// 非法返回 false。
func IsValidEventAttr(key, value string) bool {
	if key == "" || len(key) > MaxAttrKeyLen {
		return false
	}
	if len(value) > MaxAttrValueLen {
		return false
	}
	for _, c := range key {
		if c == '.' || c < 0x20 {
			return false
		}
	}
	return true
}

// IsValidEvent 校验整个事件（type + 所有属性 + 属性个数上限）。
// 任一属性非法则整个事件被拒绝。供 Runtime.EmitEvent 与 WASM emit_event host 共用。
func IsValidEvent(eventType string, attrs map[string]string) bool {
	if !IsValidEventType(eventType) {
		return false
	}
	if len(attrs) > MaxAttrsPerEvent {
		return false
	}
	for k, v := range attrs {
		if !IsValidEventAttr(k, v) {
			return false
		}
	}
	return true
}
