package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	ecrypto "github.com/edgelesssys/ego/ecrypto"

	"github.com/computeflux/tee-provider/pkg/util"
)

const (
	keysDir    = "./token_chain_data/config"
	p2pKeyFile = "p2p_node_key.sealed"
	dkgKeyFile = "validator_dkg_key.sealed"
)

// GetP2PKey loads the P2P node private key from a TEE-sealed file.
// If no key exists, a new ed25519 key pair is generated and persisted.
func GetP2PKey() (*PrivKey, error) {
	return loadOrGenerateKey(filepath.Join(keysDir, p2pKeyFile), "P2P")
}

// GetValidatorKey loads the validator (DKG) private key from a TEE-sealed file.
// If no key exists, a new ed25519 key pair is generated and persisted.
func GetValidatorKey() (*PrivKey, error) {
	return loadOrGenerateKey(filepath.Join(keysDir, dkgKeyFile), "Validator")
}

// ——— internal ———

func loadOrGenerateKey(filePath, label string) (*PrivKey, error) {
	// Try to load existing key from sealed file
	data, err := os.ReadFile(filePath)
	if err == nil {
		plain, err := ecrypto.Unseal(data, nil)
		if err != nil {
			util.LogWithRed(label+"Key", "failed to unseal, regenerating:", err)
		} else {
			priv, err := PrivateKeyFromHex(string(plain))
			if err != nil {
				util.LogWithRed(label+"Key", "stored key corrupted, regenerating:", err)
			} else {
				return priv, nil
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s sealed file: %w", label, err)
	}

	// No existing key — generate a new one and persist
	util.LogWithYellow(label+"Key", "no sealed key, generating new one")

	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate %s key: %w", label, err)
	}

	priv := &PrivKey{PrivateKey: sk}
	sealed, err := ecrypto.SealWithProductKey([]byte(hex.EncodeToString(sk)), nil)
	if err != nil {
		return nil, fmt.Errorf("seal %s key: %w", label, err)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		return nil, fmt.Errorf("mkdir for %s key: %w", label, err)
	}
	if err := os.WriteFile(filePath, sealed, 0600); err != nil {
		return nil, fmt.Errorf("write %s sealed file: %w", label, err)
	}

	util.LogWithGreen(label+"Key", "generated and sealed new key",
		"file", filePath,
		"public_key", hex.EncodeToString(sk[32:]))
	return priv, nil
}
