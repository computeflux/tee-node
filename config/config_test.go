package config

import (
	"testing"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
)

func TestGetMainChainConfig_Local(t *testing.T) {
	cfg := GetMainChainConfig("local")
	if cfg == nil {
		t.Fatal("GetMainChainConfig(local) = nil")
	}
	if cfg.ChainType != "substrate" {
		t.Errorf("ChainType = %q, want substrate", cfg.ChainType)
	}
	if len(cfg.Urls) == 0 || cfg.Urls[0] == "" {
		t.Error("local config should have non-empty Urls")
	}
	if cfg.NetworkLabel != "Local" {
		t.Errorf("NetworkLabel = %q, want Local", cfg.NetworkLabel)
	}
}

func TestGetMainChainConfig_Test(t *testing.T) {
	cfg := GetMainChainConfig("test")
	if cfg == nil {
		t.Fatal("GetMainChainConfig(test) = nil")
	}
	if cfg.ChainType != "substrate" {
		t.Errorf("ChainType = %q, want substrate", cfg.ChainType)
	}
	if cfg.Cipher != crypto.SuiteEd25519 {
		t.Errorf("Cipher = %v, want SuiteEd25519", cfg.Cipher)
	}
	if cfg.NetworkLabel != "Paseo (Asset Hub)" {
		t.Errorf("NetworkLabel = %q", cfg.NetworkLabel)
	}
}

func TestGetMainChainConfig_Main(t *testing.T) {
	cfg := GetMainChainConfig("main")
	if cfg == nil {
		t.Fatal("GetMainChainConfig(main) = nil")
	}
	if cfg.NetworkLabel != "Polkadot (Asset Hub)" {
		t.Errorf("NetworkLabel = %q", cfg.NetworkLabel)
	}
}

func TestGetMainChainConfig_UnknownEnv(t *testing.T) {
	for _, env := range []string{"", "prod", "unknown", "MAIN"} {
		if cfg := GetMainChainConfig(env); cfg != nil {
			t.Errorf("GetMainChainConfig(%q) = %+v, want nil", env, cfg)
		}
	}
}

func TestGetMainChainConfig_ReturnsCopy(t *testing.T) {
	// 修改返回值不应污染全局配置
	cfg := GetMainChainConfig("local")
	cfg.Urls[0] = "wss://hacked.example.com"
	cfg.NetworkLabel = "Hacked"

	again := GetMainChainConfig("local")
	if again.NetworkLabel == "Hacked" {
		t.Error("mutating the returned config polluted the global LocalChainConfig")
	}
	if again.Urls[0] == "wss://hacked.example.com" {
		t.Error("mutating the returned Urls polluted the global LocalChainConfig")
	}
}

func TestGetChainConfigs(t *testing.T) {
	// test：Base Sepolia 地址已写死启用；main：主网合约未部署（地址空）不启用。
	// 任何环境都不应返回 nil。
	for _, env := range []string{"local", "test", "main", "unknown"} {
		cfgs := GetChainConfigs(env)
		if cfgs == nil {
			t.Errorf("GetChainConfigs(%q) returned nil, want empty slice", env)
		}
	}
	if cfgs := GetChainConfigs("test"); len(cfgs) != 1 || cfgs[0].ChainID != 84532 {
		t.Errorf("GetChainConfigs(test) = %+v, want one Base Sepolia chain", cfgs)
	}
	if cfgs := GetChainConfigs("main"); len(cfgs) != 0 {
		t.Errorf("GetChainConfigs(main) = %+v, want empty (mainnet contracts not deployed)", cfgs)
	}
}

func TestGetChainConfigs_BaseAddressesHardcoded(t *testing.T) {
	// 环境变量不得影响链配置（防注入）：地址全部写死在 config。
	t.Setenv("BASE_SEPOLIA_TOKEN_ADDRESS", "0x1111111111111111111111111111111111111111")
	t.Setenv("BASE_SEPOLIA_BLS_ADDRESS", "0x2222222222222222222222222222222222222222")
	t.Setenv("BASE_TOKEN_ADDRESS", "0x3333333333333333333333333333333333333333")
	t.Setenv("BASE_BLS_ADDRESS", "0x4444444444444444444444444444444444444444")

	cfgs := GetChainConfigs("test")
	if len(cfgs) != 1 || cfgs[0].ChainID != 84532 {
		t.Fatalf("GetChainConfigs(test) = %+v, want one Base Sepolia chain", cfgs)
	}
	if cfgs[0].TokenAddress != "0xC7701fC65125C0a5cdBc760806F8fCf33D2aD2E0" ||
		cfgs[0].BLSAddress != "0x2921A9385F94A78572A33C2590C94B93A5Ee2841" {
		t.Errorf("Base Sepolia addresses must be hardcoded, got token=%s bls=%s", cfgs[0].TokenAddress, cfgs[0].BLSAddress)
	}
	if cfgs[0].Urls[0] != "https://sepolia.base.org" {
		t.Errorf("Base Sepolia RPC must be hardcoded, got %s", cfgs[0].Urls[0])
	}

	// 修改返回的 Urls 不应污染全局配置
	cfgs[0].Urls[0] = "https://tampered.example"
	if BaseTestChainConfig.Urls[0] == "https://tampered.example" {
		t.Error("GetChainConfigs(test) returned a shallow copy")
	}
}
