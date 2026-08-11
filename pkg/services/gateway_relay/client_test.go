package relay

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStream 构造一个 sseStream，直接注入内存 body。
func newTestStream(ctx context.Context, body string) *sseStream {
	return &sseStream{
		reader: bufio.NewReaderSize(strings.NewReader(body), 64*1024),
		body:   io.NopCloser(strings.NewReader(body)),
		ctx:    ctx,
	}
}

// ── sseStream 单元测试 ─────────────────────────────────────

func TestSseStream_BasicLines(t *testing.T) {
	s := newTestStream(context.Background(), "data: hello\n\ndata: world\n")

	chunk, err := s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "data: hello\n", string(chunk.Data))
	assert.Equal(t, 0, chunk.Index)

	chunk, err = s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "\n", string(chunk.Data))
	assert.Equal(t, 1, chunk.Index)

	chunk, err = s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "data: world\n", string(chunk.Data))
	assert.Equal(t, 2, chunk.Index)

	// EOF → IsFinal
	chunk, err = s.Next(context.Background())
	require.NoError(t, err)
	assert.True(t, chunk.IsFinal)
	assert.Equal(t, 3, chunk.Index)
}

func TestSseStream_ResidualAtEOF(t *testing.T) {
	// 末尾无换行的残留字节应先发出，再返回 IsFinal
	s := newTestStream(context.Background(), "data: a\nrest")

	chunk, err := s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "data: a\n", string(chunk.Data))

	chunk, err = s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "rest", string(chunk.Data), "residual bytes must be emitted before EOF")

	// EOF 分支内部已调用 Close，后续调用应返回 ErrStreamClosed
	_, err = s.Next(context.Background())
	assert.ErrorIs(t, err, scheduler.ErrStreamClosed)
}

func TestSseStream_LineOverLimit(t *testing.T) {
	// 回归测试：超过 sseMaxLineBytes 的单行必须报错，而不是无限累积内存
	// （此前使用 bufio.ReadBytes 无上限，恶意上游可借此 OOM）。
	huge := strings.Repeat("a", sseMaxLineBytes+1024)
	s := newTestStream(context.Background(), huge+"\n")

	_, err := s.Next(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sse line exceeds")
}

func TestSseStream_LineAtLimitOk(t *testing.T) {
	// 恰好等于上限（无换行、EOF 结束）应正常发出
	exact := strings.Repeat("b", sseMaxLineBytes)
	s := newTestStream(context.Background(), exact)

	chunk, err := s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, exact, string(chunk.Data))
}

func TestSseStream_Closed(t *testing.T) {
	s := newTestStream(context.Background(), "data: x\n")
	require.NoError(t, s.Close())

	_, err := s.Next(context.Background())
	assert.ErrorIs(t, err, scheduler.ErrStreamClosed)

	// 重复 Close 幂等
	require.NoError(t, s.Close())
}

func TestSseStream_CtxCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 已取消

	s := newTestStream(ctx, "data: x\n")
	_, err := s.Next(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSseStream_CallerCtxCanceled(t *testing.T) {
	s := newTestStream(context.Background(), "data: x\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Next(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// ── readResponse 单元测试 ──────────────────────────────────

func TestReadResponse_Success(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	b, err := readResponse(resp, nil)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(b))
}

func TestReadResponse_SuccessBodyOverLimit(t *testing.T) {
	// 成功响应超限：必须显式报错而不是静默截断出损坏的 JSON
	huge := strings.Repeat("x", upstreamErrorBodyLimit+1024)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(huge)),
	}
	_, err := readResponse(resp, nil)
	require.Error(t, err)
	var ue *scheduler.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Contains(t, ue.Message, "exceeds")
}

func TestReadResponse_ErrorBodyTruncated(t *testing.T) {
	// 错误响应超限：截断并标记，不能把整块恶意 body 暴露给上层
	huge := strings.Repeat("e", upstreamErrorBodyLimit+1024)
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(huge)),
	}
	_, err := readResponse(resp, nil)
	require.Error(t, err)
	var ue *scheduler.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusBadGateway, ue.Status)
	assert.Contains(t, ue.Message, "[truncated]")
	assert.LessOrEqual(t, len(ue.Message), upstreamErrorBodyLimit+64)
}

// ── truncateLog / modelFromBody 单元测试 ────────────────────

func TestTruncateLog(t *testing.T) {
	short := "hello"
	assert.Equal(t, short, truncateLog(short, 1024))

	long := strings.Repeat("x", 100)
	out := truncateLog(long, 10)
	assert.Contains(t, out, "[truncated")
	assert.LessOrEqual(t, len(out), 64)
}

func TestModelFromBody(t *testing.T) {
	assert.Equal(t, "gpt-4o", modelFromBody([]byte(`{"model":"gpt-4o"}`)))
	assert.Equal(t, "", modelFromBody([]byte(`not json`)))
	assert.Equal(t, "", modelFromBody(nil))
}

// ── endpointLabel 单元测试 ─────────────────────────────────

func TestEndpointLabel_Nil(t *testing.T) {
	assert.Equal(t, "endpoint=nil", endpointLabel(nil))
}

func TestEndpointLabel_NoSecrets(t *testing.T) {
	ep := &scheduler.Endpoint{
		Provider: "openai", KeyValue: "sk-SECRET-123", ModelName: "gpt-4o",
	}
	label := endpointLabel(ep)
	assert.NotContains(t, label, "sk-SECRET")
}
