package localtunnel

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-yamux/v5"
)

// ValidateAPIKeyFunc 验证 API Key 是否合法的回调函数
type ValidateAPIKeyFunc func(modelID, apiKey string) bool

// PoolMetrics 连接池指标
type PoolMetrics struct {
	TotalConnections   int64 // 累计接入连接数
	ActiveConnections  int64 // 当前活跃连接数
	TotalRequests      int64 // 累计转发请求数
	FailedRequests     int64 // 累计失败请求数
	RejectedHandshakes int64 // 因认证失败被拒绝的握手数
}

// PoolEntry 表示一个本地客户端会话
type PoolEntry struct {
	Session     *yamux.Session
	ModelID     string
	APIKey      string
	BaseURL     string
	RemoteAddr  string
	ConnectedAt time.Time
	Available   bool // 10s 预热完成后设为 true
	InFlight    int32
	mu          sync.RWMutex
}

// Pool 按 modelID 管理本地客户端 yamux session
type Pool struct {
	mu              sync.RWMutex
	entries         map[string][]*PoolEntry // modelID -> entries
	maxConnsPerModel int                     // 单模型最大连接数
	warmupDuration  time.Duration
	validateKey     ValidateAPIKeyFunc
	metrics         PoolMetrics
	closed          bool
}

// PoolOption 连接池配置选项
type PoolOption func(*Pool)

// WithMaxConnsPerModel 设置单模型最大连接数
func WithMaxConnsPerModel(n int) PoolOption {
	return func(p *Pool) { p.maxConnsPerModel = n }
}

// WithWarmupDuration 设置预热时间
func WithWarmupDuration(d time.Duration) PoolOption {
	return func(p *Pool) { p.warmupDuration = d }
}

// WithValidateKey 设置 API Key 验证回调
func WithValidateKey(fn ValidateAPIKeyFunc) PoolOption {
	return func(p *Pool) { p.validateKey = fn }
}

// NewPool 创建连接池
func NewPool(opts ...PoolOption) *Pool {
	p := &Pool{
		entries:          make(map[string][]*PoolEntry),
		maxConnsPerModel: 10,
		warmupDuration:   10 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Validate 验证握手信息是否合法
func (p *Pool) Validate(h *Handshake, remoteAddr string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	// 检查单模型最大连接数
	if p.maxConnsPerModel > 0 {
		list := p.entries[h.ModelID]
		if len(list) >= p.maxConnsPerModel {
			return fmt.Errorf("max connections reached for model %s (%d)", h.ModelID, p.maxConnsPerModel)
		}
	}

	// 验证 API Key
	if p.validateKey != nil && !p.validateKey(h.ModelID, h.APIKey) {
		atomic.AddInt64(&p.metrics.RejectedHandshakes, 1)
		return fmt.Errorf("invalid api key for model %s", h.ModelID)
	}

	return nil
}

// Add 添加新 session，启动预热计时器
func (p *Pool) Add(entry *PoolEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		entry.Session.Close()
		return
	}

	p.entries[entry.ModelID] = append(p.entries[entry.ModelID], entry)
	atomic.AddInt64(&p.metrics.TotalConnections, 1)
	atomic.AddInt64(&p.metrics.ActiveConnections, 1)

	go func() {
		timer := time.NewTimer(p.warmupDuration)
		defer timer.Stop()
		select {
		case <-timer.C:
			entry.mu.Lock()
			if !entry.Available {
				entry.Available = true
				fmt.Printf("[LOCAL-TUNNEL] model=%s warmup complete, now available\n", entry.ModelID)
			}
			entry.mu.Unlock()
		case <-entry.Session.CloseChan():
			// 连接在预热期间断开，无需标记为可用
		}
	}()
}

// Remove 移除 session
func (p *Pool) Remove(entry *PoolEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()

	list := p.entries[entry.ModelID]
	for i, e := range list {
		if e == entry {
			p.entries[entry.ModelID] = append(list[:i], list[i+1:]...)
			atomic.AddInt64(&p.metrics.ActiveConnections, -1)
			break
		}
	}
	if len(p.entries[entry.ModelID]) == 0 {
		delete(p.entries, entry.ModelID)
	}
}

// Pick 为指定 modelID 挑选一个可用 session（最小负载轮询）
// 修复了原版的竞态条件：对 best 的 InFlight 访问也加了读锁
func (p *Pool) Pick(modelID string) *PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	list := p.entries[modelID]
	if len(list) == 0 {
		return nil
	}

	var best *PoolEntry
	var bestInflight int32
	for _, e := range list {
		e.mu.RLock()
		avail := e.Available
		inflight := e.InFlight
		closed := e.Session.IsClosed()
		e.mu.RUnlock()

		if !avail || closed {
			continue
		}
		if best == nil || inflight < bestInflight {
			best = e
			bestInflight = inflight
		}
	}
	return best
}

// Count 返回指定 modelID 的可用连接数
func (p *Pool) Count(modelID string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	list := p.entries[modelID]
	count := 0
	for _, e := range list {
		e.mu.RLock()
		if e.Available && !e.Session.IsClosed() {
			count++
		}
		e.mu.RUnlock()
	}
	return count
}

// AllEntries 返回所有连接条目（用于心跳检测）
func (p *Pool) AllEntries() []*PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	all := make([]*PoolEntry, 0)
	for _, list := range p.entries {
		all = append(all, list...)
	}
	return all
}

// Metrics 返回连接池指标
func (p *Pool) Metrics() PoolMetrics {
	return PoolMetrics{
		TotalConnections:   atomic.LoadInt64(&p.metrics.TotalConnections),
		ActiveConnections:  atomic.LoadInt64(&p.metrics.ActiveConnections),
		TotalRequests:      atomic.LoadInt64(&p.metrics.TotalRequests),
		FailedRequests:     atomic.LoadInt64(&p.metrics.FailedRequests),
		RejectedHandshakes: atomic.LoadInt64(&p.metrics.RejectedHandshakes),
	}
}

// IncrRequests 增加请求计数
func (p *Pool) IncrRequests(success bool) {
	atomic.AddInt64(&p.metrics.TotalRequests, 1)
	if !success {
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
	}
}

// Close 优雅关闭连接池，关闭所有 session
func (p *Pool) Close() {
	p.mu.Lock()
	p.closed = true
	all := make([]*PoolEntry, 0)
	for _, list := range p.entries {
		all = append(all, list...)
	}
	p.entries = make(map[string][]*PoolEntry)
	p.mu.Unlock()

	for _, e := range all {
		e.Session.Close()
	}
}

// GlobalPool 全局连接池实例
var GlobalPool = NewPool()
