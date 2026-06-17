package localtunnel

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-yamux/v5"
)

// ValidateAPIKeyFunc 验证 API Key 是否合法，并返回该模型所属的 Provider ID。
// 返回 (0, true) 表示验证通过但无法识别 Provider（旧版兼容）。
type ValidateAPIKeyFunc func(modelID, apiKey string) (providerID uint64, ok bool)

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
	ProviderID  uint64 // 供应商 ID（0 表示旧版未指定）
	ModelID     string
	APIKey      string
	BaseURL     string
	RemoteAddr  string
	ConnectedAt time.Time
	Available   bool // 10s 预热完成后设为 true
	InFlight    int32
	mu          sync.RWMutex
}

// Pool 按 routeKey(providerID, modelID) 管理本地客户端 yamux session
// 一个模型同时只能有一个连接提供服务。
type Pool struct {
	mu             sync.RWMutex
	entries        map[string]*PoolEntry // routeKey -> entry
	warmupDuration time.Duration
	validateKey    ValidateAPIKeyFunc
	metrics        PoolMetrics
	closed         bool
}

// PoolOption 连接池配置选项
type PoolOption func(*Pool)

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
		entries:        make(map[string]*PoolEntry),
		warmupDuration: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Validate 验证握手信息是否合法，并根据 API Key 推导出 ProviderID 写入 h。
func (p *Pool) Validate(h *Handshake, remoteAddr string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	models := h.RegisteredModels()
	if len(models) == 0 {
		return fmt.Errorf("no model registered")
	}

	// 验证每个模型的 API Key，并解析 ProviderID
	var resolvedProviderID uint64
	var resolved bool
	if p.validateKey != nil {
		for _, modelID := range models {
			pid, ok := p.validateKey(modelID, h.APIKey)
			if !ok {
				atomic.AddInt64(&p.metrics.RejectedHandshakes, 1)
				return fmt.Errorf("invalid api key for model %s", modelID)
			}
			if !resolved {
				resolvedProviderID = pid
				resolved = true
			} else if resolvedProviderID != pid {
				return fmt.Errorf("models belong to different providers")
			}
		}
	}
	h.ProviderID = resolvedProviderID

	// 检查每个模型是否已经有活跃连接（按 providerID/modelID 隔离）
	for _, modelID := range models {
		rk := routeKey(h.ProviderID, modelID)
		if existing, ok := p.entries[rk]; ok {
			existing.mu.RLock()
			closed := existing.Session.IsClosed()
			existing.mu.RUnlock()
			if !closed {
				return fmt.Errorf("model %s already connected", modelID)
			}
		}
	}

	return nil
}

// CheckModelQuota 检查指定路由键是否已经有活跃连接
func (p *Pool) CheckModelQuota(routeKey string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}
	if existing, ok := p.entries[routeKey]; ok {
		existing.mu.RLock()
		closed := existing.Session.IsClosed()
		existing.mu.RUnlock()
		if !closed {
			return fmt.Errorf("model already connected for route %s", routeKey)
		}
	}
	return nil
}

// Add 添加新 session，启动预热计时器。
// 同一 provider/model 已存在活跃连接时会拒绝新连接；旧连接已断开则替换。
func (p *Pool) Add(entry *PoolEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		entry.Session.Close()
		return
	}

	rk := routeKey(entry.ProviderID, entry.ModelID)
	if existing, ok := p.entries[rk]; ok {
		existing.mu.RLock()
		closed := existing.Session.IsClosed()
		existing.mu.RUnlock()

		if !closed {
			// 已存在活跃连接，关闭新连接
			entry.Session.Close()
			return
		}
		// 旧连接已断开，删除旧条目并修正指标
		delete(p.entries, rk)
		atomic.AddInt64(&p.metrics.ActiveConnections, -1)
	}
	p.entries[rk] = entry
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

	rk := routeKey(entry.ProviderID, entry.ModelID)
	if existing, ok := p.entries[rk]; ok && existing == entry {
		delete(p.entries, rk)
		atomic.AddInt64(&p.metrics.ActiveConnections, -1)
	}
}

// Pick 返回指定路由键的可用 session
func (p *Pool) Pick(routeKey string) *PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	e := p.entries[routeKey]
	if e == nil {
		return nil
	}

	e.mu.RLock()
	avail := e.Available
	closed := e.Session.IsClosed()
	e.mu.RUnlock()

	if !avail || closed {
		return nil
	}
	return e
}

// Count 返回指定路由键的可用连接数（0 或 1）
func (p *Pool) Count(routeKey string) int {
	p.mu.RLock()
	e := p.entries[routeKey]
	p.mu.RUnlock()

	if e == nil {
		return 0
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.Available && !e.Session.IsClosed() {
		return 1
	}
	return 0
}

// HasEntry 检查指定 provider/model 在池中是否存在可用连接。
// 用于前端模型列表过滤：ProviderType 为 9999 的个人算力必须在线才展示。
func (p *Pool) HasEntry(providerID uint64, modelID string) bool {
	rk := routeKey(providerID, modelID)
	p.mu.RLock()
	e := p.entries[rk]
	p.mu.RUnlock()

	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Available && !e.Session.IsClosed()
}

// AllEntries 返回所有连接条目（用于心跳检测）
func (p *Pool) AllEntries() []*PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	all := make([]*PoolEntry, 0, len(p.entries))
	for _, e := range p.entries {
		all = append(all, e)
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
	all := make([]*PoolEntry, 0, len(p.entries))
	for _, e := range p.entries {
		all = append(all, e)
	}
	p.entries = make(map[string]*PoolEntry)
	p.mu.Unlock()

	for _, e := range all {
		e.Session.Close()
	}
}

// GlobalPool 全局连接池实例
var GlobalPool = NewPool()
