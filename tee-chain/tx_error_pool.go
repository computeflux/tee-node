package teechain

import (
	"fmt"
	"strings"
	"sync"
	"time"

	store "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/util"
)

const (
	// txErrMaxRetries 同一 hub call 连续失败达到此次数后进入错误池（跳过）
	txErrMaxRetries = 3
	// txErrPoolTTL 错误池条目过期时间（秒），过期后允许重新尝试
	txErrPoolTTL = 30 * 60
	// txErrPoolPrefix 错误池存储 key 前缀
	txErrPoolPrefix = "tx_err_pool_"
	// txErrPoolCleanupInterval 后台清理过期条目的周期
	txErrPoolCleanupInterval = 10 * time.Minute
)

// txErrPoolEntry 持久化的 hub call 失败状态。
type txErrPoolEntry struct {
	FailCount  int    `json:"failCount"`
	LastErr    string `json:"lastErr"`
	PoisonedAt int64  `json:"poisonedAt"` // unix 秒，进入错误池的时间
}

// txErrPool 错误交易池：跟踪反复失败的 hub call，避免每次 FinalizeTx 都重复执行。
//
// 持久化到 store.SECRET_DB（重启保留），并维护一份内存读缓存，避免每个 hub call
// 在热路径上都做一次 DB 读。DB 是唯一事实来源，缓存只是读加速。
//
// 注意：TEECallToCall 会对主链做 live dry-run，结果在各 BFT 节点间不一致，
// 因此本池是**本地节点优化**，不能写入共识 txn（否则会导致节点状态分叉）。
//
// 并发：sendPartialSignByIds 仅在 FinalizeTx（CometBFT 顺序执行）中调用，
// 唯一的并发访问者是后台 cleanup goroutine（只删除，不改计数），故无计数写冲突。
type txErrorPool struct {
	mu    sync.RWMutex
	cache map[string]*txErrPoolEntry
}

var txErrPool = &txErrorPool{cache: make(map[string]*txErrPoolEntry)}

func txErrPoolKey(chainId uint32, callId uint64) string {
	return fmt.Sprintf("%s%d_%d", txErrPoolPrefix, chainId, callId)
}

// isPoisoned 检查 hub call 是否在错误池中（应跳过）。
// 快路径走内存缓存，未命中再读 DB 并回填。字段读取在锁内做快照，避免数据竞争。
func (p *txErrorPool) isPoisoned(chainId uint32, callId uint64) bool {
	key := txErrPoolKey(chainId, callId)

	p.mu.RLock()
	e := p.cache[key]
	var snapshot *txErrPoolEntry
	if e != nil {
		s := *e
		snapshot = &s
	}
	p.mu.RUnlock()

	if snapshot == nil {
		if store.SECRET_DB == nil {
			return false
		}
		db, err := store.DBGetJson[txErrPoolEntry](store.SECRET_DB, GLOABL_STATE, key)
		if err != nil || db == nil {
			return false
		}
		snapshot = db
		p.mu.Lock()
		// 仅当缓存仍为空时回填，避免覆盖并发写入
		if p.cache[key] == nil {
			p.cache[key] = db
		}
		p.mu.Unlock()
	}

	if snapshot.FailCount < txErrMaxRetries {
		return false
	}
	if time.Now().Unix()-snapshot.PoisonedAt > txErrPoolTTL {
		p.purge(key)
		return false
	}
	return true
}

// recordFailure 记录一次 TEECallToCall 失败并持久化。
// 达到 txErrMaxRetries 时标记 poisoned 并打印醒目日志。
func (p *txErrorPool) recordFailure(chainId uint32, callId uint64, errMsg string) {
	key := txErrPoolKey(chainId, callId)

	// 取当前条目：优先缓存，未命中则从 DB 恢复（重启后不丢已有计数）
	p.mu.RLock()
	e, ok := p.cache[key]
	p.mu.RUnlock()
	if !ok {
		e = &txErrPoolEntry{}
		if store.SECRET_DB != nil {
			if db, err := store.DBGetJson[txErrPoolEntry](store.SECRET_DB, GLOABL_STATE, key); err == nil && db != nil {
				e = db
			}
		}
		p.mu.Lock()
		if existing, exists := p.cache[key]; exists {
			e = existing
		} else {
			p.cache[key] = e
		}
		p.mu.Unlock()
	}

	p.mu.Lock()
	e.FailCount++
	e.LastErr = errMsg
	if e.FailCount == txErrMaxRetries {
		e.PoisonedAt = time.Now().Unix()
	}
	poisoned := e.FailCount == txErrMaxRetries
	snapshot := *e
	p.mu.Unlock()

	if store.SECRET_DB != nil {
		if err := store.DBSetJson(store.SECRET_DB, GLOABL_STATE, key, &snapshot); err != nil {
			util.LogWithRed("txErrPool", "record failure persist error:", err)
		}
	}
	if poisoned {
		util.LogWithRed("txErrPool", "hub call poisoned after", snapshot.FailCount, "failures:",
			"chainId=", chainId, "callId=", callId, "err=", errMsg,
			"will skip for", txErrPoolTTL, "s (persistent)")
	}
}

// recordSuccess 清除成功执行（或已确认完成）的 hub call 的错误记录。
func (p *txErrorPool) recordSuccess(chainId uint32, callId uint64) {
	p.purge(txErrPoolKey(chainId, callId))
}

// purge 同时清除内存缓存与 DB 中的条目。删除不存在的 key 是幂等的。
func (p *txErrorPool) purge(key string) {
	p.mu.Lock()
	delete(p.cache, key)
	p.mu.Unlock()
	if store.SECRET_DB != nil {
		_ = store.SECRET_DB.DeleteKey(GLOABL_STATE, key)
	}
}

// cleanup 扫描 DB 前缀，删除所有过期条目（后台周期调用），防止残留 key 无限增长。
func (p *txErrorPool) cleanup() {
	if store.SECRET_DB == nil {
		return
	}
	list, keys, err := store.DBGetJsonList[txErrPoolEntry](store.SECRET_DB, GLOABL_STATE, txErrPoolPrefix)
	if err != nil {
		util.LogWithRed("txErrPool", "cleanup scan error:", err)
		return
	}
	now := time.Now().Unix()
	for i, e := range list {
		if e.FailCount < txErrMaxRetries || now-e.PoisonedAt <= txErrPoolTTL {
			continue
		}
		if err := store.SECRET_DB.DeleteByteKey(keys[i]); err != nil {
			util.LogWithRed("txErrPool", "cleanup delete error:", err)
			continue
		}
		// 同步清缓存：DB 返回的是完整 combo key（含命名空间前缀），需还原为 sub key
		subKey := strings.TrimPrefix(string(keys[i]), GLOABL_STATE+"_")
		p.mu.Lock()
		delete(p.cache, subKey)
		p.mu.Unlock()
	}
}

// cleanupLoop 后台周期清理，随节点生命周期运行（与 applyTxLoop 同模式）。
func (p *txErrorPool) cleanupLoop() {
	// 启动时先清一次，回收上次运行遗留的过期条目
	p.cleanup()
	ticker := time.NewTicker(txErrPoolCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		p.cleanup()
	}
}
