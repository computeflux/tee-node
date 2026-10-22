package gateway

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/notify"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/computeflux/tee-provider/pkg/types"
)

// ── 模型可用性通知（model_offline / model_restored）─────────────────────
//
// 「模型下架降级方案」的通知触发点：熔断/端点健康是**端点级**的（且无用户维度），
// 因此这里以「用户实际遭遇的调度结果」为准：
//
//   - 用户请求因「无可用端点 / 全部凭证失效」失败 → 给该用户推 model_offline
//   - 之后同一 用户+模型 的请求成功 → 推 model_restored（仅在曾标记 offline 后发一次）
//
// 约束：
//   - 同 用户+模型 的 offline 通知有冷却（默认 1h），避免连续失败刷屏；
//   - 推送异步 fire-and-forget，失败只记日志，绝不影响 relay 主链路；
//   - 未配置 NOTIFY_ACTIVES_URL / NOTIFY_PUSH_SECRET 时 notify 内部 no-op；
//   - 状态表有 TTL 与条数上限，防止内存无界增长。

const (
	// modelOfflineCooldown 同一 用户+模型 的 offline 通知冷却时间。
	modelOfflineCooldown = time.Hour
	// modelAvailStateTTL 状态条目保留时长（超过且已恢复的条目会被清理）。
	modelAvailStateTTL = 24 * time.Hour
	// modelAvailStateMax 状态条目上限（超过则清理最旧的）。
	modelAvailStateMax = 20000
)

type modelAvailState struct {
	offline     bool
	notifiedAt  time.Time // 上次 offline 通知时间（冷却用）
	restoredAt  time.Time
	reason      string
	lastTouched time.Time
}

var (
	modelAvailMu     sync.Mutex
	modelAvailStates = make(map[string]*modelAvailState)

	// pushNotifyFn 可测试替换：异步推送实现。
	pushNotifyFn = pushNotify
)

func modelAvailKey(wallet, model string) string { return wallet + "|" + model }

// bareModelName 去掉 "provider/" 前缀，保证失败与成功两侧的 key 一致。
func bareModelName(modelKey string) string {
	modelKey = strings.TrimSpace(modelKey)
	if modelKey == "" {
		return ""
	}
	if _, bare := parseProviderModel(modelKey); bare != "" {
		return bare
	}
	return modelKey
}

// isAvailabilityError 判断调度失败是否属于「模型不可用」（而非用户请求问题）。
// 4xx / model not found 等属于用户侧问题，不应触发模型下架通知。
func isAvailabilityError(err error) bool {
	if err == nil {
		return false
	}
	// 注意：SchedError.Is 只按 Class 匹配，4xx 也会命中 ErrNoHealthyEndpoint 这类哨兵，
	// 因此必须像 mapScheduleError 一样「带具体 status 的优先」。
	var se *scheduler.SchedError
	if errors.As(err, &se) {
		if se.Status != 0 {
			return se.Status >= 500 // 4xx = 用户请求问题，不算模型不可用
		}
		return errors.Is(err, scheduler.ErrNoHealthyEndpoint) ||
			errors.Is(err, scheduler.ErrAllEndpointsDead)
	}
	return false
}

// evmWallet 取 EVM 钱包地址（0x 形式）；非 EVM 地址返回空串（不推送）。
func evmWallet(token *types.GatewayTokenInfo) string {
	if token == nil {
		return ""
	}
	w := notify.WalletFromAddr(token.Addr)
	if strings.HasPrefix(w, "0x") {
		return w
	}
	return ""
}

// notifyModelUnavailable 调度失败时调用：标记模型不可用，并按冷却推送 model_offline。
func notifyModelUnavailable(token *types.GatewayTokenInfo, modelKey string, err error) {
	if token == nil || !isAvailabilityError(err) {
		return
	}
	wallet := evmWallet(token)
	model := bareModelName(modelKey)
	if wallet == "" || model == "" {
		return
	}

	now := time.Now()
	reason := err.Error()

	modelAvailMu.Lock()
	key := modelAvailKey(wallet, model)
	st := modelAvailStates[key]
	if st == nil {
		pruneModelAvailLocked(now)
		st = &modelAvailState{}
		modelAvailStates[key] = st
	}
	shouldNotify := !st.offline || now.Sub(st.notifiedAt) >= modelOfflineCooldown
	st.offline = true
	st.reason = reason
	st.lastTouched = now
	if shouldNotify {
		st.notifiedAt = now
	}
	modelAvailMu.Unlock()

	if !shouldNotify {
		return
	}
	GATEWAYLOG().Printf("[NOTIFY] model_offline wallet=%s model=%s reason=%s", wallet, model, reason)
	pushNotifyFn(wallet, notify.EventTypeModelOffline, map[string]string{"model": model, "reason": reason})
}

// notifyModelAvailable 请求成功时调用：若该 用户+模型 曾标记不可用，则推送 model_restored。
func notifyModelAvailable(token *types.GatewayTokenInfo, modelName string) {
	if token == nil {
		return
	}
	wallet := evmWallet(token)
	model := bareModelName(modelName)
	if wallet == "" || model == "" {
		return
	}

	now := time.Now()
	modelAvailMu.Lock()
	key := modelAvailKey(wallet, model)
	st := modelAvailStates[key]
	if st == nil || !st.offline {
		modelAvailMu.Unlock()
		return
	}
	st.offline = false
	st.restoredAt = now
	st.lastTouched = now
	modelAvailMu.Unlock()

	GATEWAYLOG().Printf("[NOTIFY] model_restored wallet=%s model=%s", wallet, model)
	pushNotifyFn(wallet, notify.EventTypeModelRestored, map[string]string{"model": model})
}

// pruneModelAvailLocked 清理过期条目（调用方需持有 modelAvailMu）。
func pruneModelAvailLocked(now time.Time) {
	if len(modelAvailStates) < modelAvailStateMax {
		return
	}
	for k, v := range modelAvailStates {
		if !v.offline && now.Sub(v.lastTouched) > modelAvailStateTTL {
			delete(modelAvailStates, k)
		}
	}
}

// pushNotify 异步推送通知（不阻塞 relay 主链路）。
func pushNotify(wallet, eventType string, vars map[string]string) {
	go func() {
		n := notify.NewFromEnv()
		if !n.Enabled() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := n.Push(ctx, wallet, eventType, vars); err != nil {
			GATEWAYLOG().Printf("[NOTIFY] push %s wallet=%s failed: %v", eventType, wallet, err)
		}
	}()
}
