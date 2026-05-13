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
	SubnetAddress: "0x722E1FfccA6c6BD303397D0e5b0236bB421d8BFB",
	TokenAddress:  "0xDe2d178b30F580dAaB84C6cbe38f6dDc657D70d4",
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
