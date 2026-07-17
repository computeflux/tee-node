package gateway

import (
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	cstore "github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── Async usage writer ───────────────────────────────────
// 批量写入 Pebble，避免每个请求都同步写磁盘

type usageRecord struct {
	user        chaintypes.UniAddr
	tokenID     uint64
	model       *cgateway.ModelInfoWithID
	routeID     uint64
	tokenIn     uint64
	tokenOut    uint64
	cachedToken uint64
}

var (
	usageCh   = make(chan usageRecord, 65536)
	usageWg   sync.WaitGroup
	usageOnce sync.Once
	usageMu   sync.Mutex // 保护 usage:* 读写，避免 flush 与 settle 竞态
)

func initUsageWriter() {
	usageOnce.Do(func() {
		usageWg.Go(func() {
			batch := make(map[string]*types.GatewayDailyUsage, 64)
			ticker := time.NewTicker(1000 * time.Millisecond)
			defer ticker.Stop()

			flush := func() {
				if len(batch) == 0 {
					return
				}
				usageMu.Lock()
				for k, u := range batch {
					existing, _ := types.DBGetJson[types.GatewayDailyUsage](types.SECRET_DB, "GATEWAY", "usage:"+k)
					if existing != nil {
						existing.Count += u.Count
						existing.TokenIn += u.TokenIn
						existing.TokenOut += u.TokenOut
						existing.CachedToken += u.CachedToken
						types.DBSetJson(types.SECRET_DB, "GATEWAY", "usage:"+k, existing)
					} else {
						types.DBSetJson(types.SECRET_DB, "GATEWAY", "usage:"+k, u)
					}
				}
				usageMu.Unlock()
				batch = make(map[string]*types.GatewayDailyUsage, 64)
			}

			for {
				select {
				case r := <-usageCh:
					key := uintToStr(r.tokenID) + "_" + uintToStr(r.routeID) + "_" + uintToStr(r.model.ID)
					if u, ok := batch[key]; ok {
						u.Count++
						u.TokenIn += r.tokenIn
						u.TokenOut += r.tokenOut
						u.CachedToken += r.cachedToken
					} else {
						batch[key] = &types.GatewayDailyUsage{
							User:        r.user,
							ModelID:     r.model.ID,
							ProviderID:  r.model.Data.ProviderID,
							RouteID:     r.routeID,
							Count:       1,
							TokenIn:     r.tokenIn,
							TokenOut:    r.tokenOut,
							CachedToken: r.cachedToken,
						}
					}
				case <-ticker.C:
					flush()
				}
			}
		})
	})
}

// recordUsageAsyncFn 可被测试替换为 mock。
var recordUsageAsyncFn = recordUsageAsync

func recordUsageAsync(user chaintypes.UniAddr, tokenID uint64, model *cgateway.ModelInfoWithID, routeID uint64, in uint64, out uint64, cachedTokens uint64) {
	initUsageWriter()
	// fmt.Println("[OPTIMIZER]", model.Data.Owner.String(), model.Data.ProviderID, model.Data.Name, tokens, cachedTokens)
	usageCh <- usageRecord{user, tokenID, model, routeID, in, out, cachedTokens}
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

var (
	settleOnce   sync.Once
	settleTicker *time.Ticker
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
	// 如果池中还有未消费的 settle_usage_with_route，说明上一批还在排队等待上链。
	// 跳过本次 tick，让 usage 继续累积，等待上一批被 ApplyToChain 消费。
	if bridge != nil && bridge.HasPendingContract("settle_usage_with_route") {
		GATEWAYLOG().Printf("[SETTLE] SKIP: pool has pending settle_usage_with_route")
		return nil
	}

	GATEWAYLOG().Printf("[SETTLE] SUBMIT: collecting usage records")

	// 1. 扫描本地 usage: 前缀的所有记录
	//    加锁防止 flush 在读和删之间写入，导致新写入的数据被误删。
	usageMu.Lock()
	records, keys, err := types.DBGetJsonList[types.GatewayDailyUsage](types.SECRET_DB, "GATEWAY", "usage:")
	if err != nil {
		usageMu.Unlock()
		return fmt.Errorf("list usage: %w", err)
	}
	if len(records) == 0 {
		usageMu.Unlock()
		return nil
	}

	// 2. 统一构造 SettlementWithRoute 大数组；RouteID 为 0 表示 None，否则 Some(routeID)。
	routeSettlements := make([]cgateway.SettlementWithRoute, 0, len(records))
	for i, r := range records {
		if r == nil {
			continue
		}

		nonce := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			usageMu.Unlock()
			return fmt.Errorf("generate nonce: %w", err)
		}

		var routeID cstore.Option[uint64]
		if r.RouteID == 0 {
			routeID = cstore.NewNone[uint64]()
		} else {
			routeID = cstore.NewSome(r.RouteID)
		}

		routeSettlements = append(routeSettlements, cgateway.SettlementWithRoute{
			User:        r.User,
			RouteID:     routeID,
			ProviderID:  r.ProviderID,
			ModelID:     r.ModelID,
			TokenIn:     r.TokenIn,
			TokenOut:    r.TokenOut,
			CachedToken: r.CachedToken,
			Count:       r.Count,
			FailCount:   0,
			Nonce:       nonce,
		})
		_ = keys[i]
	}
	if len(routeSettlements) == 0 {
		usageMu.Unlock()
		return nil
	}

	// 3. 一次性提交所有结算记录。
	//    保持加锁：submit 只是把调用写入本地交易池（非阻塞上链），耗时很短；
	//    若在此处释放锁，flush 可能在读和删之间写入同 key 的新数据，导致数据丢失。
	if err := submitSettleUsageWithRoute(routeSettlements); err != nil {
		usageMu.Unlock()
		GATEWAYLOG().Printf("[SETTLE] batch submit %d records failed: %v", len(routeSettlements), err)
		return fmt.Errorf("submit route settlements: %w", err)
	}
	GATEWAYLOG().Printf("[SETTLE] batch submitted %d records across %d providers", len(routeSettlements), len(records))

	// 4. 删除已提交的 usage 记录。
	deleted := 0
	for _, k := range keys {
		if err := types.SECRET_DB.DeleteByteKey(k); err != nil {
			usageMu.Unlock()
			GATEWAYLOG().Printf("[SETTLE] delete usage key=%x failed: %v", k, err)
			return fmt.Errorf("[SETTLE] delete usage key failed: %w", err)
		}
		deleted++
	}
	usageMu.Unlock()
	GATEWAYLOG().Printf("[SETTLE] deleted %d local usage records", deleted)

	return nil
}

// submitSettleUsageWithRoute 提交 SettleUsageWithRoute 合约调用。
func submitSettleUsageWithRoute(records []cgateway.SettlementWithRoute) error {
	bridge := getBridge()
	if bridge == nil {
		return fmt.Errorf("contract bridge not ready")
	}
	if len(records) == 0 {
		return nil
	}

	if bridge.HasPendingContract("settle_usage_with_route") {
		GATEWAYLOG().Printf("[SETTLE] tx pool already has pending settle_usage_with_route, skip this round")
		return fmt.Errorf("settle_usage_with_route already pending in tx pool, skip")
	}

	argBytes, err := cstore.ScaleEncode(records)
	if err != nil {
		return fmt.Errorf("encode route settlements: %w", err)
	}

	method := cstore.MethodToSelector("SettleUsageWithRoute")
	err = bridge.SubmitContractWithDedup("gateway", method[:], [][]byte{argBytes}, "settle_usage_with_route")
	if err == nil {
		GATEWAYLOG().Printf("[SETTLE] submitSettleUsageWithRoute OK, %d records pushed to tx pool", len(records))
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
