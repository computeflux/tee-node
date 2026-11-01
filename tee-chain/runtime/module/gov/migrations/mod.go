package migrations

import "github.com/computeflux/tee-provider/tee-chain/runtime/store"

func Migration(newVersion uint32, api store.ContractApi) error {
	if newVersion == 14 {
		return V14(api)
	}

	if newVersion == 15 || newVersion == 16 || newVersion == 17 || newVersion == 18 {
		return V15(api)
	}

	if newVersion == 20 {
		return V20(api)
	}

	if newVersion == 33 {
		return V33(api)
	}

	if newVersion == 34 {
		return V34(api)
	}

	// V35：重跑 V34（34 升级时迁移在 WASM 路径失败后的恢复版本，幂等）
	if newVersion == 35 {
		return V35(api)
	}

	// V36：重跑 V35（35 升级时 WASM 落库被截断、迁移失败后的恢复版本，幂等）
	if newVersion == 36 {
		return V36(api)
	}

	return nil
}
