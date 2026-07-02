package config

import (
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
)

var TestChainConfig = chaintypes.ChainConfigType{
	ChainID:   420420417,
	ChainType: "substrate",
	Cipher:    crypto.SuiteEd25519,
	// Urls:      []string{"wss://api2.zondax.ch/pas/assethub/node/rpc", "wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.dotters.network", "wss://pas-rpc.stakeworld.io/assethub", "wss://sys.turboflakes.io/asset-hub-paseo"},
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://sys.turboflakes.io/asset-hub-paseo"},
	SubnetAddress: "0x0d6dd3a07e04879d59fd2ed3967946a29f960e7e",
	TokenAddress:  "0xff9b30b99f1e5a24664b0eda2f24cd465cb60ea6",
	NetworkLabel:  "Paseo (Asset Hub)",
}
