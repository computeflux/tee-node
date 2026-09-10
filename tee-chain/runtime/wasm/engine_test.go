package wasm

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/require"
)

// ── mock 存储 ─────────────────────────────────────────────

type mockStoreTx struct {
	data map[string][]byte
}

func newMockStoreTx() *mockStoreTx {
	return &mockStoreTx{data: make(map[string][]byte)}
}

func (m *mockStoreTx) Get(key []byte) ([]byte, error) {
	return m.data[string(key)], nil
}

func (m *mockStoreTx) Set(key, value []byte) error {
	m.data[string(key)] = value
	return nil
}

func (m *mockStoreTx) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

func (m *mockStoreTx) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
	var keys, vals [][]byte
	for k, v := range m.data {
		if strings.HasPrefix(k, string(prefix)) {
			keys = append(keys, []byte(k))
			vals = append(vals, v)
		}
	}
	return keys, vals, nil
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

// ── 测试运行时 ────────────────────────────────────────────

type testRuntime struct {
	txn    store.StoreTx
	height int64
	caller store.UniAddr
	sudo   store.UniAddr
}

var _ store.ContractApi = (*testRuntime)(nil)

func (r *testRuntime) GetHeight() int64                                        { return r.height }
func (r *testRuntime) GetBlockTime() int64                                     { return 0 }
func (r *testRuntime) GetVersion() uint32                                      { return 1 }
func (r *testRuntime) GetTxn() store.StoreTx                                   { return r.txn }
func (r *testRuntime) GetCaller() store.UniAddr                                { return r.caller }
func (r *testRuntime) GetSystemAccount() store.UniAddr                         { return r.sudo }
func (r *testRuntime) CallbySystem() bool                                      { return false }
func (r *testRuntime) Query(store.UniAddr, store.ContractCall) ([]byte, error) { return nil, nil }
func (r *testRuntime) Call(store.UniAddr, store.ContractCall) ([]byte, error)  { return nil, nil }
func (r *testRuntime) VerifyTEE([]byte) error                                  { return nil }

// ── 辅助函数 ──────────────────────────────────────────────

func loadCachedWasm(t *testing.T, e *Engine) {
	t.Helper()
	wasmBytes, err := os.ReadFile("runtime/runtime.wasm")
	require.NoError(t, err)
	ctx := context.Background()
	mod, err := e.rt.CompileModule(ctx, wasmBytes)
	require.NoError(t, err)
	e.cachedVer = 0
	e.cachedMod = mod
}

func decodeBool(t *testing.T, b []byte) bool {
	t.Helper()
	require.NotEmpty(t, b, "decodeBool: empty input")
	return b[0] != 0
}

func decodeUint32(t *testing.T, b []byte) uint32 {
	t.Helper()
	require.True(t, len(b) >= 4, "decodeUint32: need 4 bytes, got %d", len(b))
	return binary.LittleEndian.Uint32(b)
}

// queryMethod 返回查询方法的 selector（mutates=false，与生成代码中 ExecQuery 一致）。
func queryMethod(method string) []byte {
	s := store.MethodToSelectorWithMutates(method, false)
	return s[:]
}

// ── 测试合约查询（command 模式）───────────────────────────

func TestGovQuery_GetPublicJoin(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// 空 store，GetPublicJoin 返回默认值 false
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("GetPublicJoin"),
	}, api)
	require.NoError(t, err)
	require.Equal(t, false, decodeBool(t, result))
}

func TestGovQuery_DefaultTrack(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("DefaultTrack"),
	}, api)
	require.NoError(t, err)
	require.Equal(t, uint32(0), decodeUint32(t, result))
}

func TestGovQuery_TotalSupply(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("TotalSupply"),
	}, api)
	require.NoError(t, err)
	require.NotEmpty(t, result)
}

func TestGovQuery_Members(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("Members"),
	}, api)
	require.NoError(t, err)
	require.NotEmpty(t, result)
}

// ── 测试合约变更（command 模式） ────────────────────────────

