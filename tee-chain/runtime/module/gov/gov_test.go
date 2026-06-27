package gov

import (
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGov() (*Gov, *testutil.MockRuntime) {
	rt := testutil.NewMockRuntime()
	return NewGov(rt), rt
}

func asGov(rt *testutil.MockRuntime) {
	rt.System = true
	rt.Caller = rt.Sudo
}

// ── Init ─────────────────────────────────────────────────

func TestGov_Init_Success(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)

	err := GovMutation{*gov}.Init()
	require.NoError(t, err)

	q := GovQuery{*gov}

	// Check public join is enabled
	pj, err := q.GetPublicJoin()
	require.NoError(t, err)
	assert.True(t, pj)

	// Check total supply is zero
	supply, err := q.TotalSupply()
	require.NoError(t, err)
	assert.Equal(t, int64(0), supply.Int.Int64())

	// Check default track exists
	dt, err := q.DefaultTrack()
	require.NoError(t, err)
	assert.Equal(t, uint32(0), dt)

	// Check tracks list
	tracks, err := q.Tracks()
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "default", tracks[0].Track.Name)
}

// ── Member Management ────────────────────────────────────

func TestGov_Join_Success(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	err := GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(1000)))
	require.NoError(t, err)

	bal, err := GovQuery{*gov}.BalanceOf(user)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), bal.Int.Int64())

	supply, err := GovQuery{*gov}.TotalSupply()
	require.NoError(t, err)
	assert.Equal(t, int64(1000), supply.Int.Int64())
}

func TestGov_Join_Duplicate(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(1000))))

	err := GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(500)))
	assert.ErrorIs(t, err, ErrMemberExisted)
}

func TestGov_PublicJoin_Success(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	// PublicJoin as caller
	rt.System = false
	err := GovMutation{*gov}.PublicJoin()
	require.NoError(t, err)

	members, err := GovQuery{*gov}.Members()
	require.NoError(t, err)
	assert.Len(t, members, 1)
}

func TestGov_PublicJoin_Disabled(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())
	require.NoError(t, GovMutation{*gov}.SetPublicJoin(false))

	rt.System = false
	err := GovMutation{*gov}.PublicJoin()
	assert.ErrorIs(t, err, ErrPublicJoinNotAllowed)
}

func TestGov_Leave_Success(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(0))))

	rt.Caller = user
	rt.System = false
	err := GovMutation{*gov}.Leave()
	require.NoError(t, err)

	bal, err := GovQuery{*gov}.BalanceOf(user)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.Int.Int64())
}

func TestGov_Leave_NonZeroBalance(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(1000))))

	rt.Caller = user
	rt.System = false
	err := GovMutation{*gov}.Leave()
	assert.ErrorIs(t, err, ErrMemberBalanceNotZero)
}

func TestGov_LeaveWithBurn(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(1000))))

	supplyBefore, _ := GovQuery{*gov}.TotalSupply()

	rt.Caller = user
	rt.System = false
	err := GovMutation{*gov}.LeaveWithBurn()
	require.NoError(t, err)

	supplyAfter, _ := GovQuery{*gov}.TotalSupply()
	assert.Equal(t, int64(0), supplyAfter.Int.Int64())
	assert.Equal(t, int64(1000), supplyBefore.Int.Int64())
}

func TestGov_DeleteMember(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(1000))))

	err := GovMutation{*gov}.Delete(user)
	require.NoError(t, err)

	bal, err := GovQuery{*gov}.BalanceOf(user)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.Int.Int64())
}

// ── Token Operations ─────────────────────────────────────

func TestGov_Mint(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(0))))

	err := GovMutation{*gov}.Mint(user, store.NewU256(big.NewInt(5000)))
	require.NoError(t, err)

	bal, err := GovQuery{*gov}.BalanceOf(user)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), bal.Int.Int64())

	supply, err := GovQuery{*gov}.TotalSupply()
	require.NoError(t, err)
	assert.Equal(t, int64(5000), supply.Int.Int64())
}

func TestGov_Mint_RequiresGov(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = false
	rt.Caller = store.UniAddr{T: 1, V: []byte{0xFF}} // non-sudo caller
	err := GovMutation{*gov}.Mint(user, store.NewU256(big.NewInt(5000)))
	assert.ErrorIs(t, err, ErrMustCallByGov)
}

func TestGov_Transfer_Disabled(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	alice := store.UniAddr{T: 1, V: []byte{0xAA}}
	bob := store.UniAddr{T: 1, V: []byte{0xBB}}
	require.NoError(t, GovMutation{*gov}.Join(alice, store.NewU256(big.NewInt(1000))))
	require.NoError(t, GovMutation{*gov}.Join(bob, store.NewU256(big.NewInt(0))))

	rt.Caller = alice
	rt.System = false
	err := GovMutation{*gov}.Transfer(bob, store.NewU256(big.NewInt(300)))
	assert.ErrorIs(t, err, ErrTransferDisabled)
}

