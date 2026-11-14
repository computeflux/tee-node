package config

import (
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
)

// Base 主网（8453）。⚠️ 合约尚未部署：Token/BLS 地址为空则不启用（见 GetChainConfigs）；
// 主网部署后（evms `node hack/base/deploy.js --chain base ...`）把地址填进这里（同样写死，
// 不接受环境变量覆盖，防止注入非法地址）。
var BaseMainChainConfig = baseMainChainDefaults()

func baseMainChainDefaults() chaintypes.ChainConfigType {
	return chaintypes.ChainConfigType{
		ChainID:      8453,
		ChainType:    "evm",
		Cipher:       crypto.SuiteBLS12381,
		Urls:         []string{"https://mainnet.base.org"},
		TokenAddress: "",
		BLSAddress:   "",
		NetworkLabel: "Base",
	}
}

var MainChainConfig = chaintypes.ChainConfigType{
	ChainID:       0,
	ChainType:     "evm",
	Cipher:        crypto.SuiteBLS12381,
	Urls:          []string{""},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Polkadot (Asset Hub)",
}
