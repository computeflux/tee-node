package gateway

import (
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
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

	nonceId, err := g.nonce.GetOrDefault(txn, 0)
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
	if err := g.withdrawNonces.Set(txn, nonceId, withdrawApply); err != nil {
		return fmt.Errorf("ApplyWithdraw: store nonce failed: %w", err)
	}

	// 记录到用户提现列表（用于用户查询）
	if _, err := g.withdrawByUser.Insert(txn, prov.Owner, nonceId); err != nil {
		return fmt.Errorf("ApplyWithdraw: store user withdraw failed: %w", err)
	}

	// 为下一个数据创建id
	err = g.nonce.Set(txn, nonceId+1)
	if err != nil {
		return fmt.Errorf("ApplyWithdraw: get nonceId: %w", err)
	}

	return nil
}

// WithdrawalDone 标记提现已完成。args = [nonce]
func (g GatewayMutation) WithdrawalDone(nonce uint64) error {
	return g.updateWithdrawStatus(nonce, WithdrawDone)
}

// WithdrawalCancelled 标记提现已取消。args = [nonce]
func (g GatewayMutation) WithdrawalCancelled(nonce uint64) error {
	return g.updateWithdrawStatus(nonce, WithdrawCancelled)
}

func (g GatewayMutation) updateWithdrawStatus(nonce uint64, status WithdrawStatus) error {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		fmt.Println("UpdateWithdrawStatus must call by node")
		return fmt.Errorf("UpdateWithdrawStatus: caller is not node")
	}

	txn := g.api.GetTxn()

	wa, err := g.withdrawNonces.Get(txn, nonce)
	if err != nil || wa == nil {
		return fmt.Errorf("updateWithdrawStatus: withdraw not found: %w", err)
	}

	wa.Status = status
	if err := g.withdrawNonces.Set(txn, nonce, *wa); err != nil {
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

func (g GatewayMutation) UpdateWithdrawPointer(end uint64) error {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		fmt.Println("UpdateWithdrawPointer must call by node")
		return fmt.Errorf("UpdateWithdrawPointer: caller is not node")
	}

	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX UpdateWithdrawPointer", end)
	return g.currNonce.Set(g.api.GetTxn(), end)
}

// WithdrawTodo 返回所有待处理的提现（status == pending）。
func (g GatewayQuery) WithdrawTodo() ([]WithdrawApply, error) {
	txn := g.api.GetTxn()
	curr, err := g.currNonce.Get(txn)
	var start uint64 = 0
	if err != nil {
		return nil, nil
	}

	calls := []WithdrawApply{}
	// 如果没有任何指针的时候需要将最开始的数据放进去
	if curr == nil {
		fmt.Println("1111111111111111111111111111111111111111111111111111")
		call, err := g.withdrawNonces.Get(txn, 0)
		if err != nil {
			return nil, nil
		}

		if call != nil {
			calls = append(calls, *call)
		}
	} else {
		start = *curr
	}

	nonceId, err := g.nonce.GetOrDefault(txn, 0)
	if err != nil {
		return nil, nil
	}

	fmt.Println("start", start+1, "end", nonceId-1)
	for i := start + 1; i < nonceId; i++ {
		call, err := g.withdrawNonces.Get(txn, uint64(i))
		if err != nil {
			return nil, nil
		}
		if call != nil {
			call.Nonce = uint64(i)
			fmt.Println("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbb ", call.Nonce)
			calls = append(calls, *call)
		}
	}

	return calls, nil
}
