package teechain

import (
	"fmt"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/pkg/errors"
)

const TxIndexPrefix = "tx_index_"
const TxIndexCallSuffix = "_call"
const TxIndexHubCallsSuffix = "_hubcalls"
const PartialSigPrefix = "partial_sig_"

// hubCallsByIdsStore 用于持久化 hub call IDs（替代旧的 []*model.HubCall）。
type hubCallsByIdsStore struct {
	Ids []uint64 `json:"ids"`
}

// HandlePartialSign handles the block partial sign messages received via P2P.
func (s *TeeChain) revPartialSign(msgBox any) error {
	if s.txCh == nil {
		return errors.New("txCh is nil")
	}

	msg, ok := msgBox.(*model.BlockPartialSign)
	if !ok {
		return errors.New("invalid partial sign message type")
	}
	s.txCh.Push(msg)
	return nil
}

// handlePartialSign processes the received block partial signature.
func (s *TeeChain) handlePartialSign(msg *model.BlockPartialSign) error {
	if s.dkg == nil {
		return errors.New("handlePartialSign: dkg is nil")
	}

	// Save the received partial signature
	err := s.savePartialSig(msg.From, msg)
	if err != nil {
		util.LogWithRed("PartialSign", "SaveSig error", err)
		return err
	}

	// Retrieve all partial signatures for this (txIndex, chainId)
	sigs, err := s.SigListOfTx(msg.TxIndex, msg.ChainId)
	if err != nil {
		util.LogWithRed("PartialSign", "GetSigList error", err)
		return err
	}

	// Check if we have collected enough signatures
	if len(sigs) < s.dkg.Threshold {
		util.LogWithGray("PartialSign", "ALL =", len(sigs), "TH[+1] =", s.dkg.Threshold)
		return nil
	}
	util.LogWithGreen("PartialSign", "ALL =", len(sigs), "TH[+1] =", s.dkg.Threshold)

	// Deduplication
	key := fmt.Sprintf("%d_%d", msg.TxIndex, msg.ChainId)
	s.syncMu.Lock()
	if s.syncInProgress[key] {
		s.syncMu.Unlock()
		util.LogWithGray("PartialSign", "sync already in progress for", key)
		return nil
	}
	s.syncInProgress[key] = true
	s.syncMu.Unlock()

	// Extract the DSS share signatures
	shares := make([][]byte, 0, len(sigs))
	for _, sig := range sigs {
		shares = append(shares, sig.HubSig)
	}

	// Launch a detached goroutine for the main-chain submission
	go func(txIndex int64, chainId uint32, shares [][]byte) {
		defer func() {
			s.syncMu.Lock()
			delete(s.syncInProgress, key)
			s.syncMu.Unlock()
		}()

		err := s.SyncToHub(txIndex, chainId, shares)
		if err != nil {
			util.LogWithRed("handlePartialSign", "SyncToHub error:", err)
		}
	}(msg.TxIndex, msg.ChainId, shares)

	return nil
}

// SyncToHub submits the aggregated threshold signature to the main chain.
// txIndex: 批次标记（首个 hub call ID）, chainId: 目标主链, sigs: 聚合后的部分签名
func (s *TeeChain) SyncToHub(txIndex int64, chainId uint32, sigs [][]byte) error {
	if s.dkg == nil {
		return errors.New("SyncToHub: dkg is nil")
	}

	chain, err := chains.GetChainByID(chainId)
	if err != nil {
		return errors.Wrap(err, "getChain error")
	}

	// 尝试从存储中恢复 hub call IDs（新路径）
	ids, _ := loadHubCallIds(txIndex, chainId)
	if len(ids) > 0 {
		// 新路径：从 batch store 重建 ChainCall
		call, err := buildBatchCallFromIDs(s.dkg, chainId, ids)
		if err != nil {
			return errors.Wrap(err, "buildBatchCallFromIDs")
		}

		util.LogWithGray("SyncToHub", "chainId:", chainId, "sync sigs =", len(sigs))
		signer := dkg.NewDssSigner(s.dkg, 0, chain.GetCipher())
		signer.SetSigs(sigs)

		err = chain.SignAndSubmit(signer, *call, false, 0)
		if err != nil {
			util.LogWithRed("SyncToHub", "chainId:", chainId, "error =>", err.Error())
			return s.submitSyncTxEnd(chainId, ids, false)
		}

		util.LogWithGreen("SyncToHub", "success", "chainId", chainId)
		return s.submitSyncTxEnd(chainId, ids, true)
	}

	// 回退路径：从 tx_index 存储加载已构造的 ChainCall
	call, err := loadBatchCall(txIndex, chainId)
	if err != nil {
		util.LogWithRed("SyncToHub", "error: call not found for txIndex:", txIndex, "chainId:", chainId)
		return errors.New("sync to hub error: call not found")
	}

	util.LogWithGray("SyncToHub", "chainId:", chainId, "sync sigs =", len(sigs))
	signer := dkg.NewDssSigner(s.dkg, 0, chain.GetCipher())
	signer.SetSigs(sigs)

	err = chain.SignAndSubmit(signer, *call, false, 0)
	if err != nil {
		util.LogWithRed("SyncToHub", "chainId:", chainId, "error =>", err.Error())
		return s.submitSyncTxEnd(chainId, nil, false)
	}

	util.LogWithGreen("SyncToHub", "success", "chainId", chainId)
	return s.submitSyncTxEnd(chainId, nil, true)
}

// SavePartialSig saves the block partial signature to the global state.
func (s *TeeChain) savePartialSig(user_id string, msg *model.BlockPartialSign) error {
	bt, err := msg.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal partial sig error")
	}
	return store.SetKey(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(msg.TxIndex)+"_"+fmt.Sprint(msg.ChainId)+"_"+user_id, bt)
}

// SigListOfTx retrieves a list of block partial signatures for (txIndex, chainId).
func (s *TeeChain) SigListOfTx(txIndex int64, chainId uint32) ([]*model.BlockPartialSign, error) {
	bts, _, err := store.GetList(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(txIndex)+"_"+fmt.Sprint(chainId)+"_", nil, 5000)
	if err != nil {
		return nil, err
	}

	sigs := make([]*model.BlockPartialSign, 0, len(bts))
	for _, bt := range bts {
		msg := new(model.BlockPartialSign)
		err := msg.Unmarshal(bt)
		if err != nil {
			util.LogWithRed("GetSig", "Unmarshal error", err)
			continue
		}
		if msg.TxIndex == txIndex && msg.ChainId == chainId {
			sigs = append(sigs, msg)
		}
	}
	return sigs, nil
}

// DeleteSigOfTx deletes partial signatures for a txIndex.
func (s *TeeChain) DeleteSigOfTx(txIndex int64) error {
	return store.DeletekeysByPrefix(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(txIndex)+"_")
}
