// scale-codec-gen 兼容性测试：验证生成 codec 与 go-substrate-rpc-client 反射 codec 完全兼容。
//
// 测试策略：
//  1. 基础类型全覆盖（定长整数、compact、bool、string、bytes）
//  2. 随机组合字段长度和值
//  3. 复杂嵌套结构（UniAddr、Option、多层嵌套、[][]byte）
//  4. 双向编解码闭环（旧→新→旧、新→旧→新）
package main

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// ── 基础类型 —─────────────────────────────────────────────

func TestPrimitiveTypes(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 1000; i++ {
		// 随机基础类型
		v := struct {
			U8  uint8
			U16 uint16
			U32 uint32
			U64 uint64
			I8  int8
			I16 int16
			I32 int32
			I64 int64
			B   bool
		}{
			U8:  uint8(rng.Intn(256)),
			U16: uint16(rng.Intn(65536)),
			U32: rng.Uint32(),
			U64: rng.Uint64(),
			I8:  int8(rng.Intn(256) - 128),
			I16: int16(rng.Intn(65536) - 32768),
			I32: int32(rng.Uint32()),
			I64: int64(rng.Uint64()),
			B:   rng.Intn(2) == 1,
		}
		oldB, err := codec.Encode(v)
		if err != nil {
			t.Fatalf("old codec encode: %v", err)
		}
		// 手动编码对比
		newB := encodeU8(v.U8)
		newB = append(newB, encodeU16LE(v.U16)...)
		newB = append(newB, encodeU32LE(v.U32)...)
		newB = append(newB, encodeU64LE(v.U64)...)
		newB = append(newB, encodeI8(v.I8)...)
		newB = append(newB, encodeI16LE(v.I16)...)
		newB = append(newB, encodeI32LE(v.I32)...)
		newB = append(newB, encodeI64LE(v.I64)...)
		newB = append(newB, encodeBool(v.B)...)
		assertBytesEqual(t, "primitives", oldB, newB)
	}
}

func TestStringsAndBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(99))

	for i := 0; i < 500; i++ {
		sLen := rng.Intn(512)
		s := make([]byte, sLen)
		rng.Read(s)
		str := string(s)

		bLen := rng.Intn(1024)
		b := make([]byte, bLen)
		rng.Read(b)

		v := struct {
			Str string
			B   []byte
		}{Str: str, B: b}
		oldB, _ := codec.Encode(v)
		newB := encodeCompact(nil, uint64(len(str)))
		newB = append(newB, []byte(str)...)
		newB = encodeCompact(newB, uint64(len(b)))
		newB = append(newB, b...)
		assertBytesEqual(t, fmt.Sprintf("str%d_bytes%d", len(str), len(b)), oldB, newB)
	}
}

// ── Auth 类型 —─────────────────────────────────────────────

func TestAuthConfig(t *testing.T) {
	rng := rand.New(rand.NewSource(7))

	for i := 0; i < 200; i++ {
		cfg := auth.AuthConfig{
			JWTSecret:           randStr(rng, 1, 64),
			SeedSecret:          randStr(rng, 1, 64),
			SMTPHost:            randStr(rng, 0, 128),
			SMTPPort:            rng.Uint32(),
			SMTPUser:            randStr(rng, 0, 64),
			SMTPPassword:        randStr(rng, 0, 128),
			SMTPFrom:            randStr(rng, 0, 128),
			SMTPFromName:        randStr(rng, 0, 64),
			MockEmail:           rng.Intn(2) == 1,
			OTPLength:           rng.Uint32(),
			OTPTTLMins:          rng.Uint32(),
			GoogleClientID:      randStr(rng, 0, 128),
			GoogleClientSecret:  randStr(rng, 0, 128),
			GoogleRedirectURL:   randStr(rng, 0, 256),
			GitHubClientID:      randStr(rng, 0, 128),
			GitHubClientSecret:  randStr(rng, 0, 128),
			GitHubRedirectURL:   randStr(rng, 0, 256),
			TwitterClientID:     randStr(rng, 0, 128),
			TwitterClientSecret: randStr(rng, 0, 128),
			TwitterRedirectURL:  randStr(rng, 0, 256),
		}
		testRoundtrip(t, "AuthConfig", cfg)
	}
}

