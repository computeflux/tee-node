package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cometbft/cometbft/privval"
	"github.com/computeflux/tee-provider/graph"
	"github.com/computeflux/tee-provider/pkg/api/auth"
	chain "github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	authsvc "github.com/computeflux/tee-provider/services/auth"
	"github.com/computeflux/tee-provider/services/wallet"
	sidechain "github.com/computeflux/tee-provider/side-chain"
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
		"./token_chain_data/config/priv_validator_key.json",
		"./token_chain_data/data/priv_validator_state.json",
	)
	dkgKey := validatorKey.Key.PrivKey

	// Init node key for Mainchain
	nodePriv, err := model.PrivateKeyFromOed25519(dkgKey.Bytes())
	if err != nil {
		fmt.Println("Marshal PKG_PK error:", err)
		os.Exit(1)
	}

	util.LogWithYellow("Mainchain Key", nodePriv.GetPublicForSuite(model.SuiteEd25519).SS58())
	util.LogWithYellow("Secp256k1 Key", hex.EncodeToString(nodePriv.GetPublicForSuite(model.SuiteSecp256k1).Secp256k1CompressPubkey()))
	util.LogWithYellow("P2P Key", p2pKey.GetPublicForSuite(model.SuiteEd25519).SS58())

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

	// ── 初始化 Web3Auth 服务 ──────────────────────────────
	authCfg := auth.LoadConfig()
	walletSvc := wallet.NewWalletService(authCfg)
	jwtSvc := authsvc.NewJWTService(authCfg)
	emailSvc := authsvc.NewEmailService(authCfg)
	oauthSvc := authsvc.NewOAuthService(authCfg)
	otpSvc := authsvc.NewOTPService(authCfg.OTPTTLMins)

	// ── 创建 Resolver 注入所有依赖 ──────────────────────────
	resolver := &graph.Resolver{
		SideChain: sideChain,
		AuthCfg:   authCfg,
		WalletSvc: walletSvc,
		JWTSvc:    jwtSvc,
		EmailSvc:  emailSvc,
		OAuthSvc:  oauthSvc,
		OTPSvc:    otpSvc,
	}

	// Start GraphQL server
	go graph.StartServerWithResolver(sideChain, gqlPort, resolver)

	// wait for stop signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
