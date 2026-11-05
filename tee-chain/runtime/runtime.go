// Package contracts 提供合约运行时环境和合约调用入口。
// 本文件定义了合约执行时的运行时上下文，包含区块高度、交易信息和调用者身份。
package runtime

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/teetypes"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// Runtime 合约运行时上下文，封装了合约执行所需的环境信息。
// 它实现了 store.ContractApi 接口，在 FinalizeTx / Query 等路径中传入区块高度、交易和调用方信息。
// 每次合约调用都会创建一个新的 Runtime 实例，确保执行上下文的隔离性。
type Runtime struct {
	unix    int64         // 当前区块时间戳，用于时间相关的业务逻辑判断
	height  int64         // 当前区块高度，用于时间相关的业务逻辑判断
	version uint32        // 当前执行的运行时版本号
	txn     store.StoreTx // 当前交易对象，包含交易的签名和原始数据
	caller  store.UniAddr // 调用者地址，标识合约方法的调用发起方
	sudo    store.UniAddr // 超级管理员账户地址，用于执行特权操作
	events  *[]abci.Event // 交易级事件管道（共享 sink）：合约执行期间通过 EmitEvent 累积，
	// 跨合约调用（Call/Query 派生的子 Runtime）共享同一 sink，FinalizeTx 取出后挂到 ExecTxResult
}

// NewRuntime 创建一个新的合约运行时实例。
// 参数:
//   - height: 当前区块高度
//   - txn: 交易对象，包含交易的完整信息
//   - caller: 调用者的区块链地址
//   - sudo: 超级管理员账户地址
//
// 返回:
//   - Runtime: 初始化后的运行时上下文对象
//
// 注意：events 必须初始化为 &[]abci.Event{}，否则 Call/Query 派生的子 Runtime
// 拷贝到的 events 为 nil，子合约事件会写入各自的独立 sink 而丢失。
func NewRuntime(height int64, ts time.Time, txn *dbstore.Txn, caller store.UniAddr, sudo store.UniAddr) *Runtime {
	return &Runtime{unix: ts.Unix(), height: height, version: 0, txn: txn, caller: caller, sudo: sudo, events: &[]abci.Event{}}
}

// NewRuntimeWithVersion 创建带指定版本的合约运行时实例。
// events 同样初始化为非 nil，保证跨合约调用共享同一事件 sink。
func NewRuntimeWithVersion(height int64, ts time.Time, txn *dbstore.Txn, caller store.UniAddr, sudo store.UniAddr, version uint32) *Runtime {
	return &Runtime{unix: ts.Unix(), height: height, version: version, txn: txn, caller: caller, sudo: sudo, events: &[]abci.Event{}}
}

// GetHeight 获取当前区块高度。
// 该方法用于合约内部判断区块时间或执行与区块高度相关的业务逻辑。
// 返回:
//   - int64: 当前区块高度
func (r *Runtime) GetHeight() int64 {
	return r.height
}

// GetBlockTime 获取当前区块时间戳。
// 该方法用于合约内部判断区块时间或执行与时间相关的业务逻辑。
// 返回:
//   - int64: 当前区块时间戳（单位：秒）
func (r *Runtime) GetBlockTime() int64 {
	return r.unix
}

// GetVersion 获取当前执行的运行时版本号。
// 返回:
//   - uint32: 运行时版本号
func (r *Runtime) GetVersion() uint32 {
	return r.version
}

// GetTxn 获取当前交易对象。
// 交易对象包含签名数据和原始交易数据，可用于验证交易有效性。
// 返回:
//   - *dbstore.Txn: 当前交易对象的指针
func (r *Runtime) GetTxn() store.StoreTx {
	return r.txn
}

// GetCaller 获取合约调用者的地址。
// 该地址用于权限验证和身份识别，确保只有授权的调用者能执行特定操作。
// 返回:
//   - []byte: 调用者的区块链地址字节切片
func (r *Runtime) GetCaller() store.UniAddr {
	return r.caller
}

// GetSudoAccount 获取超级管理员账户地址。
// 超级管理员拥有执行特权操作的权限，如设置公共加入开关等。
// 返回:
//   - UniAddr: 系统账户地址
func (r *Runtime) GetSystemAccount() store.UniAddr {
	return r.sudo
}

