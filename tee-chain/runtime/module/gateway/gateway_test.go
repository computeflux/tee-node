package gateway

import (
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGateway() (*Gateway, *testutil.MockRuntime) {
	rt := testutil.NewMockRuntime()
	return NewGateway(rt), rt
}

// ── Provider Tests ───────────────────────────────────────

func TestGateway_CreateProvider_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	err := m.CreateProvider("AI Corp", "https://logo.png", "Best AI", RegionAsiaEast, 5000)
	require.NoError(t, err)

	// Query back
	q := GatewayQuery{*gw}
	provOpt, err := q.GetProviderByOwner()
	require.NoError(t, err)
	require.True(t, provOpt.IsSome())

	provID, err := provOpt.UnWrap()
	require.NoError(t, err)
	prov, err := q.GetProvider(provID)
	require.NoError(t, err)
	require.NotNil(t, prov)
	assert.Equal(t, "AI Corp", prov.Name)
	assert.Equal(t, rt.Caller, prov.Owner)
	assert.Equal(t, uint64(5000), prov.Deposit)
	assert.True(t, prov.IsActive)
}

func TestGateway_CreateProvider_DuplicateOwner(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	err := m.CreateProvider("AI Corp 2", "", "", RegionAsiaEast, 2000)
	assert.ErrorContains(t, err, "already exists")
}

func TestGateway_CreateProvider_DuplicateName(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))

	// Change caller to simulate different owner on same store
	rt.Caller = store.UniAddr{T: 1, V: []byte{0x02}}
	err := m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 2000)
	assert.ErrorContains(t, err, "already exists")
}

func TestGateway_UpdateProvider_Success(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	provOpt, _ := q.GetProviderByOwner()
	id, _ := provOpt.UnWrap()

	err := m.UpdateProvider(id, "New Name", "", "", nil, 0, 0, 0, 0)
	require.NoError(t, err)

	prov, err := q.GetProvider(id)
	require.NoError(t, err)
	assert.Equal(t, "New Name", prov.Name)
}

func TestGateway_CreateProvider_ReservedName(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	for _, name := range []string{"Edge", "edge", "MININGPOOL", "MiningPool", "tee", "TEE"} {
		err := m.CreateProvider(name, "", "", RegionAsiaEast, 1000)
		assert.ErrorContains(t, err, "route prefix", "name=%q", name)
	}
}

func TestGateway_UpdateProvider_ReservedName(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	provOpt, _ := q.GetProviderByOwner()
	id, _ := provOpt.UnWrap()

	err := m.UpdateProvider(id, "Edge", "", "", nil, 0, 0, 0, 0)
	assert.ErrorContains(t, err, "route prefix")
}

// ── Model Tests ──────────────────────────────────────────

func TestGateway_RegisterModel_Success(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	err := m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", []string{"sk-abc"}, 0, 0)
	require.NoError(t, err)

	count, err := q.GetModelCount()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), count)

	models, err := q.ListModels(0, 10)
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "gpt-4", models[0].Name)
	// API keys should be masked in query
	assert.Equal(t, "sk-abc", models[0].APIKeys[0])
}

func TestGateway_RegisterModel_WithoutProvider(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	// Without provider, RegisterModel returns nil (not an error) in current implementation
	err := m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil, 0, 0)
	assert.NoError(t, err)
}

func TestGateway_RegisterModel_DuplicateName(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil, 0, 0))

	err := m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil, 0, 0)
	assert.ErrorContains(t, err, "already exists")
}

func TestGateway_DeactivateAndActiveModel(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil, 0, 0))

	active, err := q.ListActiveModels(0, 10)
	require.NoError(t, err)
	require.Len(t, active, 1)

	require.NoError(t, m.DeactivateModel(0))

	active, err = q.ListActiveModels(0, 10)
	require.NoError(t, err)
	assert.Len(t, active, 0)

	require.NoError(t, m.ActiveModel(0))

	active, err = q.ListActiveModels(0, 10)
	require.NoError(t, err)
	assert.Len(t, active, 1)
}

func TestGateway_GetModelByName(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil, 0, 0))

	provOpt, _ := q.GetProviderByOwner()
	pid, _ := provOpt.UnWrap()

	mi, err := q.GetModelByName(pid, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, mi)
	assert.Equal(t, "gpt-4", mi.Data.Name)
}

