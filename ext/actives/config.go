package actives

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
)

// FluxConfig holds FLUX_TEST contract configuration.
type FluxConfig struct {
	RPCURL        string `json:"rpc_url"`
	PrivateKey    string `json:"private_key"`
	TokenAddress  string `json:"token_address"`
	AdminPassword string `json:"admin_password"`
}

// LoadFluxConfig loads FLUX configuration from the given JSON file.
// If private_key is empty, generates a new key and saves it back to the file.
func LoadFluxConfig(path string) (*FluxConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[config] %s not found, FLUX chain disabled\n", path)
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg FluxConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Generate private key if empty
	if cfg.PrivateKey == "" {
		key, err := crypto.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("generate key: %w", err)
		}
		cfg.PrivateKey = hexEncodeKey(key)
		addr := crypto.PubkeyToAddress(key.PublicKey).Hex()
		fmt.Printf("[config] generated new key: %s\n", addr)

		if err := saveConfig(path, &cfg); err != nil {
			return nil, fmt.Errorf("save generated key: %w", err)
		}
		fmt.Printf("[config] saved to %s\n", path)
	}

	if cfg.RPCURL == "" || cfg.TokenAddress == "" {
		fmt.Printf("[config] incomplete (rpc_url or token_address missing), chain disabled\n")
		return nil, nil
	}

	fmt.Printf("[config] FLUX chain ready: signer=%s token=%s\n",
		addrFromKey(cfg.PrivateKey), cfg.TokenAddress)
	return &cfg, nil
}

func saveConfig(path string, cfg *FluxConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func hexEncodeKey(key *ecdsa.PrivateKey) string {
	return fmt.Sprintf("0x%x", crypto.FromECDSA(key))
}

func addrFromKey(privateKeyHex string) string {
	key, err := crypto.HexToECDSA(trim0x(privateKeyHex))
	if err != nil {
		return ""
	}
	return crypto.PubkeyToAddress(key.PublicKey).Hex()
}

func trim0x(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}
