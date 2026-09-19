package store

type ContractCall interface {
	GetName() []byte
	GetMethod() []byte
	GetArgs() [][]byte
}

// scale-codec-gen:generate
type InContractCall struct {
	Name   []byte
	Method []byte
	Args   [][]byte
}

func (c *InContractCall) GetName() []byte {
	return c.Name
}

func (c *InContractCall) GetMethod() []byte {
	return c.Method
}

func (c *InContractCall) GetArgs() [][]byte {
	return c.Args
}

// ContractApi 侧链合约运行时：区块高度、读写事务、已解析的调用方（如由 TxBox 回填）。
type ContractApi interface {
	GetHeight() int64
	GetBlockTime() int64
	GetVersion() uint32
	GetTxn() StoreTx
	GetCaller() UniAddr
	GetSystemAccount() UniAddr
	CallbySystem() bool
	// VerifyTEE 校验客户端传入的 TEE 报告（protoio 序列化的 model.TeeCall），
	// 成功返回 Ok 携带 TEE 证明代码三要素（TeeType/CodeSigner/CodeSignature），失败返回 Err 携带错误信息。
	VerifyTEE([]byte) Result[TeeVerifyInfo, string]
	Query(UniAddr, ContractCall) ([]byte, error)
	Call(UniAddr, ContractCall) ([]byte, error)
}

// StoreTx 抽象化存储操作接口。Native (*store.Txn) 和 WASM 均满足。
type StoreTx interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	ListByPrefix(prefix []byte) ([][]byte, [][]byte, error)
	DeletekeysByPrefix(prefix []byte) error
	Commit() error
	Rollback() error
}
