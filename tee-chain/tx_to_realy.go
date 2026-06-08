// tx_sync.go 用于记录提交到主链的交易：SyncToHub、tx_index_ 存储（call/hubCalls）、
// SyncTxStart/End 状态（HubSyncIndexKey、HubSyncStep1/2/End、IsHubSyncRuning）及清理逻辑。
package teechain

import (
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	teecrypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/store"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	cstore "github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/pkg/errors"
)

// SyncToHub submits the aggregated threshold signature to a specific main-chain.
// The call and hubCalls are now isolated per chain under tx_index_<id>_<chainId>.
func (s *TeeChain) SyncToHub(txIndex int64, chainId uint32, sigs [][]byte) error {
	fmt.Println("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	if s.dkg == nil {
		return errors.New("SyncToHub: dkg is nil")
	}

	baseKey := TxIndexPrefix + fmt.Sprint(txIndex) + "_" + fmt.Sprint(chainId)
	call, err := store.GetCodec[chaintypes.ChainCall](GLOABL_STATE, baseKey+TxIndexCallSuffix)
	if err != nil || call == nil {
		util.LogWithRed("SyncToHub", "error: call not found for txIndex:", txIndex, "chainId:", chainId)
		return errors.New("sync to hub error: call not found")
	}

	chain, err := chains.GetChainByID(chainId)
	if err != nil {
		return errors.Wrap(err, "getChain error")
	}

	// Aggregate signature
	util.LogWithGray("SyncToHub", "chainId:", chainId, "sync sigs =", len(sigs))
	signer := dkg.NewDssSigner(s.dkg, txIndex, chain.GetCipher())
	signer.SetSigs(sigs)

	// Submit sync tx to the main-chain
	err = chain.SignAndSubmit(signer, *call, false, 0)
	if err != nil {
		util.LogWithRed("SyncToHub", "chainId:", chainId, "error =>", err.Error())
		fmt.Println("                    ", " SS58 => ", s.dkg.DkgPubKeys[chain.GetCipher()])
		fmt.Println("                    ", " SYNC at batch tx id", txIndex, "chainId", chainId)

		// On failure, submit SyncTxRetry so that the next proposer can retry.
		util.LogWithYellow("SyncToHub", "submitting SyncTxRetry to mempool", "txIndex:", txIndex, "chainId:", chainId)
		tx := &model.SysCall{
			Payload: &model.SysCall_SyncTxRetry{
				SyncTxRetry: &model.SyncTxRetry{
					TxIndex: txIndex,
					ChainId: chainId,
				},
			},
		}
		return s.SubmitCallFromNode(tx)
	}

	util.LogWithGreen("SyncToHub", "success at batch tx id", fmt.Sprint(txIndex), "chainId", chainId)

	// On success, submit SyncTxEnd so all nodes clean up per-chain state.
	tx := &model.SysCall{
		Payload: &model.SysCall_SyncTxEnd{
			SyncTxEnd: &model.SyncTxEnd{
				TxIndex: txIndex,
				ChainId: chainId,
			},
		},
	}
	return s.SubmitCallFromNode(tx)
}

// deleteTxIndexStore deletes all tx_index_<id>_ keys atomically through the given txn.
func deleteTxIndexStore(txIndex int64, txn *store.Txn) error {
	prefix := store.ComboNamespaceKey(GLOABL_STATE, TxIndexPrefix+fmt.Sprint(txIndex)+"_")
	return txn.DeletekeysByPrefix(prefix)
}

// saveTxIndexChains persists the list of chainIds involved in a given txIndex (atomic via txn).
func saveTxIndexChains(txIndex int64, chainIds []uint32, txn *store.Txn) error {
	key := store.ComboNamespaceKey(GLOABL_STATE, TxIndexPrefix+fmt.Sprint(txIndex)+"_chains")
	return store.TxnSetJson(txn, key, &chainIds)
}

// getTxIndexChains retrieves the list of chainIds involved in a given txIndex, reading from txn first.
func getTxIndexChains(txIndex int64, txn *store.Txn) ([]uint32, error) {
	key := store.ComboNamespaceKey(GLOABL_STATE, TxIndexPrefix+fmt.Sprint(txIndex)+"_chains")
	chains, err := store.TxnGetJson[[]uint32](txn, key)
	if err != nil {
		return nil, err
	}
	if chains == nil {
		return []uint32{}, nil
	}
	return *chains, nil
}

