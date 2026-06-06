package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
)

// OTPService 生成、存储、验证 OTP（使用 Pebble 持久化）
// 包含防刷机制：发送频率限制、验证错误次数限制、日发送上限
type OTPService struct {
	ttl             time.Duration
	sendInterval    time.Duration // 两次发送最小间隔
	maxVerifyFails  int           // 最大验证失败次数（锁定）
	maxDailySends   int           // 每日最大发送次数
	mu              sync.RWMutex
	sendRecords     map[string]time.Time // email -> 上次发送时间
	verifyFailCount map[string]int       // email -> 连续验证失败次数
}

func NewOTPService(ttlMins int) *OTPService {
	ttl := time.Duration(ttlMins) * time.Minute
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	s := &OTPService{
		ttl:             ttl,
		sendInterval:    60 * time.Second,
		maxVerifyFails:  5,
		maxDailySends:   10,
		sendRecords:     make(map[string]time.Time),
		verifyFailCount: make(map[string]int),
	}
	// 启动后台清理
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.cleanup()
		}
	}()
	return s
}

// Generate 生成 OTP 并存储到 Pebble
func (s *OTPService) Generate(email, clientID string) (string, error) {
	key := normalizeEmail(email)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 检查发送频率
	if lastSend, ok := s.sendRecords[key]; ok {
		if time.Since(lastSend) < s.sendInterval {
			return "", fmt.Errorf("please wait %ds before requesting another OTP", int(s.sendInterval.Seconds()-time.Since(lastSend).Seconds()))
		}
	}

	// 2. 检查日发送上限
	dailyCount, err := s.getDailySendCount(key)
	if err != nil {
		util.LogWithYellow("OTP", "failed to get daily count for", email, ":", err)
	}
	if dailyCount >= s.maxDailySends {
		return "", fmt.Errorf("daily OTP limit reached, please try again tomorrow")
	}

	// 3. 检查是否因验证失败被锁定
	if fails := s.verifyFailCount[key]; fails >= s.maxVerifyFails {
		return "", fmt.Errorf("account locked due to too many failed attempts, please request a new OTP")
	}

	// 4. 生成 OTP
	otp, err := generateNumericOTP(6)
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}

	// 5. 存储到 Pebble（带过期时间戳前缀）
	data := fmt.Sprintf("%d:%s:%s", time.Now().Add(s.ttl).Unix(), otp, clientID)
	if err := store.PutOTP(key, data); err != nil {
		return "", fmt.Errorf("store otp: %w", err)
	}

	// 6. 更新发送记录
	s.sendRecords[key] = time.Now()
	s.incrementDailySendCount(key)

	// 7. 清除之前的验证失败次数
	delete(s.verifyFailCount, key)

	return otp, nil
}

// Verify 验证 OTP，可选绑定 clientID 防止跨客户端重放
func (s *OTPService) Verify(email, otp, clientID string) error {
	key := normalizeEmail(email)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否被锁定
	if fails := s.verifyFailCount[key]; fails >= s.maxVerifyFails {
		return fmt.Errorf("account locked due to too many failed attempts, please request a new OTP")
	}

	stored, err := store.GetOTP(key)
	if err != nil {
		return fmt.Errorf("no OTP requested for this email")
	}

	// 解析: expiresAt:otp:clientID
	var expiresAt int64
	var storedOTP, storedClientID string
	parts := strings.SplitN(stored, ":", 3)
	if len(parts) >= 3 {
		expiresAt, _ = strconv.ParseInt(parts[0], 10, 64)
		storedOTP = parts[1]
		storedClientID = parts[2]
	} else if len(parts) == 2 {
		expiresAt, _ = strconv.ParseInt(parts[0], 10, 64)
		storedOTP = parts[1]
	} else {
		// 兼容简单格式
		storedOTP = stored
	}

	if time.Now().Unix() > expiresAt && expiresAt > 0 {
		store.DeleteOTP(key)
		delete(s.verifyFailCount, key)
		return fmt.Errorf("OTP expired")
	}

	if storedOTP != otp {
		s.verifyFailCount[key]++
		remaining := s.maxVerifyFails - s.verifyFailCount[key]
		if remaining <= 0 {
			store.DeleteOTP(key)
			return fmt.Errorf("too many failed attempts, OTP has been invalidated, please request a new one")
		}
		return fmt.Errorf("invalid OTP, %d attempts remaining", remaining)
	}

	// 若生成时绑定了 clientID，则验证必须匹配
	if storedClientID != "" && storedClientID != clientID {
		return fmt.Errorf("OTP client mismatch")
	}

	// 验证成功，清理
	store.DeleteOTP(key)
	delete(s.verifyFailCount, key)
	delete(s.sendRecords, key)
	return nil
}

// ── 日发送计数（基于 Pebble 存储，跨重启有效）──

func (s *OTPService) dailyCountKey(email string) string {
	today := time.Now().Format("2006-01-02")
	return fmt.Sprintf("otp_daily:%s:%s", today, email)
}

func (s *OTPService) getDailySendCount(email string) (int, error) {
	key := s.dailyCountKey(email)
	b, err := store.GetKey("WEB3AUTH", key)
	if err != nil {
		return 0, nil
	}
	n, _ := strconv.Atoi(string(b))
	return n, nil
}

func (s *OTPService) incrementDailySendCount(email string) {
	key := s.dailyCountKey(email)
	count, _ := s.getDailySendCount(email)
	count++
	store.SetKey("WEB3AUTH", key, []byte(strconv.Itoa(count)))
}

// cleanup 清理过期的内存记录
func (s *OTPService) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清理超过 24 小时的发送记录
	for k, t := range s.sendRecords {
		if time.Since(t) > 24*time.Hour {
			delete(s.sendRecords, k)
		}
	}
	// 清理超过 24 小时的失败计数
	// 注意：这里不清理，因为失败计数应在成功验证或发送新 OTP 时清除
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GenerateNumericOTP 生成指定长度的数字 OTP（公开，供外部调用）
func GenerateNumericOTP(length int) (string, error) {
	return generateNumericOTP(length)
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
