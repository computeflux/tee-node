package gov

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovQuery_TotalSupply_Empty(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	q := GovQuery{Gov: *NewGov(rt)}
	supply, err := q.TotalSupply()
	require.NoError(t, err)
	require.Equal(t, 0, supply.Int.Sign())
}

func TestGovQuery_TotalSupply_AfterInit(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	_ = m.Init()

	q := GovQuery{Gov: m.Gov}
	supply, err := q.TotalSupply()
	require.NoError(t, err)
	require.Equal(t, 0, supply.Int.Sign())
}

func TestGovQuery_BalanceOf_NotMember(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	q := GovQuery{Gov: *NewGov(rt)}
	balance, err := q.BalanceOf(addr([]byte{1}))
	require.NoError(t, err)
	require.Equal(t, 0, balance.Int.Sign())
}

func TestGovQuery_BalanceOf_Success(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	q := GovQuery{Gov: m.Gov}
	balance, err := q.BalanceOf(addr([]byte{2}))
	require.NoError(t, err)
	require.Equal(t, 0, balance.Int.Cmp(big.NewInt(100)))
}

func TestGovMutation_Transfer_Disabled(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{3})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{1}), amount(big.NewInt(100)))
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	rt.caller = addr([]byte{1})
	err := m.Transfer(addr([]byte{2}), amount(big.NewInt(10)))
	require.ErrorIs(t, err, ErrTransferDisabled)
}

func TestGovMutation_Transfer_Success(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{3})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{1}), amount(big.NewInt(100)))
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	_ = m.transferEnabled.Set(rt.txn, true)

	rt.caller = addr([]byte{1})
	err := m.Transfer(addr([]byte{2}), amount(big.NewInt(10)))
	require.NoError(t, err)

	q := GovQuery{Gov: m.Gov}
	balance1, _ := q.BalanceOf(addr([]byte{1}))
	balance2, _ := q.BalanceOf(addr([]byte{2}))
	require.Equal(t, 0, balance1.Int.Cmp(big.NewInt(90)))
	require.Equal(t, 0, balance2.Int.Cmp(big.NewInt(110)))
}

func TestGovMutation_Transfer_LowBalance(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{3})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{1}), amount(big.NewInt(100)))
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	_ = m.transferEnabled.Set(rt.txn, true)

	rt.caller = addr([]byte{1})
	err := m.Transfer(addr([]byte{2}), amount(big.NewInt(200)))
	require.ErrorIs(t, err, ErrLowBalance)
}

func TestGovMutation_Transfer_MemberNotExisted(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{3})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{1}), amount(big.NewInt(100)))

	_ = m.transferEnabled.Set(rt.txn, true)

	rt.caller = addr([]byte{1})
	err := m.Transfer(addr([]byte{2}), amount(big.NewInt(10)))
	require.ErrorIs(t, err, ErrMemberNotExisted)
}
