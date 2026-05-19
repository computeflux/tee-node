package teechain

import (
	"bytes"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/tee-chain/relay"
)

// cachedEvent 缓存的主链事件
type cachedEvent struct {
	ev       relay.SyncEvent
	cachedAt time.Time
	txBytes  []byte // 预签名的 tx bytes
}

// relayCache 每个节点的 relay 事件缓存
type relayCache struct {
	mu     sync.RWMutex
	events map[uint32][]cachedEvent // chainID → sorted by nonce
}

func newRelayCache() *relayCache {
	return &relayCache{events: make(map[uint32][]cachedEvent)}
}

// ── 后台 Worker (所有节点运行) ────────────────────────────

// startRelayWorker 启动后台拉取 goroutine
func (app *TeeChain) startRelayWorker() {
	app.relayCache = newRelayCache()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			app.fetchAndCacheRelayEvents()
		}
	}()
}

// fetchAndCacheRelayEvents 拉取主链新事件 → 缓存到内存
func (app *TeeChain) fetchAndCacheRelayEvents() {
	if app.chains == nil || app.relay == nil || app.dkg == nil {
		return
	}

	for chainID, chainApi := range app.chains {
		if chainApi == nil {
			continue
		}
		c := *chainApi

		latest, err := c.GetLatestNonce()
		if err != nil || latest == 0 {
			continue
		}

		last, _ := app.relay.LastNonce(chainID)

		app.relayCache.mu.Lock()
		cached := app.relayCache.events[chainID]
		var cachedMax uint64
		if len(cached) > 0 {
			cachedMax = cached[len(cached)-1].ev.Nonce
		}
		if last > cachedMax {
			cachedMax = last
		}

		start := cachedMax + 1
		if start > latest {
			app.relayCache.mu.Unlock()
			continue
		}

		// 只拉取未缓存的新事件
		var newEvents []cachedEvent
		for n := start; n <= latest; n++ {
			target, typ, data, err := c.GetRelayEvent(n)
			if err != nil || target == nil {
				continue
			}

			ev := relay.BuildSyncEvent(chainID, n, 0, target, typ, data)
			sysCall := relay.MakeRelaySysCall(ev)
			txBytes, err := app.signAndBuildRelayTx(sysCall)
			if err != nil {
				continue
			}

			newEvents = append(newEvents, cachedEvent{
				ev:       ev,
				cachedAt: time.Now(),
				txBytes:  txBytes,
			})
		}

		app.relayCache.events[chainID] = append(cached, newEvents...)
		app.relayCache.mu.Unlock()
	}
}

// ── PrepareProposal: 取成熟事件打包 ──────────────────────

const relayMinCacheAge = 2 * time.Second

// prepareRelayTxs 从缓存中取 ≥2s 的成熟事件，创建 relay tx
func (app *TeeChain) prepareRelayTxs() [][]byte {
	if app.relayCache == nil {
		return nil
	}

	app.relayCache.mu.RLock()
	defer app.relayCache.mu.RUnlock()

	var txs [][]byte
	now := time.Now()

	for chainID, events := range app.relayCache.events {
		last, _ := app.relay.LastNonce(chainID)
		for _, ce := range events {
			if ce.ev.Nonce <= last {
				continue // 已处理
			}
			if now.Sub(ce.cachedAt) < relayMinCacheAge {
				break // 后续事件更新, 更不可能成熟
			}
			txs = append(txs, ce.txBytes)
		}
	}
	return txs
}

// signAndBuildRelayTx 用 DKG signer 签名 relay SysCall, 返回 tx bytes
func (app *TeeChain) signAndBuildRelayTx(sysCall *model.SysCall) ([]byte, error) {
	sigBytes, err := sysCall.BytesForSig()
	if err != nil {
		return nil, errors.Wrap(err, "BytesForSig")
	}

	signer := app.dkg.Signer.ToSigner(model.SuiteEd25519)
	signature, err := signer.Sign(sigBytes)
	if err != nil {
		return nil, errors.Wrap(err, "Sign")
	}

	buf := new(bytes.Buffer)
	protoio.WriteMessage(sysCall, buf)
	tx := &model.Tx{
		Caller:     signer.Public(),
		Call:       buf.Bytes(),
		CallerType: 0,
		Signature:  signature,
	}

	return GetTxBytes(tx), nil
}
