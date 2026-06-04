package config

import (
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/crypto"
)

var MainChainConfig = chaintypes.ChainConfigType{
	ChainID:       0,
	ChainType:     "evm",
	Cipher:        crypto.SuiteBLS12381,
	Urls:          []string{""},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Polkadot (Asset Hub)",
}
