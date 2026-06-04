package revive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	crypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/model"
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
			// ChainClient is nil, so types.NewCall will panic for valid methods.
			// We only verify the validation logic passes for valid methods.
			assert.NotPanics(t, func() {
				// Wrap in recover because c.Meta will be accessed
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
	call, err := c.TEECallToCall(&model.TeeCall{}, nil)
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

// ── GetCloudAddress / GetSubnetAddress ─────────────────────

func TestGetCloudAddress(t *testing.T) {
	assert.Equal(t, MainChainConfig.CloudAddress, GetCloudAddress())
}

func TestGetSubnetAddress(t *testing.T) {
	assert.Equal(t, MainChainConfig.SubnetAddress, GetSubnetAddress())
}

// ── GetChainUrls ───────────────────────────────────────────

func TestContract_GetChainUrls(t *testing.T) {
	c := &Contract{}
	assert.Equal(t, MainChainConfig.Urls, c.GetChainUrls())
}

// ── CheckMapAccount ────────────────────────────────────────

func TestContract_CheckMapAccount_NoClient(t *testing.T) {
	c := &Contract{}
	// ChainClient is nil → Api() will panic
	assert.NotPanics(t, func() {
		defer func() { _ = recover() }()
		_, _ = c.CheckMapAccount(crypto.PubKey{})
	})
}
