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
	SubnetAddress: "0x9EFCDA0410f51F5E67817e091C157B8f45b9483e",
	TokenAddress:  "0x04b51cC7482E8BC010Aa59EfffaC247D3AA16FE4",
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
