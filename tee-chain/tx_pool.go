package teechain

import (
	"crypto/rand"
	"encoding/hex"
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

// SubmitContract 将合约调用写入交易池（持久化到 DB）
func (app *TeeChain) SubmitContract(contract string, method []byte, argsBytes [][]byte) error {
	call := &PendingContractCall{
		Contract: contract,
		Method:   method,
		Args:     argsBytes,
	}
	key := contractPoolKey()
	if err := model.SetJson(contractPoolNS, key, call); err != nil {
		return fmt.Errorf("persist contract call: %w", err)
	}

	util.LogWithGreen("TX-POOL", "queued:", contract, hex.EncodeToString(method[:4]))
	return nil
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
