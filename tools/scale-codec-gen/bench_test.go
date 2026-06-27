package main

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── Benchmark: Encode —───────────────────────────────────

func BenchmarkEncode_AuthConfig_Old(b *testing.B) {
	cfg := fixedAuthConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(cfg)
	}
}

func BenchmarkEncode_AuthConfig_New(b *testing.B) {
	cfg := fixedAuthConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Encode()
	}
}

func BenchmarkEncode_UserInfo_Old(b *testing.B) {
	u := auth.UserInfo{EmailHash: []byte{1, 2, 3, 4, 5}, ShareRoot: []byte{6, 7, 8}, DerivedAddress: []byte{9, 10, 11, 12}, RegisteredAt: 12345}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(u)
	}
}

func BenchmarkEncode_UserInfo_New(b *testing.B) {
	u := auth.UserInfo{EmailHash: []byte{1, 2, 3, 4, 5}, ShareRoot: []byte{6, 7, 8}, DerivedAddress: []byte{9, 10, 11, 12}, RegisteredAt: 12345}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.Encode()
	}
}

func BenchmarkEncode_ModelInfo_Old(b *testing.B) {
	m := fixedModelInfo()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(m)
	}
}

func BenchmarkEncode_ModelInfo_New(b *testing.B) {
	m := fixedModelInfo()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Encode()
	}
}

func BenchmarkEncode_ProviderInfo_Old(b *testing.B) {
	p := fixedProviderInfo()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(p)
	}
}

func BenchmarkEncode_ProviderInfo_New(b *testing.B) {
	p := fixedProviderInfo()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Encode()
	}
}

func BenchmarkEncode_BatchCall_Old(b *testing.B) {
	bc := batch.BatchCall{Name: []byte("gateway"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{5, 6, 7}, {8, 9}, {10, 11, 12, 13}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(bc)
	}
}

func BenchmarkEncode_BatchCall_New(b *testing.B) {
	bc := batch.BatchCall{Name: []byte("gateway"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{5, 6, 7}, {8, 9}, {10, 11, 12, 13}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bc.Encode()
	}
}

// ── Benchmark: Decode —───────────────────────────────────

func BenchmarkDecode_AuthConfig_Old(b *testing.B) {
	cfg := fixedAuthConfig()
	data, _ := codec.Encode(cfg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v auth.AuthConfig
		codec.Decode(data, &v)
	}
}

func BenchmarkDecode_AuthConfig_New(b *testing.B) {
	cfg := fixedAuthConfig()
	data, _ := cfg.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v auth.AuthConfig
		v.Decode(data)
	}
}

func BenchmarkDecode_UserInfo_Old(b *testing.B) {
	u := auth.UserInfo{EmailHash: []byte{1, 2, 3, 4, 5}, ShareRoot: []byte{6, 7, 8}, DerivedAddress: []byte{9, 10, 11, 12}, RegisteredAt: 12345}
	data, _ := codec.Encode(u)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v auth.UserInfo
		codec.Decode(data, &v)
	}
}

func BenchmarkDecode_UserInfo_New(b *testing.B) {
	u := auth.UserInfo{EmailHash: []byte{1, 2, 3, 4, 5}, ShareRoot: []byte{6, 7, 8}, DerivedAddress: []byte{9, 10, 11, 12}, RegisteredAt: 12345}
	data, _ := u.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v auth.UserInfo
		v.Decode(data)
	}
}

func BenchmarkDecode_ModelInfo_Old(b *testing.B) {
	m := fixedModelInfo()
	data, _ := codec.Encode(m)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v gateway.ModelInfo
		codec.Decode(data, &v)
	}
}

func BenchmarkDecode_ModelInfo_New(b *testing.B) {
	m := fixedModelInfo()
	data, _ := m.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v gateway.ModelInfo
		v.Decode(data)
	}
}

func BenchmarkDecode_ProviderInfo_Old(b *testing.B) {
	p := fixedProviderInfo()
	data, _ := codec.Encode(p)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v gateway.ProviderInfo
		codec.Decode(data, &v)
	}
}

func BenchmarkDecode_ProviderInfo_New(b *testing.B) {
	p := fixedProviderInfo()
	data, _ := p.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v gateway.ProviderInfo
		v.Decode(data)
	}
}

func BenchmarkDecode_BatchCall_Old(b *testing.B) {
	bc := batch.BatchCall{Name: []byte("gateway"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{5, 6, 7}, {8, 9}, {10, 11, 12, 13}}}
	data, _ := codec.Encode(bc)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v batch.BatchCall
		codec.Decode(data, &v)
	}
}

func BenchmarkDecode_BatchCall_New(b *testing.B) {
	bc := batch.BatchCall{Name: []byte("gateway"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{5, 6, 7}, {8, 9}, {10, 11, 12, 13}}}
	data, _ := bc.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v batch.BatchCall
		v.Decode(data)
	}
}

