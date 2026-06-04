package teechain

import (
	"crypto/rand"
	"fmt"
	"time"

	teecrypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

const contractPoolNS = "POOL_TX"

// HasPendingContract 检查指定 dedupKey 是否已在交易池中有待处理的调用。
func (app *TeeChain) HasPendingContract(dedupKey string) bool {
	pending, _ := dbstore.GetJson[batch.BatchCall](contractPoolNS, dedupKey)
	return pending != nil
}

// SubmitContract 将合约调用写入交易池（持久化到 DB）
func (app *TeeChain) SubmitContract(contract string, method []byte, argsBytes [][]byte) error {
	return app.SubmitContractWithDedup(contract, method, argsBytes, "")
}

// SubmitContractWithDedup 将合约调用写入交易池，支持去重 key。
// 若 dedupKey 非空，同一 dedupKey 的新调用会覆盖旧调用，保证池中只有一个。
func (app *TeeChain) SubmitContractWithDedup(contract string, method []byte, argsBytes [][]byte, dedupKey string) error {
	call := &batch.BatchCall{
		Name:   []byte(contract),
		Method: method,
		Args:   argsBytes,
	}

	var key string
	if dedupKey != "" {
		key = dedupKey
		_ = dbstore.DeleteKey(contractPoolNS, key)
	} else {
		key = contractPoolKey()
	}

	if err := dbstore.SetJson(contractPoolNS, key, call); err != nil {
		return fmt.Errorf("persist contract call: %w", err)
	}

	// 如果上一个区块 commit 超过 3 秒，立即触发 ApplyToChain，减少交易等待时间
	if !app.lastCommitAt.IsZero() && time.Since(app.lastCommitAt) > 3*time.Second {
		util.LogWithYellow("TX-POOL", "last commit > 3s, notify apply immediately")
		app.notifyApplyToChain()
	}

	util.LogWithGreen("TX-POOL", "add tx of", contract)
	return nil
}

// notifyApplyToChain 非阻塞通知后台 goroutine 执行 ApplyToChain
func (app *TeeChain) notifyApplyToChain() {
	if app.txReadyCh == nil {
		return
	}
	select {
	case app.txReadyCh <- struct{}{}:
	default:
	}
}

// applyTxLoop 后台 goroutine，监听 txReadyCh 异步执行 ApplyToChain
func (app *TeeChain) applyTxLoop() {
	for range app.txReadyCh {
		app.ApplyToChain()
	}
}

// ApplyToChain 从交易池取出所有合约调用，打包为 batch 合约单交易提交。
// 多笔 pending call 合并为 execute_batch，共用节点 nonce，原子执行。
func (app *TeeChain) ApplyToChain() {
	if app.NodePriv == nil {
		return
	}

	calls, keys, err := dbstore.GetJsonList[batch.BatchCall](contractPoolNS, "")
	if err != nil {
		util.LogWithRed("TX-POOL", "read pool failed:", err)
		return
	}
	if len(calls) == 0 {
		return
	}

	util.LogWithGreen("TX-POOL", fmt.Sprintf("applying %d pending contract calls via batch", len(calls)))

	// 1) 收集有效的 pending calls
	batchCalls := make([]batch.BatchCall, 0, len(calls))
	validKeys := make([][]byte, 0, len(calls))
	for i, c := range calls {
		if c == nil {
			_ = dbstore.DeleteByteKey(keys[i])
			continue
		}
		batchCalls = append(batchCalls, *c)
		validKeys = append(validKeys, keys[i])
	}

	if len(batchCalls) == 0 {
		return
	}

	// 2) 编码参数：SCALE([]BatchCall)
	argsBytes, err := store.ScaleEncode(batchCalls)
	if err != nil {
		util.LogWithRed("TX-POOL", "encode batch calls failed:", err)
		return
	}

	// 3) 构造 batch 合约 execute_batch 调用
	signer := app.NodePriv.ToSigner(teecrypto.SuiteEd25519)
	nonce := GetNodeNextNonce(signer)
	call := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: &model.ContractCall{
				Name:   []byte("batch"),
				Method: store.MethodToSelectorBytes("execute_batch"),
				Args:   [][]byte{argsBytes},
				Nonce:  nonce,
			},
		},
	}

	// 4) 签名并提交单笔 batch 交易
	tx, err := signTx(call, signer)
	if err != nil {
		util.LogWithRed("TX-POOL", "sign batch tx failed:", err)
		return
	}

	if err := SubmitTx(tx); err != nil {
		util.LogWithRed("TX-POOL", "submit batch tx failed:", err)
		return
	}

	// 5) 提交成功，清空池中的所有相关 pending calls
	for _, k := range validKeys {
		err = dbstore.DeleteByteKey(k)
		fmt.Println("TX-POOL delete key", string(k))
		if err != nil {
			util.LogWithRed("TX-POOL", "delete key failed:", err)
		}
	}

	util.LogWithGreen("TX-POOL", "batch apply complete")
}

func contractPoolKey() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%d:%x", time.Now().UnixNano(), b)
}
