package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/computeflux/tee-provider/pkg/teetypes"
	"github.com/maximhq/bifrost/core/network"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// 这个检查程序逐步复现 Bifrost provider 实际使用的 fasthttp 网络路径，
// 用来定位在 EGo enclave 里到底哪一步不兼容。
type noopLogger struct{}

func (noopLogger) Debug(string, ...any)                   {}
func (noopLogger) Info(string, ...any)                    {}
func (noopLogger) Warn(string, ...any)                    {}
func (noopLogger) Error(string, ...any)                   {}
func (noopLogger) Fatal(string, ...any)                   {}
func (noopLogger) SetLevel(schemas.LogLevel)              {}
func (noopLogger) SetOutputType(schemas.LoggerOutputType) {}
func (noopLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}

func main() {
	fmt.Printf("TEE type=%d IsEgo=%v\n", teetypes.TeeType, teetypes.IsEgo)

	// 1. TLS 根证书池（Bifrost ConfigureTLS 会调用）
	runStep("x509.SystemCertPool", func() error {
		_, err := x509.SystemCertPool()
		return err
	})

	// 2. DNS 解析（Bifrost ConfigureDialer 会调用）
	runStep("net.DefaultResolver.LookupIP(localhost)", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := net.DefaultResolver.LookupIP(ctx, "ip", "localhost")
		return err
	})

	// 3. 起一个本地标准 net/http server，用来给 fasthttp client 请求
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	fmt.Printf("local server: %s\n", srv.URL)

	// 4. 最简 fasthttp.Client（不含 Bifrost 包装）
	runStep("plain fasthttp request", func() error {
		return doFasthttpRequest(srv.URL, &fasthttp.Client{})
	})

	// 5. Bifrost network.HTTPClientFactory 出来的 client
	runStep("Bifrost factory fasthttp request", func() error {
		factory := network.NewHTTPClientFactory(nil, noopLogger{})
		client := factory.GetFasthttpClient(network.ClientPurposeInference)
		return doFasthttpRequest(srv.URL, client)
	})

	// 6. 加上 Bifrost ConfigureDialer（含 DNS 自解析、TCP keepalive）
	runStep("Bifrost factory + ConfigureDialer", func() error {
		factory := network.NewHTTPClientFactory(nil, noopLogger{})
		client := factory.GetFasthttpClient(network.ClientPurposeInference)
		client = providerUtils.ConfigureDialer(client, true) // allowPrivateNetwork=true
		return doFasthttpRequest(srv.URL, client)
	})

	// 7. 再加上 TLS 配置（含 SystemCertPool / InsecureSkipVerify）
	runStep("Bifrost factory + ConfigureDialer + ConfigureTLS", func() error {
		factory := network.NewHTTPClientFactory(nil, noopLogger{})
		client := factory.GetFasthttpClient(network.ClientPurposeInference)
		client = providerUtils.ConfigureDialer(client, true)
		client = providerUtils.ConfigureTLS(client, schemas.NetworkConfig{
			InsecureSkipVerify: true,
		}, noopLogger{})
		return doFasthttpRequest(srv.URL, client)
	})

	// 8. TLS 服务端（测试 HTTPS 路径）
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tls":true}`))
	}))
	defer tlsSrv.Close()

	runStep("Bifrost fasthttps (TLS) request", func() error {
		factory := network.NewHTTPClientFactory(nil, noopLogger{})
		client := factory.GetFasthttpClient(network.ClientPurposeInference)
		client = providerUtils.ConfigureDialer(client, true)
		client = providerUtils.ConfigureTLS(client, schemas.NetworkConfig{
			InsecureSkipVerify: true,
		}, noopLogger{})
		return doFasthttpRequest(tlsSrv.URL, client)
	})

	fmt.Println("\nall steps completed")
}

func runStep(name string, fn func() error) {
	fmt.Printf("\n[STEP] %s\n", name)
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("  PANIC: %v\n", r)
			os.Exit(1)
		}
	}()
	if err := fn(); err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  OK\n")
	}
}

func doFasthttpRequest(urlStr string, client *fasthttp.Client) error {
	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(urlStr)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.SetContentType("application/json")

	return client.DoTimeout(req, res, 5*time.Second)
}
