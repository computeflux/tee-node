// Package indexer 提供基于 Bleve 的合约内存索引框架。
// 每个合约模块可以实现 Index 接口并注册到 Indexer，由 Indexer 统一负责周期刷新和并发访问。
package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

// Logger 定义索引器刷新失败时的日志回调。
// 默认使用 fmt.Printf，可在外部替换为项目统一的 logger。
var Logger = func(format string, args ...any) {
	fmt.Printf("[INDEXER] "+format+"\n", args...)
}

// Index 定义一个合约内存索引需要实现的能力。
// 未来 gov、batch 等模块可各自实现该接口并注册到 Indexer。
type Index interface {
	// Name 返回索引唯一标识名。
	Name() string
	// Refresh 从链上拉取最新数据并重建索引。
	Refresh(ctx context.Context, chain *teechain.TeeChain) error
	// Bleve 返回当前底层 Bleve 索引。
	Bleve() bleve.Index
}

// Indexer 统一管理多个合约索引的生命周期与刷新。
type Indexer struct {
	chain   *teechain.TeeChain
	mu      sync.RWMutex
	indexes map[string]Index
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewIndexer 创建索引管理器。
func NewIndexer(chain *teechain.TeeChain) *Indexer {
	return &Indexer{
		chain:   chain,
		indexes: make(map[string]Index),
	}
}

// Register 注册一个索引实现。
// 索引名必须唯一，重复注册返回错误。
func (idx *Indexer) Register(index Index) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	name := index.Name()
	if _, ok := idx.indexes[name]; ok {
		return fmt.Errorf("index %s already registered", name)
	}
	idx.indexes[name] = index
	return nil
}

// Start 启动周期刷新，并立即执行一次全量刷新。
func (idx *Indexer) Start(refreshInterval time.Duration) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.cancel != nil {
		return fmt.Errorf("indexer already started")
	}

	ctx, cancel := context.WithCancel(context.Background())
	idx.ctx = ctx
	idx.cancel = cancel

	// 启动时立即刷新一次。
	if err := idx.refreshAllLocked(ctx); err != nil {
		cancel()
		idx.ctx = nil
		idx.cancel = nil
		return fmt.Errorf("initial index refresh failed: %w", err)
	}

	go idx.refreshLoop(refreshInterval)
	return nil
}

// Stop 停止周期刷新并关闭所有底层 Bleve 索引。
func (idx *Indexer) Stop() {
	idx.mu.Lock()
	cancel := idx.cancel
	idx.cancel = nil
	indexes := make([]Index, 0, len(idx.indexes))
	for _, index := range idx.indexes {
		indexes = append(indexes, index)
	}
	idx.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, index := range indexes {
		if bleveIdx := index.Bleve(); bleveIdx != nil {
			_ = bleveIdx.Close()
		}
	}
}

// Get 按名称获取已注册的索引。
func (idx *Indexer) Get(name string) (Index, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	index, ok := idx.indexes[name]
	if !ok {
		return nil, fmt.Errorf("index %s not found", name)
	}
	return index, nil
}

// Refresh 立即刷新指定索引。
func (idx *Indexer) Refresh(ctx context.Context, name string) error {
	idx.mu.RLock()
	index, ok := idx.indexes[name]
	idx.mu.RUnlock()

	if !ok {
		return fmt.Errorf("index %s not found", name)
	}
	return index.Refresh(ctx, idx.chain)
}

// RefreshAll 立即刷新所有已注册索引。
func (idx *Indexer) RefreshAll(ctx context.Context) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.refreshAllLocked(ctx)
}

func (idx *Indexer) refreshAllLocked(ctx context.Context) error {
	var errs []error
	for _, index := range idx.indexes {
		if err := index.Refresh(ctx, idx.chain); err != nil {
			errs = append(errs, fmt.Errorf("refresh %s: %w", index.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

func (idx *Indexer) refreshLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-idx.ctx.Done():
			return
		case <-ticker.C:
			if err := idx.RefreshAll(idx.ctx); err != nil {
				Logger("refresh failed: %v", err)
			}
		}
	}
}
