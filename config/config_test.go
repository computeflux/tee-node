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
	// 当前实现为占位，返回空切片；任何环境都不应返回 nil 之外的异常
	for _, env := range []string{"local", "test", "main", "unknown"} {
		cfgs := GetChainConfigs(env)
		if cfgs == nil {
			t.Errorf("GetChainConfigs(%q) returned nil, want empty slice", env)
		}
	}
}
