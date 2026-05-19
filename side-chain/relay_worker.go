package sidechain

import (
	"bytes"

	"github.com/cockroachdb/errors"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/side-chain/relay"
)

// prepareRelayTxs 出块节点拉取主链事件 → 创建 relay tx (仅 proposer 调用)
func (app *SideChain) prepareRelayTxs() [][]byte {
	if app.chains == nil || app.relay == nil || app.dkg == nil {
		return nil
	}

	var txs [][]byte

	for chainID, chainApi := range app.chains {
		if chainApi == nil {
			continue
		}
		c := *chainApi

		latest, err := c.GetLatestNonce()
		if err != nil || latest == 0 {
			continue
		}

		last, _ := app.relay.LastNonce(chainID)
		if last >= latest {
			continue
		}

		for n := last + 1; n <= latest; n++ {
			target, typ, data, err := c.GetRelayEvent(n)
			if err != nil || target == nil {
				continue
			}

			ev := relay.BuildSyncEvent(chainID, n, 0, target, typ, data)
			sysCall := relay.MakeRelaySysCall(ev)

			txBytes, err := app.signAndBuildRelayTx(sysCall)
			if err != nil {
				LogWithTime("Relay: sign error:", err)
				continue
			}
			txs = append(txs, txBytes)
		}
	}
	return txs
}

// signAndBuildRelayTx 用 DKG signer 签名 relay SysCall, 返回 tx bytes
func (app *SideChain) signAndBuildRelayTx(sysCall *model.SysCall) ([]byte, error) {
	sigBytes, err := sysCall.BytesForSig()
	if err != nil {
		return nil, errors.Wrap(err, "BytesForSig")
	}

	signer := app.dkg.Signer.ToSigner(model.SuiteEd25519)
	signature, err := signer.Sign(sigBytes)
	if err != nil {
		return nil, errors.Wrap(err, "Sign")
	}

	buf := new(bytes.Buffer)
	protoio.WriteMessage(sysCall, buf)
	tx := &model.Tx{
		Caller:     signer.Public(),
		Call:       buf.Bytes(),
		CallerType: 0,
		Signature:  signature,
	}

	return GetTxBytes(tx), nil
}