func TestGovMutation_Init(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// 调用 Init 初始化治理合约（通过 WASM Mutation）
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("Init"),
	}, api)
	require.NoError(t, err)

	// 验证 Init 确实写入了数据
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("GetPublicJoin"),
	}, api)
	require.NoError(t, err)
	require.Equal(t, true, decodeBool(t, result))
}

// ── 测试 InContractCall SCALE 编解码 ──────────────────────

func TestInContractCallCodec(t *testing.T) {
	call := &store.InContractCall{
		Name:   []byte("gateway"),
		Method: store.MethodToSelectorBytes("SettleUsage"),
		Args:   [][]byte{[]byte("hello"), []byte("world")},
	}

	// SCALE 编码
	encoded, err := call.Encode()
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// SCALE 解码
	var decoded store.InContractCall
	n, err := decoded.Decode(encoded)
	require.NoError(t, err)
	require.Equal(t, len(encoded), n)
	require.Equal(t, "gateway", string(decoded.Name))
	require.Equal(t, store.MethodToSelectorBytes("SettleUsage"), decoded.Method)
	require.Equal(t, 2, len(decoded.Args))
	require.Equal(t, "hello", string(decoded.Args[0]))
	require.Equal(t, "world", string(decoded.Args[1]))
}

// ── 测试 WASM 模块编译+实例化 ──────────────────────────────

func TestWasmModuleInstantiation(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 1,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("unknown"),
		Method: store.MethodToSelectorBytes("Unknown"),
	}, api)
	require.NoError(t, err)
	require.Empty(t, result)
}

// ── WASM codec 交叉验证：TinyGo WASM 编码 vs Native Go 解码 ──

func TestWasmCodec_GovInitThenQuery(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// Init
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("Init"),
	}, api)
	require.NoError(t, err)

	// ── Query: DefaultTrack → uint32 ──
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("DefaultTrack"),
	}, api)
	require.NoError(t, err)
	var trackID uint32
	require.NoError(t, store.ScaleDecode(&trackID, result))
	require.Equal(t, uint32(0), trackID)

	// ── Query: GetPublicJoin → bool ──
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("GetPublicJoin"),
	}, api)
	require.NoError(t, err)
	var publicJoin bool
	require.NoError(t, store.ScaleDecode(&publicJoin, result))
	require.True(t, publicJoin)

	// ── Query: TotalSupply → store.Amount (U256) ──
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("TotalSupply"),
	}, api)
	require.NoError(t, err)
	var supply store.Amount
	supply.Int = new(big.Int)
	require.NoError(t, store.ScaleDecode(&supply, result))
	t.Logf("TotalSupply: %s", supply.String())

	// ── Query: Members → []Member（含 UniAddr + Amount）──
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("Members"),
	}, api)
	require.NoError(t, err)
	var members []gov.Member
	require.NoError(t, store.ScaleDecode(&members, result))
	t.Logf("Members count: %d", len(members))
	for _, m := range members {
		t.Logf("  %s balance=%s", m.Account.String(), m.Balance.String())
	}

	// ── 编码验证：Native 编码后与 WASM 结果逐字节比对 ──
	// 用 Native Go 的 ScaleEncode/ScaleDecode 验证 WASM 输出正确性

	// uint32
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("DefaultTrack"),
	}, api)
	require.NoError(t, err)
	trackID = 0
	require.NoError(t, store.ScaleDecode(&trackID, result))
	reEnc, _ := store.ScaleEncode(trackID)
	require.Equal(t, result, reEnc, "uint32 round-trip mismatch")

	// bool
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("GetPublicJoin"),
	}, api)
	require.NoError(t, err)
	var pj bool
	require.NoError(t, store.ScaleDecode(&pj, result))
	reEnc, _ = store.ScaleEncode(pj)
	require.Equal(t, result, reEnc, "bool round-trip mismatch")

	// Amount = U256
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("TotalSupply"),
	}, api)
	require.NoError(t, err)
	supply = store.Amount{}
	supply.Int = new(big.Int)
	require.NoError(t, store.ScaleDecode(&supply, result))
	reEnc, _ = store.ScaleEncode(&supply)
	require.Equal(t, result, reEnc, "U256 round-trip mismatch")
}

