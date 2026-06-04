package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestOTP(t *testing.T) *OTPService {
	t.Helper()

	db, err := pebble.Open(t.TempDir(), &pebble.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	store.DBINS = &store.DB{DB: db}

	return NewOTPService(5)
}

func TestOTPService_GenerateAndVerify(t *testing.T) {
	svc := setupTestOTP(t)
	email := "alice@example.com"

	otp, err := svc.Generate(email, "web")
	require.NoError(t, err)
	require.Len(t, otp, 6)

	err = svc.Verify(email, otp)
	assert.NoError(t, err)
}

func TestOTPService_Verify_WrongOTP(t *testing.T) {
	svc := setupTestOTP(t)
	email := "alice@example.com"

	_, err := svc.Generate(email, "web")
	require.NoError(t, err)

	err = svc.Verify(email, "000000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid OTP")
}

func TestOTPService_Verify_Expired(t *testing.T) {
	svc := setupTestOTP(t)
	email := "alice@example.com"

	// 生成 OTP
	otp, err := svc.Generate(email, "web")
	require.NoError(t, err)

	// 篡改存储的数据使其过期
	past := time.Now().Add(-10 * time.Minute).Unix()
	store.PutOTP(normalizeEmail(email), formatOTPData(past, otp, "web"))

	err = svc.Verify(email, otp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestOTPService_Verify_NoOTPRequested(t *testing.T) {
	svc := setupTestOTP(t)

	err := svc.Verify("nobody@example.com", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OTP requested")
}

func TestOTPService_Generate_RateLimit(t *testing.T) {
	svc := setupTestOTP(t)
	email := "alice@example.com"

	_, err := svc.Generate(email, "web")
	require.NoError(t, err)

	// 立即再次生成应被拒绝
	_, err = svc.Generate(email, "web")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "please wait")
}

func TestOTPService_Verify_LockAfterMaxFails(t *testing.T) {
	svc := setupTestOTP(t)
	svc.maxVerifyFails = 3
	email := "alice@example.com"

	otp, err := svc.Generate(email, "web")
	require.NoError(t, err)

	// 连续错误 3 次
	for i := 0; i < 3; i++ {
		err = svc.Verify(email, "000000")
		require.Error(t, err)
	}

	// 第 4 次即使正确也应被锁定
	err = svc.Verify(email, otp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked")
}

func TestOTPService_Generate_DailyLimit(t *testing.T) {
	svc := setupTestOTP(t)
	svc.maxDailySends = 2
	svc.sendInterval = 0 // 跳过发送间隔，专注测试日上限
	email := "alice@example.com"

	_, err := svc.Generate(email, "web")
	require.NoError(t, err)
	_, err = svc.Generate(email, "web")
	require.NoError(t, err)

	// 达到日上限
	_, err = svc.Generate(email, "web")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily OTP limit reached")
}

func TestOTPService_Generate_UnlocksAfterNewOTP(t *testing.T) {
	svc := setupTestOTP(t)
	svc.maxVerifyFails = 3
	svc.sendInterval = 100 * time.Millisecond // 缩短等待时间
	email := "alice@example.com"

	otp1, err := svc.Generate(email, "web")
	require.NoError(t, err)

	// 验证失败 2 次（未达到阈值）
	svc.Verify(email, "wrong1")
	svc.Verify(email, "wrong2")

	// 等待发送间隔后生成新 OTP（内部会清除失败次数）
	time.Sleep(svc.sendInterval + 50*time.Millisecond)

	otp2, err := svc.Generate(email, "web")
	require.NoError(t, err)
	require.NotEqual(t, otp1, otp2)

	// 新 OTP 应该可以验证
	err = svc.Verify(email, otp2)
	assert.NoError(t, err)
}

func TestOTPService_Verify_CleansAfterSuccess(t *testing.T) {
	svc := setupTestOTP(t)
	email := "alice@example.com"

	otp, err := svc.Generate(email, "web")
	require.NoError(t, err)

	err = svc.Verify(email, otp)
	require.NoError(t, err)

	// 验证成功后 OTP 应被删除
	err = svc.Verify(email, otp)
	require.Error(t, err)
}

func TestNormalizeEmail(t *testing.T) {
	assert.Equal(t, "alice@example.com", normalizeEmail("  Alice@Example.COM  "))
	assert.Equal(t, "bob@test.org", normalizeEmail("bob@test.org"))
}

func TestGenerateNumericOTP_Length(t *testing.T) {
	for _, length := range []int{4, 6, 8} {
		otp, err := generateNumericOTP(length)
		require.NoError(t, err)
		require.Len(t, otp, length)
		for _, c := range otp {
			assert.True(t, c >= '0' && c <= '9')
		}
	}
}

// helper
func formatOTPData(expiresAt int64, otp, clientID string) string {
	return fmt.Sprintf("%d:%s:%s", expiresAt, otp, clientID)
}
