package model

type ChainConfigType struct {
	ChainType     string
	Cipher        SuiteType
	Urls          []string
	SubnetAddress string
	TokenAddress  string
	CloudAddress  string // Substrate Cloud contract address
	NetworkLabel  string
}

var LocalChainConfig = ChainConfigType{
	ChainType:     "dev",
	Urls:          []string{""},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Local",
}

var TestChainConfig = ChainConfigType{
	ChainType:     "evm",
	Cipher:        SuiteSecp256k1,
	Urls:          []string{"https://eth-rpc-testnet.polkadot.io"},
	SubnetAddress: "0x6Ee5a17efe1019C13cc24fb45aB301ED809F2EBd",
	TokenAddress:  "0x9aa727B291c63A78E088E44674317A72F12902F5",
	NetworkLabel:  "Paseo (Asset Hub)",
}

var MainChainConfig = ChainConfigType{
	ChainType:     "evm",
	Cipher:        SuiteSecp256k1,
	Urls:          []string{""},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Polkadot (Asset Hub)",
}

func GetChainConfig(chainType string) *ChainConfigType {
	switch chainType {
	case "local":
		return &LocalChainConfig
	case "test":
		return &TestChainConfig
	case "main":
		return &MainChainConfig
	}
	return nil
}
