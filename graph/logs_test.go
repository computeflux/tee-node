package graph

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── handleLogsAPI ───────────────────────────────────────────

// logsTestSetup 设置 LOG_DIR（临时目录）+ LOG_API_TOKEN，返回目录。
func logsTestSetup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOG_API_TOKEN", "sekrit")
	return dir
}

// logsRequest 构造 /api/logs 请求（query 经 URL 编码, RFC3339 的 + 偏移不会被解码成空格）。
func logsRequest(t *testing.T, q url.Values, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/logs?"+q.Encode(), nil)
	if token != "" {
		req.Header.Set("X-Log-Token", token)
	}
	w := httptest.NewRecorder()
	handleLogsAPI(w, req)
	return w
}

func TestHandleLogsAPI_DisabledWhenTokenUnset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	os.Unsetenv("LOG_API_TOKEN")

	q := url.Values{"file": {"app"}, "since": {time.Now().Format(time.RFC3339)}}
	w := logsRequest(t, q, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleLogsAPI_BadToken(t *testing.T) {
	logsTestSetup(t)

	q := url.Values{"file": {"app"}, "since": {time.Now().Format(time.RFC3339)}}
	w := logsRequest(t, q, "wrong")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleLogsAPI_InvalidParams(t *testing.T) {
	logsTestSetup(t)
	since := time.Now().Format(time.RFC3339)

	assert.Equal(t, http.StatusBadRequest, logsRequest(t, url.Values{"file": {"etc"}, "since": {since}}, "sekrit").Code)
	assert.Equal(t, http.StatusBadRequest, logsRequest(t, url.Values{"file": {"app"}, "since": {"not-a-time"}}, "sekrit").Code)
	// since 超过 90d
	assert.Equal(t, http.StatusBadRequest, logsRequest(t, url.Values{"file": {"app"}, "since": {time.Now().Add(-100 * 24 * time.Hour).Format(time.RFC3339)}}, "sekrit").Code)
}

func TestHandleLogsAPI_WindowFilter(t *testing.T) {
	dir := logsTestSetup(t)
	now := time.Now()
	today := now.Format("2006-01-02")
	ts := now.Format("2006/01/02 15:04:05")
	oldTs := now.Add(-2 * time.Hour).Format("2006/01/02 15:04:05")

	content := strings.Join([]string{
		oldTs + " [ERROR] outside window",
		ts + " [ERROR] in window",
		ts + " [INFO] normal line",
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app-"+today+".log"), []byte(content), 0644))

	w := logsRequest(t, url.Values{"file": {"app"}, "since": {now.Add(-5 * time.Minute).Format(time.RFC3339)}}, "sekrit")
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// 节点只回原始行（含非错误行），过滤/分类是 monitor 的职责
	assert.Contains(t, body, "in window")
	assert.Contains(t, body, "normal line")
	assert.NotContains(t, body, "outside window")
}

func TestHandleLogsAPI_MissingFile(t *testing.T) {
	logsTestSetup(t) // 空目录, 无任何日志文件

	w := logsRequest(t, url.Values{"file": {"app"}, "since": {time.Now().Add(-time.Hour).Format(time.RFC3339)}}, "sekrit")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestHandleLogsAPI_Limit(t *testing.T) {
	dir := logsTestSetup(t)
	now := time.Now()
	today := now.Format("2006-01-02")

	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, now.Format("2006/01/02 15:04:05")+" [ERROR] line "+fmt.Sprint(i))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app-"+today+".log"), []byte(strings.Join(lines, "\n")+"\n"), 0644))

	w := logsRequest(t, url.Values{"file": {"app"}, "since": {now.Add(-time.Hour).Format(time.RFC3339)}, "limit": {"10"}}, "sekrit")
	require.Equal(t, http.StatusOK, w.Code)
	got := strings.TrimSpace(w.Body.String())
	assert.Equal(t, 10, len(strings.Split(got, "\n")))
	// 保留最后 10 条
	assert.Contains(t, got, "line 99")
	assert.NotContains(t, got, "line 89")
}

// ── tailLinesSince ──────────────────────────────────────────

func TestTailLinesSince_MissingFile(t *testing.T) {
	_, ok := tailLinesSince(filepath.Join(t.TempDir(), "nope.log"), time.Now())
	assert.False(t, ok)
}

func TestTailLinesSince_NoLinesInWindow(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	old := time.Now().Add(-72 * time.Hour).Format("2006/01/02 15:04:05")
	require.NoError(t, os.WriteFile(p, []byte(old+" [ERROR] old only\n"), 0644))

	lines, ok := tailLinesSince(p, time.Now().Add(-time.Hour))
	assert.True(t, ok)
	assert.Empty(t, lines)
}

func TestTailLinesSince_TailReadOfLargeFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")

	// 填充 ~9MB 的旧日志（超出最大分块 8MB），再追加窗口内的新日志
	maxChunk := int(logsTailChunks[len(logsTailChunks)-1])
	var b strings.Builder
	oldTs := "2020/01/01 00:00:00"
	for b.Len() < maxChunk+1024 {
		fmt.Fprintf(&b, "%s [INFO] pad line to inflate the file\n", oldTs)
	}
	newTs := time.Now().Format("2006/01/02 15:04:05")
	b.WriteString(newTs + " [ERROR] recent one\n")
	b.WriteString("continuation line without timestamp\n")
	require.NoError(t, os.WriteFile(p, []byte(b.String()), 0644))

	lines, ok := tailLinesSince(p, time.Now().Add(-time.Hour))
	require.True(t, ok)
	assert.Equal(t, []string{
		newTs + " [ERROR] recent one",
		"continuation line without timestamp",
	}, lines)
}

func TestTailLinesSince_RecentBoundaryInFirstChunk(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")

	// 填充 ~3MB 旧日志 + 尾部 2 条新日志: 边界在 1MB 分块中部,
	// 小分块即可命中（无需读整个 8MB 尾部）
	var b strings.Builder
	oldTs := "2020/01/01 00:00:00"
	for b.Len() < 3<<20 {
		fmt.Fprintf(&b, "%s [INFO] old line to inflate the file\n", oldTs)
	}
	newTs := time.Now().Format("2006/01/02 15:04:05")
	b.WriteString(newTs + " [ERROR] recent one\n")
	b.WriteString("continuation line without timestamp\n")
	require.NoError(t, os.WriteFile(p, []byte(b.String()), 0644))

	lines, ok := tailLinesSince(p, time.Now().Add(-time.Hour))
	require.True(t, ok)
	assert.Equal(t, []string{
		newTs + " [ERROR] recent one",
		"continuation line without timestamp",
	}, lines)
}

func TestTailLinesSince_OldFileSkippedByLastLineCheck(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")

	// 整文件均为旧日志（最后一行 < since）: 预检直接跳过, 无匹配
	old := time.Now().Add(-72 * time.Hour).Format("2006/01/02 15:04:05")
	require.NoError(t, os.WriteFile(p, []byte(old+" [ERROR] old one\n"+old+" [INFO] old two\n"), 0644))

	lines, ok := tailLinesSince(p, time.Now().Add(-time.Hour))
	assert.True(t, ok)
	assert.Empty(t, lines)
}

func TestCollectLogLines_HistoryDay(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)

	now := time.Now()
	// 前天和昨天的完整日志 + 今天一条新日志
	for _, daysAgo := range []int{2, 1} {
		d := now.AddDate(0, 0, -daysAgo)
		ts := d.Format("2006/01/02 15:04:05")
		name := "app-" + d.Format("2006-01-02") + ".log"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(ts+" [ERROR] day -"+fmt.Sprint(daysAgo)+"\n"), 0644))
	}
	todayTs := now.Format("2006/01/02 15:04:05")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app-"+now.Format("2006-01-02")+".log"), []byte(todayTs+" [ERROR] today\n"), 0644))

	// since = 前天零点: 应覆盖前天/昨天/今天三天（逐天文件 + 预检跳过更旧文件）
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -2)
	lines := collectLogLines("app", start, 5000)
	assert.Equal(t, 3, len(lines))
}

func TestTailLinesSince_ChunkExpansionAllRecent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")

	// 填充 ~1.5MB 全部为"最近"日志: 1MB 分块首行已 >= since（边界在分块之前）,
	// 必须扩大分块到整文件, 返回全部行
	var b strings.Builder
	newTs := time.Now().Format("2006/01/02 15:04:05")
	for b.Len() < 1536*1024 {
		fmt.Fprintf(&b, "%s [INFO] recent line\n", newTs)
	}
	require.NoError(t, os.WriteFile(p, []byte(b.String()), 0644))

	lines, ok := tailLinesSince(p, time.Now().Add(-time.Hour))
	require.True(t, ok)
	assert.Greater(t, len(lines), 1000) // 边界 = 文件头, 全部返回
}
