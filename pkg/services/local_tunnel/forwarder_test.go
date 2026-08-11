package localtunnel

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestBody 构造一个直接注入 waiter 的 tunnelResponseBody。
func newTestBody(w *responseWaiter, readTimeout time.Duration) *tunnelResponseBody {
	return &tunnelResponseBody{
		waiter:      w,
		stream:      nil,
		reqID:       "",
		readTimeout: readTimeout,
	}
}

// ── tunnelResponseBody 单元测试 ────────────────────────────

// 回归测试：Close 必须幂等。此前重复 Close 会对 closedCh 二次 close 直接 panic。
func TestTunnelResponseBody_CloseIdempotent(t *testing.T) {
	w := newResponseWaiter("id")
	body := newTestBody(w, time.Second)

	require.NoError(t, body.Close())
	require.NoError(t, body.Close(), "second Close must not panic")
}

func TestTunnelResponseBody_ReadAfterClose(t *testing.T) {
	w := newResponseWaiter("id")
	body := newTestBody(w, time.Second)
	require.NoError(t, body.Close())

	buf := make([]byte, 16)
	n, err := body.Read(buf)
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}

func TestTunnelResponseBody_ReadAllDataInOrder(t *testing.T) {
	w := newResponseWaiter("id")
	body := newTestBody(w, time.Second)

	// 模拟帧到达顺序：Data1, Data2, Done
	w.dataChan <- []byte("hello ")
	w.dataChan <- []byte("world")
	w.doneChan <- struct{}{}

	got, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(got), "all data frames must be delivered in order before EOF")
}

// 回归测试：数据已入队且 Done 已到达时，Read 必须先返回数据。
// 此前第二个 select 在 dataChan/doneChan 同时就绪时随机选择，可能直接返回 EOF 丢数据。
func TestTunnelResponseBody_DataQueuedWithDone(t *testing.T) {
	for i := 0; i < 2000; i++ {
		w := newResponseWaiter("id")
		body := newTestBody(w, time.Second)

		payload := []byte("chunk-data")
		w.dataChan <- payload
		w.doneChan <- struct{}{}

		buf := make([]byte, len(payload))
		n, err := body.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, payload, buf[:n], "queued data must be returned before EOF")

		_, err = body.Read(buf)
		assert.ErrorIs(t, err, io.EOF)
	}
}

// 并发发送回归测试：goroutine 发送 data 与 done，读取方必须完整拿到数据。
func TestTunnelResponseBody_ConcurrentSendNoLoss(t *testing.T) {
	for i := 0; i < 500; i++ {
		w := newResponseWaiter("id")
		body := newTestBody(w, 5*time.Second)

		payload := []byte("payload-" + strings.Repeat("x", 32))
		sent := make(chan struct{})
		go func() {
			w.dataChan <- payload
			w.doneChan <- struct{}{}
			close(sent)
		}()

		got, err := io.ReadAll(body)
		require.NoError(t, err)
		assert.Equal(t, payload, got, "no data may be lost when done arrives concurrently")

		<-sent
	}
}

// 小 buffer 读取：剩余数据必须缓存在内部 buffer，Done 到达后仍需先发完缓存。
func TestTunnelResponseBody_SmallBufferRetainsRemainder(t *testing.T) {
	w := newResponseWaiter("id")
	body := newTestBody(w, time.Second)

	payload := []byte("0123456789")
	w.dataChan <- payload
	w.doneChan <- struct{}{}

	small := make([]byte, 4)
	n, err := body.Read(small)
	require.NoError(t, err)
	assert.Equal(t, "0123", string(small[:n]))

	rest, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, "456789", string(rest), "buffered remainder must be flushed before EOF")
}

func TestTunnelResponseBody_ReadTimeout(t *testing.T) {
	w := newResponseWaiter("id")
	body := newTestBody(w, 30*time.Millisecond)

	buf := make([]byte, 16)
	start := time.Now()
	_, err := body.Read(buf)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tunnel read timeout")
	assert.Less(t, elapsed, 2*time.Second, "timeout must fire promptly")
}

// ── readBodyLimited 单元测试 ───────────────────────────────

func TestReadBodyLimited_Ok(t *testing.T) {
	b, err := readBodyLimited(strings.NewReader("hello"), 100)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), b)
}

func TestReadBodyLimited_Nil(t *testing.T) {
	b, err := readBodyLimited(nil, 100)
	require.NoError(t, err)
	assert.Nil(t, b)
}

func TestReadBodyLimited_OverLimit(t *testing.T) {
	_, err := readBodyLimited(strings.NewReader("1234567890"), 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max size")
}

func TestReadBodyLimited_AtLimit(t *testing.T) {
	b, err := readBodyLimited(strings.NewReader("12345"), 5)
	require.NoError(t, err)
	assert.Equal(t, []byte("12345"), b)
}

// ── routeKey 单元测试 ──────────────────────────────────────

func TestRouteKey(t *testing.T) {
	assert.Equal(t, "model-x", routeKey(0, "model-x"), "providerID 0 保持旧版兼容")
	assert.Equal(t, "42/model-x", routeKey(42, "model-x"))
}
