package revive

import (
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/config"
	"github.com/computeflux/tee-provider/pkg/chains/revive/subnet"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/util"
)

func TestMain(m *testing.M) {
	// 包内全局 MainChainConfig 为指针，未赋值时为 nil；集成测试需指向有效 RPC 与合约地址。
	MainChainConfig = &config.TestChainConfig
	os.Exit(m.Run())
}

func TestQueryWorkers(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	subnetIns, err := subnet.InitSubnetContract(client, MainChainConfig.SubnetAddress)
	if err != nil {
		util.LogWithPurple("InitSubnetContract", err)
		panic(err)
	}

	workers, _, err := subnetIns.QueryWorkers(util.NewNone[uint64](), 100, chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QueryWorkers", err)
		panic(err)
	}
	fmt.Println(workers)
}

func TestGetSecrets(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	subnetIns, err := subnet.InitSubnetContract(client, MainChainConfig.SubnetAddress)
	if err != nil {
		util.LogWithPurple("InitSubnetContract", err)
		panic(err)
	}

	secrets, _, err := subnetIns.QuerySecrets(chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QuerySecrets", err)
		panic(err)
	}
	fmt.Println(secrets)
}

func TestSetSubnetSolt(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	// pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	// if err != nil {
	// 	util.LogWithPurple("Sr25519PairFromSecret", err)
	// 	panic(err)
	// }
	pk, err := chain.Sr25519PairFromSecret("0x43c0defe545b31e6f64ee9c17b35c6444a8c3527f0c68160203717c51367512d", uint16(42))
	if err != nil {
		panic(fmt.Sprintf("init signer: %v", err))
	}

	subnetIns, err := subnet.InitSubnetContract(client, MainChainConfig.SubnetAddress)
	if err != nil {
		util.LogWithPurple("InitSubnetContract", err)
		panic(err)
	}

	err = subnetIns.ExecSetEpochSlot(72000, chain.ExecParams{
		Signer:    &pk,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})

	if err != nil {
		util.LogWithPurple("ExecSetEpochSlot", err)
		panic(err)
	}
}

func TestQueryEpochInfo(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	subnetIns, err := subnet.InitSubnetContract(client, MainChainConfig.SubnetAddress)
	if err != nil {
		util.LogWithPurple("InitSubnetContract", err)
		panic(err)
	}

	epochInfo, _, err := subnetIns.QueryEpochInfo(chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QueryEpochInfo", err)
		panic(err)
	}
	fmt.Println(epochInfo)
}

func TestCloudUpdate(t *testing.T) {
	// TODO
}

func TestSubnetUpdate(t *testing.T) {
	// TODO
}
