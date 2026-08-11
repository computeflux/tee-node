package graph

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── DecodeCaller ──────────────────────────────────────────

func TestDecodeCaller(t *testing.T) {
	t.Run("hex_with_prefix", func(t *testing.T) {
		b, err := DecodeCaller("0xdeadbeef")
		require.NoError(t, err)
		assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, b)
	})

	t.Run("invalid_hex", func(t *testing.T) {
		_, err := DecodeCaller("0xzzzz")
		require.Error(t, err)
	})

	t.Run("ss58_path_errors_gracefully", func(t *testing.T) {
		// 非 hex 前缀走 SS58 解码，非法输入应返回错误而不是 panic
		_, err := DecodeCaller("not-a-valid-ss58!!")
		require.Error(t, err)
	})
}

// ── decodeArgs ────────────────────────────────────────────

func TestDecodeArgs(t *testing.T) {
	t.Run("with_and_without_prefix", func(t *testing.T) {
		args, err := decodeArgs([]string{"0xdeadbeef", "00ff"})
		require.NoError(t, err)
		assert.Equal(t, [][]byte{{0xde, 0xad, 0xbe, 0xef}, {0x00, 0xff}}, args)
	})

	t.Run("invalid_hex", func(t *testing.T) {
		_, err := decodeArgs([]string{"0xzz"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decodeArgs")
	})

	t.Run("empty", func(t *testing.T) {
		args, err := decodeArgs(nil)
		require.NoError(t, err)
		assert.Empty(t, args)
	})
}

// ── computeHMAC ───────────────────────────────────────────

func TestComputeHMAC_Deterministic(t *testing.T) {
	// key 与 msg 为 hex 字符串
	keyHex := "deadbeef"
	msgHex := "cafebabe"
	h1 := computeHMAC(keyHex, msgHex)
	h2 := computeHMAC(keyHex, msgHex)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64) // SHA-256 → 64 hex chars

	// 非法 hex 不 panic（解码失败按空字节处理）
	assert.NotPanics(t, func() { _ = computeHMAC("zz", "yy") })
}

// ── verifyEIP191Signature ─────────────────────────────────

func TestVerifyEIP191Signature(t *testing.T) {
	// 该私钥对应地址 0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1（ganache 测试账户 #0）
	privKeyHex := "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d"
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"
	message := "login message"
	sig, err := signEIP191(privKeyHex, message)
	require.NoError(t, err)

	t.Run("valid", func(t *testing.T) {
		assert.True(t, verifyEIP191Signature(address, sig, message))
	})

	t.Run("wrong_message", func(t *testing.T) {
		assert.False(t, verifyEIP191Signature(address, sig, message+"x"))
	})

	t.Run("garbage_signature", func(t *testing.T) {
		assert.False(t, verifyEIP191Signature(address, "0xdeadbeef", message))
		assert.False(t, verifyEIP191Signature(address, "not-hex", message))
	})

	t.Run("wrong_address", func(t *testing.T) {
		wrong := "0x0000000000000000000000000000000000000000"
		assert.False(t, verifyEIP191Signature(wrong, sig, message))
	})
}

// ── initToken (AuthMiddleware) ────────────────────────────

func TestInitToken(t *testing.T) {
	_, cleanup := setupAuthTest(t, "seed")
	defer cleanup()

	apiKey := "auth_tee_" + strings.Repeat("a", 64)
	require.NoError(t, types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
		ID:        1,
		Owner:     "tok@example.com",
		Key:       apiKey,
		IsActive:  true,
		ExpiresAt: uint64(time.Now().Add(time.Hour).Unix()),
	}))

	t.Run("bearer_token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		tk := initToken(req)
		require.NotNil(t, tk)
		assert.Equal(t, "tok@example.com", tk.Owner)
	})

	t.Run("bare_key", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", apiKey)
		tk := initToken(req)
		require.NotNil(t, tk)
		assert.Equal(t, "tok@example.com", tk.Owner)
	})

	t.Run("missing_header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		assert.Nil(t, initToken(req))
	})

	t.Run("unknown_key", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+"auth_tee_"+strings.Repeat("f", 64))
		assert.Nil(t, initToken(req))
	})
}

// ── createUserAPIKey 并发 ID 分配 ─────────────────────────

func TestCreateUserAPIKey_ConcurrentUniqueIDs(t *testing.T) {
	_, cleanup := setupAuthTest(t, "seed")
	defer cleanup()

	const workers = 16
	var wg sync.WaitGroup
	ids := make(chan uint64, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			email := "user" + string(rune('a'+w)) + "@example.com"
			key, err := createUserAPIKey(email, "")
			if err != nil {
				t.Errorf("createUserAPIKey: %v", err)
				return
			}
			tk, err := types.GetGatewayTokenByKey(key)
			if err != nil || tk == nil {
				t.Errorf("token not stored for %s", email)
				return
			}
			ids <- tk.ID
		}(w)
	}
	wg.Wait()
	close(ids)

	seen := make(map[uint64]struct{}, workers)
	for id := range ids {
		_, dup := seen[id]
		assert.False(t, dup, "duplicate gateway token ID: %d", id)
		seen[id] = struct{}{}
	}
	assert.Len(t, seen, workers)
}

// ── SubmitContractCall 错误路径 ───────────────────────────

func TestSubmitContractCall_InvalidSignature(t *testing.T) {
	// 空签名必然校验失败，走错误路径而不是 panic
	_, err := SubmitContractCall([]byte("caller"), 2, 1, "token", [4]byte{1, 2, 3, 4}, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contract verify")
}
