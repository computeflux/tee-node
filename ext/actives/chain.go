package actives

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var FluxChain = &fluxChain{}

type fluxChain struct {
	mu           sync.RWMutex
	client       *ethclient.Client
	token        *FluxTest
	tokenAddr    common.Address
	auth         *bind.TransactOpts
	signerAddr   common.Address
	chainID      *big.Int
	initialized  bool
	privateKey   *ecdsa.PrivateKey
	syncDisabled bool // disable_sync: 临时关闭积分上链同步（合约数据迁移期间）
}

func InitFluxChain(configPath string) error {
	FluxChain.mu.Lock()
	defer FluxChain.mu.Unlock()
	if FluxChain.initialized {
		return nil
	}

	cfg, err := LoadFluxConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil {
		fmt.Println("[flux] disabled")
		return nil
	}

	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		client.Close()
		return fmt.Errorf("chain ID: %w", err)
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(cfg.PrivateKey, "0x"))
	if err != nil {
		client.Close()
		return fmt.Errorf("invalid key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		client.Close()
		return fmt.Errorf("transactor: %w", err)
	}

	tokenAddr := common.HexToAddress(cfg.TokenAddress)
	token, err := NewFluxTest(tokenAddr, client)
	if err != nil {
		client.Close()
		return fmt.Errorf("bind: %w", err)
	}

	FluxChain.client = client
	FluxChain.token = token
	FluxChain.tokenAddr = tokenAddr
	FluxChain.auth = auth
	FluxChain.signerAddr = crypto.PubkeyToAddress(privateKey.PublicKey)
	FluxChain.chainID = chainID
	FluxChain.initialized = true
	FluxChain.privateKey = privateKey
	FluxChain.syncDisabled = cfg.DisableSync
	if cfg.DisableSync {
		fmt.Println("[flux] sync disabled (disable_sync=true), points will NOT be minted to chain")
	}
	fmt.Printf("[flux] ready signer=%s token=%s\n", FluxChain.signerAddr.Hex(), cfg.TokenAddress)
	return nil
}

// FluxSigner returns the signer address hex, or empty if chain not initialized.
func FluxSigner() string {
	FluxChain.mu.RLock()
	defer FluxChain.mu.RUnlock()
	if !FluxChain.initialized {
		return ""
	}
	return FluxChain.signerAddr.Hex()
}

func IsFluxChainReady() bool {
	FluxChain.mu.RLock()
	defer FluxChain.mu.RUnlock()
	return FluxChain.initialized
}

// IsFluxSyncDisabled 返回积分上链同步是否被临时关闭（合约数据迁移期间）。
func IsFluxSyncDisabled() bool {
	FluxChain.mu.RLock()
	defer FluxChain.mu.RUnlock()
	return FluxChain.syncDisabled
}

// Distribute mints ERC20 tokens and sends 0.5 native token to each wallet.
// Accepts 1 or many wallets. Token amounts are points × 1e18 (18 decimals).
// syncDisabled 时直接跳过，积分只累积在本地，不 mint 上链。
func Distribute(wallets []string, points []int64) error {
	FluxChain.mu.RLock()
	defer FluxChain.mu.RUnlock()
	if !FluxChain.initialized || FluxChain.syncDisabled || len(wallets) == 0 {
		return nil
	}

	tos := make([]common.Address, len(wallets))
	amts := make([]*big.Int, len(wallets))
	for i, w := range wallets {
		tos[i] = common.HexToAddress(w)
		if i < len(points) {
			amts[i] = new(big.Int).Mul(big.NewInt(points[i]), big.NewInt(1e18))
		} else {
			amts[i] = big.NewInt(0)
		}
	}

	tx, err := FluxChain.token.BatchMint(FluxChain.auth, tos, amts)
	if err != nil {
		return fmt.Errorf("batchMint: %w", err)
	}
	fmt.Printf("[flux] batch %d users tx=%s\n", len(tos), tx.Hash().Hex())
	return nil
}

// MintToken mints ERC20 FLUX_TEST tokens (and sends 0.5 native) to an address.
// Uses batchMint with a single-element array. Amount is in whole token units.
func MintToken(to string, amount int64) error {
	return Distribute([]string{to}, []int64{amount})
}

// BurnToken burns ERC20 FLUX_TEST tokens from an address on-chain (only the contract owner can call).
// amount 为要销毁的代币数量（整数，0 表示跳过）。用于链上减少被惩罚（封禁）账户的余额。
// 返回是否真正发起了 burn 交易（amount<=0 或链未初始化/同步关闭时返回 false, nil）。
func BurnToken(user string, amount int64) (bool, error) {
	FluxChain.mu.RLock()
	defer FluxChain.mu.RUnlock()
	if !FluxChain.initialized || FluxChain.syncDisabled {
		return false, nil
	}
	if amount <= 0 {
		return false, nil
	}

	addr := common.HexToAddress(user)
	amt := new(big.Int).Mul(big.NewInt(amount), big.NewInt(1e18)) // 18 decimals
	tx, err := FluxChain.token.Burn(FluxChain.auth, addr, amt)
	if err != nil {
		return false, fmt.Errorf("burn: %w", err)
	}
	fmt.Printf("[flux] burn user=%s amount=%d tx=%s\n", addr.Hex(), amount, tx.Hash().Hex())
	return true, nil
}
