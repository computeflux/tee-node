package teechain

import (
	"fmt"
	"sort"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
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

// hubCallsStore wraps []*model.HubCall for JSON persistence (tx_index_ hubcalls), used for retry after main-chain submit.
type hubCallsStore struct {
	HubCalls []*model.HubCall `json:"hub_calls"`
}

// sendPartialSign sends partial signatures of a batch call to a specified proposer.
// It constructs a batch call from the provided hub calls, partially signs it,
// and then sends the partial signature to the proposer via P2P.
//
// Parameters:
// chainId - The target main-chain ID.
// tx_index - The index of the transaction.
// hubs - A slice of pointers to model.HubCall objects containing the calls to be batched.
// proposer - A pointer to a crypto.PubKey object representing the proposer's public key.
func (s *TeeChain) sendPartialSign(chainId uint32, tx_index int64, hubs []*model.HubCall, proposer []byte, txn *store.Txn) error {
	// Check if the list of hub calls is empty. If so, exit the function early.
	if len(hubs) == 0 {
		return errors.New("hubs is empty")
	}
	if s.dkg == nil {
		return errors.New("dkg is nil")
	}

	chain, err := chains.GetChainByID(chainId)
	if err != nil {
		return errors.Wrap(err, "getChain error")
	}

	// Initialize a slice to hold all index calls extracted from the hub calls.
	teeCalls := make([]*model.TeeCall, 0)
	// Iterate through each hub call and extract the index calls.
	for _, hub := range hubs {
		// Check if the hub call is nil. If so, log an error and skip to the next iteration.
		if hub == nil {
			util.LogWithRed("sendPartialSign", "hub is nil")
			continue
		}
		for _, c := range hub.Call {
			if c == nil {
				util.LogWithRed("sendPartialSign", "tee call is nil")
				continue
			}
			teeCalls = append(teeCalls, c)
		}
	}

	// Sort the index calls by their index in ascending order.
	sort.Slice(teeCalls, func(i, j int) bool {
		return teeCalls[i].Time < teeCalls[j].Time
	})

	cipher := chain.GetCipher()
	dkgPub, ok := s.dkg.DkgPubKeys[cipher]
	if !ok || dkgPub == nil {
		return errors.Errorf("dkg public key not found for cipher %d", cipher)
	}

	// Initialize a slice to hold ChainCall objects.
	var calls []chaintypes.ChainCall
	// Iterate through each index call and decode it.
	for _, c := range teeCalls {
		call, err := chain.TEECallToCall(c, dkgPub.Byte())
		if err != nil {
			return errors.Wrap(err, "TEECallToHubCall error")
		}
		calls = append(calls, *call)
	}

	// Create a batch call for the target chain.
	// GetChainByID returns ChainApi, but all concrete implementations also satisfy MainChainApi.
	mainChain, ok := chain.(chains.MainChainApi)
	if !ok {
		return errors.Errorf("chain %d does not support BatchCall", chainId)
	}
	call, err := mainChain.BatchCall("batch_all", calls)
	if err != nil {
		return errors.Wrap(err, "BatchCall error")
	}

	// Create a new DSS signer using the DKG instance.
	signer := dkg.NewDssSigner(s.dkg, tx_index, cipher)
	// Partially sign the batch call using the signer.
	sig, err := mainChain.PartialSign(signer, *call)
	if err != nil {
		return errors.Wrap(err, "PartialSign error")
	}

	// Create a new BlockPartialSign object with the partial signature and transaction index.
	p2pId := s.dkg.P2PId()
	if p2pId == nil {
		return errors.New("P2PId not found in validator set")
	}
	psig := &model.BlockPartialSign{
		From:    p2pId.String(),
		HubSig:  sig,
		TxIndex: tx_index,
		ChainId: chainId,
	}

	baseKey := TxIndexPrefix + fmt.Sprint(tx_index) + "_" + fmt.Sprint(chainId)
	// Store the batch call and hubCalls atomically via txn for retry after main-chain submit.
	if err := store.TxnSetCodec(txn, GLOABL_STATE, baseKey+TxIndexCallSuffix, *call); err != nil {
		return errors.Wrap(err, "Set tx call error")
	}
	hubcallsKey := store.ComboNamespaceKey(GLOABL_STATE, baseKey+TxIndexHubCallsSuffix)
	if err := store.TxnSetJson(txn, hubcallsKey, &hubCallsStore{HubCalls: hubs}); err != nil {
		return errors.Wrap(err, "Set tx hubcalls error")
	}

	// Send the partial signature to the proposer via P2P.
	// P2P send is best-effort; don't abort block on failure.
	err = s.p2p.Send(&model.PeerMsg{
		To: &model.To{
			Payload: &model.To_Node{
				Node: proposer,
			},
		},
		Payload: &model.PeerMsg_BlockPartialSign{BlockPartialSign: psig},
	})
	if err != nil {
		util.LogWithRed("sendPartialSign", "P2P Send error:", err)
	}
	return nil
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
// It saves the partial signature, retrieves all partial signatures for the transaction,
// checks if the number of signatures meets the threshold, and synchronizes the signatures to the hub if so.
//
// To avoid blocking the PersistChan consumer during long-running main-chain submissions,
// the actual SyncToHub is executed in a detached goroutine when the threshold is reached.
func (s *TeeChain) handlePartialSign(msg *model.BlockPartialSign) error {
	if s.dkg == nil {
		return errors.New("handlePartialSign: dkg is nil")
	}

	// Save the received partial signature to the global state using the sender's ID.
	err := s.SavePartialSig(msg.From, msg)
	if err != nil {
		util.LogWithRed("PartialSign", "SaveSig error", err)
		return err
	}

	// Retrieve all partial signatures for this (txIndex, chainId).
	sigs, err := s.SigListOfTx(msg.TxIndex, msg.ChainId)
	if err != nil {
		util.LogWithRed("PartialSign", "GetSigList error", err)
		return err
	}

	// Check if we have collected enough signatures.
	if len(sigs) < s.dkg.Threshold+1 {
		util.LogWithGray("PartialSign", "ALL =", len(sigs), "TH[+1] =", s.dkg.Threshold+1)
		return nil
	}

	// Deduplication: ensure only one goroutine submits this (txIndex, chainId) at a time.
	key := fmt.Sprintf("%d_%d", msg.TxIndex, msg.ChainId)
	s.syncMu.Lock()
	if s.syncInProgress[key] {
		s.syncMu.Unlock()
		util.LogWithGray("PartialSign", "sync already in progress for", key)
		return nil
	}
	s.syncInProgress[key] = true
	s.syncMu.Unlock()

	// Extract the DSS share signatures from the retrieved partial signatures.
	shares := make([][]byte, 0, len(sigs))
	for _, sig := range sigs {
		shares = append(shares, sig.HubSig)
	}

	// Launch a detached goroutine for the long-running main-chain submission.
	// This prevents blocking PersistChan.Start() so that other partial signatures
	// (including those for different chains) can be processed concurrently.
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

// SavePartialSig saves the block partial signature to the global state.
// The key now includes chainId so that signatures for different main-chains are isolated.
func (s *TeeChain) SavePartialSig(user_id string, msg *model.BlockPartialSign) error {
	bt, err := msg.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal partial sig error")
	}
	return store.SetKey(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(msg.TxIndex)+"_"+fmt.Sprint(msg.ChainId)+"_"+user_id, bt)
}

// SigListOfTx retrieves a list of block partial signatures associated with a specific
// transaction index and chainId.
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

func (s *TeeChain) DeleteSigOfTx(txIndex int64) error {
	return store.DeletekeysByPrefix(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(txIndex)+"_")
}

// DeleteSigOfTxChain deletes partial signatures for a specific (txIndex, chainId).
func (s *TeeChain) DeleteSigOfTxChain(txIndex int64, chainId uint32) error {
	return store.DeletekeysByPrefix(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(txIndex)+"_"+fmt.Sprint(chainId)+"_")
}
