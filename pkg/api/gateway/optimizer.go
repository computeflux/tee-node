package gateway

import (
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/computeflux/tee-provider/pkg/model"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/gateway"
)

// ── Async usage writer ───────────────────────────────────
// 批量写入 Pebble，避免每个请求都同步写磁盘

type usageRecord struct {
	user        model.UniAddr
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
			batch := make(map[string]*model.GatewayDailyUsage, 64)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			flush := func() {
				if len(batch) > 0 {
					for k, u := range batch {
						model.SetJson("GATEWAY", "usage:"+k, u)
					}
					batch = make(map[string]*model.GatewayDailyUsage, 64)
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
						batch[key] = &model.GatewayDailyUsage{
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

// FlushUsage 强制将内存中的用量统计同步落盘。
func FlushUsage() {
	initUsageWriter()
	select {
	case usageFlushCh <- struct{}{}:
	default:
	}
	// 等待 flush 完成（最多等 200ms）
	time.Sleep(200 * time.Millisecond)
}

// StopUsageWriter 优雅停止用量统计 worker，先 flush 再退出。
func StopUsageWriter() {
	FlushUsage()
	usageWg.Wait()
}

func recordUsageAsync(user model.UniAddr, tokenID uint64, model *cgateway.ModelInfoWithID, in uint64, out uint64, cachedTokens uint64) {
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

// ── Cache background refresh ─────────────────────────────

var cacheRefreshOnce sync.Once

func initCacheRefresh() {
	cacheRefreshOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				RefreshTokenCache()
			}
		}()
	})
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
	New: func() interface{} {
		return make([]byte, 0, 64*1024)
	},
}

func GetBuffer() []byte {
	return bufferPool.Get().([]byte)[:0]
}

func PutBuffer(buf []byte) {
	if cap(buf) <= 256*1024 {
		bufferPool.Put(buf[:0])
	}
}

// ── Usage settlement to sidechain ────────────────────────

var (
	settleOnce   sync.Once
	settleTicker *time.Ticker
	settleDone   = make(chan struct{})
)

// SettleInterval 结算周期（5分钟），可通过测试修改
var SettleInterval = 5 * time.Minute

func initUsageSettler() {
	if err := settlePendingUsage(); err != nil {
		GATEWAYLOG().Printf("[SETTLE] failed: %v", err)
	}
	settleOnce.Do(func() {
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

// StopUsageSettler 停止结算循环（优雅停机时调用）。
func StopUsageSettler() {
	close(settleDone)
}

// settlePendingUsage 扫描本地 PebbleDB 用量记录，统一打包为一个大数组一次性上链。
func settlePendingUsage() error {
	// 1. 扫描本地 usage: 前缀的所有记录
	records, keys, err := model.GetJsonList[model.GatewayDailyUsage]("GATEWAY", "usage:")
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

	// 4. 成功后再删除本地记录
	for _, k := range keys {
		if err := model.DeleteByteKey(k); err != nil {
			GATEWAYLOG().Printf("[SETTLE] delete key error: %v", err)
		}
	}

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

	// SCALE 编码 Settlement 数组
	argBytes, err := codec.Encode(records)
	if err != nil {
		return fmt.Errorf("encode settlements: %w", err)
	}

	method := model.MethodToSelector("SettleUsage")

	err = bridge.SubmitContract("gateway", method[:], [][]byte{argBytes})
	return err
}

// ── Startup init ─────────────────────────────────────────

func InitOptimizations() {
	initUsageWriter()
	initCacheRefresh()
	initRateLimiterCleanup()
	initUsageSettler()
	GATEWAYLOG().Printf("[OPT] async usage writer + cache refresh + rate limiter cleanup + usage settler started")
}
