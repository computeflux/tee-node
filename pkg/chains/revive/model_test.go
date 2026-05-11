package contracts

import (
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/util"
	"github.com/wetee-dao/tee-dsecret/pkg/chains/revive/cloud"
	"github.com/wetee-dao/tee-dsecret/pkg/chains/revive/subnet"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

func TestMain(m *testing.M) {
	// 包内全局 MainChainConfig 为指针，未赋值时为 nil；集成测试需指向有效 RPC 与合约地址。
	MainChainConfig = &model.TestChainConfig
	os.Exit(m.Run())
}

func TestCloud(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	cloudIns, err := cloud.InitCloudContract(client, MainChainConfig.CloudAddress)
	if err != nil {
		util.LogWithPurple("InitCloudContract", err)
		panic(err)
	}

	subnet, _, err := cloudIns.QuerySubnetAddress(chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QueryPodLen", err)
		panic(err)
	}
	fmt.Println(subnet.Hex())
}

func TestQueryCloudStage(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	cloudIns, err := cloud.InitCloudContract(client, MainChainConfig.CloudAddress)
	if err != nil {
		util.LogWithPurple("InitCloudContract", err)
		panic(err)
	}

	stage, _, err := cloudIns.QueryMintInterval(chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QueryPodLen", err)
		panic(err)
	}
	fmt.Println(*stage)
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
		util.LogWithPurple("InitCloudContract", err)
		panic(err)
	}

	workers, _, err := subnetIns.QueryWorkers(util.NewNone[uint64](), 100, chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QueryWorkers", err)
		panic(err)
	}
	fmt.Println(workers)
}

func TestQueryPods(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	cloudIns, err := cloud.InitCloudContract(client, MainChainConfig.CloudAddress)
	if err != nil {
		util.LogWithPurple("InitCloudContract", err)
		panic(err)
	}

	pods, _, err := cloudIns.QueryPods(util.NewNone[uint64](), 100, chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QueryPods", err)
		panic(err)
	}
	fmt.Println(pods)
}

func TestSetCloudStage(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	cloudIns, err := cloud.InitCloudContract(client, MainChainConfig.CloudAddress)
	if err != nil {
		util.LogWithPurple("InitCloudContract", err)
		panic(err)
	}

	err = cloudIns.ExecSetMintInterval(200, chain.ExecParams{
		Signer:    &pk,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
	if err != nil {
		util.LogWithPurple("ExecSetMintInterval", err)
	}
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

func TestQuerySubnetSideChainKey(t *testing.T) {
	client, err := chain.InitClient(MainChainConfig.Urls, true)
	if err != nil {
		panic(err)
	}

	pk, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		panic(err)
	}

	cloudIns, err := cloud.InitCloudContract(client, MainChainConfig.CloudAddress)
	if err != nil {
		util.LogWithPurple("InitSubnetContract", err)
		panic(err)
	}

	subnetAddress, _, err := cloudIns.QuerySubnetAddress(chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QuerySubnetAddress", err)
		panic(err)
	}
	fmt.Println(subnetAddress.Hex())

	subnetSideChainKey, _, err := cloudIns.QuerySubnetSideChainKey(chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QuerySubnetSideChainKey", err)
		panic(err)
	}
	fmt.Println(subnetSideChainKey.Hex())

	subnetIns, err := subnet.InitSubnetContract(client, MainChainConfig.SubnetAddress)
	if err != nil {
		util.LogWithPurple("InitSubnetContract", err)
		panic(err)
	}

	subnetSideChainKey, _, err = subnetIns.QuerySideChainKey(chain.DefaultParamWithOrigin(pk.AccountID()))
	if err != nil {
		util.LogWithPurple("QuerySideChainKey", err)
		panic(err)
	}
	fmt.Println(subnetSideChainKey.Hex())
}

func TestSetSubnetSolt(t *testing.T) {
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
		util.LogWithPurple("InitCloudContract", err)
		panic(err)
	}

	subnetIns.ExecSetEpochSlot(10, chain.ExecParams{
		Signer:    &pk,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
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

func TestWorkerUpdate(t *testing.T) {
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

	err = subnetIns.ExecSecretUpdate(0, []byte("v0"), subnet.Ip{
		Ipv4:   util.NewSome[uint32](3232263885),
		Ipv6:   util.NewNone[types.U128](),
		Domain: util.NewNone[[]byte](),
	}, 31000, chain.ExecParams{
		Signer:    &pk,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
	fmt.Println(err)

	subnetIns.ExecSecretUpdate(1, []byte("v1"), subnet.Ip{
		Ipv4:   util.NewSome[uint32](3232263885),
		Ipv6:   util.NewNone[types.U128](),
		Domain: util.NewNone[[]byte](),
	}, 41000, chain.ExecParams{
		Signer:    &pk,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})

	subnetIns.ExecSecretUpdate(2, []byte("v2"), subnet.Ip{
		Ipv4:   util.NewSome[uint32](3232263885),
		Ipv6:   util.NewNone[types.U128](),
		Domain: util.NewNone[[]byte](),
	}, 51000, chain.ExecParams{
		Signer:    &pk,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}
