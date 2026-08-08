package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyLogPath(t *testing.T) {
	tm := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	cases := []struct{ base, want string }{
		{"/tmp/app.log", "/tmp/app-2026-08-08.log"},
		{"token_chain_data/gateway.log", "token_chain_data/gateway-2026-08-08.log"},
		{"/tmp/nofile", "/tmp/nofile-2026-08-08"},
		{"/tmp/a.b.log", "/tmp/a.b-2026-08-08.log"},
	}
	for _, c := range cases {
		if got := DailyLogPath(c.base, tm); got != c.want {
			t.Errorf("DailyLogPath(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}

// TestDailyRotate 验证跨天后日志文件句柄被轮转(重新打开当天文件),
// 且轮转前后写入的日志都不丢失。
func TestDailyRotate(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "app.log")

	if err := SetupLog(LogOptions{Level: LevelDebug, ToStdout: false, ToFile: true, FilePath: base}); err != nil {
		t.Fatalf("SetupLog: %v", err)
	}
	LogInfof("day-one")

	gLog.mu.Lock()
	oldF := gLog.f
	gLog.mu.Unlock()
	if oldF == nil {
		t.Fatal("log file not opened after SetupLog")
	}

	// 模拟跨天: 把文件日期改成过去, 下一次写入应触发轮转(重新打开当天文件)
	gLog.mu.Lock()
	gLog.curDate = "2000-01-01"
	gLog.mu.Unlock()
	LogInfof("day-two")

	gLog.mu.Lock()
	newF := gLog.f
	curDate := gLog.curDate
	gLog.mu.Unlock()
	if newF == nil || newF == oldF {
		t.Errorf("expected file handle rotation after date change (old=%v new=%v)", oldF, newF)
	}
	if curDate != time.Now().Format("2006-01-02") {
		t.Errorf("curDate after rotate = %q, want today", curDate)
	}

	// 轮转后日志写入当天文件, 且旧内容仍在
	today := filepath.Join(dir, "app-"+time.Now().Format("2006-01-02")+".log")
	b, err := os.ReadFile(today)
	if err != nil {
		t.Fatalf("read today log %s: %v", today, err)
	}
	s := string(b)
	if !strings.Contains(s, "day-one") || !strings.Contains(s, "day-two") {
		t.Errorf("today log missing entries, got: %s", s)
	}

	// 再次写入, 确认轮转后仍可正常写文件
	LogInfof("day-two-real")
	b, err = os.ReadFile(today)
	if err != nil {
		t.Fatalf("re-read today log: %v", err)
	}
	if !strings.Contains(string(b), "day-two-real") {
		t.Errorf("log write after rotation failed, got: %s", b)
	}
}