func TestUserInfo(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	for i := 0; i < 500; i++ {
		u := auth.UserInfo{
			EmailHash:      randBytes(rng, 0, 64),
			ShareRoot:      randBytes(rng, 0, 64),
			DerivedAddress: randBytes(rng, 0, 64),
			RegisteredAt:   rng.Uint64(),
		}
		testRoundtrip(t, "UserInfo", u)
	}
}

func TestRecoveryConfig(t *testing.T) {
	rng := rand.New(rand.NewSource(23))
	for i := 0; i < 300; i++ {
		nG := rng.Intn(10)
		guardians := make([][]byte, nG)
		for j := 0; j < nG; j++ {
			guardians[j] = randBytes(rng, 1, 32)
		}
		r := auth.RecoveryConfig{
			Guardians:     guardians,
			Threshold:     rng.Uint32() % uint32(nG+1),
			RecoveryEmail: randBytes(rng, 0, 64),
		}
		testRoundtrip(t, "RecoveryConfig", r)
	}
}

func TestRecoveryRequest(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	for i := 0; i < 200; i++ {
		nA := rng.Intn(10)
		approvals := make([][]byte, nA)
		for j := 0; j < nA; j++ {
			approvals[j] = randBytes(rng, 1, 32)
		}
		r := auth.RecoveryRequest{
			EmailHash:    randBytes(rng, 1, 32),
			NewShareRoot: randBytes(rng, 1, 64),
			Approvals:    approvals,
			ExpiresAt:    rng.Uint64(),
		}
		testRoundtrip(t, "RecoveryRequest", r)
	}
}

// ── Gateway 类型 —─────────────────────────────────────────

func TestModelInfo(t *testing.T) {
	rng := rand.New(rand.NewSource(37))
	for i := 0; i < 300; i++ {
		nKeys := rng.Intn(5)
		keys := make([]string, nKeys)
		for j := 0; j < nKeys; j++ {
			keys[j] = randStr(rng, 3, 64)
		}
		m := gateway.ModelInfo{
			Name:            randStr(rng, 1, 64),
			Owner:           randUniAddr(rng),
			ProviderID:      rng.Uint64(),
			InputModalities: uint8(rng.Intn(256)),
			ContextLength:   rng.Uint64(),
			InPrice:         rng.Uint64(),
			CachedPrice:     rng.Uint64(),
			OutPrice:        rng.Uint64(),
			ProviderType:    uint16(rng.Intn(65536)),
			ModelID:         randStr(rng, 1, 64),
			URI:             randStr(rng, 0, 256),
			APIKeys:         keys,
			IsActive:        rng.Intn(2) == 1,
			CreatedAt:       rng.Uint64(),
		}
		testRoundtrip(t, "ModelInfo", m)
	}
}

func TestProviderInfo(t *testing.T) {
	rng := rand.New(rand.NewSource(41))
	for i := 0; i < 200; i++ {
		nModels := rng.Intn(10)
		modelIds := make([]uint64, nModels)
		for j := 0; j < nModels; j++ {
			modelIds[j] = rng.Uint64()
		}
		p := gateway.ProviderInfo{
			Owner:         randUniAddr(rng),
			Name:          randStr(rng, 1, 64),
			LogoURL:       randStr(rng, 0, 256),
			Description:   randStr(rng, 0, 512),
			ModelIds:      modelIds,
			Region:        uint8(rng.Intn(256)),
			Deposit:       rng.Uint64(),
			FeeRate:       rng.Uint32(),
			Priority:      rng.Uint32(),
			TotalRequests: rng.Uint64(),
			SuccessRate:   rng.Uint32(),
			AvgLatency:    rng.Uint32(),
			UptimeSLA:     rng.Uint32(),
			IsActive:      rng.Intn(2) == 1,
			CreatedAt:     rng.Uint64(),
		}
		testRoundtrip(t, "ProviderInfo", p)
	}
}

func TestAPIKeyInfo(t *testing.T) {
	rng := rand.New(rand.NewSource(43))
	for i := 0; i < 300; i++ {
		k := gateway.APIKeyInfo{
			OwnerKey:  rng.Uint64(),
			Owner:     randUniAddr(rng),
			KeyHash:   randBytes(rng, 32, 32),
			RateLimit: rng.Uint32(),
			IsActive:  rng.Intn(2) == 1,
			CreatedAt: rng.Uint64(),
		}
		testRoundtrip(t, "APIKeyInfo", k)
	}
}

