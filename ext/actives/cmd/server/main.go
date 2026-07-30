package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/cockroachdb/pebble"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/computeflux/actives"
)

func logBlue(tag, msg string) {
	fmt.Printf("\033[34m[%s]\033[0m %s\n", tag, msg)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func main() {
	port := flag.String("port", "9100", "listening port")
	dbPath := flag.String("db", "./actives_data", "PebbleDB data path")
	flag.Parse()

	db, err := pebble.Open(*dbPath, &pebble.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "init db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	actives.InitDB(db)

	// Initialize on-chain FLUX_TEST token minting (optional)
	// Initialize on-chain FLUX_TEST token minting (optional — skips if flux_config.json not found)
	if err := actives.InitFluxChain("flux_config.json"); err != nil {
		fmt.Fprintf(os.Stderr, "init flux chain: %v\n", err)
		os.Exit(1)
	}

	if addr := actives.FluxSigner(); addr != "" {
		logBlue("FLUX", fmt.Sprintf("Signer: %s", addr))
	}

	// Start background worker for auto-minting historical points
	actives.StartMintWorker()

	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	router.Handle("/", playground.Handler("Actives", "/gql"))

	es := actives.NewExecutableSchema(actives.Config{
		Resolvers: &actives.Resolver{},
	})
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

	logBlue("API", "  /gql        -> GraphQL (actives: early access)")
	logBlue("API", "  /           -> Playground")

	server := &http.Server{
		Addr:         ":" + *port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		certFile := "./ssl/ser.pem"
		keyFile := "./ssl/ser.key"
		if fileExists(certFile) && fileExists(keyFile) {
			logBlue("Listen", fmt.Sprintf("https://0.0.0.0:%s", *port))
			if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
				fmt.Fprintf(os.Stderr, "server error: %v\n", err)
				os.Exit(1)
			}
		} else {
			logBlue("Listen", fmt.Sprintf("http://0.0.0.0:%s", *port))
			if err := server.ListenAndServe(); err != nil {
				fmt.Fprintf(os.Stderr, "server error: %v\n", err)
				os.Exit(1)
			}
		}
	}()

	<-ctx.Done()
	fmt.Println("\nShutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Server stopped.")
}
