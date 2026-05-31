package teechain

import (
	"bytes"
	"fmt"

	"github.com/cockroachdb/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/mempool"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/wetee-dao/ink.go"
	"crypto/sha256"
)

// TxHash 计算交易唯一标识（sha256，与 CometBFT 一致）
func TxHash(tx *model.Tx) string {
	bt := GetTxBytes(tx)
	h := sha256.Sum256(bt)
	return fmt.Sprintf("%X", h)
}

// signTx 签名并构造 Tx
func signTx(call *model.SysCall, signer ink.SignerType) (*model.Tx, error) {
	sigBytes, err := call.BytesForSig()
	if err != nil {
		return nil, fmt.Errorf("BytesForSig: %w", err)
	}

	signature, err := signer.Sign(sigBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := protoio.WriteMessage(call, buf); err != nil {
		return nil, fmt.Errorf("marshal syscall: %w", err)
	}

	return &model.Tx{
		Caller:     signer.Public(),
		CallerType: 0,
		Call:       buf.Bytes(),
		Signature:  signature,
	}, nil
}

// Submit tx to teechain
func SubmitTx(tx *model.Tx) error {
	return submitBytes(GetTxBytes(tx))
}

func submitBytes(txBytes []byte) error {
	if TeeChainNode == nil {
		return fmt.Errorf("side chain node is not initialized")
	}
	return TeeChainNode.Mempool().CheckTx(txBytes, func(r *abci.ResponseCheckTx) {}, mempool.TxInfo{
		SenderP2PID: TeeChainNode.NodeInfo().ID(),
	})
}

// Get tx bytes
func GetTxBytes(tx *model.Tx) []byte {
	buf := new(bytes.Buffer)
	protoio.WriteMessage(tx, buf)

	return buf.Bytes()
}

// SubmitCallFromNode 使用节点私钥签名并提交系统交易（nonce=0）
func (s *TeeChain) SubmitCallFromNode(call *model.SysCall) error {
	if s.NodePriv == nil {
		return fmt.Errorf("node private key not initialized")
	}
	signer := s.NodePriv.ToSigner(model.SuiteEd25519)
	tx, err := signTx(call, signer)
	if err != nil {
		return err
	}
	return SubmitTx(tx)
}

// GetCallBytesFromNode 使用 DKG 签名构造交易并返回 tx bytes
func (s *TeeChain) GetCallBytesFromNode(call *model.SysCall) ([]byte, error) {
	dkg := s.GetDKG()
	if dkg == nil || dkg.Signer == nil {
		return nil, errors.New("DKG signer not initialized")
	}
	tx, err := signTx(call, dkg.Signer.ToSigner(model.SuiteEd25519))
	if err != nil {
		return nil, err
	}
	return GetTxBytes(tx), nil
}

// SignAndBuildRelayTx 为 relay 事件预签名交易
func (s *TeeChain) SignAndBuildRelayTx(call *model.SysCall) ([]byte, error) {
	dkg := s.GetDKG()
	if dkg == nil || dkg.Signer == nil {
		return nil, errors.New("DKG signer not initialized")
	}
	tx, err := signTx(call, dkg.Signer.ToSigner(model.SuiteEd25519))
	if err != nil {
		return nil, err
	}
	return GetTxBytes(tx), nil
}
