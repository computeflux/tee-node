package gateway

import (
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/notify"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// 测试用钱包（EVM: T=2, 20 字节）
func testToken(hexAddr string) *types.GatewayTokenInfo {
	b := make([]byte, 20)
	for i := 0; i < 20; i++ {
		b[i] = hexAddr[i%len(hexAddr)]
	}
	return &types.GatewayTokenInfo{Owner: "u1", Addr: store.UniAddr{T: 2, V: b}}
}

// stubPush 替换异步推送，收集 (eventType, wallet, model)。
func stubPush(t *testing.T) *[]string {
	t.Helper()
	var got []string
	prev := pushNotifyFn
	pushNotifyFn = func(wallet, eventType string, vars map[string]string) {
		got = append(got, eventType+":"+vars["model"])
	}
	t.Cleanup(func() {
		pushNotifyFn = prev
		resetModelAvailStates()
	})
	return &got
}

func resetModelAvailStates() {
	modelAvailMu.Lock()
	modelAvailStates = make(map[string]*modelAvailState)
	modelAvailMu.Unlock()
}

// TestModelNotify_OfflineThenRestored 不可用 → 通知；冷却期内不重复；恢复 → 通知一次。
func TestModelNotify_OfflineThenRestored(t *testing.T) {
	got := stubPush(t)
	token := testToken("0123456789abcdef")

	// 第一次不可用（无可用端点）→ 应推送 model_offline
	notifyModelUnavailable(token, "openai/gpt-4o", scheduler.ErrNoHealthyEndpoint)
	if len(*got) != 1 || (*got)[0] != notify.EventTypeModelOffline+":gpt-4o" {
		t.Fatalf("expected 1 model_offline for gpt-4o, got %v", *got)
	}

	// 冷却期内再次不可用 → 不重复推送
	notifyModelUnavailable(token, "gpt-4o", scheduler.ErrAllEndpointsDead)
	if len(*got) != 1 {
		t.Fatalf("expected no duplicate offline notify within cooldown, got %v", *got)
	}

	// 请求成功 → 推送 model_restored（用带 provider 前缀的名字，key 应归一化到 gpt-4o）
	notifyModelAvailable(token, "gpt-4o")
	if len(*got) != 2 || (*got)[1] != notify.EventTypeModelRestored+":gpt-4o" {
		t.Fatalf("expected model_restored for gpt-4o, got %v", *got)
	}

	// 再次成功（已恢复）→ 不再推送
	notifyModelAvailable(token, "gpt-4o")
	if len(*got) != 2 {
		t.Fatalf("expected no duplicate restored notify, got %v", *got)
	}

	// 恢复后再次不可用 → 应重新推送（offline 状态已清除）
	notifyModelUnavailable(token, "gpt-4o", scheduler.ErrNoHealthyEndpoint)
	if len(*got) != 3 || (*got)[2] != notify.EventTypeModelOffline+":gpt-4o" {
		t.Fatalf("expected offline notify again after restore, got %v", *got)
	}
}

// TestModelNotify_IgnoresUserErrors 用户侧错误（4xx/model not found）不触发模型下架通知。
func TestModelNotify_IgnoresUserErrors(t *testing.T) {
	got := stubPush(t)
	token := testToken("0123456789abcdef")

	notifyModelUnavailable(token, "gpt-4o", scheduler.ErrModelNotFound)
	notifyModelUnavailable(token, "gpt-4o", &scheduler.SchedError{Status: 400, Message: "bad request"})
	notifyModelUnavailable(token, "gpt-4o", nil)

	if len(*got) != 0 {
		t.Fatalf("user-side errors must not notify, got %v", *got)
	}
}

// TestModelNotify_IgnoresNonEVMAddr 非 EVM 地址（拿不到钱包）不推送。
func TestModelNotify_IgnoresNonEVMAddr(t *testing.T) {
	got := stubPush(t)
	token := &types.GatewayTokenInfo{Owner: "u1", Addr: store.UniAddr{T: 1, V: []byte{1, 2, 3}}}

	notifyModelUnavailable(token, "gpt-4o", scheduler.ErrNoHealthyEndpoint)
	if len(*got) != 0 {
		t.Fatalf("non-EVM addr must not notify, got %v", *got)
	}
}

// TestModelNotify_CooldownExpiry 冷却过期后允许再次推送。
func TestModelNotify_CooldownExpiry(t *testing.T) {
	got := stubPush(t)
	token := testToken("0123456789abcdef")

	notifyModelUnavailable(token, "gpt-4o", scheduler.ErrNoHealthyEndpoint)
	if len(*got) != 1 {
		t.Fatalf("expected first offline notify, got %v", *got)
	}

	// 手动把 notifiedAt 拨回冷却期外
	modelAvailMu.Lock()
	for _, st := range modelAvailStates {
		st.notifiedAt = time.Now().Add(-2 * modelOfflineCooldown)
	}
	modelAvailMu.Unlock()

	notifyModelUnavailable(token, "gpt-4o", scheduler.ErrNoHealthyEndpoint)
	if len(*got) != 2 {
		t.Fatalf("expected offline notify after cooldown, got %v", *got)
	}
}

// TestBareModelName 模型名归一化。
func TestBareModelName(t *testing.T) {
	cases := map[string]string{
		"openai/gpt-4o": "gpt-4o",
		"gpt-4o":        "gpt-4o",
		"  gpt-4o  ":    "gpt-4o",
		"":              "",
	}
	for in, want := range cases {
		if got := bareModelName(in); got != want {
			t.Fatalf("bareModelName(%q) = %q, want %q", in, got, want)
		}
	}
}
