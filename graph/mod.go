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

	"github.com/wetee-dao/tee-dsecret/pkg/util"
	sidechain "github.com/wetee-dao/tee-dsecret/side-chain"
)

var sideChain *sidechain.SideChain
var rsaKey *rsa.PrivateKey

func init() {
	var err error
	rsaKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Errorf("生成私钥失败: %v", err))
	}

}

// 启动GraphQL服务器
// StartServer starts the GraphQL server.
func StartServer(s *sidechain.SideChain, port int) {
	sideChain = s

	// 创建路由
	router := chi.NewRouter()

	// 添加认证中间件
	router.Use(AuthMiddleware())

	// 添加跨域中间件
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// graphql playground
	router.Handle("/", playground.Handler("WeTEE-DSECRET", "/gql"))
	srv := handler.NewDefaultServer(NewExecutableSchema(Config{
		Resolvers:  &Resolver{},
		Directives: NewDirectiveRoot(),
	}))

	// main graphql
	router.Handle("/gql", srv)

	if util.IsFileExists("./chain_data/ssl/ser.pem") && util.IsFileExists("./chain_data/ssl/ser.key") {
		util.LogWithBlue("GraphQL    ", "https://0.0.0.0:"+fmt.Sprint(port))
		http.ListenAndServeTLS(":"+fmt.Sprint(port), "./chain_data/ssl/ser.pem", "./chain_data/ssl/ser.key", router)
	} else {
		util.LogWithBlue("GraphQL    ", "http://0.0.0.0:"+fmt.Sprint(port))
		http.ListenAndServe(":"+fmt.Sprint(port), router)
	}
}
