package model

// RuntimeVersionInfo 运行时版本元数据
type RuntimeVersionInfo struct {
	Version          uint32 `json:"version"`
	ActivationHeight int64  `json:"activation_height"`
	CodeHash         []byte `json:"code_hash"`
	WasmBytes        []byte `json:"wasm_bytes"`
	Description      string `json:"description"`
}
