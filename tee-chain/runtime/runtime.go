// Package contracts 提供合约运行时环境和合约调用入口。
// 本文件定义了合约执行时的运行时上下文，包含区块高度、交易信息和调用者身份。
package runtime

import (
	"bytes"
	"fmt"
	"time"

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
	rawTx   []byte        // 当前原始交易字节（model.Tx 序列化）：用于组装 TEE 证明的被证明交易（TeeCall.Tx）
}

// NewRuntime 创建一个新的合约运行时实例。
// 参数:
//   - height: 当前区块高度
//   - txn: 交易对象，包含交易的完整信息
//   - caller: 调用者的区块链地址
//   - sudo: 超级管理员账户地址
//   - txbt: 当前原始交易字节（model.Tx 序列化）；无交易上下文（查询/干跑）时传 nil
//
// 返回:
//   - Runtime: 初始化后的运行时上下文对象
func NewRuntime(height int64, ts time.Time, txn *dbstore.Txn, caller store.UniAddr, sudo store.UniAddr, txbt []byte) *Runtime {
	return &Runtime{unix: ts.Unix(), height: height, version: 0, txn: txn, caller: caller, sudo: sudo, rawTx: txbt}
}

// NewRuntimeWithVersion 创建带指定版本的合约运行时实例。
func NewRuntimeWithVersion(height int64, ts time.Time, txn *dbstore.Txn, caller store.UniAddr, sudo store.UniAddr, version uint32, txbt []byte) *Runtime {
	return &Runtime{unix: ts.Unix(), height: height, version: version, txn: txn, caller: caller, sudo: sudo, rawTx: txbt}
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

// GetRawTx 获取当前执行的原始交易字节（model.Tx 序列化）。
// 供 TEE 证明组装 TeeCall.Tx（验证节点签发报告时所证明的交易内容）；
// 查询/干跑等无交易上下文的运行时返回 nil。
func (r *Runtime) GetRawTx() []byte {
	return r.rawTx
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

// VerifyTEE 校验客户端传入的 TEE quote。
// report 参数仅为 quote 字节（model.TeeCall 的 Report 字段，非整个 TeeCall）；
// 完整的 model.TeeCall 由合约运行时用执行上下文补齐（用户不可伪造）：
//
//	Tx:      当前原始交易（TeeCall_Text，Text=rawTx）——节点签发报告时所证明的交易内容
//	Caller:  当前交易调用方（验证节点公钥，即报告签发者）
//	Time:    当前区块时间（节点执行期间签发报告的时间）
//	TeeType: 本节点 TEE 类型（0=SGX, 1=SEV-SNP, 2=TDX, 9999=非 TEE）
//
// 节点侧签发报告须与上述参数对齐：用与交易签名相同的密钥，以当前区块时间为 Time，
// 对 [Time || Caller || marshal(TeeCall_Text{Text: 原始交易字节})] 签名后生成 quote。
// 成功返回 Ok，携带 TEE 证明的代码三要素（TeeType/CodeSigner/CodeSignature），供合约验证用户的 CodeSignature；
// 失败返回 Err，携带验证错误信息。
func (r *Runtime) VerifyTEE(report []byte) store.Result[store.TeeVerifyInfo, string] {
	call := &model.TeeCall{
		Caller:  r.caller.V,
		Time:    r.unix,
		TeeType: uint32(teetypes.TeeType),
		Report:  report,
	}
	if len(r.rawTx) > 0 {
		call.Tx = &model.TeeCall_Text{Text: r.rawTx}
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
	// 创建新的运行时上下文，保持当前区块高度和交易，但切换调用者
	newRuntime := &Runtime{
		unix:    r.unix,
		height:  r.height,
		version: r.version,
		txn:     r.txn,
		caller:  r.caller, // 保持原始调用者
		sudo:    r.sudo,
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
	// 创建新的运行时上下文，保持当前区块高度和交易，但切换调用者
	newRuntime := &Runtime{
		unix:    r.unix,
		height:  r.height,
		version: r.version,
		txn:     r.txn,
		caller:  caller,
		sudo:    r.sudo,
	}
	err := Mutation(r.version, call, newRuntime)
	return nil, err
}