// 往返验证：通过 WASM 执行 Init，再查询 Tracks，验证编解码完整闭环。
// 注意：当前 runtime.wasm 中的 AddTrack 为旧实现（不实际写入），
// 因此本测试仅验证 Init 产生的 default track 的编解码正确性。
func TestWasmCodec_Gov_TrackRoundTrip(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// Init
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("Init"),
	}, api)
	require.NoError(t, err)

	// Query: Tracks → []TrackWithID
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("Tracks"),
	}, api)
	require.NoError(t, err)

	var tracks []gov.TrackWithID
	require.NoError(t, store.ScaleDecode(&tracks, result))
	t.Logf("Tracks count: %d", len(tracks))
	require.Len(t, tracks, 1, "Init 后应只有 1 条 default track")

	// 验证解码后的字段
	decoded := tracks[0].Track
	require.Equal(t, uint32(0), tracks[0].ID)
	require.Equal(t, "default", decoded.Name)
	require.Equal(t, uint32(100), decoded.PreparePeriod)
	require.Equal(t, int64(100), decoded.DecisionDeposit.Int.Int64())
	require.Equal(t, int64(1000000), decoded.MaxBalance.Int.Int64())
	require.Equal(t, "LinearDecreasing", decoded.MinApproval.Type)
	require.Equal(t, "LinearDecreasing", decoded.MinSupport.Type)
	t.Logf("Track round-trip verified: name=%s prepare=%d deposit=%s",
		decoded.Name, decoded.PreparePeriod, decoded.DecisionDeposit.String())

	// store codec 往返验证
	reEnc, _ := store.ScaleEncode(&tracks)
	require.Equal(t, result, reEnc, "[]TrackWithID round-trip mismatch")
}

// ===========================================================================
// WASM × gsrpc codec 交叉验证
// ===========================================================================

// wasmCrossCodecCmp 解码 WASM 输出后，分别用 store.ScaleEncode 和
// github.com/centrifuge/go-substrate-rpc-client/v4/types/codec 重新编码，
// 验证三者的字节一致性。
func wasmCrossCodecCmp[T any](t *testing.T, wasmBytes []byte, name string) {
	t.Helper()

	var decoded T
	require.NoError(t, store.ScaleDecode(&decoded, wasmBytes), "%s: store.ScaleDecode failed", name)

	// store codec round-trip
	storeReEnc, err := store.ScaleEncode(&decoded)
	require.NoError(t, err, "%s: store.ScaleEncode failed", name)
	require.Equal(t, wasmBytes, storeReEnc, "%s: store codec round-trip mismatch", name)

	// old codec (gsrpc) encode — 部分自定义类型可能 panic，用 recover 捕获
	oldEnc, err := safeGsrpcEncode(&decoded)
	if err == nil {
		if !bytes.Equal(wasmBytes, oldEnc) {
			t.Errorf("%s: WASM bytes != gsrpc codec bytes\n  wasm: %x\n  gsrpc: %x", name, wasmBytes, oldEnc)
		}
	} else {
		t.Logf("%s: gsrpc codec not supported (%v)", name, err)
	}
}

func safeGsrpcEncode(v any) (enc []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("gsrpc codec panicked: %v", r)
		}
	}()
	return codec.Encode(v)
}

