package main

import (
	"context"
	"flag"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

// clientIPMiddleware 提取客户端真实 IP（无反向代理，直接取 RemoteAddr）并注入 context。
func clientIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			ip = host
		}
		next.ServeHTTP(w, r.WithContext(actives.WithClientIP(r.Context(), ip)))
	})
}

func main() {
	port := flag.String("port", "9100", "listening port")
	dbPath := flag.String("db", "./actives_data", "PebbleDB data path")
	playgroundOn := flag.Bool("playground", false, "enable GraphQL playground (dev only; off in production)")
	introspectionOn := flag.Bool("introspection", false, "enable GraphQL introspection (dev only; off in production)")
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
	router.Use(clientIPMiddleware)

	// 生产环境不暴露 /gql 接口文档：默认关闭 playground 与 introspection（-playground/-introspection 仅开发调试用）
	if *playgroundOn {
		router.Handle("/", playground.Handler("Actives", "/gql"))
	}

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
	if *introspectionOn {
		srv.Use(extension.Introspection{})
	}
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})
	router.Handle("/gql", srv)

	// 封禁状态页：/?wallet=0x... 或 /banned?wallet=0x...，向用户展示非正常使用提示与积分作废信息
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		wallet := strings.TrimSpace(r.URL.Query().Get("wallet"))
		banned, bannedAt, reason, voided := actives.Store.GetBanStatus(wallet)
		if wallet == "" {
			fmt.Fprint(w, `<!doctype html><html lang="zh"><meta charset="utf-8"><title>Actives Status</title>
<body style="font-family:system-ui;background:#0b0e14;color:#e6e6e6;display:flex;align-items:center;justify-content:center;min-height:100vh">
<div style="max-width:420px;padding:32px;border:1px solid #2a2f3a;border-radius:8px">
<h2>Actives Status Check</h2>
<p style="color:#9aa4b2">Pass your wallet in the URL:</p>
<code style="color:#7dd3fc">/?wallet=0x</code>
</div></body></html>`)
			return
		}
		if !banned {
			fmt.Fprintf(w, `<!doctype html><html lang="zh"><meta charset="utf-8"><title>Actives Status</title>
<body style="font-family:system-ui;background:#0b0e14;color:#e6e6e6;display:flex;align-items:center;justify-content:center;min-height:100vh">
<div style="max-width:420px;padding:32px;border:1px solid #2a2f3a;border-radius:8px;text-align:center">
<svg width="48" height="48" viewBox="0 0 24 24" fill="#22c55e"><path d="M9 16.2 4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4z"/></svg>
<h2>Account Active</h2>
<p style="color:#9aa4b2">No abnormal usage detected for this account.</p>
<code style="font-size:12px;color:#7dd3fc;word-break:break-all">%s</code>
</div></body></html>`, html.EscapeString(wallet))
			return
		}
		fmt.Fprintf(w, `<!doctype html><html lang="zh"><meta charset="utf-8"><title>Account Restricted</title>
<body style="font-family:system-ui;background:#0b0e14;color:#e6e6e6;display:flex;align-items:center;justify-content:center;min-height:100vh">
<div style="max-width:420px;padding:32px;border:1px solid #7f1d1d;border-radius:8px;text-align:center">
<svg width="48" height="48" viewBox="0 0 24 24" fill="#ef4444"><path d="M12 2 1 21h22zM11 10h2v5h-2zm0 7h2v2h-2z" transform="scale(0.9)"/></svg>
<h2 style="color:#ef4444">Account Restricted</h2>
<p>We detected abnormal usage behavior on this account.<br/>(检测到该账户存在非正常使用行为)</p>
<p style="color:#fbbf24"><b>积分作废 / Points voided: %d</b></p>
`, voided)
		if reason != "" {
			fmt.Fprintf(w, `<p style="color:#9aa4b2">Reason: %s</p>`, html.EscapeString(reason))
		}
		fmt.Fprintf(w, `<p style="color:#9aa4b2">Banned at: %s</p>
<code style="font-size:12px;color:#7dd3fc;word-break:break-all">%s</code>
</div></body></html>`, html.EscapeString(bannedAt), html.EscapeString(wallet))
	})

	// PII: 预检（GET /gql 返回健康检查，不暴露 schema）
	logBlue("API", "  /gql        -> GraphQL (actives: early access)")
	if *playgroundOn {
		logBlue("API", "  /           -> Playground (dev)")
	} else {
		logBlue("API", "  /?wallet=0x -> status page")
	}

	server := &http.Server{
		Addr:        ":" + *port,
		Handler:     router,
		ReadTimeout: 10 * time.Second,
		// earlyAccessExport 全量导出生成耗时可能超过 30s,
		// 过小的 WriteTimeout 会在响应写回前掐断连接(HTTP/2: stream error INTERNAL_ERROR / HTTP/1.1: unexpected EOF)。
		// 调大到 5 分钟, 给大导出留足余量。
		WriteTimeout: 300 * time.Second,
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
