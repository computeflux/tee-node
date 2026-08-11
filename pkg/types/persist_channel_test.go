package types

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistChan(t *testing.T) {
	os.RemoveAll("./token_chain_data")
	db, err := NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	key := "test_persist_chan"
	cacheLen := uint16(1000)

	pc, err := NewPersistChan[string](key, cacheLen)
	if err != nil {
		t.Fatalf("Failed to create PersistChan: %v", err)
	}

	// Test Push
	for i := range 2 {
		err = pc.Push("test message " + fmt.Sprint(i))
		if err != nil {
			t.Fatalf("Failed to push message: %v", err)
		}
	}

	// Test Start
	pc.Start(func(msg string) error {
		fmt.Println(msg)
		t.SkipNow()
		return nil
	})
}

func TestPersistChanLoad(t *testing.T) {
	// os.RemoveAll("./token_chain_data")
	db, err := NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	key := "test_persist_chan"
	cacheLen := uint16(1000)

	pc, err := NewPersistChan[string](key, cacheLen)
	if err != nil {
		t.Fatalf("Failed to create PersistChan: %v", err)
	}

	// Test Start
	pc.Start(func(msg string) error {
		fmt.Println(msg)
		t.SkipNow()
		return nil
	})
}

// ── OCR 修复回归测试 ──────────────────────────────────────

func TestPersistChan_StopBeforeStart_NoDBWriteAfterClose(t *testing.T) {
	// 回归（OCR）：Stop 先于 Start 返回后，调用方会立即关闭 DB；
	// 迟到的 Start 若再执行 pop→save 会因 DB 已关闭而 panic。
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)

	pc, err := NewPersistChan[string]("test_stop_before_start", 100)
	require.NoError(t, err)
	require.NoError(t, pc.Push("msg1"))

	pc.Stop()
	require.NoError(t, db.Close()) // 模拟调用方在 Stop 返回后立即关闭 DB

	handled := make(chan string, 8)
	go func() {
		pc.Start(func(msg string) error {
			handled <- msg
			return nil
		})
	}()

	select {
	case msg := <-handled:
		t.Fatalf("handler must not be called after Stop, got %q", msg)
	case <-time.After(200 * time.Millisecond):
	}

	// 消息必须仍保留在持久化列表（未 pop），下次启动 load 会重新入队
	pc.mu.Lock()
	assert.Equal(t, []string{"msg1"}, pc.list)
	pc.mu.Unlock()
}

func TestPersistChan_StopBeforeStart_ThenRestart_Processes(t *testing.T) {
	// 验证 Stop 前未处理的消息在“重启”（重新打开 DB）后仍能恢复处理。
	dir := t.TempDir()
	db, err := NewDBWithPath(dir)
	require.NoError(t, err)

	pc, err := NewPersistChan[string]("test_restart_reload", 100)
	require.NoError(t, err)
	require.NoError(t, pc.Push("persisted-msg"))
	pc.Stop()
	require.NoError(t, db.Close())

	// 重启：重新打开同一路径的 DB
	db2, err := NewDBWithPath(dir)
	require.NoError(t, err)
	defer db2.Close()

	pc2, err := NewPersistChan[string]("test_restart_reload", 100)
	require.NoError(t, err)

	got := make(chan string, 4)
	go func() {
		pc2.Start(func(msg string) error {
			got <- msg
			return nil
		})
	}()

	select {
	case msg := <-got:
		assert.Equal(t, "persisted-msg", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("persisted message was not re-processed after restart")
	}
	pc2.Stop()
}

func TestPersistChan_NormalShutdown_ProcessesAllMessages(t *testing.T) {
	// 正常关闭路径：Stop 等待 Start 排空缓冲，所有消息都被处理且从 DB 弹出。
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()

	pc, err := NewPersistChan[int]("test_normal_shutdown", 100)
	require.NoError(t, err)

	got := make(chan int, 16)
	done := make(chan struct{})
	go func() {
		pc.Start(func(msg int) error {
			got <- msg
			return nil
		})
		close(done)
	}()

	for i := range 5 {
		require.NoError(t, pc.Push(i))
	}

	pc.Stop()
	<-done // Start 循环已退出

	close(got)
	var all []int
	for m := range got {
		all = append(all, m)
	}
	assert.ElementsMatch(t, []int{0, 1, 2, 3, 4}, all)

	pc.mu.Lock()
	assert.Empty(t, pc.list, "normal shutdown should pop all messages")
	pc.mu.Unlock()
}

func TestPersistChan_StopTwice_NoPanic(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()

	pc, err := NewPersistChan[string]("test_stop_twice", 100)
	require.NoError(t, err)
	pc.Stop()
	assert.NotPanics(t, pc.Stop)
}