// markChainDone records that a specific chain has finished syncing for the given txIndex.
func markChainDone(txIndex int64, chainId uint32, txn *store.Txn) error {
	key := store.ComboNamespaceKey(GLOABL_STATE, TxIndexPrefix+fmt.Sprint(txIndex)+"_done")
	done, err := store.TxnGetJson[[]uint32](txn, key)
	if err != nil {
		return err
	}
	var list []uint32
	if done != nil {
		list = *done
	}

	// Avoid duplicates
	for _, cid := range list {
		if cid == chainId {
			return nil
		}
	}

	list = append(list, chainId)
	return store.TxnSetJson(txn, key, &list)
}

// checkAllChainsDone checks whether every chain involved in txIndex has been marked done.
func checkAllChainsDone(txIndex int64, txn *store.Txn) (bool, error) {
	chains, err := getTxIndexChains(txIndex, txn)
	if err != nil {
		return false, err
	}
	if len(chains) == 0 {
		return true, nil
	}

	key := store.ComboNamespaceKey(GLOABL_STATE, TxIndexPrefix+fmt.Sprint(txIndex)+"_done")
	done, err := store.TxnGetJson[[]uint32](txn, key)
	if err != nil {
		return false, err
	}
	if done == nil {
		return false, nil
	}

	doneMap := make(map[uint32]bool)
	for _, cid := range *done {
		doneMap[cid] = true
	}
	for _, cid := range chains {
		if !doneMap[cid] {
			return false, nil
		}
	}
	return true, nil
}

// ── Global txIndex generator ──────────────────────────────

const globalTxIndexKey = "tx_sync_global_index"

func nextGlobalTxIndex() (int64, error) {
	v, err := store.GetJson[int64](GLOABL_STATE, globalTxIndexKey)
	if err != nil {
		return 0, err
	}
	var next int64
	if v == nil {
		next = 1
	} else {
		next = *v + 1
	}
	return next, store.SetJson(GLOABL_STATE, globalTxIndexKey, &next)
}

// ── Per-chain AsyncBatchState ──────────────────────────────

// hubSyncIndexKey returns the DB key for a given chain's sync state.
func hubSyncIndexKey(chainId uint32) string {
	return fmt.Sprintf("tx_sync_transaction_%d", chainId)
}

type AsyncBatchState struct {
	Going    int64
	Done     int64
	LastSync int64
}

// IsHubSyncRuning checks whether ANY chain has an in-flight sync batch.
// Used by epoch/PrepareTx for global coordination.
func IsHubSyncRuning() bool {
	for _, chain := range chains.Chains {
		if isHubSyncRuningForChain(chain.ChainId()) {
			return true
		}
	}
	return false
}

func isHubSyncRuningForChain(chainId uint32) bool {
	tx, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(chainId))
	if err != nil {
		LogWithTime("IsHubSyncRuning get error: " + err.Error())
		return false
	}
	if tx == nil {
		tx = &AsyncBatchState{Going: 0, Done: 0}
	}
	return tx.Going > tx.Done && time.Now().Unix()-tx.LastSync <= 360
}

// HubSyncStep1 creates a new SyncTxStart for the given chain if no batch is in progress.
// The returned txIndex is globally unique across all chains.
func HubSyncStep1(s *TeeChain, chainId uint32) ([]byte, error) {
	if s.dkg == nil {
		return nil, errors.New("HubSyncStep1: dkg is nil")
	}

	tx, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(chainId))
	if err != nil {
		return nil, err
	}
	if tx == nil {
		tx = &AsyncBatchState{Going: 0, Done: 0}
	}
	if tx.Going > tx.Done && time.Now().Unix()-tx.LastSync <= 360 {
		return nil, errors.New("sync step1 one transaction is running")
	}

	globalIdx, err := nextGlobalTxIndex()
	if err != nil {
		return nil, errors.Wrap(err, "nextGlobalTxIndex")
	}

	ntx := &model.SysCall{
		Payload: &model.SysCall_SyncTxStart{
			SyncTxStart: &model.SyncTxStart{
				TxIndex: globalIdx,
				ChainId: chainId,
			},
		},
	}
	return s.GetCallBytesFromNode(ntx)
}

