package graph

import (
	teechain "github.com/computeflux/tee-provider/tee-chain"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/c2c"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
)

// c2cFxConfigFromContract 直接调用 c2c 合约查询 get_fx_config（query selector，mutates=false），
// 返回解码后的 FxConfig。c2c 查询方法与 contractDryRun（默认用 mutation selector）不同，
// 此处显式用 query selector 才能命中 ExecQuery。未配置时返回全零（web2 侧降级）。
func c2cFxConfigFromContract(tc *teechain.TeeChain) (*c2c.FxConfig, error) {
	cfg, err := teechain.WithRuntime(tc, func(rt *contracts.Runtime) (c2c.FxConfig, error) {
		var zero c2c.FxConfig
		sel := store.MethodToSelectorWithMutates("get_fx_config", false)
		call := &model.ContractCall{
			Name:   []byte("c2c"),
			Method: sel[:],
			Args:   nil,
		}
		data, err := rt.Query(chaintypes.UniAddr{}, call)
		if err != nil {
			return zero, err
		}
		cfg, err := store.DecodeScaleArgBytes[c2c.FxConfig](data)
		if err != nil {
			return zero, err
		}
		return cfg, nil
	})
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