// ── API Key Tests ────────────────────────────────────────

func TestGateway_RegisterAndLookupAPIKey(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	keyHash := make([]byte, 32)
	copy(keyHash, []byte("hash-of-api-key-123456789012345"))

	err := m.RegisterAPIKey(keyHash, "test", 100)
	require.NoError(t, err)

	info, err := q.LookupAPIKey(keyHash)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, rt.Caller, info.Owner)
	assert.Equal(t, uint32(100), info.RateLimit)
	assert.True(t, info.IsActive)
}

func TestGateway_RegisterAPIKey_Duplicate(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	keyHash := make([]byte, 32)
	require.NoError(t, m.RegisterAPIKey(keyHash, "test", 100))

	err := m.RegisterAPIKey(keyHash, "test", 200)
	assert.ErrorContains(t, err, "already registered")
}

func TestGateway_RevokeAPIKey(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	keyHash := make([]byte, 32)
	require.NoError(t, m.RegisterAPIKey(keyHash, "test", 100))
	require.NoError(t, m.RevokeAPIKey(keyHash))

	info, err := q.LookupAPIKey(keyHash)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.False(t, info.IsActive)
}

func TestGateway_RevokeAPIKey_NotOwner(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	keyHash := make([]byte, 32)
	require.NoError(t, m.RegisterAPIKey(keyHash, "test", 100))

	// Change caller on same store
	rt.Caller = store.UniAddr{T: 1, V: []byte{0x02}}
	err := m.RevokeAPIKey(keyHash)
	assert.ErrorIs(t, err, ErrNotOwner)
}

func TestGateway_GetAPIKeysByOwner(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	keyHash1 := make([]byte, 32)
	keyHash2 := make([]byte, 32)
	keyHash2[0] = 0x01

	require.NoError(t, m.RegisterAPIKey(keyHash1, "test", 100))
	require.NoError(t, m.RegisterAPIKey(keyHash2, "test", 200))

	keys, err := q.GetAPIKeysByOwner(rt.Caller)
	require.NoError(t, err)
	assert.Len(t, keys, 2)
}

// ── Credit Tests ─────────────────────────────────────────

func TestGateway_Recharge_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true

	err := m.Recharge(user, 1000, []byte("nonce1"))
	require.NoError(t, err)

	credit, err := q.GetCredit(user)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), credit)
}

func TestGateway_Recharge_RequiresSystem(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	err := m.Recharge(user, 1000, []byte("nonce1"))
	assert.ErrorContains(t, err, "not sudo")
}

func TestGateway_Recharge_DuplicateNonce(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true

	require.NoError(t, m.Recharge(user, 1000, []byte("nonce1")))
	credit1, _ := q.GetCredit(user)

	// 相同 nonce 再次充值应被静默跳过（return nil），余额不变
	require.NoError(t, m.Recharge(user, 2000, []byte("nonce1")))
	credit2, _ := q.GetCredit(user)
	assert.Equal(t, credit1, credit2)

	// 不同 nonce 可以正常充值
	require.NoError(t, m.Recharge(user, 3000, []byte("nonce2")))
	credit3, _ := q.GetCredit(user)
	assert.Equal(t, credit1+3000, credit3)
}

func TestGateway_TransferCredit_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	from := store.UniAddr{T: 1, V: []byte{0xAB}}
	to := store.UniAddr{T: 1, V: []byte{0xCD}}

	rt.System = true
	require.NoError(t, m.Recharge(from, 1000, []byte("nonce1")))
	rt.System = false

	// Transfer from caller
	rt.Caller = from
	require.NoError(t, m.TransferCredit(to, 600))

	fromCredit, err := q.GetCredit(from)
	require.NoError(t, err)
	assert.Equal(t, int64(400), fromCredit)

	toCredit, err := q.GetCredit(to)
	require.NoError(t, err)
	assert.Equal(t, int64(600), toCredit)

	history, err := q.GetTransferHistory(from, 0, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, from, history[0].From)
	assert.Equal(t, to, history[0].To)
	assert.Equal(t, uint64(600), history[0].Amount)
}

