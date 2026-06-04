package config

import (
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/crypto"
)

var TestChainConfig = chaintypes.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Cipher:        crypto.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo.ibp.network", "wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.dotters.network"},
	SubnetAddress: "0x8201d6ce5abdbf8b4bfabdbf4e19df7459519265",
	TokenAddress:  "0xebc1e9a1860741849c75496f8a49c2f166df7978",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []chaintypes.ChainConfigType{}