// ── Benchmark: 大字段 (模拟真实负载) ────────────────────────

func BenchmarkEncode_LargeConfig_Old(b *testing.B) {
	cfg := largeAuthConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(cfg)
	}
}

func BenchmarkEncode_LargeConfig_New(b *testing.B) {
	cfg := largeAuthConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Encode()
	}
}

func BenchmarkDecode_LargeConfig_Old(b *testing.B) {
	cfg := largeAuthConfig()
	data, _ := codec.Encode(cfg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v auth.AuthConfig
		codec.Decode(data, &v)
	}
}

func BenchmarkDecode_LargeConfig_New(b *testing.B) {
	cfg := largeAuthConfig()
	data, _ := cfg.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v auth.AuthConfig
		v.Decode(data)
	}
}

// ── Benchmark: 并行 —──────────────────────────────────────

func BenchmarkEncode_Parallel_AuthConfig_Old(b *testing.B) {
	cfg := fixedAuthConfig()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			codec.Encode(cfg)
		}
	})
}

func BenchmarkEncode_Parallel_AuthConfig_New(b *testing.B) {
	cfg := fixedAuthConfig()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cfg.Encode()
		}
	})
}

func BenchmarkDecode_Parallel_AuthConfig_Old(b *testing.B) {
	cfg := fixedAuthConfig()
	data, _ := codec.Encode(cfg)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var v auth.AuthConfig
			codec.Decode(data, &v)
		}
	})
}

func BenchmarkDecode_Parallel_AuthConfig_New(b *testing.B) {
	cfg := fixedAuthConfig()
	data, _ := cfg.Encode()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var v auth.AuthConfig
			v.Decode(data)
		}
	})
}

// ── 固定数据 —─────────────────────────────────────────────

func fixedAuthConfig() auth.AuthConfig {
	return auth.AuthConfig{
		JWTSecret: "abc123def456", SeedSecret: "seed789",
		SMTPHost: "smtp.example.com", SMTPPort: 587,
		SMTPUser: "noreply", SMTPPassword: "securepass",
		SMTPFrom: "noreply@example.com", SMTPFromName: "Example",
		MockEmail: false, OTPLength: 6, OTPTTLMins: 5,
		GoogleClientID: "google-id-123", GoogleClientSecret: "google-secret",
		GoogleRedirectURL: "https://example.com/oauth/google",
		GitHubClientID:    "github-id-456", GitHubClientSecret: "github-secret",
		GitHubRedirectURL: "https://example.com/oauth/github",
		TwitterClientID:   "twitter-id-789", TwitterClientSecret: "twitter-secret",
		TwitterRedirectURL: "https://example.com/oauth/twitter",
	}
}

func fixedModelInfo() gateway.ModelInfo {
	return gateway.ModelInfo{
		Name: "gpt-4-turbo", Owner: store.UniAddr{T: 1, V: []byte{0xaa, 0xbb, 0xcc}},
		ProviderID: 42, InputModalities: 3, ContextLength: 128000,
		InPrice: 1000, CachedPrice: 500, OutPrice: 3000,
		ProviderType: 1, ModelID: "gpt-4-turbo-preview",
		URI: "https://api.openai.com/v1", APIKeys: []string{"sk-abc123xyz"},
		IsActive: true, CreatedAt: 1700000000,
	}
}

func fixedProviderInfo() gateway.ProviderInfo {
	return gateway.ProviderInfo{
		Owner: store.UniAddr{T: 1, V: []byte{0x11, 0x22, 0x33}},
		Name:  "OpenAI", LogoURL: "https://cdn.example.com/logo.png",
		Description: "Leading AI provider with state-of-the-art models",
		ModelIds:    []uint64{1, 2, 3, 4, 5},
		Region:      0x01, Deposit: 1000000, FeeRate: 100,
		Priority: 10, TotalRequests: 999999, SuccessRate: 9950,
		AvgLatency: 200, UptimeSLA: 9990, IsActive: true, CreatedAt: 1600000000,
	}
}

func largeAuthConfig() auth.AuthConfig {
	rng := rand.New(rand.NewSource(42))
	return auth.AuthConfig{
		JWTSecret:           randStr(rng, 256, 256),
		SeedSecret:          randStr(rng, 256, 256),
		SMTPHost:            randStr(rng, 128, 128),
		SMTPPort:            587,
		SMTPUser:            randStr(rng, 64, 64),
		SMTPPassword:        randStr(rng, 128, 128),
		SMTPFrom:            randStr(rng, 128, 128),
		SMTPFromName:        randStr(rng, 64, 64),
		MockEmail:           true,
		OTPLength:           6,
		OTPTTLMins:          5,
		GoogleClientID:      randStr(rng, 128, 128),
		GoogleClientSecret:  randStr(rng, 128, 128),
		GoogleRedirectURL:   randStr(rng, 256, 256),
		GitHubClientID:      randStr(rng, 128, 128),
		GitHubClientSecret:  randStr(rng, 128, 128),
		GitHubRedirectURL:   randStr(rng, 256, 256),
		TwitterClientID:     randStr(rng, 128, 128),
		TwitterClientSecret: randStr(rng, 128, 128),
		TwitterRedirectURL:  randStr(rng, 256, 256),
	}
}