// HubSyncStep2 records that a sync batch has started for the given chain.
func HubSyncStep2(i int64, chainId uint32, txn *store.Txn) error {
	tx, err := store.TxnGetJson[AsyncBatchState](txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)))
	if err != nil {
		return err
	}
	if tx == nil {
		tx = &AsyncBatchState{Going: 0, Done: 0}
	}
	if tx.Going > tx.Done && time.Now().Unix()-tx.LastSync <= 360 {
		// Allow overlapping start for robustness
	}
	tx.Going = i
	tx.Done = i - 1
	return store.TxnSetJson(txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)), tx)
}

// HubSyncEnd records that a sync batch has completed for the given chain.
func HubSyncEnd(i int64, chainId uint32, txn *store.Txn) error {
	tx, err := store.TxnGetJson[AsyncBatchState](txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)))
	if err != nil {
		return err
	}
	if tx == nil {
		return errors.Errorf("HubSyncEnd: no active sync for chain %d", chainId)
	}
	if i != tx.Going {
		return errors.Errorf("HubSyncEnd: txIndex mismatch for chain %d, expected %d, got %d", chainId, tx.Going, i)
	}
	tx.Done = tx.Going
	tx.LastSync = time.Now().Unix()
	return store.TxnSetJson(txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)), tx)
}

var lastRun = time.Now().Unix()

// 判断是否需要提交 tee-chain => main-chain 的交易
func (app *TeeChain) TargetSyncFromRelay() bool {
	if time.Now().Unix()-lastRun < 20 {
		fmt.Println(time.Now().Unix() - lastRun)
		return false
	}

	lastRun = time.Now().Unix()
	if IsHubSyncRuning() {
		fmt.Println("------------------------------------", "HubSyncRuning")
		return false
	}

	if app.dkg == nil || app.dkg.SystemPubKey() == nil {
		fmt.Println("------------------------------------", "DKG nil")
		return false
	}

	height := app.state.Height
	txn := dbstore.DBINS.NewTransaction()
	defer txn.Rollback()
	runtime := contracts.NewRuntimeWithVersion(height, time.Now(), txn, chaintypes.UniAddr{
		T: 0,
		V: []byte{},
	}, chaintypes.UniAddr{
		V: app.dkg.SystemPubKey().Byte(),
	}, module.NativeVersion)

	ins := gateway.NewGateway(runtime)
	list, _ := gateway.GatewayQuery{Gateway: *ins}.WithdrawTodo()
	if len(list) == 0 {
		fmt.Println("------------------------------------", "list 0")
		return false
	}

	signer := app.NodePriv.ToSigner(teecrypto.SuiteEd25519)
	ts := make([]*model.TeeCall, len(list))
	for i, l := range list {
		ts[i] = &model.TeeCall{
			ChainId: l.ChainID,
			Tx: &model.TeeCall_WithdrawApply{
				WithdrawApply: &model.WithdrawApply{
					User:     l.User.V,
					UserType: l.User.T,
					Contract: l.Contract,
					Credit:   l.Credit.Int.Uint64(),
					Nonce:    l.Nonce,
				},
			},
		}
	}

	// 提交交易到排队
	wtx, err := signTx(&model.SysCall{
		Payload: &model.SysCall_HubCall{HubCall: &model.HubCall{Call: ts}},
	}, signer)

	if err != nil {
		util.LogWithRed("RELAY-WORKER", "sign tx failed:", err)
		return false
	}

	if err := SubmitTx(wtx); err != nil {
		util.LogWithRed("RELAY-WORKER", "submit tx failed:", err)
		return false
	}

	// 更新处理指针
	end := list[len(list)-1].Nonce
	endbt, _ := cstore.ScaleEncode(end)
	tx, err := signTx(&model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: &model.ContractCall{
				Name:   []byte("gateway"),
				Method: cstore.MethodToSelectorBytes("UpdateWithdrawPointer"),
				Args:   [][]byte{endbt},
			},
		},
	}, signer)
	if err != nil {
		util.LogWithRed("RELAY-WORKER", "sign gateway tx failed:", err)
		return false
	}

	if err := SubmitTx(tx); err != nil {
		util.LogWithRed("RELAY-WORKER", "submit gateway tx failed:", err)
		return false
	}

	util.LogWithYellow("RELAY-WORKER", "submitted Withdraw tx to main chains")
	return true
}