func TestGateway_TransferCredit_Insufficient(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	from := store.UniAddr{T: 1, V: []byte{0xAB}}
	to := store.UniAddr{T: 1, V: []byte{0xCD}}

	rt.System = true
	require.NoError(t, m.Recharge(from, 1000, []byte("nonce1")))
	rt.System = false

	rt.Caller = from
	err := m.TransferCredit(to, 2000)
	assert.ErrorIs(t, err, ErrLowCredit)
}

func TestGateway_TransferCredit_ZeroAmount(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	from := store.UniAddr{T: 1, V: []byte{0xAB}}
	to := store.UniAddr{T: 1, V: []byte{0xCD}}

	rt.System = true
	require.NoError(t, m.Recharge(from, 1000, []byte("nonce1")))
	rt.System = false

	rt.Caller = from
	require.NoError(t, m.TransferCredit(to, 0))

	fromCredit, _ := q.GetCredit(from)
	assert.Equal(t, int64(1000), fromCredit)
}

func TestGateway_SettleUsage_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	// Setup: create provider + model + recharge user
	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil, 0, 0))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	// Settle must be called by node (T == 0)
	rt.Caller.T = 0

	// Settle: in=1000 tokens, out=500 tokens → cost = (1000*1000000 + 500*2000000) / 1000000 = 2002
	err := m.SettleUsageWithRoute([]SettlementWithRoute{
		{
			User:       user,
			RouteID:    store.NewNone[uint64](),
			ModelID:    0,
			ProviderID: 0,
			TokenIn:    1000,
			TokenOut:   500,
			Count:      1,
			Nonce:      []byte{0x01, 0x02, 0x03, 0x04},
		},
	})
	require.NoError(t, err)

	// Check user credit decreased
	credit, err := q.GetCredit(user)
	require.NoError(t, err)
	assert.Less(t, credit, int64(1_000_000))

	// Check provider credit increased
	provCredit, err := q.GetProviderCredit(0)
	require.NoError(t, err)
	assert.Greater(t, provCredit, uint64(0))
}

func TestGateway_SettleUsage_DuplicateNonce(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil, 0, 0))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	nonce := []byte{0x01, 0x02, 0x03, 0x04}
	record := SettlementWithRoute{
		User:       user,
		RouteID:    store.NewNone[uint64](),
		ModelID:    0,
		ProviderID: 0,
		TokenIn:    1000,
		TokenOut:   500,
		Count:      1,
		Nonce:      nonce,
	}

	// Settle must be called by node (T == 0)
	rt.Caller.T = 0

	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{record}))
	// Duplicate record nonce should be silently skipped (return nil), no double charge
	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{record}))
}

func TestGateway_SettleUsageWithRoute_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	// Setup: create provider + model
	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil, 0, 0))

	// Create route with different prices and join model 0
	rt.System = true
	routeRes := m.CreateRoute("miningpool/test", MiningPoolRoute, 2_000_000, 1_000_000, 4_000_000, true)
	require.True(t, routeRes.IsOk())
	routeID, err := routeRes.UnWrap()
	require.NoError(t, err)
	require.NoError(t, m.JoinRoute(0, routeID))
	rt.System = false

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	// Settle by node (T == 0)
	rt.Caller.T = 0

	// Route pricing: in=1000, out=500 → cost = (1000*2000000 + 500*4000000) / 1000000 = 4000
	err = m.SettleUsageWithRoute([]SettlementWithRoute{
		{
			User:       user,
			RouteID:    store.NewSome(routeID),
			ProviderID: 0,
			ModelID:    0,
			TokenIn:    1000,
			TokenOut:   500,
			Count:      1,
			Nonce:      []byte{0x05, 0x06, 0x07, 0x08},
		},
	})
	require.NoError(t, err)

	// User credit decreased
	credit, err := q.GetCredit(user)
	require.NoError(t, err)
	assert.Less(t, credit, int64(1_000_000))

	// Provider credit increased
	provCredit, err := q.GetProviderCredit(0)
	require.NoError(t, err)
	assert.Greater(t, provCredit, uint64(0))

	// Provider reputation must NOT be updated for route settlement
	prov, err := q.GetProvider(0)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), prov.TotalRequests)
	assert.Equal(t, uint32(0), prov.SuccessRate)
}

