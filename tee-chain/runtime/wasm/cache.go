package wasm

import (
	"fmt"
	"sync"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
)

// 链上当前 runtime 版本的 wasm 字节缓存。
//
// 背景：Engine.LoadModule 历史上每次版本变更都要走 gov 合约查 RuntimeMeta，
// 再做 gzip 解压。FinalizeTx 末尾的版本切换 + 合约执行高频路径会让这份 IO
// 和解压被放大成 N 次重复。
//
// 设计：FinalizeTx 检测到版本切换时、NewTeeChain 启动预热时，调用
// RefreshCurrentVersion 把当前版本的解压后字节写入本缓存。Engine.LoadModule
// 优先读它，命中即跳过 gov 查询和解压；未命中（启动后第一次合约调用前、
// 查询旧版本）走原 fallback 路径，行为不变。
//
// 并发模型：sync.RWMutex 保护，读多写少（写频率 = 每次 block 至多 1 次）。
// Engine.cachedMod 仍提供"已编译模块"的二级缓存，与本缓存互补不冲突。

var (
	currentVersionMu sync.RWMutex
	currentVersion   uint32
	currentWasmBytes []byte
)

// SetCurrentVersion 写入当前版本的解压后 wasm 字节（覆盖式）。
// version=0 或 wasmBytes=nil 表示清空缓存，调用方按需决定。
func SetCurrentVersion(version uint32, wasmBytes []byte) {
	currentVersionMu.Lock()
	currentVersion = version
	currentWasmBytes = wasmBytes
	currentVersionMu.Unlock()
}

// GetCurrentVersion 查询当前缓存的版本与字节。
// ok=false 表示尚未初始化（启动后第一次合约调用前，或查询非 current 版本时的兜底）。
// 返回的切片为内部缓冲视图，调用方请勿修改。
func GetCurrentVersion() (version uint32, wasmBytes []byte, ok bool) {
	currentVersionMu.RLock()
	defer currentVersionMu.RUnlock()
	if currentVersion == 0 || len(currentWasmBytes) == 0 {
		return 0, nil, false
	}
	return currentVersion, currentWasmBytes, true
}

// RefreshCurrentVersion 从 gov 合约存储读取指定版本的 wasm 字节（含解压），
// 写入包级缓存。FinalizeTx 检测到版本切换、NewTeeChain 启动预热时调用。
// version=0 直接返回 nil（genesis 前还未装 wasm，不算错）。
func RefreshCurrentVersion(version uint32, q gov.GovQuery) error {
	if version == 0 {
		return nil
	}
	info, err := q.RuntimeMeta(version)
	if err != nil {
		return fmt.Errorf("read runtime meta v%d: %w", version, err)
	}
	if info == nil {
		return fmt.Errorf("runtime version %d not found", version)
	}
	wasmBytes, err := decompressWasm(info.WasmBytes)
	if err != nil {
		return fmt.Errorf("decompress wasm v%d: %w", version, err)
	}
	SetCurrentVersion(version, wasmBytes)
	return nil
}
