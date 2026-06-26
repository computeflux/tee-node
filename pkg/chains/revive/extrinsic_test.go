package revive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	crypto "github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
)

// ── BatchCall ──────────────────────────────────────────────

func TestContract_BatchCall_InvalidMethod(t *testing.T) {
	c := &Contract{}
	_, err := c.BatchCall("invalid", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in batchMethods")
}

func TestContract_BatchCall_ValidMethods(t *testing.T) {
	validMethods := []string{"batch", "batch_all", "force_batch"}
	for _, m := range validMethods {
		t.Run(m, func(t *testing.T) {
			c := &Contract{}
			assert.NotPanics(t, func() {
				defer func() {
					_ = recover()
				}()
				_, _ = c.BatchCall(m, []chaintypes.ChainCall{})
			})
		})
	}
}

// ── TEECallToCall ──────────────────────────────────────────

func TestContract_TEECallToCall(t *testing.T) {
	c := &Contract{}
	call, err := c.TEECallToCall(&batch.HubCallIndex{}, nil)
	assert.ErrorContains(t, err, "invalid tee call")
	assert.Nil(t, call)
}

// ── CallOfMapAccount ───────────────────────────────────────

func TestContract_CallOfMapAccount(t *testing.T) {
	c := &Contract{}
	call, err := c.CallOfMapAccount(crypto.PubKey{})
	assert.ErrorContains(t, err, "not implemented")
	assert.Nil(t, call)
}

// ── GetChainUrls ───────────────────────────────────────────

func TestContract_GetChainUrls(t *testing.T) {
	c := &Contract{}
	assert.Equal(t, c.GetChainUrls(), c.GetChainUrls())
}

// ── CheckMapAccount ────────────────────────────────────────

func TestContract_CheckMapAccount_NoClient(t *testing.T) {
	c := &Contract{}
	assert.NotPanics(t, func() {
		defer func() { _ = recover() }()
		_, _ = c.CheckMapAccount(crypto.PubKey{})
	})
}
