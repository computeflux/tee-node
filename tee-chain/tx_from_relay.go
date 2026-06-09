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
	teecrypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/model"
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
	app.relayWorkerOnce.Do(func() {
		util.LogWithGreen("RELAY-WORKER", "starting background fetcher (3s interval)")
		app.relayCache = newRelayCache()
		go func() {
			ticker := time.NewTicker(10 * time.Second) // 每 3 秒拉取 | fetch every 3s
			defer ticker.Stop()
			for range ticker.C {
				app.fetchAndCacheRelayEvents()
			}
		}()
	})
}

// fetchAndCacheRelayEvents 拉取主链新事件 → 缓存到内存 | Fetch new main chain events → cache in memory
func (app *TeeChain) fetchAndCacheRelayEvents() {
	if app.relay == nil || app.dkg == nil || app.dkg.Signer == nil {
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

		app.relayCache.mu.Unlock()

		util.LogWithCyan("RELAY-WORKER", "fetching events, chain", chainID, "from", start, "to", latest)
		var newEvents []cachedEvent

		// 批量拉取 | Batch fetch
		records, err := c.GetRelayEvents(start, latest)
		if err != nil {
			util.LogWithRed("RELAY-WORKER", "GetRelayEvents failed, chain", chainID, "err:", err)
			continue
		}

		for i, rec := range records {
			n := start + uint64(i)
			ev := relay.BuildSyncEvent(chainID, n, uint64(app.state.Height), rec.TargetContract, rec.EventType, rec.EventData)
			sysCall := relay.MakeRelaySysCall(ev)
			txBytes, err := app.SignAndBuildRelayTx(sysCall)
			if err != nil {
				util.LogWithRed("RELAY-WORKER", "SignAndBuildRelayTx failed, chain", chainID, "nonce", n, "err:", err)
				continue
			}
			newEvents = append(newEvents, cachedEvent{ev: ev, cachedAt: time.Now(), txBytes: txBytes})
		}

		// 追加到缓存 (保持 nonce 递增顺序) | append to cache (maintaining nonce order)
		app.relayCache.mu.Lock()
		app.relayCache.events[chainID] = append(app.relayCache.events[chainID], newEvents...)
		totalCached := len(app.relayCache.events[chainID])
		app.relayCache.mu.Unlock()
		if len(newEvents) > 0 {
			util.LogOk("RELAY-WORKER", "cached", len(newEvents), "new events, chain", chainID, "total cached", totalCached)
		}
	}

	// 至少5秒空闲才触发交易
	if time.Now().Unix()-app.lastCommitAt.Unix() < 5 {
		return
	}

	// 如果有需要同步到主链的交易, 直接提交
	if app.lastProposerWasMe.Load() {
		if app.targetSyncRelay() {
			return
		}
	}

	// 如果缓存中存在成熟事件且本节点是上一个出块者，提交空交易触发共识
	if app.lastProposerWasMe.Load() && app.hasMatureRelayEvents() {
		app.submitActiveTx()
	}
}

// hasMatureRelayEvents 检查缓存中是否存在已成熟但未打包的事件
func (app *TeeChain) hasMatureRelayEvents() bool {
	if app.relayCache == nil {
		return false
	}
	app.relayCache.mu.RLock()
	defer app.relayCache.mu.RUnlock()

	now := time.Now()
	for chainID, events := range app.relayCache.events {
		last, _ := app.relay.LastNonce(chainID)
		for _, ce := range events {
			if ce.ev.Nonce <= last {
				continue
			}
			if ce.ev.Nonce != last+1 {
				break
			}
			if now.Sub(ce.cachedAt) >= relayMinCacheAge {
				return true
			}
			break
		}
	}
	return false
}

// relayMinCacheAge 事件最少缓存时间 (确保其他验证者也拿到了) | Minimum cache age (ensure other validators also fetched)
const relayMinCacheAge = 3 * time.Second

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

// submitEmptyTx 提交一个交易（SysCall_Empty）到 mempool，用于在 CreateEmptyBlocks=false 时触发共识
func (app *TeeChain) submitActiveTx() {
	if app.NodePriv == nil {
		return
	}

	// 如果没有任何需求,以空块开启下一个区块
	call := &model.SysCall{Payload: &model.SysCall_Empty{Empty: time.Now().Unix()}}
	signer := app.NodePriv.ToSigner(teecrypto.SuiteEd25519)
	tx, err := signTx(call, signer)
	if err != nil {
		util.LogWithRed("RELAY-WORKER", "submit empty tx failed:", err)
		return
	}

	if err := SubmitTx(tx); err != nil {
		util.LogWithRed("RELAY-WORKER", "submit empty tx failed:", err)
		return
	}
	util.LogWithYellow("RELAY-WORKER", "submitted empty tx to trigger consensus")
}
