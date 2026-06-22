package gateway

import (
	cstore "github.com/computeflux/tee-provider/tee-chain/contracts/store"

	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/store"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
)

// ── Async usage writer ───────────────────────────────────
// 批量写入 Pebble，避免每个请求都同步写磁盘

type usageRecord struct {
	user        chaintypes.UniAddr
	tokenID     uint64
	model       *cgateway.ModelInfoWithID
	tokenIn     uint64
	tokenOut    uint64
	cachedToken uint64
}

var (
	usageCh      = make(chan usageRecord, 65536)
	usageFlushCh = make(chan struct{}, 1)
	usageWg      sync.WaitGroup
	usageOnce    sync.Once
)

func initUsageWriter() {
	usageOnce.Do(func() {
		usageWg.Add(1)
		go func() {
			defer usageWg.Done()
			batch := make(map[string]*store.GatewayDailyUsage, 64)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			flush := func() {
				if len(batch) > 0 {
					for k, u := range batch {
						store.DBSetJson(store.SECRET_DB, "GATEWAY", "usage:"+k, u)
					}
					batch = make(map[string]*store.GatewayDailyUsage, 64)
				}
			}

			for {
				select {
				case r := <-usageCh:
					key := uintToStr(r.tokenID) + "_" + uintToStr(r.model.ID)
					if u, ok := batch[key]; ok {
						u.Count++
						u.TokenIn += r.tokenIn
						u.TokenOut += r.tokenOut
						u.CachedToken += r.cachedToken
					} else {
						batch[key] = &store.GatewayDailyUsage{
							User:        r.user,
							ModelID:     r.model.ID,
							ProviderID:  r.model.Data.ProviderID,
							Count:       1,
							TokenIn:     r.tokenIn,
							TokenOut:    r.tokenOut,
							CachedToken: r.cachedToken,
						}
					}
				case <-ticker.C:
					flush()
				case <-usageFlushCh:
					flush()
				}
			}
		}()
	})
}

// recordUsageAsyncFn 可被测试替换为 mock。
var recordUsageAsyncFn = recordUsageAsync

func recordUsageAsync(user chaintypes.UniAddr, tokenID uint64, model *cgateway.ModelInfoWithID, in uint64, out uint64, cachedTokens uint64) {
	initUsageWriter()
	// fmt.Println("[OPTIMIZER]", model.Data.Owner.String(), model.Data.ProviderID, model.Data.Name, tokens, cachedTokens)
	usageCh <- usageRecord{user, tokenID, model, in, out, cachedTokens}
}

func uintToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 20)
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ── Rate limiter cleanup ─────────────────────────────────

var cleanupOnce sync.Once

func initRateLimiterCleanup() {
	cleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				limiter.mu.Lock()
				now := time.Now()
				for k, bucket := range limiter.buckets {
					valid := bucket[:0]
					for _, t := range bucket {
						if now.Sub(t) < 5*time.Minute {
							valid = append(valid, t)
						}
					}
					if len(valid) == 0 {
						delete(limiter.buckets, k)
					} else {
						limiter.buckets[k] = valid
					}
				}
				limiter.mu.Unlock()
			}
		}()
	})
}

// ── Body buffer pool ─────────────────────────────────────

var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 0, 64*1024)
	},
}

// ── Usage settlement to sidechain ────────────────────────
var (
	settleOnce   sync.Once
	settleTicker *time.Ticker
	settleDone   = make(chan struct{})
	settleMu     sync.Mutex
)

// SettleInterval 结算周期（5分钟），可通过测试修改
var SettleInterval = 5 * time.Minute

func initUsageSettler() {
	settleOnce.Do(func() {
		if err := settlePendingUsage(); err != nil {
			GATEWAYLOG().Printf("[SETTLE] init failed: %v", err)
		}
		go func() {
			settleTicker = time.NewTicker(SettleInterval)
			defer settleTicker.Stop()
			for {
				select {
				case <-settleTicker.C:
					if err := settlePendingUsage(); err != nil {
						GATEWAYLOG().Printf("[SETTLE] failed: %v", err)
					}
				case <-settleDone:
					return
				}
			}
		}()
	})
}

