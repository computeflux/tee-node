package util

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ── 全局统一日志系统 ─────────────────────────────────────────
// 特性:
//   - 等级过滤: debug < info < warn < error < off
//   - 双输出: 终端(stdout, ANSI 彩色, fmt.Println 风格, 方便调试) + 文件(log 包写入)
//   - 程序启动时调用 SetupLog 配置一次, 之后任意包直接调用 LogDebugf/LogInfof/...
//   - 等级可通过 SetLogLevel 运行时调整, 或通过 LOG_LEVEL 环境变量配置
//
// 旧的彩色 API(LogWithXxx / LogError / LogOk)已接入本系统:
// 终端保持原有彩色样式, 同时按等级过滤并写入文件(纯文本, 不含 ANSI 转义)。

// LogLevel 日志等级, 数值越大越严重。
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelOff
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelOff:
		return "OFF"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel 解析 "debug"/"info"/"warn"/"error"/"off"(大小写不敏感), 未知值回退 info。
func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "off", "none", "silent":
		return LevelOff
	default:
		return LevelInfo
	}
}

// LogOptions 全局日志配置。
type LogOptions struct {
	Level    LogLevel // 最低输出等级, 低于它的日志被丢弃
	ToStdout bool     // 输出到终端(彩色, fmt.Println 风格, 便于调试)
	ToFile   bool     // 写入文件(log 包格式, 带时间戳)
	FilePath string   // 日志文件路径, 默认 ./token_chain_data/app.log
}

// appLogger 全局日志单例。
type appLogger struct {
	mu      sync.Mutex
	level   LogLevel
	stdout  bool
	file    bool
	path    string      // 用户配置的基础文件路径(不含日期)
	curDate string      // 当前文件对应的日期(YYYY-MM-DD), 跨天时轮转
	f       *os.File    // 当前日志文件
	flog    *log.Logger // 文件 logger(nil 表示未开启文件输出)
}

var gLog = &appLogger{level: LevelInfo, stdout: true}

// DailyLogPath 返回按天切分的日志文件路径:
//
//	/path/app.log → /path/app-2026-08-08.log (当天日期)
//
// 无扩展名时直接拼接: /path/nofile → /path/nofile-2026-08-08
func DailyLogPath(base string, t time.Time) string {
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return fmt.Sprintf("%s-%s%s", name, t.Format("2006-01-02"), ext)
}

// SetupLog 初始化全局日志。可重复调用以运行时调整配置。
func SetupLog(opts LogOptions) error {
	gLog.mu.Lock()
	defer gLog.mu.Unlock()

	if opts.FilePath == "" {
		opts.FilePath = filepath.Join("token_chain_data", "app.log")
	}

	gLog.level = opts.Level
	gLog.stdout = opts.ToStdout
	gLog.file = opts.ToFile
	gLog.path = opts.FilePath
	gLog.closeFileLocked()

	if !opts.ToFile {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(opts.FilePath), 0755); err != nil {
		return fmt.Errorf("create log dir %s: %w", filepath.Dir(opts.FilePath), err)
	}
	if err := gLog.openDailyFileLocked(); err != nil {
		return err
	}
	return nil
}

// SetLogLevel 运行时调整全局日志等级。
func SetLogLevel(l LogLevel) {
	gLog.mu.Lock()
	defer gLog.mu.Unlock()
	gLog.level = l
}

// GetLogLevel 返回当前全局日志等级。
func GetLogLevel() LogLevel {
	gLog.mu.Lock()
	defer gLog.mu.Unlock()
	return gLog.level
}

// StdoutEnabled 返回终端输出是否开启(供 gateway 等专用 logger 决定是否镜像到终端)。
func StdoutEnabled() bool {
	gLog.mu.Lock()
	defer gLog.mu.Unlock()
	return gLog.stdout
}

// 等级 → 终端颜色
var levelColor = map[LogLevel]string{
	LevelDebug: Cyan,
	LevelInfo:  Green,
	LevelWarn:  Yellow,
	LevelError: Red,
}

// ── 等级化日志 API ─────────────────────────────────────────

// LogDebugf 调试级日志。
func LogDebugf(format string, args ...any) { logAt(LevelDebug, format, args...) }

// LogInfof 信息级日志。
func LogInfof(format string, args ...any) { logAt(LevelInfo, format, args...) }

// LogWarnf 警告级日志。
func LogWarnf(format string, args ...any) { logAt(LevelWarn, format, args...) }

// LogErrorf 错误级日志。
func LogErrorf(format string, args ...any) { logAt(LevelError, format, args...) }

func logAt(l LogLevel, format string, args ...any) {
	gLog.mu.Lock()
	defer gLog.mu.Unlock()

	if l < gLog.level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	tag := "[" + l.String() + "]"

	if gLog.stdout {
		fmt.Println(levelColor[l]+" "+tag, Reset, msg)
	}
	if gLog.file {
		gLog.rotateIfNeededLocked()
	}
	if gLog.file && gLog.flog != nil {
		gLog.flog.Printf("%s %s", tag, msg)
	}
}

// logColored 供旧式彩色 API(LogWithXxx / LogError / LogOk)使用:
// 终端保持原有彩色样式; 文件输出为纯文本(自动剥离 ANSI 颜色)。
// 参数拼接与旧实现(fmt.Println)保持一致: 各参数间以空格分隔。
func logColored(l LogLevel, color, tag string, a ...any) {
	gLog.mu.Lock()
	defer gLog.mu.Unlock()

	if l < gLog.level {
		return
	}
	msg := strings.TrimSuffix(fmt.Sprintln(a...), "\n") // Println 风格: 参数间空格

	if gLog.stdout {
		fmt.Println(color+" "+tag, Reset, msg)
	}
	if gLog.file {
		gLog.rotateIfNeededLocked()
	}
	if gLog.file && gLog.flog != nil {
		gLog.flog.Printf("[%s] %s %s", l.String(), tag, msg)
	}
}

// ── 按天切分 ────────────────────────────────────────────────

// openDailyFileLocked 按当前日期打开日志文件(调用方须持有 mu)。
// 文件命名: <base>-YYYY-MM-DD.log, 例如 app-2026-08-08.log。
func (g *appLogger) openDailyFileLocked() error {
	now := time.Now()
	p := DailyLogPath(g.path, now)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", p, err)
	}
	g.f = f
	g.flog = log.New(f, "", log.LstdFlags)
	g.curDate = now.Format("2006-01-02")
	return nil
}

// closeFileLocked 关闭当前日志文件(调用方须持有 mu)。
func (g *appLogger) closeFileLocked() {
	if g.f != nil {
		g.f.Close()
	}
	g.f = nil
	g.flog = nil
	g.curDate = ""
}

// rotateIfNeededLocked 检测到跨天时关闭旧文件并打开新日期文件(调用方须持有 mu)。
// 轮转失败时降级为不再写文件(避免把当天日志误写进旧文件), 并输出 stderr 提示。
func (g *appLogger) rotateIfNeededLocked() {
	if g.flog == nil {
		return
	}
	if time.Now().Format("2006-01-02") == g.curDate {
		return
	}
	g.closeFileLocked()
	if err := g.openDailyFileLocked(); err != nil {
		fmt.Fprintf(os.Stderr, "[log] rotate daily log file failed: %v\n", err)
	}
}