// newAbciEvent 构造一个 abci.Event：属性按 key 排序后追加，保证属性顺序确定。
// Go map 迭代顺序随机，而事件会持久化进区块结果（LastResultsHash）并被 TxIndexer 索引，
// 顺序不确定会导致节点间结果不一致（潜在共识分歧）。所有属性标记 Index=true。
// 调用前必须已通过 store.IsValidEvent 校验（type + 属性 + 个数上限）。
func newAbciEvent(eventType string, attrs map[string]string) abci.Event {
	event := abci.Event{Type: eventType}
	if len(attrs) == 0 {
		return event
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		event.Attributes = append(event.Attributes, abci.EventAttribute{
			Key:   k,
			Value: attrs[k],
			Index: true,
		})
	}
	return event
}

// EmitEvent 发射一个交易级事件，累积到运行时事件管道（共享 sink）。
// 事件 type 统一小写下划线命名（如 "credit_transfer"、"balance_changed"）。
// 完整校验（store.IsValidEvent）：type 格式/保留名 + 属性 key/value 格式与长度 +
// 属性个数上限；单笔交易事件条数上限 store.MaxEventsPerTx。超限/非法静默丢弃，
// 防止合约资源放大与伪造系统事件/保留 tag。
// 并发安全非必须：FinalizeTx 单线程顺序执行。
func (r *Runtime) EmitEvent(eventType string, attrs map[string]string) {
	if !store.IsValidEvent(eventType, attrs) {
		return
	}
	if r.events == nil {
		r.events = &[]abci.Event{}
	}
	if len(*r.events) >= store.MaxEventsPerTx {
		return
	}
	*r.events = append(*r.events, newAbciEvent(eventType, attrs))
}

// TakeEvents 取出并清空当前运行时累积的所有事件（含跨合约调用子 Runtime 发射的事件）。
// FinalizeTx 在合约执行完毕后调用，将事件挂到 abci.ExecTxResult.Events。
func (r *Runtime) TakeEvents() []abci.Event {
	if r.events == nil {
		return nil
	}
	events := *r.events
	*r.events = nil
	return events
}

// CallbySystem 模拟系统调用，使用系统账户地址作为调用者。
// 该方法用于执行需要系统权限的操作，如合约初始化等。
// 返回:
//   - bool: true 表示调用者是系统账户，false 表示不是系统账户
func (r *Runtime) CallbySystem() bool {
	if r.sudo.T != r.caller.T {
		return false
	}

	return bytes.Equal(r.sudo.V, r.caller.V)
}

// teeTimeDrift 签名时间（节点签发 quote 的墙钟时间）与区块时间的允许偏差（秒）。
// 区块时间由 proposer 时钟设定，quote 时间由执行节点时钟设定，两者存在秒级偏差；
// 超出窗口视为旧 quote 重放，拒绝。
const teeTimeDrift = int64(600)

// VerifyTEE 校验客户端传入的 TEE quote。
// report 仅为 quote 字节（model.TeeCall 的 Report 字段，非整个 TeeCall）；
// data 为被证明的数据字节（绑定为 TeeCall.Tx = TeeCall_Text{Text: data}，如放款数据的 SCALE 编码）；
// t 为节点签发 quote 时的时间戳（节点对 [t || Caller || marshal(Tx)] 签名所用的 Time）；
// caller 为报告签发者公钥（quote 绑定的 Caller，须与节点交易签名密钥一致，节点侧签名时即绑定）；
// teeType 为报告签发节点的 TEE 类型（0=SGX, 1=SEV-SNP, 2=TDX, 9999=非 TEE），
// VerifyReport 据此分发到对应验签分支（类型与 quote 不符 → 硬件验签失败，fail-closed）。
// 区块时间 r.unix 仅用于新鲜度窗口校验（节点签发时间与实际区块时间必有秒级偏差，
// 不能直接用作签名时间）。
//
// 节点侧签发报告须与上述参数对齐：用与交易签名相同的密钥，以签发瞬间的墙钟时间为 t，
// 对 [t || Caller || marshal(TeeCall_Text{Text: data})] 签名后生成 quote，并把同一 t 随交易上报。
// 成功返回 Ok，携带 TEE 证明的代码三要素（TeeType/CodeSigner/CodeSignature），供合约验证用户的 CodeSignature；
// 失败返回 Err，携带验证错误信息。
func (r *Runtime) VerifyTEE(report, data []byte, t int64, caller []byte, teeType uint32) store.Result[store.TeeVerifyInfo, string] {
	if d := r.unix - t; d > teeTimeDrift {
		return store.NewErr[store.TeeVerifyInfo, string](
			fmt.Sprintf("tee report time out of window: block=%d quote=%d", r.unix, t))
	}

	call := &model.TeeCall{
		Caller:  caller,
		Time:    t,
		TeeType: teeType,
		Report:  report,
	}
	if len(data) > 0 {
		call.Tx = &model.TeeCall_Text{Text: data}
	}

	res, err := teetypes.VerifyReport(call)
	if err != nil {
		return store.NewErr[store.TeeVerifyInfo, string](err.Error())
	}
	if res == nil {
		return store.NewErr[store.TeeVerifyInfo, string]("tee verify: nil result")
	}

	return store.NewOk[store.TeeVerifyInfo, string](store.TeeVerifyInfo{
		TeeType:       res.TeeType,
		CodeSigner:    res.CodeSigner,
		CodeSignature: res.CodeSignature,
	})
}

