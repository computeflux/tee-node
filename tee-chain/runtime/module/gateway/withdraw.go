package gateway

import (
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

func (g GatewayMutation) ApplyWithdraw(chainID uint32, token []byte, providerID uint64, amount uint64, nonce []byte) error {
	caller := g.api.GetCaller()
	txn := g.api.GetTxn()

	chainIds := UserTypeToChainID[caller.T]
	if !slices.Contains(chainIds, chainID) {
		return errors.New("chainID not allowed for user type")
	}

	prov, err := g.providers.Get(txn, providerID)
	if err != nil || prov == nil {
		return ErrProviderNotFound
	}

	if caller.String() != prov.Owner.String() {
		return errors.New("User is not provider owner")
	}

	pb, err := g.providerCredits.Get(txn, providerID)
	if err != nil || pb == nil {
		return ErrProviderNotFound
	}

	if *pb < amount {
		return ErrLowCredit
	}

	*pb -= amount
	if err := g.providerCredits.Set(txn, providerID, *pb); err != nil {
		return err
	}

	nonceId, err := g.withdrawNonces.Len(txn)
	if err != nil {
		return fmt.Errorf("ApplyWithdraw: get nonceId: %w", err)
	}

	// 记录 WithdrawApply
	withdrawApply := WithdrawApply{
		ChainID:  chainID,
		Contract: token,
		User:     prov.Owner,
		Nonce:    nonceId,
		Credit:   store.NewU256(new(big.Int).SetUint64(amount)),
		Status:   WithdrawPending,
	}

	// 标记 nonce
	if _, err := g.withdrawNonces.Insert(txn, withdrawApply); err != nil {
		return fmt.Errorf("ApplyWithdraw: store nonce failed: %w", err)
	}

	if _, err := g.withdrawByUser.Insert(txn, caller, nonceId); err != nil {
		return fmt.Errorf("withdrawByUser: store failed: %w", err)
	}

	ins := batch.NewBatch(g.api)
	b := batch.BatchSecret{Batch: *ins}
	b.PackCall(0, chainID, nonceId)

	return err
}

// WithdrawalDone 标记提现已完成。args = [nonce]
func (g GatewayMutation) WithdrawalDone(nonce uint64) error {
	return g.updateWithdrawStatus(nonce, WithdrawDone)
}

// WithdrawalCancelled 标记提现已取消，并将扣除的 credits 加回 provider。args = [nonce]
func (g GatewayMutation) WithdrawalCancelled(nonce uint64) error {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		fmt.Println("WithdrawalCancelled must call by node")
		return fmt.Errorf("WithdrawalCancelled: caller is not node")
	}

	txn := g.api.GetTxn()
	wa, err := g.withdrawNonces.Get(txn, nonce)
	if err != nil {
		return fmt.Errorf("WithdrawalCancelled: get withdraw apply: %w", err)
	}
	if wa == nil {
		return fmt.Errorf("WithdrawalCancelled: withdraw apply not found")
	}
	if wa.Status != WithdrawPending {
		return fmt.Errorf("WithdrawalCancelled: withdraw status is not pending")
	}

	providerIDPtr, err := g.providerByOwner.Get(txn, wa.User)
	if err != nil || providerIDPtr == nil {
		return ErrProviderNotFound
	}
	providerID := *providerIDPtr

	pb, err := g.providerCredits.Get(txn, providerID)
	if err != nil || pb == nil {
		return ErrProviderNotFound
	}

	amount := wa.Credit.Int.Uint64()
	*pb += amount
	if err := g.providerCredits.Set(txn, providerID, *pb); err != nil {
		return fmt.Errorf("WithdrawalCancelled: restore provider credits: %w", err)
	}

	return g.updateWithdrawStatus(nonce, WithdrawCancelled)
}

func (g GatewayMutation) updateWithdrawStatus(nonce uint64, status WithdrawStatus) error {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		fmt.Println("UpdateWithdrawStatus must call by node")
		return fmt.Errorf("UpdateWithdrawStatus: caller is not node")
	}

	txn := g.api.GetTxn()
	if err := g.withdrawNonces.UpdateFn(txn, nonce, func(wa *WithdrawApply) {
		wa.Status = status
	}); err != nil {
		return fmt.Errorf("updateWithdrawStatus: store failed: %w", err)
	}
	return nil
}

// WithdrawalsByUser 查询用户的提现列表。args = [user, offset, limit]
func (g GatewayQuery) WithdrawalsByUser(user store.UniAddr, offset uint64, limit uint32) ([]WithdrawApply, error) {
	txn := g.api.GetTxn()

	_, nonceIds, err := g.withdrawByUser.List(txn, user, uint32(offset), limit)
	if err != nil {
		return nil, err
	}

	result := make([]WithdrawApply, 0, len(nonceIds))
	for _, nonceId := range nonceIds {
		wa, err := g.withdrawNonces.Get(txn, nonceId)
		if err != nil {
			return nil, err
		}
		if wa != nil {
			result = append(result, *wa)
		}
	}

	return result, nil
}

// WithdrawTodo 返回所有待处理的提现（status == pending）。
func (g GatewayQuery) Withdraw(nonce uint64) (*WithdrawApply, error) {
	txn := g.api.GetTxn()
	return g.withdrawNonces.Get(txn, nonce)
}
