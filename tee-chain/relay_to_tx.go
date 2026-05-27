// relay_worker.go 后台拉取 + 缓存层 | Background fetcher + cache layer
//
// 设计 | Design:
//
//	所有节点后台 goroutine 每 3s 拉取主链事件 → 预签名 → 缓存 | all nodes fetch main chain events every 3s → pre-sign → cache
//	出块节点 PrepareProposal 取 ≥2s 成熟事件打包 | proposer packs events cached ≥2s into block
//	成熟度保证其他验证者也拉到了 | maturity ensures other validators also fetched
package teechain

import (
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/relay"
)

// cachedEvent 缓存的主链事件 (tx 预签名, 出块时零开销) | Cached main chain event (tx pre-signed, zero cost at block time)
type cachedEvent struct {
	ev       relay.SyncEvent // 事件数据 | event data
	cachedAt time.Time       // 缓存时间 | cache timestamp (used for maturity check)
	txBytes  []byte          // 预签名的 tx bytes | pre-signed transaction bytes
}

// relayCache 每个节点的 relay 事件缓存 | Per-node relay event cache
type relayCache struct {
	mu     sync.RWMutex             // 读写锁 | read-write mutex
	events map[uint32][]cachedEvent // chainID → 排序的缓存事件 | chainID → sorted cached events
}

// newRelayCache 创建缓存 | Create cache
func newRelayCache() *relayCache {
	return &relayCache{events: make(map[uint32][]cachedEvent)}
}

// startRelayWorker 启动后台拉取 goroutine | Start background fetch goroutine
func (app *TeeChain) startRelayWorker() {
	util.LogWithGreen("RELAY-WORKER", "starting background fetcher (3s interval)")
	app.relayCache = newRelayCache()
	go func() {
		ticker := time.NewTicker(3 * time.Second) // 每 3 秒拉取 | fetch every 3s
		for range ticker.C {
			app.fetchAndCacheRelayEvents()
		}
	}()
}

// fetchAndCacheRelayEvents 拉取主链新事件 → 缓存到内存 | Fetch new main chain events → cache in memory
func (app *TeeChain) fetchAndCacheRelayEvents() {
	if app.relay == nil || app.dkg == nil {
		util.LogWithGray("RELAY-WORKER", "fetch skipped: chains/relay/dkg not ready")
		return
	}
	// 遍历所有已注册的主链 | iterate all registered main chains
	for _, c := range chains.Chains {
		if c == nil {
			continue
		}

		chainID := c.ChainId()

		// 查询主链最新 nonce | query main chain latest nonce
		latest, err := c.GetLatestNonce()
		if err != nil {
			util.LogWithRed("RELAY-WORKER", "GetLatestNonce failed, chain", c.ChainId(), "err:", err)
			continue
		}
		if latest == 0 {
			continue
		}

		// 侧链已处理的 nonce | sidechain last processed nonce
		last, _ := app.relay.LastNonce(c.ChainId())

		app.relayCache.mu.Lock()
		cached := app.relayCache.events[chainID]

		// 计算本地已缓存的最大 nonce | compute max locally cached nonce
		var cachedMax uint64
		if len(cached) > 0 {
			cachedMax = cached[len(cached)-1].ev.Nonce
		}
		if last > cachedMax {
			cachedMax = last
		}

		// 只拉取未缓存的新事件 | only fetch uncached new events
		start := cachedMax + 1
		if start > latest {
			app.relayCache.mu.Unlock()
			continue
		}

		util.LogWithCyan("RELAY-WORKER", "fetching events, chain", chainID, "from", start, "to", latest)
		var newEvents []cachedEvent

		// 批量拉取 | Batch fetch
		records, err := c.GetRelayEvents(start, latest)
		if err != nil {
			util.LogWithRed("RELAY-WORKER", "GetRelayEvents failed, chain", chainID, "err:", err)
			app.relayCache.mu.Unlock()
			continue
		}
		for i, rec := range records {
			n := start + uint64(i)
			ev := relay.BuildSyncEvent(chainID, n, 0, rec.TargetContract, rec.EventType, rec.EventData)
			sysCall := relay.MakeRelaySysCall(ev)
			txBytes, err := app.SignAndBuildRelayTx(sysCall)
			if err != nil {
				continue
			}
			newEvents = append(newEvents, cachedEvent{ev: ev, cachedAt: time.Now(), txBytes: txBytes})
		}

		// 追加到缓存 (保持 nonce 递增顺序) | append to cache (maintaining nonce order)
		app.relayCache.events[chainID] = append(cached, newEvents...)
		app.relayCache.mu.Unlock()
		if len(newEvents) > 0 {
			util.LogOk("RELAY-WORKER", "cached", len(newEvents), "new events, chain", chainID, "total cached", len(app.relayCache.events[chainID]))
		}
	}
}

// relayMinCacheAge 事件最少缓存时间 (确保其他验证者也拿到了) | Minimum cache age (ensure other validators also fetched)
const relayMinCacheAge = 2 * time.Second

// prepareRelayTxs 按 nonce 顺序取 >= 2s 成熟事件 (PrepareProposal 调用) | Take mature events ≥2s in nonce order (called by PrepareProposal)
func (app *TeeChain) prepareRelayTxs() [][]byte {
	if app.relayCache == nil {
		return nil
	}
	app.relayCache.mu.RLock()
	defer app.relayCache.mu.RUnlock()

	var txs [][]byte
	now := time.Now()

	for chainID, events := range app.relayCache.events {
		// 已处理的 nonce (侧链状态) | last processed nonce (sidechain state)
		last, _ := app.relay.LastNonce(chainID)
		for _, ce := range events {
			if ce.ev.Nonce <= last {
				// 已处理, 跳过 | already processed, skip
				continue
			}
			if ce.ev.Nonce != last+1 {
				// nonce 不连续 (前序事件未成熟), 停止等待下一轮 | nonce gap (previous event not mature), stop and wait
				break
			}
			if now.Sub(ce.cachedAt) < relayMinCacheAge {
				// 不成熟, 后续事件更不可能成熟 | not mature yet, later events even less likely
				break
			}
			// 成熟 → 打包进区块 | mature → pack into block
			txs = append(txs, ce.txBytes)
			last = ce.ev.Nonce
			util.LogWithYellow("RELAY-WORKER", "packing tx, chain", chainID, "nonce", ce.ev.Nonce)
		}
	}
	return txs
}