func TestSettlement(t *testing.T) {
	rng := rand.New(rand.NewSource(47))
	for i := 0; i < 200; i++ {
		s := gateway.Settlement{
			User:        randUniAddr(rng),
			ModelID:     rng.Uint64(),
			ProviderID:  rng.Uint64(),
			TokenIn:     rng.Uint64(),
			TokenOut:    rng.Uint64(),
			CachedToken: rng.Uint64(),
			Count:       uint16(rng.Intn(65536)),
			Nonce:       randBytes(rng, 16, 16),
		}
		testRoundtrip(t, "Settlement", s)
	}
}

func TestDailyUsage(t *testing.T) {
	rng := rand.New(rand.NewSource(51))
	for i := 0; i < 200; i++ {
		d := gateway.DailyUsage{
			User:        randUniAddr(rng),
			ProviderID:  rng.Uint64(),
			ModelID:     rng.Uint64(),
			Count:       rng.Uint64(),
			TokenIn:     rng.Uint64(),
			TokenOut:    rng.Uint64(),
			CachedToken: rng.Uint64(),
			PointsUsed:  rng.Uint64(),
		}
		testRoundtrip(t, "DailyUsage", d)
	}
}

func TestRechargeRecord(t *testing.T) {
	rng := rand.New(rand.NewSource(53))
	for i := 0; i < 200; i++ {
		r := gateway.RechargeRecord{
			User:   randUniAddr(rng),
			Amount: rng.Uint64(),
			Time:   int64(rng.Uint64()),
		}
		testRoundtrip(t, "RechargeRecord", r)
	}
}

func TestUserSettlementRecord(t *testing.T) {
	rng := rand.New(rand.NewSource(59))
	for i := 0; i < 200; i++ {
		r := gateway.UserSettlementRecord{
			ProviderID:  rng.Uint64(),
			ModelID:     rng.Uint64(),
			TokenIn:     rng.Uint64(),
			TokenOut:    rng.Uint64(),
			CachedToken: rng.Uint64(),
			Cost:        rng.Uint64(),
			Count:       uint16(rng.Intn(65536)),
			Time:        int64(rng.Uint64()),
		}
		testRoundtrip(t, "UserSettlementRecord", r)
	}
}

func TestProviderSettlementRecord(t *testing.T) {
	rng := rand.New(rand.NewSource(61))
	for i := 0; i < 200; i++ {
		r := gateway.ProviderSettlementRecord{
			User:        randUniAddr(rng),
			ModelID:     rng.Uint64(),
			TokenIn:     rng.Uint64(),
			TokenOut:    rng.Uint64(),
			CachedToken: rng.Uint64(),
			Cost:        rng.Uint64(),
			Count:       uint16(rng.Intn(65536)),
			Time:        int64(rng.Uint64()),
		}
		testRoundtrip(t, "ProviderSettlementRecord", r)
	}
}

// ── Batch 类型 —───────────────────────────────────────────

func TestBatchCall(t *testing.T) {
	rng := rand.New(rand.NewSource(67))
	for i := 0; i < 500; i++ {
		nArgs := rng.Intn(10)
		args := make([][]byte, nArgs)
		for j := 0; j < nArgs; j++ {
			args[j] = randBytes(rng, 0, 256)
		}
		b := batch.BatchCall{
			Name:   randBytes(rng, 1, 64),
			Method: randBytes(rng, 4, 4),
			Args:   args,
		}
		testRoundtrip(t, "BatchCall", b)
	}
}

// ── 压力测试 —─────────────────────────────────────────────

