package gov

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wetee-dao/ink.go/util"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

func addr(b []byte) model.UniAddr {
	return model.UniAddr{T: 0, V: b}
}

func amount(i *big.Int) model.Amount {
	return model.Amount{Int: i}
}

func TestGovQuery_Members(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	d := GovQuery{Gov: *NewGov(rt)}
	members, err := d.Members()
	require.NoError(t, err)
	require.Empty(t, members)
}

func TestGovQuery_PublicJoin_Default(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	d := GovQuery{Gov: *NewGov(rt)}
	enabled, err := d.GetPublicJoin()
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestGovMutation_Init_Basic(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	err := m.Init()
	require.NoError(t, err)

	enabled, err := GovQuery{Gov: m.Gov}.GetPublicJoin()
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestGovMutation_Init_WithDefaultTrack(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	err := m.Init()
	require.NoError(t, err)

	dt, err := GovQuery{Gov: m.Gov}.DefaultTrack()
	require.NoError(t, err)
	require.Equal(t, uint32(0), dt)

	tr, err := GovQuery{Gov: m.Gov}.Track(0)
	require.NoError(t, err)
	require.True(t, tr.IsSome())
}

func TestGovMutation_PublicJoin_NotAllowed(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	rt.caller = addr([]byte{1})
	err := m.PublicJoin()
	require.ErrorIs(t, err, ErrPublicJoinNotAllowed)
}

func TestGovMutation_SetPublicJoin_NotGov(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	rt.caller = addr([]byte{1})
	err := m.SetPublicJoin(true)
	require.ErrorIs(t, err, ErrMustCallByGov)
}

func TestGovMutation_SetPublicJoin_ByGov(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	err := m.SetPublicJoin(true)
	require.NoError(t, err)

	enabled, err := GovQuery{Gov: m.Gov}.GetPublicJoin()
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestGovMutation_Join_NotGov(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	rt.caller = addr([]byte{1})
	err := m.Join(addr([]byte{2}), amount(big.NewInt(100)))
	require.ErrorIs(t, err, ErrMustCallByGov)
}

func TestGovMutation_Join_ByGov(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	err := m.Join(addr([]byte{2}), amount(big.NewInt(100)))
	require.NoError(t, err)

	member, err := m.members.GetOrDefault(rt.txn, addr([]byte{2}), model.ZeroAmount)
	require.NoError(t, err)
	require.Equal(t, 0, member.Int.Cmp(big.NewInt(100)))
}

func TestGovMutation_Join_MemberExisted(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	err := m.Join(addr([]byte{2}), amount(big.NewInt(50)))
	require.ErrorIs(t, err, ErrMemberExisted)
}

func TestGovMutation_Leave_NotMember(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	rt.caller = addr([]byte{1})
	err := m.Leave()
	require.ErrorIs(t, err, ErrMemberNotExisted)
}

func TestGovMutation_Leave_BalanceNotZero(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	rt.caller = addr([]byte{2})
	err := m.Leave()
	require.ErrorIs(t, err, ErrMemberBalanceNotZero)
}

func TestGovMutation_Leave_Success(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{2}), model.ZeroAmount)

	rt.caller = addr([]byte{2})
	err := m.Leave()
	require.NoError(t, err)

	member, err := m.members.Get(rt.txn, addr([]byte{2}))
	require.NoError(t, err)
	require.Nil(t, member)
}

func TestGovMutation_PublicJoin_Success(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.SetPublicJoin(true)

	rt.caller = addr([]byte{2})
	err := m.PublicJoin()
	require.NoError(t, err)

	member, err := m.members.Get(rt.txn, addr([]byte{2}))
	require.NoError(t, err)
	require.NotNil(t, member)
}

func TestGovMutation_Delete_NotMember(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	err := m.Delete(addr([]byte{2}))
	require.ErrorIs(t, err, ErrMemberNotExisted)
}

func TestGovMutation_Delete_ByGov(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	err := m.Delete(addr([]byte{2}))
	require.NoError(t, err)

	member, err := m.members.Get(rt.txn, addr([]byte{2}))
	require.NoError(t, err)
	require.Nil(t, member)

	supply, err := GovQuery{Gov: m.Gov}.TotalSupply()
	require.NoError(t, err)
	require.Equal(t, 0, supply.Int.Sign())
}

func TestGovMutation_LeaveWithBurn_Success(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	sudo := addr([]byte{1})
	_ = m.Init()
	rt.sudoAccount = sudo

	rt.caller = sudo
	_ = m.Join(addr([]byte{2}), amount(big.NewInt(100)))

	rt.caller = addr([]byte{2})
	err := m.LeaveWithBurn()
	require.NoError(t, err)

	member, err := m.members.Get(rt.txn, addr([]byte{2}))
	require.NoError(t, err)
	require.Nil(t, member)

	supply, err := GovQuery{Gov: m.Gov}.TotalSupply()
	require.NoError(t, err)
	require.Equal(t, 0, supply.Int.Sign())
}

func TestGovMutation_SubmitProposal_Multiple(t *testing.T) {
	rt, cleanup := setupTestDB(t)
	defer cleanup()

	m := GovMutation{Gov: *NewGov(rt)}
	_ = m.Init()

	// 设置 sudo 账户并加入成员
	sudo := addr([]byte{0xff})
	rt.sudoAccount = sudo
	rt.caller = sudo
	err := m.Join(addr([]byte{1}), amount(big.NewInt(1000)))
	require.NoError(t, err)

	// 使用普通成员提交提案
	rt.caller = addr([]byte{1})

	// 第一次提交提案
	call1 := CallContent{
		Contract: []byte("test1"),
		Selector: [4]byte{0x01, 0x02, 0x03, 0x04},
		Amount:   model.ZeroAmount,
	}
	err = m.SubmitProposal(call1, 0)
	require.NoError(t, err)

	// 检查提案数量
	q := GovQuery{Gov: m.Gov}
	proposals, err := q.Proposals(util.NewNone[uint32](), 10)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	require.Equal(t, uint32(0), proposals[0].ID)

	// 第二次提交提案
	call2 := CallContent{
		Contract: []byte("test2"),
		Selector: [4]byte{0x05, 0x06, 0x07, 0x08},
		Amount:   model.ZeroAmount,
	}
	err = m.SubmitProposal(call2, 0)
	require.NoError(t, err)

	// 检查提案数量
	proposals, err = q.Proposals(util.NewNone[uint32](), 10)
	require.NoError(t, err)
	require.Len(t, proposals, 2)

	// 验证两个提案的 ID 不同
	require.Equal(t, uint32(0), proposals[1].ID) // 第一个提案
	require.Equal(t, uint32(1), proposals[0].ID) // 第二个提案 (DescList 倒序返回)

	// 验证提案内容未被覆盖
	prop0, err := q.Proposal(0)
	require.NoError(t, err)
	require.True(t, prop0.IsSome())
	call0Val, _ := prop0.V.Proposal.Call.UnWrap()
	require.Equal(t, []byte("test1"), call0Val.Contract)

	prop1, err := q.Proposal(1)
	require.NoError(t, err)
	require.True(t, prop1.IsSome())
	call1Val, _ := prop1.V.Proposal.Call.UnWrap()
	require.Equal(t, []byte("test2"), call1Val.Contract)
}
