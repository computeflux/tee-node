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

	return nil
}