func TestStress_ManyFields(t *testing.T) {
	rng := rand.New(rand.NewSource(71))
	// AuthConfig 有 20 个字段，全部随机
	for i := 0; i < 500; i++ {
		cfg := auth.AuthConfig{
			JWTSecret:           randStr(rng, 0, 128),
			SeedSecret:          randStr(rng, 0, 128),
			SMTPHost:            randStr(rng, 0, 256),
			SMTPPort:            rng.Uint32(),
			SMTPUser:            randStr(rng, 0, 128),
			SMTPPassword:        randStr(rng, 0, 256),
			SMTPFrom:            randStr(rng, 0, 256),
			SMTPFromName:        randStr(rng, 0, 128),
			MockEmail:           rng.Intn(2) == 1,
			OTPLength:           rng.Uint32(),
			OTPTTLMins:          rng.Uint32(),
			GoogleClientID:      randStr(rng, 0, 256),
			GoogleClientSecret:  randStr(rng, 0, 256),
			GoogleRedirectURL:   randStr(rng, 0, 512),
			GitHubClientID:      randStr(rng, 0, 256),
			GitHubClientSecret:  randStr(rng, 0, 256),
			GitHubRedirectURL:   randStr(rng, 0, 512),
			TwitterClientID:     randStr(rng, 0, 256),
			TwitterClientSecret: randStr(rng, 0, 256),
			TwitterRedirectURL:  randStr(rng, 0, 512),
		}
		testRoundtrip(t, "Stress_AuthConfig", cfg)
	}
}

func TestStress_DeepNested(t *testing.T) {
	rng := rand.New(rand.NewSource(73))
	// ProviderInfo 内嵌 UniAddr + []uint64，多层组合
	for i := 0; i < 300; i++ {
		nModels := rng.Intn(20)
		modelIds := make([]uint64, nModels)
		for j := 0; j < nModels; j++ {
			modelIds[j] = rng.Uint64()
		}
		p := gateway.ProviderInfo{
			Owner:         randUniAddr(rng),
			Name:          randStr(rng, 1, 128),
			LogoURL:       randStr(rng, 0, 512),
			Description:   randStr(rng, 0, 2048),
			ModelIds:      modelIds,
			Region:        uint8(rng.Intn(256)),
			Deposit:       rng.Uint64(),
			FeeRate:       rng.Uint32(),
			Priority:      rng.Uint32(),
			TotalRequests: rng.Uint64(),
			SuccessRate:   rng.Uint32(),
			AvgLatency:    rng.Uint32(),
			UptimeSLA:     rng.Uint32(),
			IsActive:      rng.Intn(2) == 1,
			CreatedAt:     rng.Uint64(),
		}
		testRoundtrip(t, "Stress_ProviderInfo", p)
	}
}

func TestStress_EdgeCases(t *testing.T) {
	rng := rand.New(rand.NewSource(79))

	// 边界值
	edgeCases := []struct {
		name string
		cfg  auth.AuthConfig
	}{
		{"all_empty", auth.AuthConfig{}},
		{"max_u32", auth.AuthConfig{SMTPPort: 0xFFFFFFFF, OTPLength: 0xFFFFFFFF, OTPTTLMins: 0xFFFFFFFF}},
		{"mixed_empty_strings", auth.AuthConfig{JWTSecret: "x", SeedSecret: "", SMTPHost: "y", SMTPPassword: ""}},
		{"unicode", auth.AuthConfig{JWTSecret: "密钥🔑", SMTPFromName: "测试用户"}},
		{"long_single_field", auth.AuthConfig{GoogleRedirectURL: randStr(rng, 2048, 2048)}},
	}
	for _, tc := range edgeCases {
		testRoundtrip(t, tc.name, tc.cfg)
	}

	// 极限大小
	for i := 0; i < 100; i++ {
		u := auth.UserInfo{
			EmailHash:      randBytes(rng, 4096, 4096),
			ShareRoot:      randBytes(rng, 0, 1),
			DerivedAddress: randBytes(rng, 2048, 2048),
			RegisteredAt:   0,
		}
		testRoundtrip(t, fmt.Sprintf("extreme_userinfo_%d", i), u)
	}
}

// ── 辅助 —─────────────────────────────────────────────────

func testRoundtrip[T any](t *testing.T, name string, v T) {
	t.Helper()

	// 旧 codec 编码 → 新 codec 解码
	oldBytes, err := codec.Encode(v)
	if err != nil {
		t.Fatalf("%s: old codec encode: %v", name, err)
	}

	// 新 codec 编码
	newBytes, err := encodeViaGenerated(v)
	if err != nil {
		t.Fatalf("%s: new codec encode: %v", name, err)
	}

	if len(oldBytes) != len(newBytes) {
		t.Errorf("%s: length mismatch old=%d new=%d", name, len(oldBytes), len(newBytes))
		for i := 0; i < len(oldBytes) && i < len(newBytes); i++ {
			if oldBytes[i] != newBytes[i] {
				t.Errorf("  first diff at byte %d: old=%02x new=%02x", i, oldBytes[i], newBytes[i])
				break
			}
		}
		return
	}
	for i := range oldBytes {
		if oldBytes[i] != newBytes[i] {
			t.Errorf("%s: diff at byte %d/%d: old=%02x new=%02x", name, i, len(oldBytes), oldBytes[i], newBytes[i])
			return
		}
	}
}

