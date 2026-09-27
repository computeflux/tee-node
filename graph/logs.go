package graph

import (
	"bytes"
	"crypto/subtle"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
)

// ── /api/logs: 只读日志查询端点（供 tee-node-monitor 远程拉取）──────────────
//
// 节点日志文件在宿主机磁盘（token_chain_data bind mount），monitor 部署在其它机器
// 时无法直接读取。本端点把最近窗口内的原始日志行以 text/plain 返回，
// 解析/分类完全由 monitor 侧完成（保持两边职责不变）。
//
// 安全设计:
//   - 受 LOG_API_TOKEN 环境变量控制; 未设置时端点 404（默认关闭, 零暴露）
//   - 请求须带 X-Log-Token 头（常量时间比较）; token 不放 query, 避免进 access log
//   - 仅允许 app/gateway 两个固定基名, 不可能路径穿越
//   - since 最多 90d 之前; 单次响应最多 5000 行 / 2MB
//
// 增量读取: monitor 每轮只传上一轮结束点作为 since（而非每次整 24h 窗口），
// 节点按 1MB→8MB 递增分块读文件尾部, 增量请求通常 1MB 分块即可命中边界,
// 不再每次全读 8MB。按天查看(历史天)时 since 为当天零点, 逐天文件用
// "最后一行时间戳 < since" 预检直接跳过, 只有边界文件才做分块扫描。

const (
	logsMaxLines    = 5000
	logsMaxResponse = 2 << 20 // 2MB
	logsMaxSinceAgo = 90 * 24 * time.Hour
)

// logsTailChunks 尾部读取分块递增序列（1MB→8MB, 末块 8MB 可覆盖 24h 窗口）。
var logsTailChunks = []int64{1 << 20, 2 << 20, 4 << 20, 8 << 20}

// handleLogsAPI GET /api/logs?file=app|gateway&since=<RFC3339>[&limit=N]
func handleLogsAPI(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("LOG_API_TOKEN")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(r.Header.Get("X-Log-Token"))) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	file := q.Get("file")
	if file != "app" && file != "gateway" {
		http.Error(w, "file must be app|gateway", http.StatusBadRequest)
		return
	}
	since, err := time.Parse(time.RFC3339, q.Get("since"))
	if err != nil {
		http.Error(w, "since must be RFC3339", http.StatusBadRequest)
		return
	}
	if since.Before(time.Now().Add(-logsMaxSinceAgo)) {
		http.Error(w, "since at most 24h in the past", http.StatusBadRequest)
		return
	}
	limit := logsMaxLines
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v < limit {
		limit = v
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	var buf bytes.Buffer
	for _, line := range collectLogLines(file, since, limit) {
		buf.WriteString(line)
		buf.WriteByte('\n')
		if buf.Len() >= logsMaxResponse {
			break
		}
	}
	_, _ = w.Write(buf.Bytes())
}

// logsBaseDir 节点日志所在目录（LOG_DIR 环境变量, 默认 token_chain_data）。
func logsBaseDir() string {
	return util.GetEnv("LOG_DIR", "token_chain_data")
}

// collectLogLines 读取 app/gateway 日志文件 since 所在天起至今的逐天文件（分块增量读）,
// 返回时间戳 >= since 的最后 limit 行。
func collectLogLines(file string, since time.Time, limit int) []string {
	now := time.Now()
	var out []string
	// 从 since 的本地日期逐天读到今天（文件按节点本地日期切分）
	for d := since.In(time.Local); !d.After(now); d = d.AddDate(0, 0, 1) {
		p := filepath.Join(logsBaseDir(), util.DailyLogPath(file+".log", d))
		lines, ok := tailLinesSince(p, since)
		if !ok {
			continue // 文件不存在（如 gateway 尚未产生日志）
		}
		out = append(out, lines...)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// tailLinesSince 分块（1MB→8MB）读文件尾部, 返回时间戳 >= since 的那行及其之后的所有行。
// 日志按时间顺序追加: 增量请求（since 较近）小分块通常即可命中边界, 无需每次全读 8MB;
// 分块首行已 >= since（边界可能更早）或整块 < since 时扩大分块重读。
// 文件不存在/不可读时 ok=false。
func tailLinesSince(path string, since time.Time) ([]string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, false
	}
	size := fi.Size()
	if size == 0 {
		return nil, true
	}
	// 快速预检: 文件最后一行的时间戳 < since 时整文件无匹配（日志按时间追加）,
	// 直接跳过, 避免历史文件每次读 8MB 尾部
	if lastTs, ok := lastLineTs(f, size); ok && lastTs.Before(since) {
		return nil, true
	}

	for i, chunk := range logsTailChunks {
		readFrom := int64(0)
		if size > chunk {
			readFrom = size - chunk
		}
		if _, err := f.Seek(readFrom, io.SeekStart); err != nil {
			return nil, false
		}
		data, _ := io.ReadAll(f)
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		// 从偏移读取时首行可能是截断行, 直接丢弃
		if readFrom > 0 && len(lines) > 1 {
			lines = lines[1:]
		}
		// 找到第一个时间戳 >= since 的行, 从该行取起（含后续无时间戳的续行）; 没有则返回空
		startIdx := -1
		for j, l := range lines {
			if ts, ok := parseLogTs(l); ok && !ts.Before(since) {
				startIdx = j
				break
			}
		}
		if readFrom == 0 {
			// 整文件已读, 结果最终
			if startIdx >= 0 {
				return lines[startIdx:], true
			}
			return nil, true
		}
		if startIdx > 0 {
			// 边界行在分块中部（其前行均 < since）, 结果最终
			return lines[startIdx:], true
		}
		// startIdx==0（分块首部已 >= since, 边界可能更早）或未找到（整块 < since）→ 扩大分块重读;
		// 末块（8MB）仍无法覆盖（日志速率极高）时返回手头数据
		if i == len(logsTailChunks)-1 {
			if startIdx >= 0 {
				return lines, true
			}
			return nil, true
		}
	}
	return nil, true // 不可达（logsTailChunks 非空）
}

// lastLineTs 读文件尾部 512B, 返回最后一个带时间戳行的时间戳（供预检跳过旧文件）。
func lastLineTs(f *os.File, size int64) (time.Time, bool) {
	from := int64(512)
	if size < from {
		from = 0
	}
	if _, err := f.Seek(size-from, io.SeekStart); err != nil {
		return time.Time{}, false
	}
	data, _ := io.ReadAll(f)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if ts, ok := parseLogTs(lines[i]); ok {
			return ts, true
		}
	}
	return time.Time{}, false
}

// logsTsRe 时间戳前缀: "2026-08-08 04:40:42"（log.LstdFlags 格式）
var logsTsRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s`)

// parseLogTs 解析时间戳前缀, 无前缀（多行续行）时 ok=false。
func parseLogTs(line string) (time.Time, bool) {
	m := logsTsRe.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006/01/02 15:04:05", m[1], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
