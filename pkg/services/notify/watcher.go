package notify

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/contractquery"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ModelEventWatcher 周期性对比链上模型活跃状态，检测「下架/恢复」事件并推送通知。
//
// 通知目标：模型 Owner（注册该模型的提供方地址）。模型下架时 Owner 最需要知道，
// 且 Owner 地址可直接从链上模型记录取得，无需额外的「关注」关系。
//
// 全链路无 LLM 依赖：文案由 Notifier 模板化生成（LLM 仅作为可选增强）。
type ModelEventWatcher struct {
	chain    *teechain.TeeChain
	notifier *Notifier
	interval time.Duration

	// active 记录上一轮快照：modelID -> 模型信息（仅活跃模型）。
	// 快照内保存 Owner/Provider，使「下架」事件（模型已离开活跃列表）仍能定位通知目标。
	active map[uint64]modelSnap
}

// modelSnap 单个模型的快照信息。
type modelSnap struct {
	name     string
	owner    store.UniAddr
	provider string
}

// NewModelEventWatcher 创建模型事件观察器。
func NewModelEventWatcher(chain *teechain.TeeChain, notifier *Notifier, interval time.Duration) *ModelEventWatcher {
	if interval <= 0 {
		interval = time.Minute
	}
	return &ModelEventWatcher{
		chain:    chain,
		notifier: notifier,
		interval: interval,
		active:   make(map[uint64]modelSnap),
	}
}

// Start 启动后台观察循环（非阻塞）。首轮仅建立基线快照，不产生通知，
// 避免节点重启时把全量模型误判为「新上线」。
func (w *ModelEventWatcher) Start(ctx context.Context) {
	go func() {
		// 首轮建立基线
		if snap, err := w.snapshot(ctx); err == nil {
			w.active = snap
			log.Printf("[NOTIFY] model watcher baseline: %d active models", len(snap))
		} else {
			log.Printf("[NOTIFY] model watcher baseline failed: %v", err)
		}

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick(ctx)
			}
		}
	}()
}

// tick 执行一轮对比与通知推送。
func (w *ModelEventWatcher) tick(ctx context.Context) {
	snap, err := w.snapshot(ctx)
	if err != nil {
		log.Printf("[NOTIFY] model watcher snapshot failed: %v", err)
		return
	}

	// 下架：上一轮活跃、本轮不在（用上一轮快照里的 Owner 定位通知目标）
	for id, prev := range w.active {
		if _, still := snap[id]; !still {
			w.emit(ctx, EventModelDeactivated, prev)
		}
	}
	// 恢复/新上线：本轮活跃、上一轮不在
	for id, cur := range snap {
		if _, was := w.active[id]; !was {
			w.emit(ctx, EventModelActivated, cur)
		}
	}

	w.active = snap
}

// snapshot 拉取当前全部活跃模型（modelID -> 快照）。
func (w *ModelEventWatcher) snapshot(ctx context.Context) (map[uint64]modelSnap, error) {
	models, err := contractquery.GatewayListActiveModelsWithinProvider(w.chain, 0, 0, 0, 0, 0, 100000)
	if err != nil {
		return nil, err
	}
	snap := make(map[uint64]modelSnap, len(models))
	for _, m := range models {
		snap[m.ID] = modelSnap{
			name:     m.Model.Name,
			owner:    m.Model.Owner,
			provider: m.Provider.Name,
		}
	}
	return snap, nil
}

// emit 推送一条模型状态变更通知给模型 Owner。
// Owner 非 20 字节 EVM 地址时无法映射到 actives 用户，跳过。
func (w *ModelEventWatcher) emit(ctx context.Context, evType string, s modelSnap) {
	owner := uniAddrToEVM(s.owner)
	if owner == "" {
		log.Printf("[NOTIFY] skip %s model %s: no EVM owner", evType, s.name)
		return
	}
	ev := Event{
		Type:         evType,
		UserID:       owner,
		ModelName:    s.name,
		ProviderName: s.provider,
	}
	if via, err := w.notifier.Push(ctx, ev); err != nil {
		log.Printf("[NOTIFY] push %s %s -> %s failed: %v", evType, s.name, owner, err)
	} else {
		log.Printf("[NOTIFY] pushed %s %s -> %s (viaLLM=%v)", evType, s.name, owner, via)
	}
}

// uniAddrToEVM 将链上 UniAddr 转为小写 EVM 地址；非 20 字节返回空串。
func uniAddrToEVM(a store.UniAddr) string {
	if len(a.V) != 20 {
		return ""
	}
	return strings.ToLower("0x" + hexEncode(a.V))
}

func hexEncode(b []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = h[c>>4]
		out[i*2+1] = h[c&0x0f]
	}
	return string(out)
}
