package model

type ChainConfigType struct {
	ChainType     string
	Urls          []string
	SubnetAddress string
	CloudAddress  string
	NetworkLabel  string
}

var LocalChainConfig = ChainConfigType{
	ChainType:     "dev",
	Urls:          []string{"wss://xiaobai.asyou.me:30001/ws"},
	SubnetAddress: "0x496806883725e8544340dd35fe743b3b8af67b19",
	CloudAddress:  "0x9faed02b7624207dc0fedfded4842be33cad4eb3",
	NetworkLabel:  "Local",
}

var TestChainConfig = ChainConfigType{
	ChainType:     "polkadot",
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com"},
	SubnetAddress: "0x442f45995ed1bc0793e45f1affc464bf8bdfda45",
	CloudAddress:  "0x6a581b4db56bb9865a494cbc3c164b301c0b7809",
	NetworkLabel:  "Paseo (Asset Hub)",
}

var MainChainConfig = ChainConfigType{
	ChainType:     "polkadot",
	Urls:          []string{"wss://polkadot-asset-hub-rpc.polkadot.io"},
	SubnetAddress: "",
	CloudAddress:  "",
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
