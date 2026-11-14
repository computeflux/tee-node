package config

import (
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
)

var TestChainConfig = chaintypes.ChainConfigType{
	ChainID:       420420417,
	ChainType:     "substrate",
	Cipher:        crypto.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://sys.turboflakes.io/asset-hub-paseo"},
	SubnetAddress: "0x0d6dd3a07e04879d59fd2ed3967946a29f960e7e",
	TokenAddress:  "0xff9b30b99f1e5a24664b0eda2f24cd465cb60ea6",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// Base Sepolia 测试链（积分充值/提现 relay + BaseBLS 网关）。
// 地址写死（2026-11-14 部署，见 evms hack/base/deployments.json），不接受环境变量覆盖：
//
//	Token（UUPS 代理）0xC7701fC65125C0a5cdBc760806F8fCf33D2aD2E0（impl 0x0eBFcbDA82979f22F1AfCDaF7cbe9A3E56A8C05e）
//	BaseBLS 0x2921A9385F94A78572A33C2590C94B93A5Ee2841（target=部署者 EOA，aggPubkey 已设）
//
// RPC 写死；⚠️ 公共 sepolia.base.org 有速率限制，生产化前需换私有节点。
var BaseTestChainConfig = chaintypes.ChainConfigType{
	ChainID:      84532,
	ChainType:    "evm",
	Cipher:       crypto.SuiteBLS12381,
	Urls:         []string{"https://sepolia.base.org"},
	TokenAddress: "0xC7701fC65125C0a5cdBc760806F8fCf33D2aD2E0",
	BLSAddress:   "0x2921A9385F94A78572A33C2590C94B93A5Ee2841",
	NetworkLabel: "Base Sepolia",
}
