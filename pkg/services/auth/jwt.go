package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTService JWT 签发与验证
type JWTService struct {
	secret []byte
}

func NewJWTService(cfg *AuthConfig) *JWTService {
	return &JWTService{secret: []byte(cfg.JWTSecret)}
}

type customClaims struct {
	jwt.RegisteredClaims
	Email        string `json:"email"`
	Address      string `json:"address"`
	AuthProvider string `json:"authProvider"`
}

// IssueToken 签发 JWT (24h 有效期)
func (s *JWTService) IssueToken(email, address, authProvider string) (string, error) {
	now := time.Now()
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "tee-provider",
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		Email:        email,
		Address:      address,
		AuthProvider: authProvider,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// IssueRecoveryToken 签发恢复 token (10min 有效期)
func (s *JWTService) IssueRecoveryToken(email string) (string, error) {
	now := time.Now()
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "tee-provider-recovery",
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
		Email:        email,
		AuthProvider: "recovery",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ParseToken 解析并验证 JWT
func (s *JWTService) ParseToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &customClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*customClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return &JWTClaims{
		Email:        claims.Email,
		Address:      claims.Address,
		AuthProvider: claims.AuthProvider,
	}, nil
}
