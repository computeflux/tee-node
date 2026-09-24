// Command sgx-verify-service 是 ego eclient 的独立 HTTP 封装。
//
// 非 TEE 节点（IsEgo=false）通过 HTTP 调用本服务来验证 SGX report。
// 验证逻辑由 ego 自带的 eclient 完成（cgo 链接 /opt/ego/lib 的
// liboehostverify，静态库，编译期链接），参考
// samples/remote_attestation/ra_client/client.go。
// 部署为本地 systemd 服务（不走 docker），见 hack/sgx-verify-service-deploy.sh。
// 服务无状态、fail-closed：验证失败返回 502，调用方必须按错误处理。
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/edgelesssys/ego/attestation"
	"github.com/edgelesssys/ego/attestation/tcbstatus"
	"github.com/edgelesssys/ego/eclient"
)

const (
	maxBodyBytes = 1 << 20 // 1MB 请求体上限，report 实际只有几 KB

	defaultAddr    = ":8004"
	defaultMaxConc = 16
)

var (
	addr          = flag.String("addr", envOr("SGX_VERIFY_SERVICE_ADDR", defaultAddr), "监听地址，如 0.0.0.0:8004")
	maxConcurrent = flag.Int("max-concurrent", envOrInt("SGX_VERIFY_MAX_CONCURRENT", defaultMaxConc), "最大并发验证数，<=0 表示不限制")
)

func main() {
	flag.Parse()

	var sem chan struct{}
	if *maxConcurrent > 0 {
		sem = make(chan struct{}, *maxConcurrent)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /verify", func(w http.ResponseWriter, r *http.Request) {
		verifyHandler(w, r, sem)
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("sgx-verify-service listening on %s (max-concurrent=%d, verifier=eclient)",
		*addr, *maxConcurrent)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

type verifyRequest struct {
	Report string `json:"report"` // base64 编码的原始 report 字节
}

func verifyHandler(w http.ResponseWriter, r *http.Request, sem chan struct{}) {
	var req verifyRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes)).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	reportBytes, err := base64.StdEncoding.DecodeString(req.Report)
	if err != nil || len(reportBytes) == 0 {
		http.Error(w, "invalid request: report must be non-empty base64", http.StatusBadRequest)
		return
	}

	if sem != nil {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			http.Error(w, "server busy, retry later", http.StatusServiceUnavailable)
			return
		}
	}

	// 验证超时兜底（eclient 验证是本地操作，正常毫秒级完成）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan struct{})
	var report attestation.Report
	var verifyErr error
	go func() {
		report, verifyErr = eclient.VerifyRemoteReport(reportBytes)
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		http.Error(w, "verify failed: timeout", http.StatusGatewayTimeout)
		return
	}

	if verifyErr == attestation.ErrTCBLevelInvalid {
		// 与节点内 SgxVerify 行为一致：TCB 过期仅告警，不拒绝
		log.Printf("verify ok with TCB warning: %v %v",
			report.TCBStatus, tcbstatus.Explain(report.TCBStatus))
	} else if verifyErr != nil {
		log.Printf("verify failed: %v", verifyErr)
		http.Error(w, "verify failed: "+verifyErr.Error(), http.StatusBadGateway)
		return
	}

	resp, err := json.Marshal(report)
	if err != nil {
		http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("verify ok mrenclave=%x isvprodid=%x", report.UniqueID, report.ProductID)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