// ── 性能对比报告 —──────────────────────────────────────────

func TestPrintPerformanceComparison(t *testing.T) {
	// Encode
	t.Run("encode", func(t *testing.T) {
		types := []struct {
			name    string
			oldFunc func() []byte
			newFunc func() []byte
		}{
			{"AuthConfig", func() []byte { b, _ := codec.Encode(fixedAuthConfig()); return b }, func() []byte { c := fixedAuthConfig(); b, _ := c.Encode(); return b }},
			{"UserInfo", func() []byte {
				b, _ := codec.Encode(auth.UserInfo{EmailHash: []byte{1, 2, 3}, ShareRoot: []byte{4, 5, 6}, DerivedAddress: []byte{7, 8}, RegisteredAt: 99})
				return b
			}, func() []byte {
				u := auth.UserInfo{EmailHash: []byte{1, 2, 3}, ShareRoot: []byte{4, 5, 6}, DerivedAddress: []byte{7, 8}, RegisteredAt: 99}
				b, _ := u.Encode()
				return b
			}},
			{"ModelInfo", func() []byte { b, _ := codec.Encode(fixedModelInfo()); return b }, func() []byte { m := fixedModelInfo(); b, _ := m.Encode(); return b }},
			{"ProviderInfo", func() []byte { b, _ := codec.Encode(fixedProviderInfo()); return b }, func() []byte { p := fixedProviderInfo(); b, _ := p.Encode(); return b }},
			{"BatchCall", func() []byte {
				b, _ := codec.Encode(batch.BatchCall{Name: []byte("g"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{1}, {2}}})
				return b
			}, func() []byte {
				bc := batch.BatchCall{Name: []byte("g"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{1}, {2}}}
				b, _ := bc.Encode()
				return b
			}},
		}

		fmt.Println("\n=== Encode 性能对比 (ns/op) ===")
		fmt.Printf("%-20s %12s %12s %8s\n", "类型", "旧(codec)", "新(gen)", "提升")
		for _, tc := range types {
			oldR := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					tc.oldFunc()
				}
			})
			newR := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					tc.newFunc()
				}
			})
			ratio := float64(oldR.NsPerOp()) / float64(newR.NsPerOp())
			fmt.Printf("%-20s %12d %12d %7.1fx\n", tc.name, oldR.NsPerOp(), newR.NsPerOp(), ratio)
		}
	})

	// Decode
	t.Run("decode", func(t *testing.T) {
		fmt.Println("\n=== Decode 性能对比 (ns/op) ===")
		fmt.Printf("%-20s %12s %12s %8s\n", "类型", "旧(codec)", "新(gen)", "提升")

		// AuthConfig
		cfg := fixedAuthConfig()
		data, _ := cfg.Encode()
		oldR := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var v auth.AuthConfig
				codec.Decode(data, &v)
			}
		})
		newR := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var v auth.AuthConfig
				v.Decode(data)
			}
		})
		fmt.Printf("%-20s %12d %12d %7.1fx\n", "AuthConfig", oldR.NsPerOp(), newR.NsPerOp(), float64(oldR.NsPerOp())/float64(newR.NsPerOp()))

		// ModelInfo
		m := fixedModelInfo()
		data2, _ := m.Encode()
		oldR2 := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var v gateway.ModelInfo
				codec.Decode(data2, &v)
			}
		})
		newR2 := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var v gateway.ModelInfo
				v.Decode(data2)
			}
		})
		fmt.Printf("%-20s %12d %12d %7.1fx\n", "ModelInfo", oldR2.NsPerOp(), newR2.NsPerOp(), float64(oldR2.NsPerOp())/float64(newR2.NsPerOp()))

		// ProviderInfo
		p := fixedProviderInfo()
		data3, _ := p.Encode()
		oldR3 := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var v gateway.ProviderInfo
				codec.Decode(data3, &v)
			}
		})
		newR3 := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var v gateway.ProviderInfo
				v.Decode(data3)
			}
		})
		fmt.Printf("%-20s %12d %12d %7.1fx\n", "ProviderInfo", oldR3.NsPerOp(), newR3.NsPerOp(), float64(oldR3.NsPerOp())/float64(newR3.NsPerOp()))
	})
}
