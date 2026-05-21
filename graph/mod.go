package graph

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/computeflux/tee-provider/pkg/api/gateway"
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

// StartServer starts the GraphQL server with an empty resolver (legacy).
func StartServer(s *teechain.TeeChain, port int) {
	StartServerWithResolver(s, port, &Resolver{})
}

// StartServerWithResolver starts the GraphQL + Auth REST + Gateway REST server.
func StartServerWithResolver(s *teechain.TeeChain, port int, resolver *Resolver) {
	teeChain = s

	// Phase 2: seed data
	gateway.InitSeedData()

	router := chi.NewRouter()
	router.Use(AuthMiddleware())
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// GraphQL playground
	router.Handle("/", playground.Handler("WeTEE-DSECRET", "/gql"))

	srv := handler.NewDefaultServer(NewExecutableSchema(Config{
		Resolvers:  resolver,
		Directives: NewDirectiveRoot(),
	}))
	router.Handle("/gql", srv)

	// Gateway REST routes (OpenAI SDK compatible)
	gateway.RegisterGatewayRoutes(router)

	// REST routes for dapp (Web3Auth SDK compatible)
	restMux := http.NewServeMux()
	RegisterRESTRoutes(restMux, resolver)
	router.Mount("/", restMux)

	util.LogWithBlue("API", "  /gql        -> GraphQL (auth, wallet, secrets)")
	util.LogWithBlue("API", "  /v1/*       -> REST (AI relay)")
	util.LogWithBlue("API", "  /health     -> health check")

	if util.IsFileExists("./token_chain_data/ssl/ser.pem") && util.IsFileExists("./token_chain_data/ssl/ser.key") {
		util.LogWithBlue("Listen", fmt.Sprintf("https://0.0.0.0:%d", port))
		http.ListenAndServeTLS(":"+fmt.Sprint(port), "./token_chain_data/ssl/ser.pem", "./token_chain_data/ssl/ser.key", router)
	} else {
		util.LogWithBlue("Listen", fmt.Sprintf("http://0.0.0.0:%d", port))
		http.ListenAndServe(":"+fmt.Sprint(port), router)
	}
}