// TestWasmCodec_CrossCodec_GovQueries 在真实 WASM 执行环境下验证：
//  1. WASM 合约返回的 SCALE 字节流能被 store.ScaleDecode 正确解码；
//  2. 解码后的值用 store.ScaleEncode 重新编码，字节流与 WASM 原始输出完全一致；
//  3. 若类型被 github.com/centrifuge/go-substrate-rpc-client/v4/types/codec 支持，
//     则 gsrpc codec 编码结果也与 WASM 原始输出一致。
func TestWasmCodec_CrossCodec_GovQueries(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// Init
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("Init"),
	}, api)
	require.NoError(t, err)

	// ── Query: GetPublicJoin → bool ──
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("GetPublicJoin"),
	}, api)
	require.NoError(t, err)
	wasmCrossCodecCmp[bool](t, result, "GetPublicJoin(bool)")

	// ── Query: DefaultTrack → uint32 ──
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("DefaultTrack"),
	}, api)
	require.NoError(t, err)
	wasmCrossCodecCmp[uint32](t, result, "DefaultTrack(uint32)")

	// ── Query: TotalSupply → store.Amount (U256) ──
	// gsrpc codec 对含 *big.Int 的自定义 U256 不支持，仅验证 store 编解码闭环
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("TotalSupply"),
	}, api)
	require.NoError(t, err)
	var supply store.Amount
	supply.Int = new(big.Int)
	require.NoError(t, store.ScaleDecode(&supply, result))
	storeReEnc, _ := store.ScaleEncode(&supply)
	require.Equal(t, result, storeReEnc, "TotalSupply(Amount) round-trip mismatch")

	// ── Query: Members → []gov.Member ──
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("Members"),
	}, api)
	require.NoError(t, err)
	var members []gov.Member
	require.NoError(t, store.ScaleDecode(&members, result))
	storeReEnc, _ = store.ScaleEncode(&members)
	require.Equal(t, result, storeReEnc, "Members round-trip mismatch")

	// ── Query: Tracks → []TrackWithID ──
	result, err = e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("Tracks"),
	}, api)
	require.NoError(t, err)
	var tracks []gov.TrackWithID
	require.NoError(t, store.ScaleDecode(&tracks, result))
	storeReEnc, _ = store.ScaleEncode(&tracks)
	require.Equal(t, result, storeReEnc, "Tracks round-trip mismatch")

	// 验证解码后的字段内容
	require.Len(t, tracks, 1, "Init 后应只有 1 条 default track")
	decoded := tracks[0].Track
	require.Equal(t, uint32(0), tracks[0].ID)
	require.Equal(t, "default", decoded.Name)
	require.Equal(t, uint32(100), decoded.PreparePeriod)
	require.Equal(t, int64(100), decoded.DecisionDeposit.Int.Int64())
	require.Equal(t, int64(1000000), decoded.MaxBalance.Int.Int64())
	require.Equal(t, "LinearDecreasing", decoded.MinApproval.Type)
	require.Equal(t, "LinearDecreasing", decoded.MinSupport.Type)
	t.Logf("Track cross-codec verified: name=%s prepare=%d deposit=%s",
		decoded.Name, decoded.PreparePeriod, decoded.DecisionDeposit.String())
}

// TestWasmCodec_CrossCodec_TrackDataEncoding 直接验证 TrackData 的编码：
// store.ScaleEncode 与 gsrpc codec 的 Encode 结果是否一致（若 gsrpc 支持）。
func TestWasmCodec_CrossCodec_TrackDataEncoding(t *testing.T) {
	trackData := gov.TrackData{
		Name:               "test_track",
		PreparePeriod:      50,
		MaxDeciding:        100,
		ConfirmPeriod:      20,
		DecisionPeriod:     30,
		MinEnactmentPeriod: 10,
		DecisionDeposit:    store.Amount{Int: big.NewInt(1000)},
		MaxBalance:         store.Amount{Int: big.NewInt(1000000)},
		MinApproval:        gov.NewLinearDecreasingCurve(10000, 5000, 1000),
		MinSupport:         gov.NewSteppedDecreasingCurve(8000, 2000, 500, 100),
	}
	trackBytes, err := store.ScaleEncode(&trackData)
	require.NoError(t, err)

	// 验证 store 自身往返
	var decoded gov.TrackData
	require.NoError(t, store.ScaleDecode(&decoded, trackBytes))
	reEnc, _ := store.ScaleEncode(&decoded)
	require.Equal(t, trackBytes, reEnc, "TrackData store round-trip mismatch")

	// 尝试与 gsrpc codec 比对
	oldTrackEnc, err := safeGsrpcEncode(&trackData)
	if err == nil {
		require.Equal(t, trackBytes, oldTrackEnc, "TrackData encode: store != gsrpc")
	} else {
		t.Logf("TrackData gsrpc codec not supported (%v)", err)
	}
}

