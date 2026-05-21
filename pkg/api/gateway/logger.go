package gateway

import (
	"log"
	"os"
	"path/filepath"
)

var gatewayLogger *log.Logger

func init() {
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("[GATEWAY-INIT] failed to create log dir %s: %v", logDir, err)
		gatewayLogger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
		return
	}
	f, err := os.OpenFile(filepath.Join(logDir, "gateway.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[GATEWAY-INIT] failed to open gateway.log: %v", err)
		gatewayLogger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
		return
	}
	gatewayLogger = log.New(f, "", log.LstdFlags)
}

// GatewayLog 返回 gateway 专用 logger，写入 ./logs/gateway.log
func GATEWAYLOG() *log.Logger {
	return gatewayLogger
}
