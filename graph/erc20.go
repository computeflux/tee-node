package graph

// ERC20 列表缓存实现。
//
// 注意：不要把这些代码放进 erc20.resolvers.go —— 该文件由 gqlgen 重新生成，
// 非 resolver 的辅助代码会在重新生成时被丢弃（历史事故：commit f9c9968 中被删，
// 导致 undefined: erc20ListCache）。此文件不受 gqlgen 管理，安全。

import (
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
)

// erc20ListCacheTTL 缓存有效期（10 分钟）。用户始终优先读取缓存：
// 缓存新鲜直接返回；过期时先返回旧数据再后台刷新（stale-while-revalidate）。
const erc20ListCacheTTL = 10 * time.Minute

type erc20CacheEntry struct {
	items      []*model.Erc20Item
	fetchedAt  time.Time
	refreshing bool // 后台刷新进行中，避免过期瞬间的并发请求打爆节点
}

// 按 chainId 缓存的 ERC20 列表。
var erc20ListCache = struct {
	mu      sync.Mutex
	entries map[uint32]*erc20CacheEntry
}{entries: make(map[uint32]*erc20CacheEntry)}

// refreshErc20ListCache 后台刷新指定链的 ERC20 缓存。
// 刷新失败时保留旧缓存（下次过期仍会再次尝试），刷新成功则更新 fetchedAt。
func refreshErc20ListCache(cid uint32) {
	items, err := fetchErc20List(cid)
	if err != nil {
		util.LogError("ERC20", "refresh cache", cid, err)
	}

	erc20ListCache.mu.Lock()
	entry := erc20ListCache.entries[cid]
	if err == nil {
		if entry == nil {
			entry = &erc20CacheEntry{}
			erc20ListCache.entries[cid] = entry
		}
		entry.items = items
		entry.fetchedAt = time.Now()
	}
	if entry != nil {
		entry.refreshing = false
	}
	erc20ListCache.mu.Unlock()
}

// fetchErc20List 通过 pkg/chains 获取指定链的 ERC20 列表并转换为 GraphQL 模型。
func fetchErc20List(chainId uint32) ([]*model.Erc20Item, error) {
	chain, err := chains.GetChainByID(chainId)
	if err != nil {
		return nil, err
	}
	tokens, err := chain.GetErc20List()
	if err != nil {
		return nil, err
	}

	items := make([]*model.Erc20Item, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, &model.Erc20Item{
			Address:  t.Address,
			Active:   t.Active,
			Rate:     t.Rate,
			Unit:     t.Unit,
			Name:     t.Name,
			Symbol:   t.Symbol,
			Decimals: int(t.Decimals),
		})
	}
	return items, nil
}