// IsContractIsInited 检查合约是否已初始化。
// 通过查询数据库中的初始化标记来判断合约是否已经执行过初始化操作。
//
// 参数:
//   - txn: 交易对象，用于数据库读写
//   - contract: 合约名称
//
// 返回:
//   - bool: true 表示已初始化，false 表示未初始化
func IsContractIsInited(txn store.StoreTx, contract string) bool {
	b, err := dbstore.TxnGetCodec[bool](txn, "CONTRACT___INITED", contract)
	if err != nil {
		fmt.Println("contract init db error", err)
		return false
	}

	if b == nil {
		return false
	}

	return *b
}

// SetContractInited 设置合约为已初始化状态。
// 在合约初始化成功后调用，将初始化标记写入数据库，防止重复初始化。
//
// 参数:
//   - txn: 交易对象，用于数据库读写
//   - contract: 合约名称
//
// 返回:
//   - error: 写入数据库时的错误信息
func SetContractInited(txn store.StoreTx, contract string) error {
	return dbstore.TxnSetCodec(txn, "CONTRACT___INITED", contract, true)
}

// Query 查询其他合约（只读操作）。
// 该方法允许合约内部调用其他合约的查询方法，实现跨合约数据读取。
// 查询操作不会修改链上状态，仅返回查询结果。
//
// 参数:
//   - target: 目标合约地址（暂未使用，保留用于后续扩展）
//   - call: 合约调用请求，包含合约名称、方法名和参数
//
// 返回:
//   - []byte: 查询结果的字节切片
//   - error: 错误信息，当合约不存在或执行失败时返回
func (r *Runtime) Query(target store.UniAddr, call store.ContractCall) ([]byte, error) {
	// 创建新的运行时上下文，保持当前区块高度和交易，但切换调用者；
	// 共享事件 sink：子合约发射的事件累积到同一管道，不丢失。
	newRuntime := &Runtime{
		unix:    r.unix,
		height:  r.height,
		version: r.version,
		txn:     r.txn,
		caller:  r.caller, // 保持原始调用者
		sudo:    r.sudo,
		events:  r.events,
	}
	return Query(r.version, call, newRuntime)
}

// Call 调用其他合约（状态变更操作）。
// 该方法允许合约内部调用其他合约的状态变更方法，实现跨合约交互。
// 调用操作会修改链上状态，需要谨慎使用。
//
// 参数:
//   - target: 目标合约地址（暂未使用，保留用于后续扩展）
//   - call: 合约调用请求，包含合约名称、方法名和参数
//
// 返回:
//   - []byte: 执行结果的字节切片（暂未使用）
//   - error: 错误信息，当合约不存在或执行失败时返回
func (r *Runtime) Call(caller store.UniAddr, call store.ContractCall) ([]byte, error) {
	// 创建新的运行时上下文，保持当前区块高度和交易，但切换调用者；
	// 共享事件 sink：子合约发射的事件累积到同一管道，不丢失。
	newRuntime := &Runtime{
		unix:    r.unix,
		height:  r.height,
		version: r.version,
		txn:     r.txn,
		caller:  caller,
		sudo:    r.sudo,
		events:  r.events,
	}
	err := Mutation(r.version, call, newRuntime)
	return nil, err
}
