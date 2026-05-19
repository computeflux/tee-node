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

type cachedEvent struct {
	ev       relay.SyncEvent
	cachedAt time.Time
	txBytes  []byte
}

type relayCache struct {
	mu     sync.RWMutex
	events map[uint32][]cachedEvent
}

func newRelayCache() *relayCache {
	return &relayCache{events: make(map[uint32][]cachedEvent)}
}

func (app *TeeChain) startRelayWorker() {
	app.relayCache = newRelayCache()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			app.fetchAndCacheRelayEvents()
		}
	}()
}

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
			newEvents = append(newEvents, cachedEvent{ev: ev, cachedAt: time.Now(), txBytes: txBytes})
		}
		app.relayCache.events[chainID] = append(cached, newEvents...)
		app.relayCache.mu.Unlock()
	}
}

const relayMinCacheAge = 2 * time.Second

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
				continue
			}
			if ce.ev.Nonce != last+1 {
				break
			}
			if now.Sub(ce.cachedAt) < relayMinCacheAge {
				break
			}
			txs = append(txs, ce.txBytes)
			last = ce.ev.Nonce
		}
	}
	return txs
}

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
