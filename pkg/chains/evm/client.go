package evm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model"
)

// EvmChain implements ChainApi + MainChainApi for EVM chains.
type EvmChain struct {
	cipher     model.SuiteType
	urls       []string
	mu         sync.RWMutex
	client     *ethclient.Client
	token      *bindings.Token
	tokenAddr  common.Address
	subnet     *bindings.Subnet
	subnetAddr common.Address
	chainID    *big.Int
	chainName  string
	privateKey *ecdsa.PrivateKey
	auth       *bind.TransactOpts
	signerAddr string
}

// ── Constructor ───────────────────────────────────────────

func NewContract(config *model.ChainConfigType, pk *model.PrivKey) (*EvmChain, error) {
	if len(config.Urls) == 0 {
		return nil, fmt.Errorf("no RPC URLs configured")
	}

	client, err := dialFirst(config.Urls)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("chain ID: %w", err)
	}

	signer := pk.ToSigner(config.Cipher)
	if signer == nil {
		return nil, fmt.Errorf("unsupported cipher: %v", config.Cipher)
	}

	var ecdsaKey *ecdsa.PrivateKey
	if skSigner, ok := signer.(*model.Secp256k1Signer); ok {
		ecdsaKey = skSigner.PrivateKey()
	}
	if ecdsaKey == nil {
		return nil, fmt.Errorf("failed to extract secp256k1 private key from signer")
	}

	auth, err := bind.NewKeyedTransactorWithChainID(ecdsaKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("transactor: %w", err)
	}

	tAddr := common.HexToAddress(config.TokenAddress)
	token, err := bindings.NewToken(tAddr, client)
	if err != nil {
		return nil, fmt.Errorf("bind Token: %w", err)
	}

	sAddr := common.HexToAddress(config.SubnetAddress)
	subnet, err := bindings.NewSubnet(sAddr, client)
	if err != nil {
		return nil, fmt.Errorf("bind Subnet: %w", err)
	}

	return &EvmChain{
		urls:       config.Urls,
		client:     client,
		token:      token,
		tokenAddr:  tAddr,
		subnet:     subnet,
		subnetAddr: sAddr,
		chainID:    chainID,
		chainName:  config.NetworkLabel,
		privateKey: ecdsaKey,
		cipher:     config.Cipher,
		auth:       auth,
		signerAddr: crypto.PubkeyToAddress(ecdsaKey.PublicKey).Hex(),
	}, nil
}

// ── Failover ──────────────────────────────────────────────

// dialFirst tries each URL in order, returning the first successful connection.
func dialFirst(urls []string) (*ethclient.Client, error) {
	var lastErr error
	for _, u := range urls {
		client, err := ethclient.Dial(u)
		if err == nil {
			return client, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all URLs failed: %w", lastErr)
}

// reconnect cycles URLs and rebinds contracts on failure.
func (e *EvmChain) reconnect() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, url := range e.urls {
		client, err := ethclient.Dial(url)
		if err != nil {
			continue
		}
		id, err := client.ChainID(context.Background())
		if err != nil || id.Cmp(e.chainID) != 0 {
			client.Close()
			continue
		}
		token, err := bindings.NewToken(e.tokenAddr, client)
		if err != nil {
			client.Close()
			continue
		}
		subnet, err := bindings.NewSubnet(e.subnetAddr, client)
		if err != nil {
			client.Close()
			continue
		}
		e.client.Close()
		e.client = client
		e.token = token
		e.subnet = subnet
		return nil
	}
	return fmt.Errorf("reconnect: all %d URLs failed", len(e.urls))
}

// ── Identity ──────────────────────────────────────────────

func (e *EvmChain) ChainType() string          { return "evm" }
func (e *EvmChain) GetCipher() model.SuiteType { return e.cipher }
func (e *EvmChain) GetChainUrls() []string     { return e.urls }
func (e *EvmChain) GetSignerAddress() string   { return e.signerAddr }

func (e *EvmChain) GetClient() *model.ChainClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return &model.ChainClient{EVM: e.client}
}
