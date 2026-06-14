package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cometbft/cometbft/privval"
	"github.com/computeflux/tee-provider/config"
	"github.com/computeflux/tee-provider/graph"
	"github.com/computeflux/tee-provider/pkg/chains"
	chain "github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/dkg"
	authsvc "github.com/computeflux/tee-provider/pkg/services/auth"
	"github.com/computeflux/tee-provider/pkg/services/wallet"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
	_ "github.com/computeflux/tee-provider/pkg/util/cgostubs"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

var DefaultChainUrl string = "ws://wetee-node.worker-addon.svc.cluster.local:9944"

func main() {
	// 获取环境变量
	gqlPort := util.GetEnvInt("GQL_PORT", 61000)
	chainEnv := util.GetEnv("CHAIN_ENV", "local")
	chainPort := util.GetEnvInt("SIDE_CHAIN_PORT", 61001)

	// Init app db
	db, err := store.NewDB()
	if err != nil {
		fmt.Println("Init db error:", err)
		os.Exit(1)
	}
	defer db.Close()

	// TEE 节点启动

	// init teechain node key
	_, p2pKey, err := crypto.GetP2PKey()
	if err != nil {
		fmt.Println("failed to load node key:", err)
		os.Exit(1)
	}

	// Init key for DKG
	validatorKey := privval.LoadFilePV(
		"./token_chain_data/config/priv_validator_key.json",
		"./token_chain_data/data/priv_validator_state.json",
	)
	dkgKey := validatorKey.Key.PrivKey

	// Init node key for Mainchain
	nodePriv, err := crypto.PrivateKeyFromOed25519(dkgKey.Bytes())
	if err != nil {
		fmt.Println("Marshal PKG_PK error:", err)
		os.Exit(1)
	}

	util.LogWithYellow("Ed25519 Key", nodePriv.GetPublicForSuite(crypto.SuiteEd25519).SS58())
	util.LogWithYellow("ETH Key", hex.EncodeToString(nodePriv.ToSecp256k1Signer().Public()))
	util.LogWithYellow("BLS12381 Key", hex.EncodeToString(nodePriv.GetPublicForSuite(crypto.SuiteBLS12381).Byte()))
	util.LogWithYellow("P2P Key", p2pKey.GetPublicForSuite(crypto.SuiteEd25519).SS58())

	chainConfig := config.GetMainChainConfig(chainEnv)
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

	chains.ConnectChain(chainEnv, nodePriv)

	// Init node
	nodeFunc, teeChain, dkgReactor, err := teechain.InitTeeChain(nodePriv, chainPort, false, func() {
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

	// Set DKG to teeChain
	teeChain.SetDKG(dkgIns)

	// if err := teechain.LoadRuntimeForDebug(); err != nil {
	// 	fmt.Println("LoadRuntimeForDebug:", err)
	// 	os.Exit(1)
	// }

	// load chains
	err = teeChain.LoadChains()
	if err != nil {
		log.Fatalf("loadChains error: %v", err)
		os.Exit(1)
	}

	// Get BTF node
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

	// ── 初始化 Web3Auth 服务 ──────────────────────────────
	walletSvc := wallet.NewWalletService(teeChain)
	emailSvc := authsvc.NewEmailService(teeChain)
	oauthSvc := authsvc.NewOAuthService(teeChain)
	// Start GraphQL server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	otpSvc := authsvc.NewOTPService(ctx, 5)

	// 启动后台清理任务，每小时扫描并删除过期 token 和临时会话
	store.StartCleanupTask(ctx, time.Hour)

	// ── 创建 Resolver 注入所有依赖 ──────────────────────────
	resolver := &graph.Resolver{
		TeeChain:  teeChain,
		WalletSvc: walletSvc,
		EmailSvc:  emailSvc,
		OAuthSvc:  oauthSvc,
		OTPSvc:    otpSvc,
	}

	go func() {
		if err := graph.StartServerWithResolver(ctx, teeChain, gqlPort, resolver); err != nil {
			fmt.Println("server error:", err)
		}
	}()

	// wait for stop signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
}
