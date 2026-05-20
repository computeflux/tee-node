package chains

import (
	"fmt"
	"os"

	"github.com/computeflux/tee-provider/config"
	"github.com/computeflux/tee-provider/pkg/chains/evm"
	"github.com/computeflux/tee-provider/pkg/chains/revive"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/wetee-dao/ink.go"
)

var MainChain MainChainApi
var Chains []ChainApi

// ── ChainApi ──────────────────────────────────────────────

// ChainApi is the minimal interface for any side-chain (Substrate, EVM, …).
type ChainApi interface {
	ChainType() string
	ChainId() uint32
	GetCipher() model.SuiteType
	TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*model.ChainCall, error)

	// Relay 同步 (替代旧的 SyncRecharge/SyncWithdraw/Events)
	// 主链合约 emit 事件 → relayer 拉取 → 侧链 relay engine 验证执行
	GetLatestNonce() (uint64, error)
	GetRelayEvents(from, to uint64) ([]model.RelayEvent, error)

	PartialSign(signer ink.PartialSignerType, call model.ChainCall) ([]byte, error)
	SignAndSubmit(signer ink.SignerType, call model.ChainCall, untilFinalized bool, nonce uint64) error
}

// ── MainChainApi ──────────────────────────────────────────

// MainChainApi extends ChainApi with methods needed for the main / governance chain.
type MainChainApi interface {
	ChainApi

	GetChainUrls() []string
	GetSignerAddress() string

	// Node / validator discovery.
	GetBootPeers() ([]model.P2PAddr, error)
	GetNodes() ([]*model.Validator, []*model.PubKey, error)
	GetValidatorList() ([]*model.Validator, error)

	// Epoch management.
	GetEpoch() (epoch uint32, epochSlot uint32, lastEpochBlock uint32, now uint32, teeChainPub model.PubKey, err error)
	GetNextEpochValidatorList() ([]*model.Validator, error)
	SetNewEpoch(nodeId uint64) error

	// Extrinsic construction helpers.
	TxCallOfSetNextEpoch(nodeId uint64, signer model.PubKey) (*model.ChainCall, error)
	BatchCall(method string, calls []model.ChainCall) (*model.ChainCall, error)
	CheckMapAccount(addr model.PubKey) (bool, error)
	CallOfMapAccount(signer model.PubKey) (*model.ChainCall, error)
}

// ── Factory ───────────────────────────────────────────────

// ConnectMainChain creates or reuses the singleton main-chain connection.
func ConnectMainChain(env string, pk *model.PrivKey) (MainChainApi, error) {
	config := config.GetMainChainConfig(env)
	if config == nil {
		return nil, fmt.Errorf("invalid environment: %s", env)
	}

	var err error
	MainChain, err = connectMainChain(config, pk)
	return MainChain, err
}

// ConnectChain creates a side-chain connection.
func ConnectChain(env string, pk *model.PrivKey) {
	var chains []ChainApi = []ChainApi{MainChain}
	chainConfigs := config.GetChainConfigs(env)
	for _, chainConfig := range chainConfigs {
		chain, err := connectChain(&chainConfig, pk)
		if err != nil {
			util.LogError("CHAIN", "failed to connect chain", chainConfig.NetworkLabel, err)
			continue
		}
		chains = append(chains, chain)
	}
	Chains = chains
}

// connectMainChain creates a MainChainApi for the given config.
func connectMainChain(config *model.ChainConfigType, pk *model.PrivKey) (MainChainApi, error) {
	switch config.ChainType {
	case "evm":
		return evm.NewContract(config, pk)
	case "substrate":
		return revive.NewContract(config, pk)
	default:
		util.LogError("CHAIN", "unsupported chain type", config.ChainType)
		os.Exit(1)
	}
	return nil, fmt.Errorf("unsupported chain type: %s", config.ChainType)
}

func GetChainByID(chainId uint32) (ChainApi, error) {
	for _, chain := range Chains {
		if chain.ChainId() == chainId {
			return chain, nil
		}
	}
	return nil, fmt.Errorf("chain not found: %d", chainId)
}

// connectChain creates a ChainApi for the given config.
func connectChain(config *model.ChainConfigType, pk *model.PrivKey) (ChainApi, error) {
	switch config.ChainType {
	case "evm":
		return evm.NewContract(config, pk)
	case "substrate":
		return revive.NewContract(config, pk)
	default:
		util.LogError("CHAIN", "unsupported chain type", config.ChainType)
		os.Exit(1)
	}
	return nil, fmt.Errorf("unsupported chain type: %s", config.ChainType)
}
