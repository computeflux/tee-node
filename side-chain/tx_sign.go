package sidechain

import (
	"bytes"
	"fmt"

	"github.com/cockroachdb/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/mempool"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
)

// Submit tx to sidechain
func SubmitTx(tx *model.Tx) error {
	if SideChainNode == nil {
		return fmt.Errorf("side chain node is not initialized")
	}
	return SideChainNode.Mempool().CheckTx(GetTxBytes(tx), func(r *abci.ResponseCheckTx) {}, mempool.TxInfo{
		SenderP2PID: SideChainNode.NodeInfo().ID(),
	})
}

// Get tx bytes
func GetTxBytes(tx *model.Tx) []byte {
	buf := new(bytes.Buffer)
	protoio.WriteMessage(tx, buf)

	return buf.Bytes()
}

func (s *SideChain) SubmitCallFromNode(call *model.SysCall) error {
	sigbt, err := call.BytesForSig()
	if err != nil {
		return errors.Errorf("BytesForSig: %v", err)
	}

	signer := s.NodePriv.ToSigner()
	signature, err := signer.Sign(sigbt)
	if err != nil {
		return errors.Errorf("Sign: %v", err)
	}

	buf := new(bytes.Buffer)
	protoio.WriteMessage(call, buf)
	tx := &model.Tx{
		Caller:     signer.PublicKey,
		Call:       buf.Bytes(),
		CallerType: 0,
		Signature:  signature,
	}

	return SubmitTx(tx)
}

func (s *SideChain) GetCallBytesFromNode(call *model.SysCall) ([]byte, error) {
	sigbt, err := call.BytesForSig()
	if err != nil {
		return nil, errors.Errorf("BytesForSig: %v", err)
	}
	dkg := s.GetDKG()
	if dkg == nil || dkg.Signer == nil {
		return nil, errors.Errorf("DKG signer not initialized")
	}

	signer := dkg.Signer.ToSigner()
	signature, err := signer.Sign(sigbt)
	if err != nil {
		return nil, errors.Errorf("Sign: %v", err)
	}

	buf := new(bytes.Buffer)
	protoio.WriteMessage(call, buf)
	tx := &model.Tx{
		Caller:     signer.PublicKey,
		Call:       buf.Bytes(),
		CallerType: 0,
		Signature:  signature,
	}
	return GetTxBytes(tx), nil
}

// func (s *SideChain) SignTxWithNode(tx *model.Tx) error {
// 	// 获取 DKG Signer
// 	dkg := s.GetDKG()
// 	if dkg == nil || dkg.Signer == nil {
// 		return errors.Errorf("DKG signer not initialized")
// 	}
// 	signer := dkg.Signer.ToSigner()
// 	fmt.Println(signer.PublicKey)

// 	// 计算签名数据并签名
// 	txBytes, err := model.TxBytesForSigning(tx)
// 	if err != nil {
// 		return errors.Errorf("SignTxWithNode: %v", err)
// 	}

// 	signature, err := signer.Sign(txBytes)
// 	if err != nil {
// 		return errors.Errorf("Sign: %v", err)
// 	}

// 	// 设置签名者信息
// 	tx.Caller = signer.PublicKey
// 	tx.CallerType = 10000
// 	tx.Signature = signature

// 	return nil
// }
