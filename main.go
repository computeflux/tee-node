package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cometbft/cometbft/privval"
	"github.com/computeflux/tee-provider/config"
	"github.com/computeflux/tee-provider/graph"
	"github.com/computeflux/tee-provider/pkg/api/auth"
	"github.com/computeflux/tee-provider/pkg/chains"
	chain "github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	authsvc "github.com/computeflux/tee-provider/pkg/services/auth"
	"github.com/computeflux/tee-provider/pkg/services/wallet"
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
	db, err := model.NewDB()
	if err != nil {
		fmt.Println("Init db error:", err)
		os.Exit(1)
	}
	defer db.Close()

	// Init TEE KMS Global Key
	gk, err := model.InitGlobalKeyAtStartup()
	if err != nil {
		if os.Getenv("TEE_GENESIS_NODE") == "true" {
			util.LogWithYellow("TEE KMS", "Generating new global key as genesis node")
			gk, err = model.GenerateAndSealGlobalKey()
			if err != nil {
				log.Fatalf("failed to generate global key: %v", err)
			}
			// 导出 GK（供管理员记录，但新节点现在通过 DKG 自动同步，无需手动设置）
			exported, version, err := model.ExportGlobalKeyBase64()
			if err == nil {
				util.LogWithYellow("TEE KMS", "Genesis GK generated (new nodes will auto-sync via DKG):")
				fmt.Printf("TEE_KMS_BACKUP_KEY=%s TEE_KMS_BACKUP_VERSION=%d\n", exported, version)
			}

			// 从环境变量读取敏感配置并密封存储
			if !model.HasConfigNamespace("auth") {
				authEnvKeys := []string{
					"JWT_SECRET", "SEED_SECRET", "CORS_ORIGINS",
					"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL",
					"GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_REDIRECT_URL",
					"TWITTER_CLIENT_ID", "TWITTER_CLIENT_SECRET", "TWITTER_REDIRECT_URL",
					"SMTP_HOST", "SMTP_USER", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_FROM_NAME",
				}
				if err := model.SealAllConfigFromEnv("auth", authEnvKeys, gk); err != nil {
					log.Fatalf("failed to seal auth config: %v", err)
				}
				// 特殊处理：整数类型需要额外密封（转为字符串）
				model.SealConfigValue("auth", "SMTP_PORT",
					fmt.Sprintf("%d", util.GetEnvInt("SMTP_PORT", 587)), gk)
				model.SealConfigValue("auth", "MOCK_EMAIL",
					fmt.Sprintf("%t", util.GetEnv("MOCK_EMAIL", "true") == "true"), gk)
				util.LogWithYellow("TEE CFG", "Auth config sealed from env vars")
			}
		} else {
			// 非 genesis 节点：允许暂时无 GK，DKG 组网成功后会自动从 peers 同步
			util.LogWithYellow("TEE KMS", "Global key not found locally, will sync from peers after DKG join")
		}
	} else {
		util.LogWithYellow("TEE KMS", "Global key loaded from local seal")
	}
	_ = gk

	// init teechain node key
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

	util.LogWithYellow("Ed25519 Key", nodePriv.GetPublicForSuite(model.SuiteEd25519).SS58())
	util.LogWithYellow("ETH Key", hex.EncodeToString(nodePriv.ToSecp256k1Signer().Public()))
	util.LogWithYellow("BLS12381 Key", hex.EncodeToString(nodePriv.GetPublicForSuite(model.SuiteBLS12381).Byte()))
	util.LogWithYellow("P2P Key", p2pKey.GetPublicForSuite(model.SuiteEd25519).SS58())

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

	// load chains
	err = teeChain.LoadChains()
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
	emailSvc := authsvc.NewEmailService(authCfg)
	oauthSvc := authsvc.NewOAuthService(authCfg)
	otpSvc := authsvc.NewOTPService(authCfg.OTPTTLMins)

	// ── 创建 Resolver 注入所有依赖 ──────────────────────────
	resolver := &graph.Resolver{
		TeeChain:  teeChain,
		AuthCfg:   authCfg,
		WalletSvc: walletSvc,
		EmailSvc:  emailSvc,
		OAuthSvc:  oauthSvc,
		OTPSvc:    otpSvc,
	}

	// Start GraphQL server
	go graph.StartServerWithResolver(teeChain, gqlPort, resolver)

	// wait for stop signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