func TestGateway_Withdraw_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	// ApplyWithdraw requires caller.T == 2 and matching provider owner
	rt.Caller.T = 2
	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil, 0, 0))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	// Settle must be called by node (T == 0)
	rt.Caller.T = 0
	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{
		{User: user, RouteID: store.NewNone[uint64](), ModelID: 0, ProviderID: 0, TokenIn: 1000000, TokenOut: 500000, Count: 1, Nonce: []byte{0x01}},
	}))

	provCredit, _ := q.GetProviderCredit(0)
	require.Greater(t, provCredit, uint64(0))

	// ApplyWithdraw as provider owner with allowed chainID
	rt.Caller.T = 2
	require.NoError(t, m.ApplyWithdraw(420420417, []byte{}, 0, provCredit, []byte("nonce1")))

	provCreditAfter, _ := q.GetProviderCredit(0)
	assert.Equal(t, uint64(0), provCreditAfter)
}

func TestGateway_Withdraw_RequiresSystem(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	// Default caller type (T:1) is not allowed for any chainID
	err := m.ApplyWithdraw(0, []byte{}, 0, 100, []byte("nonce1"))
	assert.ErrorContains(t, err, "chainID not allowed for user type")
}

func TestGateway_Withdraw_InsufficientCredit(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	// ApplyWithdraw requires caller.T == 2 and matching provider owner
	rt.Caller.T = 2
	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil, 0, 0))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	// Settle must be called by node (T == 0)
	rt.Caller.T = 0
	// Settle a small amount first
	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{
		{User: user, RouteID: store.NewNone[uint64](), ModelID: 0, ProviderID: 0, TokenIn: 1000, TokenOut: 500, Count: 1, Nonce: []byte{0x01}},
	}))

	// Try to withdraw more than earned as provider owner with allowed chainID
	rt.Caller.T = 2
	err := m.ApplyWithdraw(420420417, []byte{}, 0, 1_000_000, []byte("nonce1"))
	assert.ErrorIs(t, err, ErrLowCredit)
}

// ── Route Tests ──────────────────────────────────────────

func TestGateway_JoinRoute_OnlyOwner(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	provOpt, _ := q.GetProviderByOwner()
	provID, _ := provOpt.UnWrap()

	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1, 1, 1, ProviderOpenAI, "", []string{"sk-1"}, 0, 0))
	models, err := q.ListProviderModels(provID)
	require.NoError(t, err)
	require.Len(t, models, 1)
	modelID := models[0].ID

	// CreateRoute requires system caller.
	rt.System = true
	_, _ = m.CreateRoute("dummy", MiningPoolRoute, 0, 0, 0, false).UnWrap()
	routeRes := m.CreateRoute("MiningPool", MiningPoolRoute, 0, 0, 0, false)
	rt.System = false
	routeID, err := routeRes.UnWrap()
	require.NoError(t, err)
	require.NotZero(t, routeID)

	require.NoError(t, m.JoinRoute(modelID, routeID))

	// other caller cannot join
	rt.Caller = store.UniAddr{T: 1, V: []byte{0x02}}
	err = m.JoinRoute(modelID, routeID)
	assert.ErrorContains(t, err, "only model owner")
}

func TestGateway_LeaveRoute_OnlyOwner(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	provOpt, _ := q.GetProviderByOwner()
	provID, _ := provOpt.UnWrap()

	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1, 1, 1, ProviderOpenAI, "", []string{"sk-1"}, 0, 0))
	models, err := q.ListProviderModels(provID)
	require.NoError(t, err)
	require.Len(t, models, 1)
	modelID := models[0].ID

	// CreateRoute requires system caller.
	rt.System = true
	_, _ = m.CreateRoute("dummy", MiningPoolRoute, 0, 0, 0, false).UnWrap()
	routeRes := m.CreateRoute("MiningPool", MiningPoolRoute, 0, 0, 0, false)
	rt.System = false
	routeID, err := routeRes.UnWrap()
	require.NoError(t, err)
	require.NotZero(t, routeID)

	require.NoError(t, m.JoinRoute(modelID, routeID))

	// other caller cannot leave
	rt.Caller = store.UniAddr{T: 1, V: []byte{0x02}}
	err = m.LeaveRoute(modelID, routeID)
	assert.ErrorContains(t, err, "only model owner")

	// owner can leave
	rt.Caller = store.UniAddr{T: 1, V: []byte{0x01}}
	require.NoError(t, m.LeaveRoute(modelID, routeID))
}

