package teechain

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
)

const contractPoolNS = "CONTRACT_TX"

// PendingContractCall 池中的待提交合约调用
type PendingContractCall struct {
	Contract string   `json:"contract"`
	Method   []byte   `json:"method"`
	Args     [][]byte `json:"args"`
}

// HasPendingContract 检查指定 dedupKey 是否已在交易池中有待处理的调用。
func (app *TeeChain) HasPendingContract(dedupKey string) bool {
	pending, _ := model.GetJson[PendingContractCall](contractPoolNS, dedupKey)
	return pending != nil
}

// SubmitContract 将合约调用写入交易池（持久化到 DB）
func (app *TeeChain) SubmitContract(contract string, method []byte, argsBytes [][]byte) error {
	return app.SubmitContractWithDedup(contract, method, argsBytes, "")
}

// SubmitContractWithDedup 将合约调用写入交易池，支持去重 key。
// 若 dedupKey 非空，同一 dedupKey 的新调用会覆盖旧调用，保证池中只有一个。
func (app *TeeChain) SubmitContractWithDedup(contract string, method []byte, argsBytes [][]byte, dedupKey string) error {
	call := &PendingContractCall{
		Contract: contract,
		Method:   method,
		Args:     argsBytes,
	}

	var key string
	if dedupKey != "" {
		key = dedupKey
		_ = model.DeleteKey(contractPoolNS, key)
	} else {
		key = contractPoolKey()
	}

	if err := model.SetJson(contractPoolNS, key, call); err != nil {
		return fmt.Errorf("persist contract call: %w", err)
	}

	// 如果上一个区块 commit 超过 3 秒，立即触发 ApplyToChain，减少交易等待时间
	if !app.lastCommitAt.IsZero() && time.Since(app.lastCommitAt) > 3*time.Second {
		util.LogWithYellow("TX-POOL", "last commit > 3s, notify apply immediately")
		app.notifyApplyToChain()
	}

	util.LogWithGreen("TX-POOL", "queued:", contract)
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

// ApplyToChain 从交易池取出所有合约调用，批量签名并提交到链上
func (app *TeeChain) ApplyToChain() {
	if app.NodePriv == nil {
		return
	}

	calls, keys, err := model.GetJsonList[PendingContractCall](contractPoolNS, "")
	if err != nil {
		util.LogWithRed("TX-POOL", "read pool failed:", err)
		return
	}
	if len(calls) == 0 {
		return
	}

	util.LogWithGreen("TX-POOL", fmt.Sprintf("applying %d pending contract calls", len(calls)))

	signer := app.NodePriv.ToSigner(model.SuiteEd25519)
	nonce := GetNodeNextNonce(signer)
	for i, c := range calls {
		if c == nil {
			_ = model.DeleteByteKey(keys[i])
			continue
		}

		call := &model.SysCall{
			Payload: &model.SysCall_Contract{
				Contract: &model.ContractCall{
					Name:   []byte(c.Contract),
					Method: c.Method,
					Args:   c.Args,
					Nonce:  nonce,
				},
			},
		}
		nonce++

		tx, err := signTx(call, signer)
		if err != nil {
			util.LogWithRed("TX-POOL", "sign failed:", c.Contract, err)
			continue
		}

		if err := SubmitTx(tx); err != nil {
			util.LogWithRed("TX-POOL", "submit failed:", c.Contract, err)
			continue
		}

		// 提交成功，从池中删除
		_ = model.DeleteByteKey(keys[i])
	}

	util.LogWithGreen("TX-POOL", "batch apply complete")
}

func contractPoolKey() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%d:%x", time.Now().UnixNano(), b)
}
