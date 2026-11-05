package teechain

import (
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/relay"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/require"
)

// setupFinalizeTxTest 初始化 FinalizeTx 事件管道测试环境：
// 1. 创建临时 DB 并设为全局 SECRET_DB
// 2. 写入 DKG 公钥（ContractMutation / buildContractRuntime 需要）
// 3. 通过 gov.RuntimeUpgrade 设置运行时版本为 NativeVersion（走原生路径，无需 WASM）
func setupFinalizeTxTest(t *testing.T) (*TeeChain, *crypto.PrivKey) {
	t.Helper()
	db := setupTestDB(t)

	// 生成 DKG 公钥并写入 DB
	priv, _, err := crypto.GenerateEd25519KeyPair(nil)
	require.NoError(t, err)
	pub := priv.GetPublicForSuite(crypto.SuiteEd25519)
	require.NoError(t, db.SetKey(GLOABL_STATE, DKG_PUB_KEY, pub.PublicKey))

	// 直接写入运行时版本为 NativeVersion（走原生 Go 路径，无需 WASM 字节码）
	// 绕过 RuntimeUpgrade 的 version=current+1 检查
	txn := db.NewTransaction()
	runtimeVersion := &store.StoreValue[uint32]{Namespace: "gov", Key: "runtime_version_"}
	require.NoError(t, runtimeVersion.Set(txn, module.NativeVersion))
	// GetRuntimeVersionAtHeight 优先查 runtime_at_height_（height → version 精确匹配）
	runtimeAtHeight := &store.StoreMapping[uint64, uint32]{Namespace: "gov", KeyPrefix: "runtime_at_height_"}
	require.NoError(t, runtimeAtHeight.Set(txn, 1, module.NativeVersion))
	require.NoError(t, txn.Commit())

	app := &TeeChain{
		state:          AppState{RuntimeVersion: module.NativeVersion},
		syncInProgress: make(map[string]bool),
		relay:          relay.NewEngine(),
	}
	return app, priv
}

// TestFinalizeTx_RelaySync_EmitsEvents 验收测试：
// 构造 Runtime → RelaySync 交易触发 gateway.Recharge → FinalizeTx 后
// ExecTxResult.Events 包含合约发射的事件。
func TestFinalizeTx_RelaySync_EmitsEvents(t *testing.T) {
	app, priv := setupFinalizeTxTest(t)
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	// 构造 relay 交易：gateway.Recharge（nonce=1，首次事件）
	userAddr := chaintypes.UniAddr{T: 1, V: []byte{0xaa, 0xbb}}
	userBytes, err := store.ScaleEncode(userAddr)
	require.NoError(t, err)
	amountBytes, err := store.ScaleEncode(uint64(5000))
	require.NoError(t, err)

	ev := relay.SyncEvent{
		ChainID:        1,
		Nonce:          1,
		BlockNum:       100,
		TargetContract: []byte("gateway"),
		EventType:      []byte("Recharge"),
		EventData:      [][]byte{userBytes, amountBytes},
	}
	sysCall := relay.MakeRelaySysCall(ev)

	// 用 DKG 私钥签名
	signer := priv.ToSigner(crypto.SuiteEd25519)
	tx, err := signTx(sysCall, signer)
	require.NoError(t, err)
	txBytes := GetTxBytes(tx)

	// 执行 FinalizeTx
	height := int64(1)
	ts := time.Unix(1700000000, 0)
	results, err := app.FinalizeTx([][]byte{txBytes}, txn, height, ts, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 验证交易成功
	require.Equal(t, uint32(abci.CodeTypeOK), results[0].Code)

	// 验证事件管道：ExecTxResult.Events 非空
	require.NotEmpty(t, results[0].Events, "FinalizeTx should attach events to ExecTxResult")

	// 管道事件：relay_synced 必须存在，且属性完整
	re := findEvent(results[0].Events, "relay_synced")
	require.NotNil(t, re, "relay_synced pipeline event must be present")
	require.Equal(t, "1", attrValue(re, "chain_id"))
	require.Equal(t, "1", attrValue(re, "nonce"))
	require.Equal(t, "100", attrValue(re, "block_num"))
	require.Equal(t, "gateway", attrValue(re, "contract"))
	require.Equal(t, "Recharge", attrValue(re, "event_type"))
	for _, a := range re.Attributes {
		require.True(t, a.Index)
	}

	for _, e := range results[0].Events {
		t.Logf("event: type=%s attrs=%v", e.Type, e.Attributes)
	}
}

// TestFinalizeTx_Contract_PipelineEvent 验证：合约交易 → ExecTxResult.Events 含 contract_call 管道事件。
func TestFinalizeTx_Contract_PipelineEvent(t *testing.T) {
	app, priv := setupFinalizeTxTest(t)
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	// 构造一个未知合约的 ContractCall（静默忽略，不发射合约事件，但有管道事件）
	sysCall := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: &model.ContractCall{
				Name:   []byte("unknown_contract"),
				Method: []byte{0x01, 0x02, 0x03, 0x04},
				Nonce:  1,
			},
		},
	}

	signer := priv.ToSigner(crypto.SuiteEd25519)
	tx, err := signTx(sysCall, signer)
	require.NoError(t, err)
	txBytes := GetTxBytes(tx)

	height := int64(1)
	ts := time.Unix(1700000000, 0)
	results, err := app.FinalizeTx([][]byte{txBytes}, txn, height, ts, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, uint32(abci.CodeTypeOK), results[0].Code)

	// 管道事件：contract_call 必须存在
	ce := findEvent(results[0].Events, "contract_call")
	require.NotNil(t, ce, "contract_call pipeline event must be present")
	require.Equal(t, "unknown_contract", attrValue(ce, "contract"))
	require.Equal(t, "01020304", attrValue(ce, "method"))
	require.Equal(t, "1", attrValue(ce, "nonce"))
	// 属性全部标记 Index=true（TxIndexer 可索引）
	for _, a := range ce.Attributes {
		require.True(t, a.Index)
	}
}

