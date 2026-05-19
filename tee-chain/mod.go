package teechain

import (
	"errors"
	"fmt"
	"os"
	"strings"

	cfg "github.com/cometbft/cometbft/config"
	cmtflags "github.com/cometbft/cometbft/libs/cli/flags"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	"github.com/cometbft/cometbft/version"
	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	bftbrigde "github.com/computeflux/tee-provider/pkg/network/bft-brigde"
	"github.com/computeflux/tee-provider/pkg/util"
)

var TeeChainNode *nm.Node
var P2PKey *model.PubKey

// init side chain
func InitTeeChain(
	nodePriv *model.PrivKey,
	chainPort int,
	light bool,
	callback func(),
) (func() (*nm.Node, error), *TeeChain, *bftbrigde.BTFReactor, error) {
	if chains.MainChain == nil {
		return nil, nil, nil, errors.New("MainChain is nil")
	}

	// Get boot peers
	boots, err := chains.MainChain.GetBootPeers()
	if err != nil {
		return nil, nil, nil, errors.New("GetBootPeers error: " + err.Error())
	}

	// 创建侧链实例
	teeChain, err := NewTeeChain(light)
	if err != nil {
		return nil, nil, nil, errors.New("NewTeeChain error: " + err.Error())
	}

	addr := util.GetEnv("SIDE_CHAIN_ADDR", "0.0.0.0")
	p2pConf := cfg.DefaultP2PConfig()
	p2pConf.ListenAddress = "tcp://" + addr + ":" + fmt.Sprint(chainPort)
	p2pConf.AllowDuplicateIP = true
	p2pConf.Seeds = ""

	consensusConf := cfg.DefaultConsensusConfig()
	consensusConf.CreateEmptyBlocks = false

	rpcConf := cfg.DefaultRPCConfig()
	rpcConf.CORSAllowedOrigins = []string{"*"}
	rpcConf.ListenAddress = "tcp://0.0.0.0:" + fmt.Sprint(chainPort+1)
	// rpcConf.ListenAddress = ""
	rpcConf.TLSCertFile = "/token_chain_data/ssl/ser.pem"
	rpcConf.TLSKeyFile = "/token_chain_data/ssl/ser.key"

	// init BFT node config
	config := &cfg.Config{
		BaseConfig: cfg.BaseConfig{
			Version:            version.TMCoreSemVer,
			Genesis:            "config/genesis.json",
			PrivValidatorKey:   "config/priv_validator_key.json",
			PrivValidatorState: "data/priv_validator_state.json",
			NodeKey:            "config/node_key.json",
			Moniker:            "WeTEE Chain",
			ProxyApp:           "tcp://127.0.0.1:" + fmt.Sprint(chainPort+1),
			ABCI:               "socket",
			LogLevel:           "error",
			LogFormat:          cfg.LogFormatPlain,
			FilterPeers:        false,
			DBBackend:          "goleveldb",
			DBPath:             "BFT",
		},
		RPC:             rpcConf,
		P2P:             p2pConf,
		Mempool:         cfg.DefaultMempoolConfig(),
		StateSync:       cfg.DefaultStateSyncConfig(),
		BlockSync:       cfg.DefaultBlockSyncConfig(),
		Consensus:       consensusConf,
		Storage:         cfg.DefaultStorageConfig(),
		TxIndex:         cfg.DefaultTxIndexConfig(),
		Instrumentation: cfg.DefaultInstrumentationConfig(),
	}
	config.SetRoot("./token_chain_data")

	// init teechain node key
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, nil, nil, errors.New("failed to load node key: " + err.Error())
	}

	_, p2pKey, _ := model.GetP2PKey()
	P2PKey = p2pKey.GetPublicForSuite(model.SuiteEd25519)

	// add boot nodes
	seeds := []string{}
	for _, boot := range boots {
		if util.ToTeeChainNodeID(boot.Id[:]) == nodeKey.ID() {
			continue
		}
		seeds = append(seeds, boot.TeeChainUrl())
	}
	config.P2P.Seeds = strings.Join(seeds, ",")

	util.LogWithYellow("PersistentPeers", strings.Join(seeds, ","))

	// 将 boot 节点同时设置为 PersistentPeers，确保断开后自动重连
	config.P2P.PersistentPeers = strings.Join(seeds, ",")

	// load validator key
	validatorKey := privval.LoadFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	// init logger
	logger := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout))
	logger, err = cmtflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel)
	if err != nil {
		return nil, nil, nil, errors.New("init logger error: " + err.Error())
	}

	// add DKG to chain node
	p2pReactor := bftbrigde.NewBTFReactor("DKG")
	if P2PKey != nil {
		p2pReactor.SetID(P2PKey)
	}

	var StartTeeChain = func() (*nm.Node, error) {
		// init BFT node
		TeeChainNode, err = nm.NewNode(
			config,
			validatorKey,
			nodeKey,
			proxy.NewLocalClientCreator(teeChain),
			nm.DefaultGenesisDocProviderFunc(config),
			cfg.DefaultDBProvider,
			nm.DefaultMetricsProvider(config.Instrumentation),
			logger,
			nm.CustomReactors(map[string]p2p.Reactor{
				"DKG": p2pReactor,
			}),
		)

		if err != nil {
			return nil, errors.New("init BFT node error: " + err.Error())
		}

		return TeeChainNode, nil
	}

	// call callback function
	callback()

	teeChain.p2p = p2pReactor
	teeChain.NodePriv = nodePriv
	if !light {
		// add hook for partial sign
		p2pReactor.Sub("block-partial-sign", teeChain.revPartialSign)
		go teeChain.txCh.Start(teeChain.handlePartialSign)
	}
	p2pReactor.Sub("secret", teeChain.revSecret)

	return StartTeeChain, teeChain, p2pReactor, err
}

func (s *TeeChain) SetDKG(dkg *dkg.DKG) {
	s.dkg = dkg
}

func (s *TeeChain) GetDKG() *dkg.DKG {
	return s.dkg
}
