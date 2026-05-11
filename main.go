package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cometbft/cometbft/privval"
	"github.com/wetee-dao/tee-dsecret/graph"
	chain "github.com/wetee-dao/tee-dsecret/pkg/chains"
	"github.com/wetee-dao/tee-dsecret/pkg/dkg"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
	sidechain "github.com/wetee-dao/tee-dsecret/side-chain"
)

var DefaultChainUrl string = "ws://wetee-node.worker-addon.svc.cluster.local:9944"

func main() {
	// 获取环境变量
	gqlPort := util.GetEnvInt("GQL_PORT", 61000)
	chainEnv := util.GetEnv("CHAIN_ENV", "local")
	chainPort := util.GetEnvInt("SIDE_CHAIN_PORT", 61001)

	// Init app db
	db, err := model.NewDB()
	if err != nil {
		fmt.Println("Init db error:", err)
		os.Exit(1)
	}
	defer db.Close()

	// init sidechain node key
	_, p2pKey, err := model.GetP2PKey()
	if err != nil {
		fmt.Println("failed to load node key:", err)
		os.Exit(1)
	}

	// Init key for DKG
	validatorKey := privval.LoadFilePV(
		"./chain_data/config/priv_validator_key.json",
		"./chain_data/data/priv_validator_state.json",
	)
	dkgKey := validatorKey.Key.PrivKey

	// Init node key for Mainchain
	nodePriv, err := model.PrivateKeyFromOed25519(dkgKey.Bytes())
	if err != nil {
		fmt.Println("Marshal PKG_PK error:", err)
		os.Exit(1)
	}

	util.LogWithYellow("Mainchain Key", nodePriv.GetPublic().SS58())
	util.LogWithYellow("P2P Key", p2pKey.GetPublic().SS58())

	chainConfig := model.GetChainConfig(chainEnv)
	if chainConfig == nil {
		fmt.Println("Invalid chain environment:", chainEnv)
		os.Exit(1)
	}

	// Link to polkadot
	_, err = chain.ConnectMainChain(chainEnv, nodePriv)
	if err != nil {
		fmt.Println("Connect to chain error:", err)
		os.Exit(1)
	}

	// Init node
	nodeFunc, sideChain, dkgReactor, err := sidechain.InitSideChain(nodePriv, chainPort, false, func() {
		util.LogWithYellow("Main Chain", chainConfig.Urls)
	})
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
		os.Exit(1)
	}

	dkgIns, err := dkg.NewDKG(nodePriv, dkgReactor, dkg.Logger{
		NodeTag: "DKG",
	})
	if err != nil {
		fmt.Println("Create DKG error:", err)
		os.Exit(1)
	}
	go dkgIns.Start()
	defer dkgIns.Stop()

	if dkgIns.DkgPubKey != nil {
		util.LogWithYellow("DKG PubKey", dkgIns.DkgPubKey.SS58())
	}

	// Set DKG to sideChain
	sideChain.SetDKG(dkgIns)

	// load chains
	err = sideChain.LoadChains()
	if err != nil {
		log.Fatalf("loadChains error: %v", err)
		os.Exit(1)
	}

	node, err := nodeFunc()
	if err != nil {
		log.Fatalf("failed to create BFT node: %v", err)
		os.Exit(1)
	}

	// Start BFT node
	if err := node.Start(); err != nil {
		log.Fatalf("failed to start BFT node: %v", err)
		os.Exit(1)
	}
	defer func() {
		_ = node.Stop()
		node.Wait()
	}()

	// 启动 graphql 服务器
	go graph.StartServer(sideChain, gqlPort)

	// wait for stop signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
