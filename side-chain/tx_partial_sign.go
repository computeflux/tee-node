package sidechain

import (
	"fmt"
	"sort"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/pkg/errors"
	"github.com/wetee-dao/tee-dsecret/pkg/chains"
	"github.com/wetee-dao/tee-dsecret/pkg/dkg"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
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
// tx_index - The index of the transaction.
// hubs - A slice of pointers to model.HubCall objects containing the calls to be batched.
// proposer - A pointer to a model.PubKey object representing the proposer's public key.
func (s *SideChain) sendPartialSign(chainId uint32, tx_index int64, hubs []*model.HubCall, proposer *model.PubKey) error {
	// Check if the list of hub calls is empty. If so, exit the function early.
	if len(hubs) == 0 {
		return errors.New("hubs is empty")
	}

	chain, err := s.getChain(chainId)
	if err != nil {
		return errors.Wrap(err, "getChain error")
	}

	// Initialize a slice to hold all index calls extracted from the hub calls.
	teeCalls := make([]*model.TeeCall, 0, len(hubs))
	// Iterate through each hub call and extract the index calls.
	for _, hub := range hubs {
		// Check if the hub call is nil. If so, log an error and skip to the next iteration.
		if hub == nil {
			util.LogWithRed("sendPartialSign", "hub is nil")
			continue
		}
		teeCalls = append(teeCalls, hub.Call...)
	}

	// Sort the index calls by their index in ascending order.
	sort.Slice(teeCalls, func(i, j int) bool {
		return teeCalls[i].Time < teeCalls[j].Time
	})

	// Initialize a slice to hold the decoded types.Call objects.
	calls := make([]types.Call, 0, len(teeCalls))
	// Iterate through each index call and decode it into a types.Call object.
	for _, c := range teeCalls {
		call, err := chain.TEECallToCall(c, s.dkg.DkgPubKey.AccountID())
		if err != nil {
			return errors.Wrap(err, "TEECallToHubCall error")
		}

		// Send the partial signature to the proposer vi
		// Decode the byte slice of the index call into the types.Call object.
		calls = append(calls, *call)
	}

	// Get the client for the main chain.
	client := chains.MainChain.GetClient()
	// Create a batch call using the decoded calls.
	call, err := client.BatchCall("batch_all", calls)
	if err != nil {
		return errors.Wrap(err, "BatchCall error")
	}

	// Create a new DSS signer using the DKG instance.
	signer := dkg.NewDssSigner(s.dkg, tx_index)
	// Partially sign the batch call using the signer.
	sig, err := client.PartialSign(signer, *call)
	if err != nil {
		return errors.Wrap(err, "PartialSign error")
	}

	// Create a new BlockPartialSign object with the partial signature and transaction index.
	psig := &model.BlockPartialSign{
		From:    s.dkg.P2PId().String(),
		HubSig:  sig,
		TxIndex: tx_index,
	}

	baseKey := TxIndexPrefix + fmt.Sprint(tx_index)
	// Store the batch call and hubCalls for retry after main-chain submit.
	err = model.SetCodec(GLOABL_STATE, baseKey+TxIndexCallSuffix, *call)
	if err != nil {
		return errors.Wrap(err, "Set tx call error")
	}
	err = model.SetJson(GLOABL_STATE, baseKey+TxIndexHubCallsSuffix, &hubCallsStore{HubCalls: hubs})
	if err != nil {
		return errors.Wrap(err, "Set tx hubcalls error")
	}

	// Send the partial signature to the proposer vio the proposer via P2P.
	err = s.p2p.Send(&model.PeerMsg{
		To:      model.SendToNode(proposer),
		Payload: &model.PeerMsg_BlockPartialSign{BlockPartialSign: psig},
	})
	if err != nil {
		return errors.Wrap(err, "P2P Send error")
	}
	return nil
}

// HandlePartialSign handles the block partial sign messages received via P2P.
func (s *SideChain) revPartialSign(msgBox any) error {
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

// handle block partial sign message
// handlePartialSign processes the received block partial signature.
// It saves the partial signature, retrieves all partial signatures for the transaction,
// checks if the number of signatures meets the threshold, and synchronizes the signatures to the hub if so.
//
// Parameters:
// msg - A pointer to the BlockPartialSign object containing the partial signature information.
//
// Returns:
// error - An error object if an error occurs during the process, otherwise nil.
func (s *SideChain) handlePartialSign(msg *model.BlockPartialSign) error {
	// Save the received partial signature to the global state using the sender's ID.
	err := s.SavePartialSig(msg.From, msg)
	if err != nil {
		util.LogWithRed("PartialSign", "SaveSig error", err)
		return err
	}

	// 4. 检索所有部分签名
	sigs, err := s.SigListOfTx(msg.TxIndex)
	if err != nil {
		util.LogWithRed("PartialSign", "GetSigList error", err)
		return err
	}

	// 5. 检查是否收集到足够的签名
	if len(sigs) < s.dkg.Threshold+1 {
		util.LogWithGray("PartialSign", "ALL =", len(sigs), "TH[+1] =", s.dkg.Threshold+1)
		return nil
	}

	// Extract the DSS share signatures from the retrieved partial signatures.
	shares := make([][]byte, 0, len(sigs))
	for _, sig := range sigs {
		shares = append(shares, sig.HubSig)
	}

	// 8. 同步到主链
	err = s.SyncToHub(msg.TxIndex, shares)
	if err != nil {
		// 同步失败，但状态已经在 SyncToHub 中更新
		util.LogWithRed("handlePartialSign", "SyncToHub error:", err)
		return err
	}

	return nil
}

// SavePartialSig saves the block partial signature to the global state.
// It serializes the provided BlockPartialSign object and stores it// in the global state
// with a key constructed using the partial signature prefix, transaction index, and user ID.
//
// Parameters:
// user_id - The ID of the user who created the partial signature.
// msg - A pointer to the BlockPartialSign object containing the partial signature information.
//
// Returns:
// error - An error object if an error occurs during serialization or storage, otherwise nil.
func (s *SideChain) SavePartialSig(user_id string, msg *model.BlockPartialSign) error {
	// Serialize the BlockPartialSign object into a byte slice.
	// Ignore the error returned by Marshal as it's not handled in the current implementation.
	// Note: This should be improved to handle errors properly.
	bt, err := msg.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal partial sig error")
	}
	// Store the serialized data in the global state with a constructed key.
	// The key is formed by combining the partial signature prefix, transaction index, and user ID.
	return model.SetKey(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(msg.TxIndex)+"_"+user_id, bt)
}

// SigListOfTx retrieves a list of block partial signatures associated with a specific transaction index.
// It fetches the serialized data from the global state using the provided transaction index,
// deserializes each data entry into a BlockPartialSign object, and filters out any objects
// that do not match the given transaction index.
//
// Parameters:
// txIndex - The index of the transaction for which to retrieve partial signatures.
//
// Returns:
// []*model.BlockPartialSign - A slice of pointers to BlockPartialSign objects representing the partial signatures.
// error - An error object if an error occurs during the process, otherwise nil.
func (s *SideChain) SigListOfTx(txIndex int64) ([]*model.BlockPartialSign, error) {
	// Fetch the list of serialized partial signatures from the global state with the given prefix.
	// The prefix is constructed using the PartialSigPrefix and the transaction index.
	// The method fetches a maximum of 5000 items starting from the first item.
	bts, _, err := model.GetList(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(txIndex)+"_", nil, 5000)
	if err != nil {
		// If an error occurs during the retrieval, return nil and the error.
		return nil, err
	}

	// Initialize an empty slice to hold the deserialized BlockPartialSign objects.
	// The capacity of the slice is set to the number of serialized items retrieved.
	sigs := make([]*model.BlockPartialSign, 0, len(bts))
	for _, bt := range bts {
		// Create a new BlockPartialSign object to hold the deserialized data.
		msg := new(model.BlockPartialSign)
		// Deserialize the byte slice into the BlockPartialSign object.
		err := msg.Unmarshal(bt)
		if err != nil {
			// If deserialization fails, log the error in red and skip to the next item.
			util.LogWithRed("GetSig", "Unmarshal error", err)
			continue
		}

		// Check if the transaction index of the deserialized object matches the given transaction index.
		if msg.TxIndex == txIndex {
			// If it matches, add the object to the slice of partial signatures.
			sigs = append(sigs, msg)
		}
	}

	// Return the slice of partial signatures and nil error.
	return sigs, nil
}

func (s *SideChain) DeleteSigOfTx(txIndex int64) error {
	return model.DeletekeysByPrefix(GLOABL_STATE, PartialSigPrefix+fmt.Sprint(txIndex)+"_")
}
