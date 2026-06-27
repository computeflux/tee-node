// ego/main.go — 薄壳：调用 wasm.Engine.ExecuteQuery 触发 engine.go 的代码路径。
//
// 这个 example 不重新实现任何 wazero/SCALE 逻辑，全部委托给
// tee-chain/runtime/wasm.Engine 和 tee-chain/runtime/store，
// 这样 engine.go 一旦有 bug，这里会原样重现。
//
// 按 EGo 官方文档的最小结构：https://docs.edgeless.systems/ego/workflows/build
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/wasm"
	"github.com/edgelesssys/ego/enclave"
	"github.com/tetratelabs/wazero"
)

func main() {
	wasmPath := flag.String("wasm", defaultWasmPath(), "path to runtime.wasm (or runtime.wasm.gz)")
	method := flag.String("method", "runtime_version", "query method to invoke (snake_case)")
	flag.Parse()

	if _, err := os.Stat(*wasmPath); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: wasm not found: %s\n", *wasmPath)
		fmt.Fprintf(os.Stderr, "  tried: %v\n", triedPaths)
		fmt.Fprintf(os.Stderr, "  hint 1: build it via bash tee-chain/runtime/wasm/runtime/build.sh\n")
		fmt.Fprintf(os.Stderr, "  hint 2: or pass -wasm /path/to/runtime.wasm explicitly\n")
		os.Exit(1)
	}

	fmt.Println("=== ego: run wasm.Engine.ExecuteQuery ===")
	fmt.Printf("wasm:   %s\n", *wasmPath)
	fmt.Printf("method: %s\n", *method)

	if report, err := enclave.GetSelfReport(); err == nil {
		fmt.Printf("sgx:    in-SGX enclave  measurement=%x\n", report.UniqueID)
	} else {
		fmt.Println("sgx:    not in SGX enclave (plain Go)")
	}

	// ── 1. 读取 wasm ─────────────────────────────────────
	raw, err := os.ReadFile(*wasmPath)
	check("read wasm", err)

	// ── 2. 构造 Engine + 预填缓存（绕过 LoadModule 从 gov 存储读 wasm）──
	e := wasm.NewEngine()
	mod, err := e.CompileWasm(raw)
	check("compile wasm", err)
	e.SetCache(0, mod)

	// ── 3. 构造 ContractApi ──────────────────────────────
	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 1,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// ── 4. 构造合约调用 ──────────────────────────────────
	sel := store.MethodToSelectorWithMutates(toSnake(*method), false)
	call := &store.InContractCall{
		Name:   []byte("gov"),
		Method: sel[:],
	}

	// ── 5. 调用 engine.go（这就是要测试的代码）───────────
	result, err := e.ExecuteQuery(0, call, api)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: ExecuteQuery: %v\n", err)
		os.Exit(1)
	}
	if len(result) > 0 {
		fmt.Printf("✓ result (%d bytes): %q\n", len(result), string(result))
	} else {
		fmt.Printf("✓ result: <empty>\n")
	}
	fmt.Println("=== test passed ===")
}

// triedPaths 记录 defaultWasmPath 尝试过的所有路径，用于错误信息。
var triedPaths []string

// defaultWasmPath 解析默认 wasm 路径，SGX/普通 Go 双模式适配。
//
// 优先级：
//  1. 环境变量 RUNTIME_WASM
//  2. SGX enclave 下使用 enclave.json files 嵌入的路径 "runtime.wasm"
//  3. 从当前 cwd 出发，依次尝试以下候选：
//     - ./runtime.wasm（ego/ 下或拷贝后）
//     - tee-chain/runtime/wasm/runtime/runtime.wasm（从 tee-node/ 出发）
//     - tee-chain/runtime/wasm/ego/runtime.wasm（从 tee-node/ 出发，ego/ 里拷贝过）
//     - ../runtime/runtime.wasm（从 ego/ 出发）
//     - ../runtime.wasm（从 ego/ 出发，ego/ 里拷贝过）
//  4. 找不到时返回第一个候选 + 记录所有尝试路径
func defaultWasmPath() string {
	if env := os.Getenv("RUNTIME_WASM"); env != "" {
		triedPaths = append(triedPaths, "RUNTIME_WASM="+env)
		return env
	}
	if _, err := enclave.GetSelfReport(); err == nil {
		triedPaths = append(triedPaths, "runtime.wasm (enclave in-memfs)")
		return "runtime.wasm"
	}
	candidates := []string{
		"runtime.wasm", // 当前目录（ego/ 下或拷贝后）
		"tee-chain/runtime/wasm/runtime/runtime.wasm", // tee-node/ 出发，源码目录
		"tee-chain/runtime/wasm/ego/runtime.wasm",     // tee-node/ 出发，ego/ 里拷过
		"../runtime/runtime.wasm",                     // ego/ 出发，源码目录
		"../runtime.wasm",                             // ego/ 出发，ego/ 里拷过
	}
	triedPaths = append(triedPaths, candidates...)
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0] // fallthrough; main() 会检测到不存在并报错
}

func check(stage string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", stage, err)
		os.Exit(1)
	}
}

// toSnake 把 runtime_version / RuntimeVersion / runtimeVersion 统一成 snake_case。
func toSnake(s string) string {
	var out []byte
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		out = append(out, byte(r))
	}
	return string(out)
}

// ── 以下两个类型来自 engine_test.go 的 testRuntime / mockStoreTx ──
//    复制过来是因为 _test.go 文件不可被 main 包 import。

type mockStoreTx struct{ data map[string][]byte }

func newMockStoreTx() *mockStoreTx                  { return &mockStoreTx{data: map[string][]byte{}} }
func (m *mockStoreTx) Get(k []byte) ([]byte, error) { return m.data[string(k)], nil }
func (m *mockStoreTx) Set(k, v []byte) error        { m.data[string(k)] = v; return nil }
func (m *mockStoreTx) Delete(k []byte) error        { delete(m.data, string(k)); return nil }
func (m *mockStoreTx) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
	var ks, vs [][]byte
	for k, v := range m.data {
		if strings.HasPrefix(k, string(prefix)) {
			ks = append(ks, []byte(k))
			vs = append(vs, v)
		}
	}
	return ks, vs, nil
}
func (m *mockStoreTx) DeletekeysByPrefix(prefix []byte) error {
	for k := range m.data {
		if strings.HasPrefix(k, string(prefix)) {
			delete(m.data, k)
		}
	}
	return nil
}
func (m *mockStoreTx) Commit() error   { return nil }
func (m *mockStoreTx) Rollback() error { return nil }

type testRuntime struct {
	txn    store.StoreTx
	height int64
	caller store.UniAddr
	sudo   store.UniAddr
}

func (r *testRuntime) GetHeight() int64                { return r.height }
func (r *testRuntime) GetBlockTime() int64             { return 0 }
func (r *testRuntime) GetVersion() uint32              { return 1 }
func (r *testRuntime) GetTxn() store.StoreTx           { return r.txn }
func (r *testRuntime) GetCaller() store.UniAddr        { return r.caller }
func (r *testRuntime) GetSystemAccount() store.UniAddr { return r.sudo }
func (r *testRuntime) CallbySystem() bool              { return false }
func (r *testRuntime) Query(store.UniAddr, store.ContractCall) ([]byte, error) {
	return nil, nil
}
func (r *testRuntime) Call(store.UniAddr, store.ContractCall) ([]byte, error) { return nil, nil }

// 兜底（避免 _ import）
var _ = context.Background
var _ = wazero.NewRuntime