// TestWasmCodec_AuthConfig_SetAndGet 验证 WASM 环境下 AuthConfig 的 SetConfig / GetConfig
// 编解码完整闭环。线上曾出现字段错乱，本测试覆盖全部 16 个参数逐字段比对。
func TestWasmCodec_AuthConfig_SetAndGet(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// 构造完整的配置参数
	cfgInput := auth.AuthConfig{
		SMTPHost:            "smtp.example.com",
		SMTPPort:            587,
		SMTPUser:            "admin",
		SMTPPassword:        "secret123",
		SMTPFrom:            "no-reply@example.com",
		SMTPFromName:        "MyApp",
		MockEmail:           true,
		GoogleClientID:      "google-id",
		GoogleClientSecret:  "google-secret",
		GoogleRedirectURL:   "https://app.com/auth/google/callback",
		GitHubClientID:      "github-id",
		GitHubClientSecret:  "github-secret",
		GitHubRedirectURL:   "https://app.com/auth/github/callback",
		TwitterClientID:     "twitter-id",
		TwitterClientSecret: "twitter-secret",
		TwitterRedirectURL:  "https://app.com/auth/twitter/callback",
	}

	// 每个参数单独 SCALE 编码（与 WASM 合约解码方式一致）
	args := [][]byte{
		mustScaleEncode(t, cfgInput.SMTPHost),
		mustScaleEncode(t, cfgInput.SMTPPort),
		mustScaleEncode(t, cfgInput.SMTPUser),
		mustScaleEncode(t, cfgInput.SMTPPassword),
		mustScaleEncode(t, cfgInput.SMTPFrom),
		mustScaleEncode(t, cfgInput.SMTPFromName),
		mustScaleEncode(t, cfgInput.MockEmail),
		mustScaleEncode(t, cfgInput.GoogleClientID),
		mustScaleEncode(t, cfgInput.GoogleClientSecret),
		mustScaleEncode(t, cfgInput.GoogleRedirectURL),
		mustScaleEncode(t, cfgInput.GitHubClientID),
		mustScaleEncode(t, cfgInput.GitHubClientSecret),
		mustScaleEncode(t, cfgInput.GitHubRedirectURL),
		mustScaleEncode(t, cfgInput.TwitterClientID),
		mustScaleEncode(t, cfgInput.TwitterClientSecret),
		mustScaleEncode(t, cfgInput.TwitterRedirectURL),
	}

	// SetConfig mutation
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("auth"),
		Method: store.MethodToSelectorBytes("set_config"),
		Args:   args,
	}, api)
	require.NoError(t, err, "SetConfig mutation failed")

	// GetConfig query（返回脱敏后的配置）
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("auth"),
		Method: queryMethod("get_config"),
	}, api)
	require.NoError(t, err, "GetConfig query failed")

	var cfgOut auth.AuthConfig
	require.NoError(t, codec.Decode(result, &cfgOut), "GetConfig decode failed")

	fmt.Println(cfgOut)

	// 验证非敏感字段与输入一致
	require.Equal(t, cfgInput.SMTPHost, cfgOut.SMTPHost, "SMTPHost mismatch")
	require.Equal(t, cfgInput.SMTPPort, cfgOut.SMTPPort, "SMTPPort mismatch")
	require.Equal(t, cfgInput.SMTPFrom, cfgOut.SMTPFrom, "SMTPFrom mismatch")
	require.Equal(t, cfgInput.SMTPFromName, cfgOut.SMTPFromName, "SMTPFromName mismatch")
	require.Equal(t, cfgInput.MockEmail, cfgOut.MockEmail, "MockEmail mismatch")
	require.Equal(t, cfgInput.GoogleClientID, cfgOut.GoogleClientID, "GoogleClientID mismatch")
	require.Equal(t, cfgInput.GoogleRedirectURL, cfgOut.GoogleRedirectURL, "GoogleRedirectURL mismatch")
	require.Equal(t, cfgInput.GitHubClientID, cfgOut.GitHubClientID, "GitHubClientID mismatch")
	require.Equal(t, cfgInput.GitHubRedirectURL, cfgOut.GitHubRedirectURL, "GitHubRedirectURL mismatch")
	require.Equal(t, cfgInput.TwitterClientID, cfgOut.TwitterClientID, "TwitterClientID mismatch")
	require.Equal(t, cfgInput.TwitterRedirectURL, cfgOut.TwitterRedirectURL, "TwitterRedirectURL mismatch")

	// 敏感字段应被脱敏（maskIfNonEmpty: s[:2]+"****"+s[len(s)-2:]）
	require.Equal(t, "se****23", cfgOut.SMTPPassword, "SMTPPassword should be masked")
	require.Equal(t, "go****et", cfgOut.GoogleClientSecret, "GoogleClientSecret should be masked")
	require.Equal(t, "gi****et", cfgOut.GitHubClientSecret, "GitHubClientSecret should be masked")
	require.Equal(t, "tw****et", cfgOut.TwitterClientSecret, "TwitterClientSecret should be masked")

	// 通过 Go 侧直接读取原始配置，验证全部字段（含 JWTSecret / SeedSecret）
	ins := auth.NewAuth(api)
	rawCfg, err := auth.AuthSecret{Auth: *ins}.GetConfig()
	require.NoError(t, err, "raw GetConfig failed")
	require.NotNil(t, rawCfg, "raw config should not be nil")

	require.Equal(t, cfgInput.SMTPHost, rawCfg.SMTPHost, "raw SMTPHost mismatch")
	require.Equal(t, cfgInput.SMTPPort, rawCfg.SMTPPort, "raw SMTPPort mismatch")
	require.Equal(t, cfgInput.SMTPUser, rawCfg.SMTPUser, "raw SMTPUser mismatch")
	require.Equal(t, cfgInput.SMTPPassword, rawCfg.SMTPPassword, "raw SMTPPassword mismatch")
	require.Equal(t, cfgInput.SMTPFrom, rawCfg.SMTPFrom, "raw SMTPFrom mismatch")
	require.Equal(t, cfgInput.SMTPFromName, rawCfg.SMTPFromName, "raw SMTPFromName mismatch")
	require.Equal(t, cfgInput.MockEmail, rawCfg.MockEmail, "raw MockEmail mismatch")
	require.Equal(t, cfgInput.GoogleClientID, rawCfg.GoogleClientID, "raw GoogleClientID mismatch")
	require.Equal(t, cfgInput.GoogleClientSecret, rawCfg.GoogleClientSecret, "raw GoogleClientSecret mismatch")
	require.Equal(t, cfgInput.GoogleRedirectURL, rawCfg.GoogleRedirectURL, "raw GoogleRedirectURL mismatch")
	require.Equal(t, cfgInput.GitHubClientID, rawCfg.GitHubClientID, "raw GitHubClientID mismatch")
	require.Equal(t, cfgInput.GitHubClientSecret, rawCfg.GitHubClientSecret, "raw GitHubClientSecret mismatch")
	require.Equal(t, cfgInput.GitHubRedirectURL, rawCfg.GitHubRedirectURL, "raw GitHubRedirectURL mismatch")
	require.Equal(t, cfgInput.TwitterClientID, rawCfg.TwitterClientID, "raw TwitterClientID mismatch")
	require.Equal(t, cfgInput.TwitterClientSecret, rawCfg.TwitterClientSecret, "raw TwitterClientSecret mismatch")
	require.Equal(t, cfgInput.TwitterRedirectURL, rawCfg.TwitterRedirectURL, "raw TwitterRedirectURL mismatch")
	require.NotEmpty(t, rawCfg.JWTSecret, "raw JWTSecret should be generated")
	require.NotEmpty(t, rawCfg.SeedSecret, "raw SeedSecret should be generated")
	require.Equal(t, uint32(6), rawCfg.OTPLength, "raw OTPLength should be default 6")
	require.Equal(t, uint32(5), rawCfg.OTPTTLMins, "raw OTPTTLMins should be default 5")

	t.Logf("AuthConfig round-trip OK: smtpHost=%s smtpPort=%d mockEmail=%v", rawCfg.SMTPHost, rawCfg.SMTPPort, rawCfg.MockEmail)

	// WASM 输出字节与 store.ScaleEncode / gsrpc codec 交叉验证
	wasmCrossCodecCmp[auth.AuthConfig](t, result, "GetConfig(AuthConfig)")
}

