package auth

import (
	"strings"
	"testing"
	"time"

	apiAuth "github.com/computeflux/tee-provider/pkg/api/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJWTService() *JWTService {
	return NewJWTService(&apiAuth.AuthConfig{JWTSecret: "test-secret-32bytes-long-ok!!!"})
}

func TestJWTService_IssueToken_RoundTrip(t *testing.T) {
	svc := newTestJWTService()
	tokenStr, err := svc.IssueToken("alice@example.com", "0x1234", "google")
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	claims, err := svc.ParseToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", claims.Email)
	assert.Equal(t, "0x1234", claims.Address)
	assert.Equal(t, "google", claims.AuthProvider)
}

func TestJWTService_IssueRecoveryToken_RoundTrip(t *testing.T) {
	svc := newTestJWTService()
	tokenStr, err := svc.IssueRecoveryToken("alice@example.com")
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	claims, err := svc.ParseToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", claims.Email)
	assert.Equal(t, "recovery", claims.AuthProvider)
}

func TestJWTService_ParseToken_Expired(t *testing.T) {
	svc := newTestJWTService()

	// 签发一个已经过期的 token
	now := time.Now().Add(-25 * time.Hour)
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "tee-provider",
			Subject:   "alice@example.com",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
		Email: "alice@example.com",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(svc.secret)

	_, err := svc.ParseToken(tokenStr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestJWTService_ParseToken_Tampered(t *testing.T) {
	svc := newTestJWTService()
	tokenStr, err := svc.IssueToken("alice@example.com", "0x1234", "google")
	require.NoError(t, err)

	// 篡改 payload
	tampered := tokenStr[:len(tokenStr)-10] + "deadbeef00"
	_, err = svc.ParseToken(tampered)
	require.Error(t, err)
}

func TestJWTService_ParseToken_WrongSecret(t *testing.T) {
	svc1 := newTestJWTService()
	svc2 := NewJWTService(&apiAuth.AuthConfig{JWTSecret: "different-secret-32bytes-long!!"})

	tokenStr, err := svc1.IssueToken("alice@example.com", "0x1234", "google")
	require.NoError(t, err)

	_, err = svc2.ParseToken(tokenStr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}

func TestJWTService_ParseToken_Empty(t *testing.T) {
	svc := newTestJWTService()
	_, err := svc.ParseToken("")
	require.Error(t, err)
}

func TestJWTService_RecoveryToken_ShorterExpiry(t *testing.T) {
	svc := newTestJWTService()

	// 解析 recovery token 的 exp 字段
	now := time.Now()
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "tee-provider-recovery",
			Subject:   "alice@example.com",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
		AuthProvider: "recovery",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(svc.secret)

	parsed, err := svc.ParseToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "recovery", parsed.AuthProvider)
}

func TestJWTService_ParseToken_WrongAlg(t *testing.T) {
	// 使用 None 算法的 token（无签名）
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"email": "alice@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	svc := newTestJWTService()
	_, err := svc.ParseToken(tokenStr)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "signing method") || strings.Contains(err.Error(), "signature"))
}
