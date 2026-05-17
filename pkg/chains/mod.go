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

// ── ChainApi ──────────────────────────────────────────────

// ChainApi is the minimal interface for any side-chain (Substrate, EVM, etc.).
type ChainApi interface {
	// ChainType returns the chain type string ("substrate", "evm", …).
	ChainType() string
	// GetCipher returns the signing suite used by this chain.
	GetCipher() model.SuiteType

	// TEECallToCall converts a TEE attestation into a chain call payload.
	TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*model.ChainCall, error)

	// Points / token tracking.
	SyncRecharge(user model.UniAddr, amount uint64, txHash []byte) error
	SyncWithdraw(user model.UniAddr, amount uint64, txHash []byte) error
	GetTotalRecharged(user model.UniAddr) (uint64, error)

	// Event sync from chain logs.
	SyncRechargeEvents(fromBlock uint64) ([]model.RechargeEvent, error)
	SyncWithdrawEvents(fromBlock uint64) ([]model.WithdrawEvent, error)

	// Signing / submission.
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
	GetEpoch() (epoch uint32, epochSlot uint32, lastEpochBlock uint32, now uint32, sideChainPub model.ChainH160, err error)
	GetNextEpochValidatorList() ([]*model.Validator, error)
	SetNewEpoch(nodeId uint64) error

	// Extrinsic construction helpers.
	TxCallOfSetNextEpoch(nodeId uint64, signer model.ChainAccountID) (*model.ChainCall, error)
	BatchCall(method string, calls []model.ChainCall) (*model.ChainCall, error)
	CheckMapAccount(addr model.ChainH160) (bool, error)
	CallOfMapAccount(signer model.ChainAccountID) (*model.ChainCall, error)
}

// ── Factory ───────────────────────────────────────────────

// ConnectMainChain creates or reuses the singleton main-chain connection.
func ConnectMainChain(env string, pk *model.PrivKey) (MainChainApi, error) {
	config := config.GetChainConfig(env)
	if config == nil {
		return nil, fmt.Errorf("invalid environment: %s", env)
	}

	var err error
	MainChain, err = connectMainChain(config, pk)
	return MainChain, err
}

// ConnectChain creates a side-chain connection.
func ConnectChain(config *model.ChainConfigType, pk *model.PrivKey) (ChainApi, error) {
	return connectChain(config, pk)
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
