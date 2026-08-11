package auth

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/computeflux/tee-provider/pkg/services/auth"
)

// smtpTestEnvFile 是本地凭据文件：被 .gitignore 忽略，不提交仓库。
// 模板见同目录下的 smtp_test.env.example。
const smtpTestEnvFile = "smtp_test.env"

// TestSMTPEmailService_SendOTP 直接用 NewSMTPEmailService 构造服务并真实发送。
//
// 配置来源（环境变量优先，其次本地文件 ext/smtp_test.env）：
//
//	SMTP_HOST       SMTP 服务器地址（必填，未配置时测试自动跳过）
//	SMTP_PORT       端口（默认 587）
//	SMTP_USER       登录用户名
//	SMTP_PASSWORD   登录密码 / 应用专用密码
//	SMTP_FROM       发件人地址（必填，未配置时测试自动跳过）
//	SMTP_FROM_NAME  发件人显示名（默认 "Computeflux"）
//	SMTP_RECIPIENT  收件人邮箱（必填，未配置时测试自动跳过）
//
// 本地文件用法：
//
//	cp ext/smtp_test.env.example ext/smtp_test.env   # 填入真实值
//	go test -count=1 -v -run TestSMTPEmailService_SendOTP ./ext/
//
// 环境变量用法（可覆盖文件中的同名配置）：
//
//	SMTP_HOST=smtp.example.com SMTP_PORT=587 SMTP_USER=social@example.com \
//	SMTP_PASSWORD='your-app-password' SMTP_FROM=social@example.com \
//	SMTP_RECIPIENT=you@example.com \
//	go test -count=1 -v -run TestSMTPEmailService_SendOTP ./ext/
//
// 未配置（或 SMTP_HOST 仍是占位符 smtp.example.com）时自动跳过，
// 默认 `go test ./ext/` 不会误发邮件。
//
// 要点：
//   - 端口 465：SSL/TLS 直连
//   - 端口 587：STARTTLS（明文起手，EHLO 后升级）
//   - 端口 25 ：明文（基本被云厂商封了）
//   - Gmail 必须用「应用专用密码」，不能填登录密码
func TestSMTPEmailService_SendOTP(t *testing.T) {
	fileEnv := loadSMTPTestEnv(t)

	host := firstNonEmpty(os.Getenv("SMTP_HOST"), fileEnv["SMTP_HOST"])
	from := firstNonEmpty(os.Getenv("SMTP_FROM"), fileEnv["SMTP_FROM"])
	recipient := firstNonEmpty(os.Getenv("SMTP_RECIPIENT"), fileEnv["SMTP_RECIPIENT"])

	if host == "" || from == "" || recipient == "" || host == "smtp.example.com" {
		t.Skipf("未配置 SMTP 凭据（本地文件 %s 或 SMTP_* 环境变量），跳过真实发信测试", smtpTestEnvFile)
	}

	port := 587
	if v := firstNonEmpty(os.Getenv("SMTP_PORT"), fileEnv["SMTP_PORT"]); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p <= 0 || p > 65535 {
			t.Fatalf("SMTP_PORT=%q 不是合法端口", v)
		}
		port = p
	}

	fromName := firstNonEmpty(os.Getenv("SMTP_FROM_NAME"), fileEnv["SMTP_FROM_NAME"])
	if fromName == "" {
		fromName = "Computeflux"
	}

	smtpConfig := &auth.AuthConfig{
		SMTPHost:     host,
		SMTPPort:     port,
		SMTPUser:     firstNonEmpty(os.Getenv("SMTP_USER"), fileEnv["SMTP_USER"]),
		SMTPPassword: firstNonEmpty(os.Getenv("SMTP_PASSWORD"), fileEnv["SMTP_PASSWORD"]),
		SMTPFrom:     from,
		SMTPFromName: fromName,
	}

	otp := "123456" // 测试 OTP

	t.Logf("SMTP server: %s:%d (user=%q, from=%q)", host, port, smtpConfig.SMTPUser, smtpConfig.SMTPFrom)

	svc := auth.NewSMTPEmailService(smtpConfig)
	if err := svc.SendOTP(recipient, otp); err != nil {
		t.Fatalf("SendOTP failed: %v", err)
	}
	t.Logf("✅ OTP %q sent to %s — check your inbox", otp, recipient)
}

// loadSMTPTestEnv 读取测试文件同目录下的本地凭据文件（不存在时返回空 map，不报错）。
// 支持 KEY=VALUE 每行一条、# 注释、值可带双引号/单引号。
func loadSMTPTestEnv(t *testing.T) map[string]string {
	t.Helper()
	env := make(map[string]string)

	path := smtpTestEnvPath()
	if path == "" {
		return env
	}
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Logf("读取 %s 失败: %v", path, err)
		}
		return env
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		env[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	if err := sc.Err(); err != nil {
		t.Logf("解析 %s 失败: %v", path, err)
	}
	return env
}

// smtpTestEnvPath 返回凭据文件的绝对路径（与 email_test.go 同目录），
// 保证从任意工作目录运行 `go test` 都能找到。
func smtpTestEnvPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(file), smtpTestEnvFile)
}

// firstNonEmpty 返回第一个非空值。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
