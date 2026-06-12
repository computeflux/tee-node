package graph

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/computeflux/tee-provider/pkg/api/gateway"
	localtunnel "github.com/computeflux/tee-provider/pkg/services/local_tunnel"
	"github.com/computeflux/tee-provider/pkg/util"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

var teeChain *teechain.TeeChain
var rsaKey *rsa.PrivateKey

func init() {
	var err error
	rsaKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Errorf("generate RSA key: %v", err))
	}
}

// StartServerWithResolver starts the GraphQL + Auth REST + Gateway REST server
// and blocks until ctx is cancelled. On cancellation it performs a graceful
// shutdown with a 10-second timeout, closing both the HTTP server and the
// local AI tunnel.
func StartServerWithResolver(ctx context.Context, s *teechain.TeeChain, port int, resolver *Resolver) error {
	teeChain = s

	// 注入合约查询桥接（Gateway 数据从合约读取）
	gateway.SetContractBridge(s)

	// 启动 Gateway 优化组件：异步用量 writer + 缓存刷新 + 限流器清理 + 用量结算
	gateway.InitOptimizations()

	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))
	router.Use(AuthMiddleware())

	// GraphQL playground
	router.Handle("/", playground.Handler("TEE-CHAIN", "/gql"))

	es := NewExecutableSchema(Config{
		Resolvers:  resolver,
		Directives: NewDirectiveRoot(),
	})

	// Start gql server
	srv := handler.New(es)
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})
	router.Handle("/gql", srv)

	// Gateway REST routes (OpenAI SDK compatible)
	gateway.RegisterGatewayRoutes(router)

	// Local AI tunnel TCP endpoint (本地模型反向隧道，Yamux)
	if err := localtunnel.StartGlobalServer(":19000", func(modelID, apiKey string) (uint64, bool) {
		return gateway.FindProviderByModelAPIKey(modelID, apiKey)
	}); err != nil {
		fmt.Printf("[LOCAL-TUNNEL] failed to start: %v\n", err)
	}

	// REST routes for dapp (Web3Auth SDK compatible)
	restMux := http.NewServeMux()
	RegisterRESTRoutes(restMux, resolver)
	router.Mount("/", restMux)

	util.LogWithBlue("API", "  /gql        -> GraphQL (auth, wallet, secrets)")
	util.LogWithBlue("API", "  /v1/*       -> REST (AI relay)")
	util.LogWithBlue("API", "  /health     -> health check")

	server := &http.Server{
		Addr:         ":" + fmt.Sprint(port),
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 300 * time.Second, // 本地模型推理可能需要较长时间
		IdleTimeout:  120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		if util.IsFileExists("./token_chain_data/ssl/ser.pem") && util.IsFileExists("./token_chain_data/ssl/ser.key") {
			util.LogWithBlue("Listen", fmt.Sprintf("https://0.0.0.0:%d", port))
			serverErr <- server.ListenAndServeTLS("./token_chain_data/ssl/ser.pem", "./token_chain_data/ssl/ser.key")
		} else {
			util.LogWithBlue("Listen", fmt.Sprintf("http://0.0.0.0:%d", port))
			serverErr <- server.ListenAndServe()
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := localtunnel.ShutdownGlobalServer(10 * time.Second); err != nil {
			util.LogWithRed("API", "local tunnel shutdown error:", err)
		}

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		util.LogWithBlue("API", "server shutdown gracefully")
		return nil
	case err := <-serverErr:
		return err
	}
}
