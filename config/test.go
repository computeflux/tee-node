package config

import (
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/crypto"
)

var TestChainConfig = chaintypes.ChainConfigType{
	ChainID:       420420417,
	ChainType:     "substrate",
	Cipher:        crypto.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://sys.turboflakes.io/asset-hub-paseo"},
	SubnetAddress: "0xf8e6c4dec077b148b55e31533efea751c91f25e4",
	TokenAddress:  "0xc0849f08bd8f1028a499d6e689f3431c0730a582",
	NetworkLabel:  "Paseo (Asset Hub)",
}