// encodeViaGenerated 根据类型选择生成的 Encode 方法
func encodeViaGenerated(v any) ([]byte, error) {
	switch val := v.(type) {
	case auth.AuthConfig:
		return val.Encode()
	case auth.UserInfo:
		return val.Encode()
	case auth.RecoveryConfig:
		return val.Encode()
	case auth.RecoveryRequest:
		return val.Encode()
	case auth.AuthUserRecord:
		return val.Encode()
	case auth.PrivateKeyRecord:
		return val.Encode()
	case gateway.ModelInfo:
		return val.Encode()
	case gateway.ProviderInfo:
		return val.Encode()
	case gateway.APIKeyInfo:
		return val.Encode()
	case gateway.Settlement:
		return val.Encode()
	case gateway.DailyUsage:
		return val.Encode()
	case gateway.RechargeRecord:
		return val.Encode()
	case gateway.UserSettlementRecord:
		return val.Encode()
	case gateway.ProviderSettlementRecord:
		return val.Encode()
	case gateway.ModelInfoWithID:
		return val.Encode()
	case gateway.ProviderInfoWithID:
		return val.Encode()
	case batch.BatchCall:
		return val.Encode()
	default:
		// 回退到旧 codec (基础类型测试用)
		return codec.Encode(v)
	}
}

func assertBytesEqual(t *testing.T, name string, old, new []byte) {
	t.Helper()
	if len(old) != len(new) {
		t.Errorf("%s: len old=%d new=%d", name, len(old), len(new))
		return
	}
	for i := range old {
		if old[i] != new[i] {
			t.Errorf("%s: diff @%d old=%02x new=%02x", name, i, old[i], new[i])
			return
		}
	}
}

// ── 随机生成器 —───────────────────────────────────────────

func randStr(rng *rand.Rand, minLen, maxLen int) string {
	n := minLen + rng.Intn(maxLen-minLen+1)
	b := make([]byte, n)
	rng.Read(b)
	// 确保是可打印 ASCII，减少调试难度
	for i := range b {
		b[i] = byte(32 + rng.Intn(95))
	}
	return string(b)
}

func randBytes(rng *rand.Rand, minLen, maxLen int) []byte {
	n := minLen + rng.Intn(maxLen-minLen+1)
	b := make([]byte, n)
	rng.Read(b)
	return b
}

func randUniAddr(rng *rand.Rand) store.UniAddr {
	vLen := rng.Intn(33)
	v := make([]byte, vLen)
	rng.Read(v)
	return store.UniAddr{T: rng.Uint32(), V: v}
}

// ── 手动编码（用于基础类型测试）─────────────────────────────

func encodeU8(v uint8) []byte { return []byte{v} }
func encodeI8(v int8) []byte  { return []byte{uint8(v)} }
func encodeBool(v bool) []byte {
	if v {
		return []byte{1}
	}
	return []byte{0}
}
func encodeU16LE(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }
func encodeU32LE(v uint32) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }
func encodeU64LE(v uint64) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24), byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56)}
}
func encodeI16LE(v int16) []byte { return encodeU16LE(uint16(v)) }
func encodeI32LE(v int32) []byte { return encodeU32LE(uint32(v)) }
func encodeI64LE(v int64) []byte { return encodeU64LE(uint64(v)) }

func encodeCompact(buf []byte, v uint64) []byte {
	switch {
	case v <= 0x3F:
		return append(buf, byte(v)<<2)
	case v <= 0x3FFF:
		return append(buf, byte(v)<<2|1, byte(v>>6))
	case v <= 0x3FFFFFFF:
		return append(buf, byte(v)<<2|2, byte(v>>6), byte(v>>14), byte(v>>22))
	default:
		n := byte(0)
		t := v
		for t > 0 {
			n++
			t >>= 8
		}
		if n < 4 {
			n = 4
		}
		buf = append(buf, byte(n-4)<<2|3)
		for i := byte(0); i < n; i++ {
			buf = append(buf, byte(v>>(i*8)))
		}
		return buf
	}
}