func TestGov_TransferFrom_Disabled(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	alice := store.UniAddr{T: 1, V: []byte{0xAA}}
	bob := store.UniAddr{T: 1, V: []byte{0xBB}}
	charlie := store.UniAddr{T: 1, V: []byte{0xCC}}
	require.NoError(t, GovMutation{*gov}.Join(alice, store.NewU256(big.NewInt(1000))))
	require.NoError(t, GovMutation{*gov}.Join(bob, store.NewU256(big.NewInt(0))))

	rt.Caller = charlie
	rt.System = false
	err := GovMutation{*gov}.TransferFrom(alice, bob, store.NewU256(big.NewInt(300)))
	assert.ErrorIs(t, err, ErrTransferDisabled)
}

// ── Track Management ─────────────────────────────────────

func TestGov_AddTrack(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	track := TrackData{
		Name:           "fast",
		PreparePeriod:  10,
		MaxDeciding:    50,
		ConfirmPeriod:  20,
		DecisionPeriod: 30,
		MinApproval:    NewLinearDecreasingCurve(10000, 50, 100),
		MinSupport:     NewLinearDecreasingCurve(10000, 50, 100),
	}
	err := GovMutation{*gov}.AddTrack(track)
	require.NoError(t, err)

	tracks, err := GovQuery{*gov}.Tracks()
	require.NoError(t, err)
	assert.Len(t, tracks, 2)

	// Check track with ID 1
	opt, err := GovQuery{*gov}.Track(1)
	require.NoError(t, err)
	require.True(t, opt.IsSome())
	tr, _ := opt.UnWrap()
	assert.Equal(t, "fast", tr.Name)
}

func TestGov_AddTrack_RequiresGov(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	rt.System = false
	rt.Caller = store.UniAddr{T: 1, V: []byte{0xFF}} // non-sudo caller
	err := GovMutation{*gov}.AddTrack(TrackData{Name: "fast"})
	assert.ErrorIs(t, err, ErrMustCallByGov)
}

func TestGov_SetDefaultTrack(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	track := TrackData{Name: "fast", MinApproval: NewLinearDecreasingCurve(10000, 50, 100), MinSupport: NewLinearDecreasingCurve(10000, 50, 100)}
	require.NoError(t, GovMutation{*gov}.AddTrack(track))

	err := GovMutation{*gov}.SetDefaultTrack(1)
	require.NoError(t, err)

	dt, err := GovQuery{*gov}.DefaultTrack()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), dt)
}

func TestGov_SetDefaultTrack_InvalidTrack(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	err := GovMutation{*gov}.SetDefaultTrack(99)
	assert.ErrorIs(t, err, ErrNoTrack)
}

// ── Proposal ─────────────────────────────────────────────

func TestGov_SubmitProposal_Success(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	caller := store.UniAddr{T: 1, V: []byte{0xAA}}
	require.NoError(t, GovMutation{*gov}.Join(caller, store.NewU256(big.NewInt(10000))))

	rt.Caller = caller
	rt.System = false
	err := GovMutation{*gov}.SubmitProposal(CallContent{
		Contract: []byte("test"),
		Selector: [4]byte{0x01, 0x02, 0x03, 0x04},
		Args:     [][]byte{},
		Amount:   store.NewU256(big.NewInt(100)),
	}, 0)
	require.NoError(t, err)

	// Check proposal exists
	propOpt, err := GovQuery{*gov}.Proposal(0)
	require.NoError(t, err)
	require.True(t, propOpt.IsSome())
	prop, _ := propOpt.UnWrap()
	assert.Equal(t, uint32(0), prop.Proposal.TrackID)
	assert.Equal(t, ProposalStatusPending, prop.Proposal.Status.State)
}

func TestGov_SubmitProposal_NotMember(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	caller := store.UniAddr{T: 1, V: []byte{0xAA}}
	rt.Caller = caller
	rt.System = false
	err := GovMutation{*gov}.SubmitProposal(CallContent{
		Contract: []byte("test"),
		Selector: [4]byte{0x01, 0x02, 0x03, 0x04},
		Args:     [][]byte{},
		Amount:   store.NewU256(big.NewInt(100)),
	}, 0)
	assert.ErrorIs(t, err, ErrMemberNotExisted)
}

func TestGov_SubmitProposal_InvalidTrack(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	caller := store.UniAddr{T: 1, V: []byte{0xAA}}
	require.NoError(t, GovMutation{*gov}.Join(caller, store.NewU256(big.NewInt(10000))))

	rt.Caller = caller
	rt.System = false
	err := GovMutation{*gov}.SubmitProposal(CallContent{
		Contract: []byte("test"),
		Selector: [4]byte{0x01, 0x02, 0x03, 0x04},
		Args:     [][]byte{},
		Amount:   store.NewU256(big.NewInt(100)),
	}, 99)
	assert.ErrorIs(t, err, ErrNoTrack)
}
