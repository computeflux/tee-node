package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
)

// OTPService 生成、存储、验证 OTP（使用 Pebble 持久化）
type OTPService struct {
	ttl time.Duration
}

func NewOTPService(ttlMins int) *OTPService {
	ttl := time.Duration(ttlMins) * time.Minute
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	// 启动后台清理
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			// OTP 通过 TTL 在 Verify 时检查过期
		}
	}()
	return &OTPService{ttl: ttl}
}

// Generate 生成 OTP 并存储到 Pebble
func (s *OTPService) Generate(email, clientID string) (string, error) {
	otp, err := generateNumericOTP(6)
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}

	key := normalizeEmail(email)

	// 存储到 Pebble（带过期时间戳前缀）
	data := fmt.Sprintf("%d:%s:%s", time.Now().Add(s.ttl).Unix(), otp, clientID)
	if err := model.PutOTP(key, data); err != nil {
		return "", fmt.Errorf("store otp: %w", err)
	}

	return otp, nil
}

// Verify 验证 OTP
func (s *OTPService) Verify(email, otp string) error {
	key := normalizeEmail(email)

	stored, err := model.GetOTP(key)
	if err != nil {
		return fmt.Errorf("no OTP requested for this email")
	}

	// 解析: expiresAt:otp:clientID
	var expiresAt int64
	var storedOTP string
	if _, err := fmt.Sscanf(stored, "%d:%s:", &expiresAt, &storedOTP); err != nil {
		// 兼容简单格式
		storedOTP = stored
	}
	if len(storedOTP) > 6 {
		storedOTP = storedOTP[:6]
	}

	if time.Now().Unix() > expiresAt && expiresAt > 0 {
		model.DeleteOTP(key)
		return fmt.Errorf("OTP expired")
	}

	if storedOTP != otp {
		return fmt.Errorf("invalid OTP")
	}

	// 验证成功，删除
	model.DeleteOTP(key)
	return nil
}

func normalizeEmail(email string) string {
	return email
}

func generateNumericOTP(length int) (string, error) {
	const digits = "0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		result[i] = digits[n.Int64()]
	}
	return string(result), nil
}