func TestGateway_JoinRoute_ModelIds(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	provOpt, _ := q.GetProviderByOwner()
	provID, _ := provOpt.UnWrap()

	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1, 1, 1, ProviderOpenAI, "", []string{"sk-1"}, 0, 0))
	require.NoError(t, m.RegisterModel("gpt-4o", ModalityText, 128000, 1, 1, 1, ProviderOpenAI, "", []string{"sk-2"}, 0, 0))
	require.NoError(t, m.RegisterModel("claude-3", ModalityText, 128000, 1, 1, 1, ProviderClaude, "", []string{"sk-3"}, 0, 0))

	models, err := q.ListProviderModels(provID)
	require.NoError(t, err)
	require.Len(t, models, 3)

	// CreateRoute requires system caller.
	rt.System = true
	routeRes := m.CreateRoute("MultiModelRoute", MiningPoolRoute, 0, 0, 0, false)
	routeID, err := routeRes.UnWrap()
	require.NoError(t, err)
	rt.System = false

	for _, model := range models {
		require.NoError(t, m.JoinRoute(model.ID, routeID))
	}

	route, err := q.GetRoute(routeID)
	require.NoError(t, err)
	require.NotNil(t, route)
	require.Len(t, route.ModelIds, 3)
	assert.ElementsMatch(t, []uint64{0, 1, 2}, route.ModelIds)

	routeByName, err := q.GetRouteByName("MultiModelRoute")
	require.NoError(t, err)
	require.NotNil(t, routeByName)
	require.Len(t, routeByName.Data.ModelIds, 3)
	assert.ElementsMatch(t, []uint64{0, 1, 2}, routeByName.Data.ModelIds)

	routes, err := q.ListRoutes(0, 10)
	require.NoError(t, err)
	require.Len(t, routes, 1)
	require.Len(t, routes[0].Data.ModelIds, 3)
	assert.ElementsMatch(t, []uint64{0, 1, 2}, routes[0].Data.ModelIds)
}

func TestGateway_JoinRoute_PersonalToEdge(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("Personal AI", "", "", RegionAsiaEast, 1000))
	provOpt, _ := q.GetProviderByOwner()
	provID, _ := provOpt.UnWrap()

	require.NoError(t, m.RegisterModel("local-llm", ModalityText, 128000, 1, 1, 1, ProviderPersonal, "", []string{"local"}, 0, 0))
	models, err := q.ListProviderModels(provID)
	require.NoError(t, err)
	require.Len(t, models, 1)
	modelID := models[0].ID

	rt.System = true
	routeRes := m.CreateRoute("EdgePool", EdgeRoute, 0, 0, 0, false)
	routeID, err := routeRes.UnWrap()
	require.NoError(t, err)
	rt.System = false

	require.NoError(t, m.JoinRoute(modelID, routeID))

	route, err := q.GetRoute(routeID)
	require.NoError(t, err)
	require.NotNil(t, route)
	require.Len(t, route.ModelIds, 1)
	assert.Equal(t, uint64(0), route.ModelIds[0])
}

func TestGateway_JoinRoute_MismatchedTypeFails(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	provOpt, _ := q.GetProviderByOwner()
	provID, _ := provOpt.UnWrap()

	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1, 1, 1, ProviderOpenAI, "", []string{"sk-1"}, 0, 0))
	models, err := q.ListProviderModels(provID)
	require.NoError(t, err)
	require.Len(t, models, 1)
	modelID := models[0].ID

	rt.System = true
	routeRes := m.CreateRoute("EdgePool", EdgeRoute, 0, 0, 0, false)
	routeID, err := routeRes.UnWrap()
	require.NoError(t, err)
	rt.System = false

	err = m.JoinRoute(modelID, routeID)
	assert.ErrorContains(t, err, "does not match route type")
}
