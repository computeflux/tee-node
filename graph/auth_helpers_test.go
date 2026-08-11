package graph

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withRandError 让 randReadFn 返回错误，用于测生成函数的错误路径。
func withRandError(t *testing.T) {
	t.Helper()
	orig := randReadFn
	randReadFn = func(b []byte) (int, error) { return 0, errors.New("no entropy") }
	t.Cleanup(func() { randReadFn = orig })
}

// ── generateAPIKey / generateWalletNonce / generateRefreshToken ──

func TestGenerateAPIKey_Format(t *testing.T) {
	key, err := generateAPIKey()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "auth_tee_"))
	assert.Len(t, key, len("auth_tee_")+64)

	// 两次生成不重复
	key2, err := generateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2)
}

func TestGenerateAPIKey_RandError(t *testing.T) {
	withRandError(t)
	_, err := generateAPIKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entropy")
}

func TestGenerateWalletNonce_Format(t *testing.T) {
	n, err := generateWalletNonce()
	require.NoError(t, err)
	assert.Len(t, n, 32) // 16 字节 hex

	n2, err := generateWalletNonce()
	require.NoError(t, err)
	assert.NotEqual(t, n, n2)
}

func TestGenerateWalletNonce_RandError(t *testing.T) {
	withRandError(t)
	_, err := generateWalletNonce()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entropy")
}

func TestGenerateRefreshToken_Format(t *testing.T) {
	tok, err := generateRefreshToken()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(tok, "refresh_tee_"))
	assert.Len(t, tok, len("refresh_tee_")+64)
}

func TestGenerateRefreshToken_RandError(t *testing.T) {
	withRandError(t)
	_, err := generateRefreshToken()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entropy")
}

// ── createUserAPIKey 的地址解析分支 ────────────────────────

func TestCreateUserAPIKey_AddrParsing(t *testing.T) {
	_, cleanup := setupAuthTest(t, "seed")
	defer cleanup()

	tests := []struct {
		name     string
		addrHex  string
		wantVLen int
		wantV    []byte
	}{
		{name: "valid_20_byte", addrHex: "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1", wantVLen: 20,
			wantV: []byte{0x90, 0xf8, 0xbf, 0x6a, 0x47, 0x9f, 0x32, 0x0e, 0xad, 0x07, 0x44, 0x11, 0xa4, 0xb0, 0xe7, 0x94, 0x4e, 0xa8, 0xc9, 0xc1}},
		{name: "no_prefix", addrHex: "90F8bf6A479f320ead074411a4B0e7944Ea8c9C1", wantVLen: 20},
		{name: "invalid_hex_keeps_empty", addrHex: "0xzz", wantVLen: 0},
		{name: "wrong_length_21_bytes_keeps_empty", addrHex: "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1ff", wantVLen: 0},
		{name: "empty_keeps_empty", addrHex: "", wantVLen: 0},
		{name: "bare_prefix_keeps_empty", addrHex: "0x", wantVLen: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := createUserAPIKey("addr-test@example.com", tt.addrHex)
			require.NoError(t, err)

			tk, err := types.GetGatewayTokenByKey(key)
			require.NoError(t, err)
			require.NotNil(t, tk)
			assert.Len(t, tk.Addr.V, tt.wantVLen)
			if tt.wantV != nil {
				assert.Equal(t, tt.wantV, tk.Addr.V)
			}
		})
	}
}

// ── resolveEmailFromToken ──────────────────────────────────

func TestResolveEmailFromToken_SignSession(t *testing.T) {
	_, cleanup := setupAuthTest(t, "seed")
	defer cleanup()

	t.Run("valid", func(t *testing.T) {
		tok := "sig_" + strings.Repeat("a", 32)
		require.NoError(t, types.PutSignSession(&types.SignSession{
			Token:     tok,
			Email:     "sig@example.com",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}))

		email, err := resolveEmailFromToken(tok)
		require.NoError(t, err)
		assert.Equal(t, "sig@example.com", email)
	})

	t.Run("unknown_token", func(t *testing.T) {
		_, err := resolveEmailFromToken("sig_" + strings.Repeat("b", 32))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sign token")
	})

	t.Run("expired", func(t *testing.T) {
		tok := "sig_" + strings.Repeat("c", 32)
		require.NoError(t, types.PutSignSession(&types.SignSession{
			Token:     tok,
			Email:     "old@example.com",
			ExpiresAt: time.Now().Add(-time.Hour).Unix(),
		}))

		_, err := resolveEmailFromToken(tok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sign token")
	})
}

func TestResolveEmailFromToken_APIToken(t *testing.T) {
	_, cleanup := setupAuthTest(t, "seed")
	defer cleanup()

	t.Run("valid", func(t *testing.T) {
		apiKey := "auth_tee_" + strings.Repeat("d", 64)
		require.NoError(t, types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
			ID:        1,
			Owner:     "tok@example.com",
			Key:       apiKey,
			IsActive:  true,
			ExpiresAt: uint64(time.Now().Add(time.Hour).Unix()),
		}))

		email, err := resolveEmailFromToken(apiKey)
		require.NoError(t, err)
		assert.Equal(t, "tok@example.com", email)
	})

	t.Run("unknown_key", func(t *testing.T) {
		_, err := resolveEmailFromToken("auth_tee_" + strings.Repeat("e", 64))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
	})

	t.Run("inactive", func(t *testing.T) {
		apiKey := "auth_tee_" + strings.Repeat("f", 64)
		require.NoError(t, types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
			ID:        2,
			Owner:     "inactive@example.com",
			Key:       apiKey,
			IsActive:  false,
			ExpiresAt: uint64(time.Now().Add(time.Hour).Unix()),
		}))

		_, err := resolveEmailFromToken(apiKey)
		require.Error(t, err)
	})

	t.Run("expired", func(t *testing.T) {
		apiKey := "auth_tee_" + strings.Repeat("g", 64)
		require.NoError(t, types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
			ID:        3,
			Owner:     "expired@example.com",
			Key:       apiKey,
			IsActive:  true,
			ExpiresAt: uint64(time.Now().Add(-time.Hour).Unix()),
		}))

		_, err := resolveEmailFromToken(apiKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("empty", func(t *testing.T) {
		_, err := resolveEmailFromToken("")
		require.Error(t, err)
	})
}