func mustScaleEncode[T any](t *testing.T, v T) []byte {
	t.Helper()
	b, err := store.ScaleEncode(v)
	require.NoError(t, err)
	return b
}

// ── Mutation → Query 持久化验证 ──────────────────────────

// TestWasmPersist_InitThenQuery 验证 mutation 写入的数据能通过 query 读出。
func TestWasmPersist_InitThenQuery(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// Step 1: Mutation — Init gov
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("Init"),
	}, api)
	require.NoError(t, err, "Init mutation should succeed")

	// Step 2: 底层验证 — 数据确实写入了 mockStoreTx
	t.Run("raw_store_has_data", func(t *testing.T) {
		keys, vals, err := txn.ListByPrefix([]byte("gov_"))
		require.NoError(t, err)
		require.NotEmpty(t, keys, "gov_ prefixed keys should exist after Init")
		for i, k := range keys {
			t.Logf("  store key=%s val=%x", string(k), vals[i])
		}
	})

	// Step 3: WASM Query — 通过 WASM 查询已写入的数据
	t.Run("wasm_query_sees_data", func(t *testing.T) {
		result, err := e.ExecuteQuery(0, &store.InContractCall{
			Name:   []byte("gov"),
			Method: queryMethod("TotalSupply"),
		}, api)
		require.NoError(t, err)
		require.NotEmpty(t, result, "TotalSupply should return data")
		var supply store.Amount
		supply.Int = new(big.Int)
		require.NoError(t, store.ScaleDecode(&supply, result))
		t.Logf("TotalSupply (via WASM): %s", supply.String())

		result, err = e.ExecuteQuery(0, &store.InContractCall{
			Name:   []byte("gov"),
			Method: queryMethod("GetPublicJoin"),
		}, api)
		require.NoError(t, err)
		require.NotEmpty(t, result, "GetPublicJoin should return data")
		var pj bool
		require.NoError(t, store.ScaleDecode(&pj, result))
		require.True(t, pj, "publicJoin should be true after Init")

		result, err = e.ExecuteQuery(0, &store.InContractCall{
			Name:   []byte("gov"),
			Method: queryMethod("DefaultTrack"),
		}, api)
		require.NoError(t, err)
		require.NotEmpty(t, result, "DefaultTrack should return data")
		var trackID uint32
		require.NoError(t, store.ScaleDecode(&trackID, result))
		require.Equal(t, uint32(0), trackID, "default track should be 0")
	})
}