// TestFinalizeTx_MultiTx_EventsIsolated 验证：同区块多笔交易的事件互不串扰。
func TestFinalizeTx_MultiTx_EventsIsolated(t *testing.T) {
	app, priv := setupFinalizeTxTest(t)
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	signer := priv.ToSigner(crypto.SuiteEd25519)

	// tx1: 未知合约 A（仅管道事件 contract_call）
	call1 := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: &model.ContractCall{
				Name:   []byte("unknown_a"),
				Method: []byte{0x01, 0x02, 0x03, 0x04},
				Nonce:  1,
			},
		},
	}
	tx1, err := signTx(call1, signer)
	require.NoError(t, err)

	// tx2: 未知合约 B（仅管道事件 contract_call）
	call2 := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: &model.ContractCall{
				Name:   []byte("unknown_b"),
				Method: []byte{0x01, 0x02, 0x03, 0x04},
				Nonce:  2,
			},
		},
	}
	tx2, err := signTx(call2, signer)
	require.NoError(t, err)

	height := int64(1)
	ts := time.Unix(1700000000, 0)
	results, err := app.FinalizeTx([][]byte{GetTxBytes(tx1), GetTxBytes(tx2)}, txn, height, ts, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// 每笔交易只含自己的 contract_call 事件，互不串扰
	ce1 := findEvent(results[0].Events, "contract_call")
	require.NotNil(t, ce1)
	require.Equal(t, "unknown_a", attrValue(ce1, "contract"))
	require.Nil(t, findEvent(results[0].Events, "relay_synced"))

	ce2 := findEvent(results[1].Events, "contract_call")
	require.NotNil(t, ce2)
	require.Equal(t, "unknown_b", attrValue(ce2, "contract"))
	require.Nil(t, findEvent(results[1].Events, "relay_synced"))
}

// findEvent 在 ExecTxResult 中查找指定 type 的事件（不存在返回 nil）
func findEvent(events []abci.Event, eventType string) *abci.Event {
	for i := range events {
		if events[i].Type == eventType {
			return &events[i]
		}
	}
	return nil
}

// attrValue 取事件属性值（不存在返回 ""）
func attrValue(e *abci.Event, key string) string {
	for _, a := range e.Attributes {
		if a.Key == key {
			return a.Value
		}
	}
	return ""
}