// settlePendingUsage 扫描本地 PebbleDB 用量记录，统一打包为一个大数组一次性上链。
func settlePendingUsage() error {
	settleMu.Lock()
	defer settleMu.Unlock()

	GATEWAYLOG().Printf("[SETTLE] tick enter at %s", time.Now().Format("15:04:05"))

	bridge := getBridge()

	// 如果池中还有未消费的 settle_usage，说明上一批还在排队等待上链。
	// 跳过本次 tick，让 usage 继续累积，等待上一批被 ApplyToChain 消费。
	// 这样确保同一时间只有一个 settle_usage 批次在流转。
	if bridge != nil && bridge.HasPendingContract("settle_usage") {
		GATEWAYLOG().Printf("[SETTLE] SKIP: pool has pending settle_usage")
		return nil
	}

	GATEWAYLOG().Printf("[SETTLE] SUBMIT: pool empty, collecting usage records")

	// 1. 扫描本地 usage: 前缀的所有记录
	records, keys, err := store.DBGetJsonList[store.GatewayDailyUsage](store.SECRET_DB, "GATEWAY", "usage:")
	if err != nil {
		return fmt.Errorf("list usage: %w", err)
	}
	if len(records) == 0 {
		return nil
	}

	// 2. 统一构造 Settlement 大数组，每条记录带唯一随机 Nonce（防重放）
	settlements := make([]cgateway.Settlement, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}

		nonce := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return fmt.Errorf("generate nonce: %w", err)
		}

		settlements = append(settlements, cgateway.Settlement{
			User:        r.User,
			ModelID:     r.ModelID,
			ProviderID:  r.ProviderID,
			TokenIn:     r.TokenIn,
			TokenOut:    r.TokenOut,
			CachedToken: r.CachedToken,
			Count:       r.Count,
			Nonce:       nonce,
		})
	}
	if len(settlements) == 0 {
		return nil
	}

	// 3. 一次性提交所有结算记录
	if err := submitSettleUsage(settlements); err != nil {
		GATEWAYLOG().Printf("[SETTLE] batch submit %d records failed: %v", len(settlements), err)
		return fmt.Errorf("submit settlements: %w", err)
	}
	GATEWAYLOG().Printf("[SETTLE] batch submitted %d records across %d providers", len(settlements), len(records))

	// 4. 删除已提交的 usage 记录。
	//    此时 pool 中已有 settle_usage（刚提交），下次 tick 会因 HasPendingContract 跳过，
	//    等到 ApplyToChain 消费 pool 后，下次成功 tick 才会提交新一轮 usage。
	deleted := 0
	for _, k := range keys {
		if err := store.SECRET_DB.DeleteByteKey(k); err != nil {
			GATEWAYLOG().Printf("[SETTLE] delete usage key=%x failed: %v", k, err)
			return fmt.Errorf("[SETTLE] delete usage key failed: %w", err)
		}
		deleted++
	}
	GATEWAYLOG().Printf("[SETTLE] deleted %d local usage records", deleted)

	return nil
}

// submitSettleUsage 提交 SettleUsage 合约调用（通过 TxManager 统一提交，自动重试）。
func submitSettleUsage(records []cgateway.Settlement) error {
	bridge := getBridge()
	if bridge == nil {
		return fmt.Errorf("contract bridge not ready")
	}
	if len(records) == 0 {
		return nil
	}

	// 检查交易池中是否已有待处理的 settle_usage，
	// 若有则跳过本次提交，避免重复写入池中。
	if bridge.HasPendingContract("settle_usage") {
		GATEWAYLOG().Printf("[SETTLE] tx pool already has pending settle_usage, skip this round")
		return fmt.Errorf("settle_usage already pending in tx pool, skip")
	}

	// SCALE 编码 Settlement 数组
	argBytes, err := cstore.ScaleEncode(records)
	if err != nil {
		return fmt.Errorf("encode settlements: %w", err)
	}

	method := cstore.MethodToSelector("SettleUsage")

	// 使用固定 dedupKey，确保交易池中最多只有一个 settle_usage 调用。
	err = bridge.SubmitContractWithDedup("gateway", method[:], [][]byte{argBytes}, "settle_usage")
	if err == nil {
		GATEWAYLOG().Printf("[SETTLE] submitSettleUsage OK, %d records pushed to tx pool", len(records))
	}
	return err
}

// ── Startup init ─────────────────────────────────────────

func InitOptimizations() {
	initUsageWriter()
	initRateLimiterCleanup()
	initUsageSettler()
	GATEWAYLOG().Printf("[OPT] async usage writer + rate limiter cleanup + usage settler started")
}