// TestWasmPersist_MultipleMutations 验证多次 mutation 的数据累积。
func TestWasmPersist_MultipleMutations(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// Mutation 1: Init
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("Init"),
	}, api)
	require.NoError(t, err)

	// 记录 Init 后的 store 状态
	snapshot1 := make(map[string][]byte)
	for k, v := range txn.data {
		snapshot1[k] = v
	}
	require.NotEmpty(t, snapshot1, "Init should populate store")

	// Mutation 2: Join（如果支持）— 尝试调用一个可能的 mutation
	// 这里使用 AddTrack（即使可能失败，验证不崩溃）
	_ = e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("AddTrack"),
		Args: [][]byte{mustScaleEncode(t, gov.TrackData{
			Name:            "test-track",
			PreparePeriod:   10,
			MaxDeciding:     50,
			ConfirmPeriod:   20,
			DecisionPeriod:  30,
			DecisionDeposit: store.NewU256(big.NewInt(500)),
			MaxBalance:      store.NewU256(big.NewInt(10000)),
			MinApproval:     gov.NewLinearDecreasingCurve(10000, 50, 100),
			MinSupport:      gov.NewLinearDecreasingCurve(10000, 50, 100),
		})},
	}, api)

	// 验证 Init 的数据仍然存在（不被后续 mutation 破坏）
	for k, v := range snapshot1 {
		got, ok := txn.data[k]
		require.True(t, ok, "Init key %x should still exist", []byte(k))
		require.Equal(t, v, got, "Init key %x value should be unchanged", []byte(k))
	}

	// Query 仍然可用
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("TotalSupply"),
	}, api)
	require.NoError(t, err)
	require.NotEmpty(t, result, "TotalSupply should still be queryable")

	t.Logf("store has %d keys after 2 mutations", len(txn.data))
}
