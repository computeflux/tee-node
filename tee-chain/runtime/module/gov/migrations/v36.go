package migrations

import (
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// V36 重跑 V35/V34 的 BlockHeight 迁移（充值历史 + 用户结算历史）。
//
// 背景：35 升级时执行 RuntimeUpgrade 的节点二进制仍是旧版本，交易被路由到
// WASM 兼容路径；WASM guest 的共享缓冲区只有 256KB，400KB+ 的
// runtime.wasm.gz 被截断落库，链上 v35 WASM 残缺（decompress unexpected EOF），
// 迁移也随之失败。而链上版本已推进到 35、app.state.RuntimeVersion 同步为 35，
// 不会再触发 V35。因此新增 V36 作为恢复版本，重跑幂等的点查迁移。
//
// 35→36 的 RuntimeUpgrade 在持有 NativeVersion=36 的节点上会走原生路径
// （见 teechain.nativeVersionForCall），v36 WASM 可完整落库，迁移亦在原生
// 路径执行；V36→V35 均为点查实现，即使被路由到 WASM 也不受缓冲区上限影响。
func V36(api store.ContractApi) error {
	fmt.Printf("[V36] rerun BlockHeight migration (V35 logic)\n")
	return V35(api)
}
