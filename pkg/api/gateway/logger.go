package gateway

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
)

// gatewayLoggerT 是 gateway 专用 logger: 按天写入 token_chain_data/gateway-YYYY-MM-DD.log。
// 终端镜像跟随全局日志配置(LOG_STDOUT / SetupLog 的 ToStdout), 便于本地调试;
// 全局日志等级为 off 时静默。
type gatewayLoggerT struct {
	mu      sync.Mutex
	f       *os.File
	file    *log.Logger
	curDate string // 当前文件对应的日期(YYYY-MM-DD), 跨天时轮转
}

func (g *gatewayLoggerT) Printf(format string, v ...any) {
	if g == nil {
		return
	}
	if util.GetLogLevel() == util.LevelOff {
		return
	}
	msg := fmt.Sprintf(format, v...)

	g.mu.Lock()
	defer g.mu.Unlock()
	g.rotateIfNeeded()
	if g.file != nil {
		g.file.Print(msg)
	}
	if util.StdoutEnabled() {
		fmt.Printf("%s %s\n", time.Now().Format("2006/01/02 15:04:05"), msg)
	}
}

// rotateIfNeeded 首次打开或跨天时轮转日志文件(调用方须持有 mu)。
func (g *gatewayLoggerT) rotateIfNeeded() {
	now := time.Now()
	date := now.Format("2006-01-02")
	if g.file != nil && date == g.curDate {
		return
	}
	if g.f != nil {
		g.f.Close()
	}
	f, err := os.OpenFile(util.DailyLogPath(filepath.Join("token_chain_data", "gateway.log"), now),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[GATEWAY-INIT] open gateway.log failed: %v", err)
		g.f, g.file, g.curDate = nil, nil, ""
		return
	}
	g.f = f
	g.file = log.New(f, "", log.LstdFlags)
	g.curDate = date
}

var gatewayLogger *gatewayLoggerT

func init() {
	gatewayLogger = &gatewayLoggerT{}
	// 与主服务运行目录一致(相对路径): ./token_chain_data/gateway-YYYY-MM-DD.log
	logDir := "token_chain_data"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("[GATEWAY-INIT] failed to create log dir %s: %v", logDir, err)
		return
	}
	gatewayLogger.rotateIfNeeded()
	if gatewayLogger.file == nil {
		log.Printf("[GATEWAY-INIT] failed to open gateway.log")
	}
}

// GATEWAYLOG 返回 gateway 专用 logger, 按天写入 ./token_chain_data/gateway-YYYY-MM-DD.log。
func GATEWAYLOG() *gatewayLoggerT {
	return gatewayLogger
}
