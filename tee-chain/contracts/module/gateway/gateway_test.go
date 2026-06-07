package gateway

import (
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/computeflux/tee-provider/tee-chain/contracts/testutil"
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

// ── Model Tests ──────────────────────────────────────────

func TestGateway_RegisterModel_Success(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	err := m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", []string{"sk-abc"})
	require.NoError(t, err)

	count, err := q.GetModelCount()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), count)

	models, err := q.ListModels(0, 10)
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "gpt-4", models[0].Name)
	// API keys should be masked in query
	assert.Equal(t, "sk****", models[0].APIKeys[0])
}

func TestGateway_RegisterModel_WithoutProvider(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	// Without provider, RegisterModel returns nil (not an error) in current implementation
	err := m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil)
	assert.NoError(t, err)
}

func TestGateway_RegisterModel_DuplicateName(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil))

	err := m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil)
	assert.ErrorContains(t, err, "already exists")
}

func TestGateway_DeactivateAndActiveModel(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil))

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
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 5000, 2500, 15000, ProviderOpenAI, "", nil))

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

	err := m.RegisterAPIKey(keyHash, 100)
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
	require.NoError(t, m.RegisterAPIKey(keyHash, 100))

	err := m.RegisterAPIKey(keyHash, 200)
	assert.ErrorContains(t, err, "already registered")
}

func TestGateway_RevokeAPIKey(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	keyHash := make([]byte, 32)
	require.NoError(t, m.RegisterAPIKey(keyHash, 100))
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
	require.NoError(t, m.RegisterAPIKey(keyHash, 100))

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

	require.NoError(t, m.RegisterAPIKey(keyHash1, 100))
	require.NoError(t, m.RegisterAPIKey(keyHash2, 200))

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

func TestGateway_SettleUsage_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	// Setup: create provider + model + recharge user
	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	// Settle: in=1000 tokens, out=500 tokens → cost = (1000*1000000 + 500*2000000) / 1000000 = 2002
	err := m.SettleUsage([]Settlement{
		{
			User:       user,
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
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	nonce := []byte{0x01, 0x02, 0x03, 0x04}
	record := Settlement{
		User:       user,
		ModelID:    0,
		ProviderID: 0,
		TokenIn:    1000,
		TokenOut:   500,
		Count:      1,
		Nonce:      nonce,
	}

	require.NoError(t, m.SettleUsage([]Settlement{record}))
	// Duplicate record nonce should be silently skipped (return nil), no double charge
	require.NoError(t, m.SettleUsage([]Settlement{record}))
}

func TestGateway_Withdraw_Success(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	require.NoError(t, m.SettleUsage([]Settlement{
		{User: user, ModelID: 0, ProviderID: 0, TokenIn: 1000000, TokenOut: 500000, Count: 1, Nonce: []byte{0x01}},
	}))

	provCredit, _ := q.GetProviderCredit(0)
	require.Greater(t, provCredit, uint64(0))

	rt.System = true
	require.NoError(t, m.ApplyWithdraw(0, []byte{}, 0, provCredit, []byte("nonce1")))

	provCreditAfter, _ := q.GetProviderCredit(0)
	assert.Equal(t, uint64(0), provCreditAfter)
}

func TestGateway_Withdraw_RequiresSystem(t *testing.T) {
	gw, _ := newTestGateway()
	m := GatewayMutation{*gw}

	err := m.ApplyWithdraw(0, []byte{}, 0, 100, []byte("nonce1"))
	assert.ErrorContains(t, err, "not sudo")
}

func TestGateway_Withdraw_InsufficientCredit(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	require.NoError(t, m.CreateProvider("AI Corp", "", "", RegionAsiaEast, 1000))
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, ProviderOpenAI, "", nil))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, []byte("nonce1")))
	rt.System = false

	// Settle a small amount first
	require.NoError(t, m.SettleUsage([]Settlement{
		{User: user, ModelID: 0, ProviderID: 0, TokenIn: 1000, TokenOut: 500, Count: 1, Nonce: []byte{0x01}},
	}))

	// Try to withdraw more than earned
	rt.System = true
	err := m.ApplyWithdraw(0, []byte{}, 0, 1_000_000, []byte("nonce1"))
	assert.ErrorIs(t, err, ErrLowCredit)
}
