// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// AssetInfo is an auto generated low-level Go binding around an user-defined struct.
type AssetInfo struct {
	Name        string
	AssetType   uint8
	TotalSupply *big.Int
}

// EpochInfo is an auto generated low-level Go binding around an user-defined struct.
type EpochInfo struct {
	Epoch          uint32
	EpochSlot      uint32
	LastEpochBlock *big.Int
	Now            uint32
	SideChainPub   common.Address
}

// Ip is an auto generated low-level Go binding around an user-defined struct.
type Ip struct {
	A uint8
	B uint8
	C uint8
	D uint8
}

// K8sCluster is an auto generated low-level Go binding around an user-defined struct.
type K8sCluster struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
}

// RunPrice is an auto generated low-level Go binding around an user-defined struct.
type RunPrice struct {
	Cpu           *big.Int
	Mem           *big.Int
	StorageAmount *big.Int
	Network       *big.Int
}

// SecretNode is an auto generated low-level Go binding around an user-defined struct.
type SecretNode struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	ValidatorId   [32]byte
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
	ValidatorSecp []byte
}

// SubnetMetaData contains all meta data concerning the Subnet contract.
var SubnetMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"CloudContractNotSet\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"DepositNotEnough\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"EpochNotExpired\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InsufficientPoints\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidSideChainCaller\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"MortgageNotEnough\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"MustCallByMainContract\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NodeIsRunning\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NodeNotExist\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"RegionNotExist\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ResourceNotEnough\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"SlashAmountTooLarge\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"TokenContractNotSet\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"TransferFailed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"WorkerMortgageNotExist\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"WorkerNotExist\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"WorkerNotOwnedByCaller\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"WorkerStatusNotReady\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"previousAdmin\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newAdmin\",\"type\":\"address\"}],\"name\":\"AdminChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"beacon\",\"type\":\"address\"}],\"name\":\"BeaconUpgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"Upgraded\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"}],\"name\":\"asset\",\"outputs\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"assetType\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"totalSupply\",\"type\":\"uint256\"}],\"internalType\":\"structAssetInfo\",\"name\":\"\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"name\":\"assetInfos\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"assetType\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"totalSupply\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"name\":\"assetPrices\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"name\":\"bootNodes\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"bootNodesLen\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"cloudContract\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epoch\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"uint32\",\"name\":\"epoch\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"epochSlot\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"lastEpochBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint32\",\"name\":\"now\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"sideChainPub\",\"type\":\"address\"}],\"internalType\":\"structEpochInfo\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochSlot\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getBootNodes\",\"outputs\":[{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"validatorId\",\"type\":\"bytes32\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"validatorSecp\",\"type\":\"bytes\"}],\"internalType\":\"structSecretNode[]\",\"name\":\"\",\"type\":\"tuple[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getCloudContract\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"}],\"name\":\"getLevelMinMortgage\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"}],\"name\":\"getLevelPrice\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"cpu\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mem\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"storageAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"network\",\"type\":\"uint256\"}],\"internalType\":\"structRunPrice\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getMinMortgage\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getPendingSecrets\",\"outputs\":[{\"internalType\":\"uint64[]\",\"name\":\"\",\"type\":\"uint64[]\"},{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getRegions\",\"outputs\":[{\"internalType\":\"uint32[]\",\"name\":\"ids\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes[]\",\"name\":\"names\",\"type\":\"bytes[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"start\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"size\",\"type\":\"uint256\"}],\"name\":\"getSecrets\",\"outputs\":[{\"internalType\":\"uint64[]\",\"name\":\"ids\",\"type\":\"uint64[]\"},{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"validatorId\",\"type\":\"bytes32\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"validatorSecp\",\"type\":\"bytes\"}],\"internalType\":\"structSecretNode[]\",\"name\":\"nodes\",\"type\":\"tuple[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getValidators\",\"outputs\":[{\"internalType\":\"uint64[]\",\"name\":\"\",\"type\":\"uint64[]\"},{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"validatorId\",\"type\":\"bytes32\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"validatorSecp\",\"type\":\"bytes\"}],\"internalType\":\"structSecretNode[]\",\"name\":\"\",\"type\":\"tuple[]\"},{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"}],\"name\":\"getWorker\",\"outputs\":[{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"}],\"internalType\":\"structK8sCluster\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"start\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"size\",\"type\":\"uint64\"}],\"name\":\"getWorkers\",\"outputs\":[{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"}],\"internalType\":\"structK8sCluster[]\",\"name\":\"\",\"type\":\"tuple[]\"},{\"internalType\":\"uint64[]\",\"name\":\"\",\"type\":\"uint64[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"govContract\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_govContract\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_sideChainMultiKey\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastEpochBlock\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"name\":\"levelMinMortgages\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"name\":\"levelPrices\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"cpu\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mem\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"storageAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"network\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minMortgageAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"mintOfWorker\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"id\",\"type\":\"bytes32\"}],\"name\":\"mintWorker\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"},{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"}],\"internalType\":\"structK8sCluster\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nextAssetId\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nextEpochValidators\",\"outputs\":[{\"internalType\":\"uint64[]\",\"name\":\"\",\"type\":\"uint64[]\"},{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"validatorId\",\"type\":\"bytes32\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"validatorSecp\",\"type\":\"bytes\"}],\"internalType\":\"structSecretNode[]\",\"name\":\"\",\"type\":\"tuple[]\"},{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nextRegionId\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nextSecretId\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nextWorkerId\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"ownerOfWorker\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"pendingValidatorIds\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"pendingValidators\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"proxiableUUID\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"}],\"name\":\"region\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"regionWorkers\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"name\":\"regions\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"runningValidatorIds\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"runningValidators\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"}],\"name\":\"secretDelete\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"uint256\",\"name\":\"deposit\",\"type\":\"uint256\"}],\"name\":\"secretDeposit\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"secretMortgages\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"secretOfUser\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"validatorId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"bytes\",\"name\":\"validatorSecp\",\"type\":\"bytes\"}],\"name\":\"secretRegister\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"validatorSecp\",\"type\":\"bytes\"}],\"name\":\"secretSetSecpKey\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"}],\"name\":\"secretUpdate\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"secrets\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"validatorId\",\"type\":\"bytes32\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"validatorSecp\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"assetType\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"totalSupply\",\"type\":\"uint256\"}],\"internalType\":\"structAssetInfo\",\"name\":\"info\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"price\",\"type\":\"uint256\"}],\"name\":\"setAsset\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64[]\",\"name\":\"nodes\",\"type\":\"uint64[]\"}],\"name\":\"setBootNodes\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setCloudContract\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"_epochSlot\",\"type\":\"uint32\"}],\"name\":\"setEpochSlot\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"setLevelMinMortgage\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"cpu\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mem\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"storageAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"network\",\"type\":\"uint256\"}],\"internalType\":\"structRunPrice\",\"name\":\"price\",\"type\":\"tuple\"}],\"name\":\"setLevelPrice\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"setMinMortgage\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"setNextEpoch\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"}],\"name\":\"setRegion\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_token\",\"type\":\"address\"}],\"name\":\"setTokenContract\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"sideChainKey\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"sideChainMultiKey\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"workerId\",\"type\":\"uint64\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"}],\"name\":\"slashWorkerMortgage\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tokenContract\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"}],\"name\":\"upgradeTo\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"upgradeToAndCall\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"}],\"name\":\"userWorker\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"},{\"components\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"}],\"internalType\":\"structK8sCluster\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"}],\"name\":\"validatorDelete\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"}],\"name\":\"validatorJoin\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"ethAmount\",\"type\":\"uint256\"}],\"name\":\"withdrawToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"cpu\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"mem\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"cvmCpu\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"cvmMem\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"disk\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"gpu\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"deposit\",\"type\":\"uint256\"}],\"name\":\"workerMortgage\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"}],\"name\":\"workerRegister\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"}],\"name\":\"workerStart\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"workerStatus\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"}],\"name\":\"workerStop\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"workerId\",\"type\":\"uint64\"}],\"name\":\"workerTotalMortgage\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"workerId\",\"type\":\"uint64\"}],\"name\":\"workerTotalResources\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"workerId\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"mortgageId\",\"type\":\"uint64\"}],\"name\":\"workerUnmortgage\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"}],\"name\":\"workerUpdate\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"workers\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"id\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"name\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"level\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"regionId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stopBlock\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"terminalBlock\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint8\",\"name\":\"a\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"b\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"c\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"d\",\"type\":\"uint8\"}],\"internalType\":\"structIp\",\"name\":\"ip\",\"type\":\"tuple\"},{\"internalType\":\"uint32\",\"name\":\"port\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"p2pId\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
}

// SubnetABI is the input ABI used to generate the binding from.
// Deprecated: Use SubnetMetaData.ABI instead.
var SubnetABI = SubnetMetaData.ABI

// Subnet is an auto generated Go binding around an Ethereum contract.
type Subnet struct {
	SubnetCaller     // Read-only binding to the contract
	SubnetTransactor // Write-only binding to the contract
	SubnetFilterer   // Log filterer for contract events
}

// SubnetCaller is an auto generated read-only Go binding around an Ethereum contract.
type SubnetCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SubnetTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SubnetTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SubnetFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SubnetFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SubnetSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SubnetSession struct {
	Contract     *Subnet           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// SubnetCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SubnetCallerSession struct {
	Contract *SubnetCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// SubnetTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SubnetTransactorSession struct {
	Contract     *SubnetTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// SubnetRaw is an auto generated low-level Go binding around an Ethereum contract.
type SubnetRaw struct {
	Contract *Subnet // Generic contract binding to access the raw methods on
}

// SubnetCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SubnetCallerRaw struct {
	Contract *SubnetCaller // Generic read-only contract binding to access the raw methods on
}

// SubnetTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SubnetTransactorRaw struct {
	Contract *SubnetTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSubnet creates a new instance of Subnet, bound to a specific deployed contract.
func NewSubnet(address common.Address, backend bind.ContractBackend) (*Subnet, error) {
	contract, err := bindSubnet(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Subnet{SubnetCaller: SubnetCaller{contract: contract}, SubnetTransactor: SubnetTransactor{contract: contract}, SubnetFilterer: SubnetFilterer{contract: contract}}, nil
}

// NewSubnetCaller creates a new read-only instance of Subnet, bound to a specific deployed contract.
func NewSubnetCaller(address common.Address, caller bind.ContractCaller) (*SubnetCaller, error) {
	contract, err := bindSubnet(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SubnetCaller{contract: contract}, nil
}

// NewSubnetTransactor creates a new write-only instance of Subnet, bound to a specific deployed contract.
func NewSubnetTransactor(address common.Address, transactor bind.ContractTransactor) (*SubnetTransactor, error) {
	contract, err := bindSubnet(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SubnetTransactor{contract: contract}, nil
}

// NewSubnetFilterer creates a new log filterer instance of Subnet, bound to a specific deployed contract.
func NewSubnetFilterer(address common.Address, filterer bind.ContractFilterer) (*SubnetFilterer, error) {
	contract, err := bindSubnet(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SubnetFilterer{contract: contract}, nil
}

// bindSubnet binds a generic wrapper to an already deployed contract.
func bindSubnet(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := SubnetMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Subnet *SubnetRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Subnet.Contract.SubnetCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Subnet *SubnetRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Subnet.Contract.SubnetTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Subnet *SubnetRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Subnet.Contract.SubnetTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Subnet *SubnetCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Subnet.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Subnet *SubnetTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Subnet.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Subnet *SubnetTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Subnet.Contract.contract.Transact(opts, method, params...)
}

// Asset is a free data retrieval call binding the contract method 0x5d3da41a.
//
// Solidity: function asset(uint32 id) view returns((string,uint8,uint256), uint256)
func (_Subnet *SubnetCaller) Asset(opts *bind.CallOpts, id uint32) (AssetInfo, *big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "asset", id)

	if err != nil {
		return *new(AssetInfo), *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(AssetInfo)).(*AssetInfo)
	out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return out0, out1, err

}

// Asset is a free data retrieval call binding the contract method 0x5d3da41a.
//
// Solidity: function asset(uint32 id) view returns((string,uint8,uint256), uint256)
func (_Subnet *SubnetSession) Asset(id uint32) (AssetInfo, *big.Int, error) {
	return _Subnet.Contract.Asset(&_Subnet.CallOpts, id)
}

// Asset is a free data retrieval call binding the contract method 0x5d3da41a.
//
// Solidity: function asset(uint32 id) view returns((string,uint8,uint256), uint256)
func (_Subnet *SubnetCallerSession) Asset(id uint32) (AssetInfo, *big.Int, error) {
	return _Subnet.Contract.Asset(&_Subnet.CallOpts, id)
}

// AssetInfos is a free data retrieval call binding the contract method 0x19c2f0c8.
//
// Solidity: function assetInfos(uint32 ) view returns(string name, uint8 assetType, uint256 totalSupply)
func (_Subnet *SubnetCaller) AssetInfos(opts *bind.CallOpts, arg0 uint32) (struct {
	Name        string
	AssetType   uint8
	TotalSupply *big.Int
}, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "assetInfos", arg0)

	outstruct := new(struct {
		Name        string
		AssetType   uint8
		TotalSupply *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Name = *abi.ConvertType(out[0], new(string)).(*string)
	outstruct.AssetType = *abi.ConvertType(out[1], new(uint8)).(*uint8)
	outstruct.TotalSupply = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// AssetInfos is a free data retrieval call binding the contract method 0x19c2f0c8.
//
// Solidity: function assetInfos(uint32 ) view returns(string name, uint8 assetType, uint256 totalSupply)
func (_Subnet *SubnetSession) AssetInfos(arg0 uint32) (struct {
	Name        string
	AssetType   uint8
	TotalSupply *big.Int
}, error) {
	return _Subnet.Contract.AssetInfos(&_Subnet.CallOpts, arg0)
}

// AssetInfos is a free data retrieval call binding the contract method 0x19c2f0c8.
//
// Solidity: function assetInfos(uint32 ) view returns(string name, uint8 assetType, uint256 totalSupply)
func (_Subnet *SubnetCallerSession) AssetInfos(arg0 uint32) (struct {
	Name        string
	AssetType   uint8
	TotalSupply *big.Int
}, error) {
	return _Subnet.Contract.AssetInfos(&_Subnet.CallOpts, arg0)
}

// AssetPrices is a free data retrieval call binding the contract method 0xeebaaaee.
//
// Solidity: function assetPrices(uint32 ) view returns(uint256)
func (_Subnet *SubnetCaller) AssetPrices(opts *bind.CallOpts, arg0 uint32) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "assetPrices", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AssetPrices is a free data retrieval call binding the contract method 0xeebaaaee.
//
// Solidity: function assetPrices(uint32 ) view returns(uint256)
func (_Subnet *SubnetSession) AssetPrices(arg0 uint32) (*big.Int, error) {
	return _Subnet.Contract.AssetPrices(&_Subnet.CallOpts, arg0)
}

// AssetPrices is a free data retrieval call binding the contract method 0xeebaaaee.
//
// Solidity: function assetPrices(uint32 ) view returns(uint256)
func (_Subnet *SubnetCallerSession) AssetPrices(arg0 uint32) (*big.Int, error) {
	return _Subnet.Contract.AssetPrices(&_Subnet.CallOpts, arg0)
}

// BootNodes is a free data retrieval call binding the contract method 0x67052f80.
//
// Solidity: function bootNodes(uint32 ) view returns(uint64)
func (_Subnet *SubnetCaller) BootNodes(opts *bind.CallOpts, arg0 uint32) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "bootNodes", arg0)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// BootNodes is a free data retrieval call binding the contract method 0x67052f80.
//
// Solidity: function bootNodes(uint32 ) view returns(uint64)
func (_Subnet *SubnetSession) BootNodes(arg0 uint32) (uint64, error) {
	return _Subnet.Contract.BootNodes(&_Subnet.CallOpts, arg0)
}

// BootNodes is a free data retrieval call binding the contract method 0x67052f80.
//
// Solidity: function bootNodes(uint32 ) view returns(uint64)
func (_Subnet *SubnetCallerSession) BootNodes(arg0 uint32) (uint64, error) {
	return _Subnet.Contract.BootNodes(&_Subnet.CallOpts, arg0)
}

// BootNodesLen is a free data retrieval call binding the contract method 0x30763651.
//
// Solidity: function bootNodesLen() view returns(uint32)
func (_Subnet *SubnetCaller) BootNodesLen(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "bootNodesLen")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// BootNodesLen is a free data retrieval call binding the contract method 0x30763651.
//
// Solidity: function bootNodesLen() view returns(uint32)
func (_Subnet *SubnetSession) BootNodesLen() (uint32, error) {
	return _Subnet.Contract.BootNodesLen(&_Subnet.CallOpts)
}

// BootNodesLen is a free data retrieval call binding the contract method 0x30763651.
//
// Solidity: function bootNodesLen() view returns(uint32)
func (_Subnet *SubnetCallerSession) BootNodesLen() (uint32, error) {
	return _Subnet.Contract.BootNodesLen(&_Subnet.CallOpts)
}

// CloudContract is a free data retrieval call binding the contract method 0x464e6996.
//
// Solidity: function cloudContract() view returns(address)
func (_Subnet *SubnetCaller) CloudContract(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "cloudContract")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// CloudContract is a free data retrieval call binding the contract method 0x464e6996.
//
// Solidity: function cloudContract() view returns(address)
func (_Subnet *SubnetSession) CloudContract() (common.Address, error) {
	return _Subnet.Contract.CloudContract(&_Subnet.CallOpts)
}

// CloudContract is a free data retrieval call binding the contract method 0x464e6996.
//
// Solidity: function cloudContract() view returns(address)
func (_Subnet *SubnetCallerSession) CloudContract() (common.Address, error) {
	return _Subnet.Contract.CloudContract(&_Subnet.CallOpts)
}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint32)
func (_Subnet *SubnetCaller) Epoch(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "epoch")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint32)
func (_Subnet *SubnetSession) Epoch() (uint32, error) {
	return _Subnet.Contract.Epoch(&_Subnet.CallOpts)
}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint32)
func (_Subnet *SubnetCallerSession) Epoch() (uint32, error) {
	return _Subnet.Contract.Epoch(&_Subnet.CallOpts)
}

// EpochInfo is a free data retrieval call binding the contract method 0x1f1b8a8c.
//
// Solidity: function epochInfo() view returns((uint32,uint32,uint256,uint32,address))
func (_Subnet *SubnetCaller) EpochInfo(opts *bind.CallOpts) (EpochInfo, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "epochInfo")

	if err != nil {
		return *new(EpochInfo), err
	}

	out0 := *abi.ConvertType(out[0], new(EpochInfo)).(*EpochInfo)

	return out0, err

}

// EpochInfo is a free data retrieval call binding the contract method 0x1f1b8a8c.
//
// Solidity: function epochInfo() view returns((uint32,uint32,uint256,uint32,address))
func (_Subnet *SubnetSession) EpochInfo() (EpochInfo, error) {
	return _Subnet.Contract.EpochInfo(&_Subnet.CallOpts)
}

// EpochInfo is a free data retrieval call binding the contract method 0x1f1b8a8c.
//
// Solidity: function epochInfo() view returns((uint32,uint32,uint256,uint32,address))
func (_Subnet *SubnetCallerSession) EpochInfo() (EpochInfo, error) {
	return _Subnet.Contract.EpochInfo(&_Subnet.CallOpts)
}

// EpochSlot is a free data retrieval call binding the contract method 0x0a3d1494.
//
// Solidity: function epochSlot() view returns(uint32)
func (_Subnet *SubnetCaller) EpochSlot(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "epochSlot")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// EpochSlot is a free data retrieval call binding the contract method 0x0a3d1494.
//
// Solidity: function epochSlot() view returns(uint32)
func (_Subnet *SubnetSession) EpochSlot() (uint32, error) {
	return _Subnet.Contract.EpochSlot(&_Subnet.CallOpts)
}

// EpochSlot is a free data retrieval call binding the contract method 0x0a3d1494.
//
// Solidity: function epochSlot() view returns(uint32)
func (_Subnet *SubnetCallerSession) EpochSlot() (uint32, error) {
	return _Subnet.Contract.EpochSlot(&_Subnet.CallOpts)
}

// GetBootNodes is a free data retrieval call binding the contract method 0x45d1d988.
//
// Solidity: function getBootNodes() view returns((uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[])
func (_Subnet *SubnetCaller) GetBootNodes(opts *bind.CallOpts) ([]SecretNode, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getBootNodes")

	if err != nil {
		return *new([]SecretNode), err
	}

	out0 := *abi.ConvertType(out[0], new([]SecretNode)).(*[]SecretNode)

	return out0, err

}

// GetBootNodes is a free data retrieval call binding the contract method 0x45d1d988.
//
// Solidity: function getBootNodes() view returns((uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[])
func (_Subnet *SubnetSession) GetBootNodes() ([]SecretNode, error) {
	return _Subnet.Contract.GetBootNodes(&_Subnet.CallOpts)
}

// GetBootNodes is a free data retrieval call binding the contract method 0x45d1d988.
//
// Solidity: function getBootNodes() view returns((uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[])
func (_Subnet *SubnetCallerSession) GetBootNodes() ([]SecretNode, error) {
	return _Subnet.Contract.GetBootNodes(&_Subnet.CallOpts)
}

// GetCloudContract is a free data retrieval call binding the contract method 0xceaa87ff.
//
// Solidity: function getCloudContract() view returns(address)
func (_Subnet *SubnetCaller) GetCloudContract(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getCloudContract")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetCloudContract is a free data retrieval call binding the contract method 0xceaa87ff.
//
// Solidity: function getCloudContract() view returns(address)
func (_Subnet *SubnetSession) GetCloudContract() (common.Address, error) {
	return _Subnet.Contract.GetCloudContract(&_Subnet.CallOpts)
}

// GetCloudContract is a free data retrieval call binding the contract method 0xceaa87ff.
//
// Solidity: function getCloudContract() view returns(address)
func (_Subnet *SubnetCallerSession) GetCloudContract() (common.Address, error) {
	return _Subnet.Contract.GetCloudContract(&_Subnet.CallOpts)
}

// GetLevelMinMortgage is a free data retrieval call binding the contract method 0x1251ac07.
//
// Solidity: function getLevelMinMortgage(uint8 level) view returns(uint256)
func (_Subnet *SubnetCaller) GetLevelMinMortgage(opts *bind.CallOpts, level uint8) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getLevelMinMortgage", level)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetLevelMinMortgage is a free data retrieval call binding the contract method 0x1251ac07.
//
// Solidity: function getLevelMinMortgage(uint8 level) view returns(uint256)
func (_Subnet *SubnetSession) GetLevelMinMortgage(level uint8) (*big.Int, error) {
	return _Subnet.Contract.GetLevelMinMortgage(&_Subnet.CallOpts, level)
}

// GetLevelMinMortgage is a free data retrieval call binding the contract method 0x1251ac07.
//
// Solidity: function getLevelMinMortgage(uint8 level) view returns(uint256)
func (_Subnet *SubnetCallerSession) GetLevelMinMortgage(level uint8) (*big.Int, error) {
	return _Subnet.Contract.GetLevelMinMortgage(&_Subnet.CallOpts, level)
}

// GetLevelPrice is a free data retrieval call binding the contract method 0x6f8e2516.
//
// Solidity: function getLevelPrice(uint8 level) view returns((uint256,uint256,uint256,uint256))
func (_Subnet *SubnetCaller) GetLevelPrice(opts *bind.CallOpts, level uint8) (RunPrice, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getLevelPrice", level)

	if err != nil {
		return *new(RunPrice), err
	}

	out0 := *abi.ConvertType(out[0], new(RunPrice)).(*RunPrice)

	return out0, err

}

// GetLevelPrice is a free data retrieval call binding the contract method 0x6f8e2516.
//
// Solidity: function getLevelPrice(uint8 level) view returns((uint256,uint256,uint256,uint256))
func (_Subnet *SubnetSession) GetLevelPrice(level uint8) (RunPrice, error) {
	return _Subnet.Contract.GetLevelPrice(&_Subnet.CallOpts, level)
}

// GetLevelPrice is a free data retrieval call binding the contract method 0x6f8e2516.
//
// Solidity: function getLevelPrice(uint8 level) view returns((uint256,uint256,uint256,uint256))
func (_Subnet *SubnetCallerSession) GetLevelPrice(level uint8) (RunPrice, error) {
	return _Subnet.Contract.GetLevelPrice(&_Subnet.CallOpts, level)
}

// GetMinMortgage is a free data retrieval call binding the contract method 0xa93f6c79.
//
// Solidity: function getMinMortgage() view returns(uint256)
func (_Subnet *SubnetCaller) GetMinMortgage(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getMinMortgage")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetMinMortgage is a free data retrieval call binding the contract method 0xa93f6c79.
//
// Solidity: function getMinMortgage() view returns(uint256)
func (_Subnet *SubnetSession) GetMinMortgage() (*big.Int, error) {
	return _Subnet.Contract.GetMinMortgage(&_Subnet.CallOpts)
}

// GetMinMortgage is a free data retrieval call binding the contract method 0xa93f6c79.
//
// Solidity: function getMinMortgage() view returns(uint256)
func (_Subnet *SubnetCallerSession) GetMinMortgage() (*big.Int, error) {
	return _Subnet.Contract.GetMinMortgage(&_Subnet.CallOpts)
}

// GetPendingSecrets is a free data retrieval call binding the contract method 0x8479f2be.
//
// Solidity: function getPendingSecrets() view returns(uint64[], uint32[])
func (_Subnet *SubnetCaller) GetPendingSecrets(opts *bind.CallOpts) ([]uint64, []uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getPendingSecrets")

	if err != nil {
		return *new([]uint64), *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint64)).(*[]uint64)
	out1 := *abi.ConvertType(out[1], new([]uint32)).(*[]uint32)

	return out0, out1, err

}

// GetPendingSecrets is a free data retrieval call binding the contract method 0x8479f2be.
//
// Solidity: function getPendingSecrets() view returns(uint64[], uint32[])
func (_Subnet *SubnetSession) GetPendingSecrets() ([]uint64, []uint32, error) {
	return _Subnet.Contract.GetPendingSecrets(&_Subnet.CallOpts)
}

// GetPendingSecrets is a free data retrieval call binding the contract method 0x8479f2be.
//
// Solidity: function getPendingSecrets() view returns(uint64[], uint32[])
func (_Subnet *SubnetCallerSession) GetPendingSecrets() ([]uint64, []uint32, error) {
	return _Subnet.Contract.GetPendingSecrets(&_Subnet.CallOpts)
}

// GetRegions is a free data retrieval call binding the contract method 0xb71b9dab.
//
// Solidity: function getRegions() view returns(uint32[] ids, bytes[] names)
func (_Subnet *SubnetCaller) GetRegions(opts *bind.CallOpts) (struct {
	Ids   []uint32
	Names [][]byte
}, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getRegions")

	outstruct := new(struct {
		Ids   []uint32
		Names [][]byte
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Ids = *abi.ConvertType(out[0], new([]uint32)).(*[]uint32)
	outstruct.Names = *abi.ConvertType(out[1], new([][]byte)).(*[][]byte)

	return *outstruct, err

}

// GetRegions is a free data retrieval call binding the contract method 0xb71b9dab.
//
// Solidity: function getRegions() view returns(uint32[] ids, bytes[] names)
func (_Subnet *SubnetSession) GetRegions() (struct {
	Ids   []uint32
	Names [][]byte
}, error) {
	return _Subnet.Contract.GetRegions(&_Subnet.CallOpts)
}

// GetRegions is a free data retrieval call binding the contract method 0xb71b9dab.
//
// Solidity: function getRegions() view returns(uint32[] ids, bytes[] names)
func (_Subnet *SubnetCallerSession) GetRegions() (struct {
	Ids   []uint32
	Names [][]byte
}, error) {
	return _Subnet.Contract.GetRegions(&_Subnet.CallOpts)
}

// GetSecrets is a free data retrieval call binding the contract method 0xfc9e6f56.
//
// Solidity: function getSecrets(uint256 start, uint256 size) view returns(uint64[] ids, (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[] nodes)
func (_Subnet *SubnetCaller) GetSecrets(opts *bind.CallOpts, start *big.Int, size *big.Int) (struct {
	Ids   []uint64
	Nodes []SecretNode
}, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getSecrets", start, size)

	outstruct := new(struct {
		Ids   []uint64
		Nodes []SecretNode
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Ids = *abi.ConvertType(out[0], new([]uint64)).(*[]uint64)
	outstruct.Nodes = *abi.ConvertType(out[1], new([]SecretNode)).(*[]SecretNode)

	return *outstruct, err

}

// GetSecrets is a free data retrieval call binding the contract method 0xfc9e6f56.
//
// Solidity: function getSecrets(uint256 start, uint256 size) view returns(uint64[] ids, (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[] nodes)
func (_Subnet *SubnetSession) GetSecrets(start *big.Int, size *big.Int) (struct {
	Ids   []uint64
	Nodes []SecretNode
}, error) {
	return _Subnet.Contract.GetSecrets(&_Subnet.CallOpts, start, size)
}

// GetSecrets is a free data retrieval call binding the contract method 0xfc9e6f56.
//
// Solidity: function getSecrets(uint256 start, uint256 size) view returns(uint64[] ids, (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[] nodes)
func (_Subnet *SubnetCallerSession) GetSecrets(start *big.Int, size *big.Int) (struct {
	Ids   []uint64
	Nodes []SecretNode
}, error) {
	return _Subnet.Contract.GetSecrets(&_Subnet.CallOpts, start, size)
}

// GetValidators is a free data retrieval call binding the contract method 0xb7ab4db5.
//
// Solidity: function getValidators() view returns(uint64[], (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[], uint32[])
func (_Subnet *SubnetCaller) GetValidators(opts *bind.CallOpts) ([]uint64, []SecretNode, []uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getValidators")

	if err != nil {
		return *new([]uint64), *new([]SecretNode), *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint64)).(*[]uint64)
	out1 := *abi.ConvertType(out[1], new([]SecretNode)).(*[]SecretNode)
	out2 := *abi.ConvertType(out[2], new([]uint32)).(*[]uint32)

	return out0, out1, out2, err

}

// GetValidators is a free data retrieval call binding the contract method 0xb7ab4db5.
//
// Solidity: function getValidators() view returns(uint64[], (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[], uint32[])
func (_Subnet *SubnetSession) GetValidators() ([]uint64, []SecretNode, []uint32, error) {
	return _Subnet.Contract.GetValidators(&_Subnet.CallOpts)
}

// GetValidators is a free data retrieval call binding the contract method 0xb7ab4db5.
//
// Solidity: function getValidators() view returns(uint64[], (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[], uint32[])
func (_Subnet *SubnetCallerSession) GetValidators() ([]uint64, []SecretNode, []uint32, error) {
	return _Subnet.Contract.GetValidators(&_Subnet.CallOpts)
}

// GetWorker is a free data retrieval call binding the contract method 0x355483b4.
//
// Solidity: function getWorker(uint64 id) view returns((uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetCaller) GetWorker(opts *bind.CallOpts, id uint64) (K8sCluster, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getWorker", id)

	if err != nil {
		return *new(K8sCluster), err
	}

	out0 := *abi.ConvertType(out[0], new(K8sCluster)).(*K8sCluster)

	return out0, err

}

// GetWorker is a free data retrieval call binding the contract method 0x355483b4.
//
// Solidity: function getWorker(uint64 id) view returns((uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetSession) GetWorker(id uint64) (K8sCluster, error) {
	return _Subnet.Contract.GetWorker(&_Subnet.CallOpts, id)
}

// GetWorker is a free data retrieval call binding the contract method 0x355483b4.
//
// Solidity: function getWorker(uint64 id) view returns((uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetCallerSession) GetWorker(id uint64) (K8sCluster, error) {
	return _Subnet.Contract.GetWorker(&_Subnet.CallOpts, id)
}

// GetWorkers is a free data retrieval call binding the contract method 0xd854e08d.
//
// Solidity: function getWorkers(uint64 start, uint64 size) view returns((uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32)[], uint64[])
func (_Subnet *SubnetCaller) GetWorkers(opts *bind.CallOpts, start uint64, size uint64) ([]K8sCluster, []uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "getWorkers", start, size)

	if err != nil {
		return *new([]K8sCluster), *new([]uint64), err
	}

	out0 := *abi.ConvertType(out[0], new([]K8sCluster)).(*[]K8sCluster)
	out1 := *abi.ConvertType(out[1], new([]uint64)).(*[]uint64)

	return out0, out1, err

}

// GetWorkers is a free data retrieval call binding the contract method 0xd854e08d.
//
// Solidity: function getWorkers(uint64 start, uint64 size) view returns((uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32)[], uint64[])
func (_Subnet *SubnetSession) GetWorkers(start uint64, size uint64) ([]K8sCluster, []uint64, error) {
	return _Subnet.Contract.GetWorkers(&_Subnet.CallOpts, start, size)
}

// GetWorkers is a free data retrieval call binding the contract method 0xd854e08d.
//
// Solidity: function getWorkers(uint64 start, uint64 size) view returns((uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32)[], uint64[])
func (_Subnet *SubnetCallerSession) GetWorkers(start uint64, size uint64) ([]K8sCluster, []uint64, error) {
	return _Subnet.Contract.GetWorkers(&_Subnet.CallOpts, start, size)
}

// GovContract is a free data retrieval call binding the contract method 0x2fa4d12b.
//
// Solidity: function govContract() view returns(address)
func (_Subnet *SubnetCaller) GovContract(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "govContract")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GovContract is a free data retrieval call binding the contract method 0x2fa4d12b.
//
// Solidity: function govContract() view returns(address)
func (_Subnet *SubnetSession) GovContract() (common.Address, error) {
	return _Subnet.Contract.GovContract(&_Subnet.CallOpts)
}

// GovContract is a free data retrieval call binding the contract method 0x2fa4d12b.
//
// Solidity: function govContract() view returns(address)
func (_Subnet *SubnetCallerSession) GovContract() (common.Address, error) {
	return _Subnet.Contract.GovContract(&_Subnet.CallOpts)
}

// LastEpochBlock is a free data retrieval call binding the contract method 0xc2362dd5.
//
// Solidity: function lastEpochBlock() view returns(uint256)
func (_Subnet *SubnetCaller) LastEpochBlock(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "lastEpochBlock")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastEpochBlock is a free data retrieval call binding the contract method 0xc2362dd5.
//
// Solidity: function lastEpochBlock() view returns(uint256)
func (_Subnet *SubnetSession) LastEpochBlock() (*big.Int, error) {
	return _Subnet.Contract.LastEpochBlock(&_Subnet.CallOpts)
}

// LastEpochBlock is a free data retrieval call binding the contract method 0xc2362dd5.
//
// Solidity: function lastEpochBlock() view returns(uint256)
func (_Subnet *SubnetCallerSession) LastEpochBlock() (*big.Int, error) {
	return _Subnet.Contract.LastEpochBlock(&_Subnet.CallOpts)
}

// LevelMinMortgages is a free data retrieval call binding the contract method 0xa859ea71.
//
// Solidity: function levelMinMortgages(uint8 ) view returns(uint256)
func (_Subnet *SubnetCaller) LevelMinMortgages(opts *bind.CallOpts, arg0 uint8) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "levelMinMortgages", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LevelMinMortgages is a free data retrieval call binding the contract method 0xa859ea71.
//
// Solidity: function levelMinMortgages(uint8 ) view returns(uint256)
func (_Subnet *SubnetSession) LevelMinMortgages(arg0 uint8) (*big.Int, error) {
	return _Subnet.Contract.LevelMinMortgages(&_Subnet.CallOpts, arg0)
}

// LevelMinMortgages is a free data retrieval call binding the contract method 0xa859ea71.
//
// Solidity: function levelMinMortgages(uint8 ) view returns(uint256)
func (_Subnet *SubnetCallerSession) LevelMinMortgages(arg0 uint8) (*big.Int, error) {
	return _Subnet.Contract.LevelMinMortgages(&_Subnet.CallOpts, arg0)
}

// LevelPrices is a free data retrieval call binding the contract method 0x2e50d906.
//
// Solidity: function levelPrices(uint8 ) view returns(uint256 cpu, uint256 mem, uint256 storageAmount, uint256 network)
func (_Subnet *SubnetCaller) LevelPrices(opts *bind.CallOpts, arg0 uint8) (struct {
	Cpu           *big.Int
	Mem           *big.Int
	StorageAmount *big.Int
	Network       *big.Int
}, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "levelPrices", arg0)

	outstruct := new(struct {
		Cpu           *big.Int
		Mem           *big.Int
		StorageAmount *big.Int
		Network       *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Cpu = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Mem = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.StorageAmount = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.Network = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// LevelPrices is a free data retrieval call binding the contract method 0x2e50d906.
//
// Solidity: function levelPrices(uint8 ) view returns(uint256 cpu, uint256 mem, uint256 storageAmount, uint256 network)
func (_Subnet *SubnetSession) LevelPrices(arg0 uint8) (struct {
	Cpu           *big.Int
	Mem           *big.Int
	StorageAmount *big.Int
	Network       *big.Int
}, error) {
	return _Subnet.Contract.LevelPrices(&_Subnet.CallOpts, arg0)
}

// LevelPrices is a free data retrieval call binding the contract method 0x2e50d906.
//
// Solidity: function levelPrices(uint8 ) view returns(uint256 cpu, uint256 mem, uint256 storageAmount, uint256 network)
func (_Subnet *SubnetCallerSession) LevelPrices(arg0 uint8) (struct {
	Cpu           *big.Int
	Mem           *big.Int
	StorageAmount *big.Int
	Network       *big.Int
}, error) {
	return _Subnet.Contract.LevelPrices(&_Subnet.CallOpts, arg0)
}

// MinMortgageAmount is a free data retrieval call binding the contract method 0x1a8ee79e.
//
// Solidity: function minMortgageAmount() view returns(uint256)
func (_Subnet *SubnetCaller) MinMortgageAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "minMortgageAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MinMortgageAmount is a free data retrieval call binding the contract method 0x1a8ee79e.
//
// Solidity: function minMortgageAmount() view returns(uint256)
func (_Subnet *SubnetSession) MinMortgageAmount() (*big.Int, error) {
	return _Subnet.Contract.MinMortgageAmount(&_Subnet.CallOpts)
}

// MinMortgageAmount is a free data retrieval call binding the contract method 0x1a8ee79e.
//
// Solidity: function minMortgageAmount() view returns(uint256)
func (_Subnet *SubnetCallerSession) MinMortgageAmount() (*big.Int, error) {
	return _Subnet.Contract.MinMortgageAmount(&_Subnet.CallOpts)
}

// MintOfWorker is a free data retrieval call binding the contract method 0xe8d64184.
//
// Solidity: function mintOfWorker(bytes32 ) view returns(uint64)
func (_Subnet *SubnetCaller) MintOfWorker(opts *bind.CallOpts, arg0 [32]byte) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "mintOfWorker", arg0)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// MintOfWorker is a free data retrieval call binding the contract method 0xe8d64184.
//
// Solidity: function mintOfWorker(bytes32 ) view returns(uint64)
func (_Subnet *SubnetSession) MintOfWorker(arg0 [32]byte) (uint64, error) {
	return _Subnet.Contract.MintOfWorker(&_Subnet.CallOpts, arg0)
}

// MintOfWorker is a free data retrieval call binding the contract method 0xe8d64184.
//
// Solidity: function mintOfWorker(bytes32 ) view returns(uint64)
func (_Subnet *SubnetCallerSession) MintOfWorker(arg0 [32]byte) (uint64, error) {
	return _Subnet.Contract.MintOfWorker(&_Subnet.CallOpts, arg0)
}

// MintWorker is a free data retrieval call binding the contract method 0x0b1a5d2c.
//
// Solidity: function mintWorker(bytes32 id) view returns(uint64, (uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetCaller) MintWorker(opts *bind.CallOpts, id [32]byte) (uint64, K8sCluster, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "mintWorker", id)

	if err != nil {
		return *new(uint64), *new(K8sCluster), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)
	out1 := *abi.ConvertType(out[1], new(K8sCluster)).(*K8sCluster)

	return out0, out1, err

}

// MintWorker is a free data retrieval call binding the contract method 0x0b1a5d2c.
//
// Solidity: function mintWorker(bytes32 id) view returns(uint64, (uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetSession) MintWorker(id [32]byte) (uint64, K8sCluster, error) {
	return _Subnet.Contract.MintWorker(&_Subnet.CallOpts, id)
}

// MintWorker is a free data retrieval call binding the contract method 0x0b1a5d2c.
//
// Solidity: function mintWorker(bytes32 id) view returns(uint64, (uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetCallerSession) MintWorker(id [32]byte) (uint64, K8sCluster, error) {
	return _Subnet.Contract.MintWorker(&_Subnet.CallOpts, id)
}

// NextAssetId is a free data retrieval call binding the contract method 0x8ed91318.
//
// Solidity: function nextAssetId() view returns(uint32)
func (_Subnet *SubnetCaller) NextAssetId(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "nextAssetId")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// NextAssetId is a free data retrieval call binding the contract method 0x8ed91318.
//
// Solidity: function nextAssetId() view returns(uint32)
func (_Subnet *SubnetSession) NextAssetId() (uint32, error) {
	return _Subnet.Contract.NextAssetId(&_Subnet.CallOpts)
}

// NextAssetId is a free data retrieval call binding the contract method 0x8ed91318.
//
// Solidity: function nextAssetId() view returns(uint32)
func (_Subnet *SubnetCallerSession) NextAssetId() (uint32, error) {
	return _Subnet.Contract.NextAssetId(&_Subnet.CallOpts)
}

// NextEpochValidators is a free data retrieval call binding the contract method 0xfdce0cc0.
//
// Solidity: function nextEpochValidators() view returns(uint64[], (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[], uint32[])
func (_Subnet *SubnetCaller) NextEpochValidators(opts *bind.CallOpts) ([]uint64, []SecretNode, []uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "nextEpochValidators")

	if err != nil {
		return *new([]uint64), *new([]SecretNode), *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint64)).(*[]uint64)
	out1 := *abi.ConvertType(out[1], new([]SecretNode)).(*[]SecretNode)
	out2 := *abi.ConvertType(out[2], new([]uint32)).(*[]uint32)

	return out0, out1, out2, err

}

// NextEpochValidators is a free data retrieval call binding the contract method 0xfdce0cc0.
//
// Solidity: function nextEpochValidators() view returns(uint64[], (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[], uint32[])
func (_Subnet *SubnetSession) NextEpochValidators() ([]uint64, []SecretNode, []uint32, error) {
	return _Subnet.Contract.NextEpochValidators(&_Subnet.CallOpts)
}

// NextEpochValidators is a free data retrieval call binding the contract method 0xfdce0cc0.
//
// Solidity: function nextEpochValidators() view returns(uint64[], (uint64,bytes,address,bytes32,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32,bytes)[], uint32[])
func (_Subnet *SubnetCallerSession) NextEpochValidators() ([]uint64, []SecretNode, []uint32, error) {
	return _Subnet.Contract.NextEpochValidators(&_Subnet.CallOpts)
}

// NextRegionId is a free data retrieval call binding the contract method 0xb937e139.
//
// Solidity: function nextRegionId() view returns(uint32)
func (_Subnet *SubnetCaller) NextRegionId(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "nextRegionId")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// NextRegionId is a free data retrieval call binding the contract method 0xb937e139.
//
// Solidity: function nextRegionId() view returns(uint32)
func (_Subnet *SubnetSession) NextRegionId() (uint32, error) {
	return _Subnet.Contract.NextRegionId(&_Subnet.CallOpts)
}

// NextRegionId is a free data retrieval call binding the contract method 0xb937e139.
//
// Solidity: function nextRegionId() view returns(uint32)
func (_Subnet *SubnetCallerSession) NextRegionId() (uint32, error) {
	return _Subnet.Contract.NextRegionId(&_Subnet.CallOpts)
}

// NextSecretId is a free data retrieval call binding the contract method 0x28e32de1.
//
// Solidity: function nextSecretId() view returns(uint64)
func (_Subnet *SubnetCaller) NextSecretId(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "nextSecretId")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// NextSecretId is a free data retrieval call binding the contract method 0x28e32de1.
//
// Solidity: function nextSecretId() view returns(uint64)
func (_Subnet *SubnetSession) NextSecretId() (uint64, error) {
	return _Subnet.Contract.NextSecretId(&_Subnet.CallOpts)
}

// NextSecretId is a free data retrieval call binding the contract method 0x28e32de1.
//
// Solidity: function nextSecretId() view returns(uint64)
func (_Subnet *SubnetCallerSession) NextSecretId() (uint64, error) {
	return _Subnet.Contract.NextSecretId(&_Subnet.CallOpts)
}

// NextWorkerId is a free data retrieval call binding the contract method 0xc84a4922.
//
// Solidity: function nextWorkerId() view returns(uint64)
func (_Subnet *SubnetCaller) NextWorkerId(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "nextWorkerId")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// NextWorkerId is a free data retrieval call binding the contract method 0xc84a4922.
//
// Solidity: function nextWorkerId() view returns(uint64)
func (_Subnet *SubnetSession) NextWorkerId() (uint64, error) {
	return _Subnet.Contract.NextWorkerId(&_Subnet.CallOpts)
}

// NextWorkerId is a free data retrieval call binding the contract method 0xc84a4922.
//
// Solidity: function nextWorkerId() view returns(uint64)
func (_Subnet *SubnetCallerSession) NextWorkerId() (uint64, error) {
	return _Subnet.Contract.NextWorkerId(&_Subnet.CallOpts)
}

// OwnerOfWorker is a free data retrieval call binding the contract method 0x2041560b.
//
// Solidity: function ownerOfWorker(address ) view returns(uint64)
func (_Subnet *SubnetCaller) OwnerOfWorker(opts *bind.CallOpts, arg0 common.Address) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "ownerOfWorker", arg0)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// OwnerOfWorker is a free data retrieval call binding the contract method 0x2041560b.
//
// Solidity: function ownerOfWorker(address ) view returns(uint64)
func (_Subnet *SubnetSession) OwnerOfWorker(arg0 common.Address) (uint64, error) {
	return _Subnet.Contract.OwnerOfWorker(&_Subnet.CallOpts, arg0)
}

// OwnerOfWorker is a free data retrieval call binding the contract method 0x2041560b.
//
// Solidity: function ownerOfWorker(address ) view returns(uint64)
func (_Subnet *SubnetCallerSession) OwnerOfWorker(arg0 common.Address) (uint64, error) {
	return _Subnet.Contract.OwnerOfWorker(&_Subnet.CallOpts, arg0)
}

// PendingValidatorIds is a free data retrieval call binding the contract method 0x43bfdb53.
//
// Solidity: function pendingValidatorIds(uint256 ) view returns(uint64)
func (_Subnet *SubnetCaller) PendingValidatorIds(opts *bind.CallOpts, arg0 *big.Int) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "pendingValidatorIds", arg0)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// PendingValidatorIds is a free data retrieval call binding the contract method 0x43bfdb53.
//
// Solidity: function pendingValidatorIds(uint256 ) view returns(uint64)
func (_Subnet *SubnetSession) PendingValidatorIds(arg0 *big.Int) (uint64, error) {
	return _Subnet.Contract.PendingValidatorIds(&_Subnet.CallOpts, arg0)
}

// PendingValidatorIds is a free data retrieval call binding the contract method 0x43bfdb53.
//
// Solidity: function pendingValidatorIds(uint256 ) view returns(uint64)
func (_Subnet *SubnetCallerSession) PendingValidatorIds(arg0 *big.Int) (uint64, error) {
	return _Subnet.Contract.PendingValidatorIds(&_Subnet.CallOpts, arg0)
}

// PendingValidators is a free data retrieval call binding the contract method 0x6f8a2bbb.
//
// Solidity: function pendingValidators(uint64 ) view returns(uint32)
func (_Subnet *SubnetCaller) PendingValidators(opts *bind.CallOpts, arg0 uint64) (uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "pendingValidators", arg0)

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// PendingValidators is a free data retrieval call binding the contract method 0x6f8a2bbb.
//
// Solidity: function pendingValidators(uint64 ) view returns(uint32)
func (_Subnet *SubnetSession) PendingValidators(arg0 uint64) (uint32, error) {
	return _Subnet.Contract.PendingValidators(&_Subnet.CallOpts, arg0)
}

// PendingValidators is a free data retrieval call binding the contract method 0x6f8a2bbb.
//
// Solidity: function pendingValidators(uint64 ) view returns(uint32)
func (_Subnet *SubnetCallerSession) PendingValidators(arg0 uint64) (uint32, error) {
	return _Subnet.Contract.PendingValidators(&_Subnet.CallOpts, arg0)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_Subnet *SubnetCaller) ProxiableUUID(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "proxiableUUID")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_Subnet *SubnetSession) ProxiableUUID() ([32]byte, error) {
	return _Subnet.Contract.ProxiableUUID(&_Subnet.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_Subnet *SubnetCallerSession) ProxiableUUID() ([32]byte, error) {
	return _Subnet.Contract.ProxiableUUID(&_Subnet.CallOpts)
}

// Region is a free data retrieval call binding the contract method 0x2f49b117.
//
// Solidity: function region(uint32 id) view returns(bytes)
func (_Subnet *SubnetCaller) Region(opts *bind.CallOpts, id uint32) ([]byte, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "region", id)

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// Region is a free data retrieval call binding the contract method 0x2f49b117.
//
// Solidity: function region(uint32 id) view returns(bytes)
func (_Subnet *SubnetSession) Region(id uint32) ([]byte, error) {
	return _Subnet.Contract.Region(&_Subnet.CallOpts, id)
}

// Region is a free data retrieval call binding the contract method 0x2f49b117.
//
// Solidity: function region(uint32 id) view returns(bytes)
func (_Subnet *SubnetCallerSession) Region(id uint32) ([]byte, error) {
	return _Subnet.Contract.Region(&_Subnet.CallOpts, id)
}

// RegionWorkers is a free data retrieval call binding the contract method 0xbf543894.
//
// Solidity: function regionWorkers(uint32 , uint256 ) view returns(uint64)
func (_Subnet *SubnetCaller) RegionWorkers(opts *bind.CallOpts, arg0 uint32, arg1 *big.Int) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "regionWorkers", arg0, arg1)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// RegionWorkers is a free data retrieval call binding the contract method 0xbf543894.
//
// Solidity: function regionWorkers(uint32 , uint256 ) view returns(uint64)
func (_Subnet *SubnetSession) RegionWorkers(arg0 uint32, arg1 *big.Int) (uint64, error) {
	return _Subnet.Contract.RegionWorkers(&_Subnet.CallOpts, arg0, arg1)
}

// RegionWorkers is a free data retrieval call binding the contract method 0xbf543894.
//
// Solidity: function regionWorkers(uint32 , uint256 ) view returns(uint64)
func (_Subnet *SubnetCallerSession) RegionWorkers(arg0 uint32, arg1 *big.Int) (uint64, error) {
	return _Subnet.Contract.RegionWorkers(&_Subnet.CallOpts, arg0, arg1)
}

// Regions is a free data retrieval call binding the contract method 0x981cc42e.
//
// Solidity: function regions(uint32 ) view returns(bytes)
func (_Subnet *SubnetCaller) Regions(opts *bind.CallOpts, arg0 uint32) ([]byte, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "regions", arg0)

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// Regions is a free data retrieval call binding the contract method 0x981cc42e.
//
// Solidity: function regions(uint32 ) view returns(bytes)
func (_Subnet *SubnetSession) Regions(arg0 uint32) ([]byte, error) {
	return _Subnet.Contract.Regions(&_Subnet.CallOpts, arg0)
}

// Regions is a free data retrieval call binding the contract method 0x981cc42e.
//
// Solidity: function regions(uint32 ) view returns(bytes)
func (_Subnet *SubnetCallerSession) Regions(arg0 uint32) ([]byte, error) {
	return _Subnet.Contract.Regions(&_Subnet.CallOpts, arg0)
}

// RunningValidatorIds is a free data retrieval call binding the contract method 0x34ff648e.
//
// Solidity: function runningValidatorIds(uint256 ) view returns(uint64)
func (_Subnet *SubnetCaller) RunningValidatorIds(opts *bind.CallOpts, arg0 *big.Int) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "runningValidatorIds", arg0)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// RunningValidatorIds is a free data retrieval call binding the contract method 0x34ff648e.
//
// Solidity: function runningValidatorIds(uint256 ) view returns(uint64)
func (_Subnet *SubnetSession) RunningValidatorIds(arg0 *big.Int) (uint64, error) {
	return _Subnet.Contract.RunningValidatorIds(&_Subnet.CallOpts, arg0)
}

// RunningValidatorIds is a free data retrieval call binding the contract method 0x34ff648e.
//
// Solidity: function runningValidatorIds(uint256 ) view returns(uint64)
func (_Subnet *SubnetCallerSession) RunningValidatorIds(arg0 *big.Int) (uint64, error) {
	return _Subnet.Contract.RunningValidatorIds(&_Subnet.CallOpts, arg0)
}

// RunningValidators is a free data retrieval call binding the contract method 0xcc4b552f.
//
// Solidity: function runningValidators(uint64 ) view returns(uint32)
func (_Subnet *SubnetCaller) RunningValidators(opts *bind.CallOpts, arg0 uint64) (uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "runningValidators", arg0)

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// RunningValidators is a free data retrieval call binding the contract method 0xcc4b552f.
//
// Solidity: function runningValidators(uint64 ) view returns(uint32)
func (_Subnet *SubnetSession) RunningValidators(arg0 uint64) (uint32, error) {
	return _Subnet.Contract.RunningValidators(&_Subnet.CallOpts, arg0)
}

// RunningValidators is a free data retrieval call binding the contract method 0xcc4b552f.
//
// Solidity: function runningValidators(uint64 ) view returns(uint32)
func (_Subnet *SubnetCallerSession) RunningValidators(arg0 uint64) (uint32, error) {
	return _Subnet.Contract.RunningValidators(&_Subnet.CallOpts, arg0)
}

// SecretMortgages is a free data retrieval call binding the contract method 0x50857aca.
//
// Solidity: function secretMortgages(uint64 ) view returns(uint256)
func (_Subnet *SubnetCaller) SecretMortgages(opts *bind.CallOpts, arg0 uint64) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "secretMortgages", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SecretMortgages is a free data retrieval call binding the contract method 0x50857aca.
//
// Solidity: function secretMortgages(uint64 ) view returns(uint256)
func (_Subnet *SubnetSession) SecretMortgages(arg0 uint64) (*big.Int, error) {
	return _Subnet.Contract.SecretMortgages(&_Subnet.CallOpts, arg0)
}

// SecretMortgages is a free data retrieval call binding the contract method 0x50857aca.
//
// Solidity: function secretMortgages(uint64 ) view returns(uint256)
func (_Subnet *SubnetCallerSession) SecretMortgages(arg0 uint64) (*big.Int, error) {
	return _Subnet.Contract.SecretMortgages(&_Subnet.CallOpts, arg0)
}

// SecretOfUser is a free data retrieval call binding the contract method 0x76f4055f.
//
// Solidity: function secretOfUser(address ) view returns(uint64)
func (_Subnet *SubnetCaller) SecretOfUser(opts *bind.CallOpts, arg0 common.Address) (uint64, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "secretOfUser", arg0)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// SecretOfUser is a free data retrieval call binding the contract method 0x76f4055f.
//
// Solidity: function secretOfUser(address ) view returns(uint64)
func (_Subnet *SubnetSession) SecretOfUser(arg0 common.Address) (uint64, error) {
	return _Subnet.Contract.SecretOfUser(&_Subnet.CallOpts, arg0)
}

// SecretOfUser is a free data retrieval call binding the contract method 0x76f4055f.
//
// Solidity: function secretOfUser(address ) view returns(uint64)
func (_Subnet *SubnetCallerSession) SecretOfUser(arg0 common.Address) (uint64, error) {
	return _Subnet.Contract.SecretOfUser(&_Subnet.CallOpts, arg0)
}

// Secrets is a free data retrieval call binding the contract method 0xf9df5b78.
//
// Solidity: function secrets(uint64 ) view returns(uint64 id, bytes name, address owner, bytes32 validatorId, uint8 level, uint32 regionId, uint256 startBlock, uint256 stopBlock, uint256 terminalBlock, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 status, bytes32 p2pId, bytes validatorSecp)
func (_Subnet *SubnetCaller) Secrets(opts *bind.CallOpts, arg0 uint64) (struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	ValidatorId   [32]byte
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
	ValidatorSecp []byte
}, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "secrets", arg0)

	outstruct := new(struct {
		Id            uint64
		Name          []byte
		Owner         common.Address
		ValidatorId   [32]byte
		Level         uint8
		RegionId      uint32
		StartBlock    *big.Int
		StopBlock     *big.Int
		TerminalBlock *big.Int
		Ip            Ip
		Port          uint32
		Status        uint8
		P2pId         [32]byte
		ValidatorSecp []byte
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Id = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.Name = *abi.ConvertType(out[1], new([]byte)).(*[]byte)
	outstruct.Owner = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)
	outstruct.ValidatorId = *abi.ConvertType(out[3], new([32]byte)).(*[32]byte)
	outstruct.Level = *abi.ConvertType(out[4], new(uint8)).(*uint8)
	outstruct.RegionId = *abi.ConvertType(out[5], new(uint32)).(*uint32)
	outstruct.StartBlock = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.StopBlock = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.TerminalBlock = *abi.ConvertType(out[8], new(*big.Int)).(**big.Int)
	outstruct.Ip = *abi.ConvertType(out[9], new(Ip)).(*Ip)
	outstruct.Port = *abi.ConvertType(out[10], new(uint32)).(*uint32)
	outstruct.Status = *abi.ConvertType(out[11], new(uint8)).(*uint8)
	outstruct.P2pId = *abi.ConvertType(out[12], new([32]byte)).(*[32]byte)
	outstruct.ValidatorSecp = *abi.ConvertType(out[13], new([]byte)).(*[]byte)

	return *outstruct, err

}

// Secrets is a free data retrieval call binding the contract method 0xf9df5b78.
//
// Solidity: function secrets(uint64 ) view returns(uint64 id, bytes name, address owner, bytes32 validatorId, uint8 level, uint32 regionId, uint256 startBlock, uint256 stopBlock, uint256 terminalBlock, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 status, bytes32 p2pId, bytes validatorSecp)
func (_Subnet *SubnetSession) Secrets(arg0 uint64) (struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	ValidatorId   [32]byte
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
	ValidatorSecp []byte
}, error) {
	return _Subnet.Contract.Secrets(&_Subnet.CallOpts, arg0)
}

// Secrets is a free data retrieval call binding the contract method 0xf9df5b78.
//
// Solidity: function secrets(uint64 ) view returns(uint64 id, bytes name, address owner, bytes32 validatorId, uint8 level, uint32 regionId, uint256 startBlock, uint256 stopBlock, uint256 terminalBlock, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 status, bytes32 p2pId, bytes validatorSecp)
func (_Subnet *SubnetCallerSession) Secrets(arg0 uint64) (struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	ValidatorId   [32]byte
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
	ValidatorSecp []byte
}, error) {
	return _Subnet.Contract.Secrets(&_Subnet.CallOpts, arg0)
}

// SideChainKey is a free data retrieval call binding the contract method 0x03bf6a76.
//
// Solidity: function sideChainKey() view returns(address)
func (_Subnet *SubnetCaller) SideChainKey(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "sideChainKey")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SideChainKey is a free data retrieval call binding the contract method 0x03bf6a76.
//
// Solidity: function sideChainKey() view returns(address)
func (_Subnet *SubnetSession) SideChainKey() (common.Address, error) {
	return _Subnet.Contract.SideChainKey(&_Subnet.CallOpts)
}

// SideChainKey is a free data retrieval call binding the contract method 0x03bf6a76.
//
// Solidity: function sideChainKey() view returns(address)
func (_Subnet *SubnetCallerSession) SideChainKey() (common.Address, error) {
	return _Subnet.Contract.SideChainKey(&_Subnet.CallOpts)
}

// SideChainMultiKey is a free data retrieval call binding the contract method 0x97c10778.
//
// Solidity: function sideChainMultiKey() view returns(address)
func (_Subnet *SubnetCaller) SideChainMultiKey(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "sideChainMultiKey")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SideChainMultiKey is a free data retrieval call binding the contract method 0x97c10778.
//
// Solidity: function sideChainMultiKey() view returns(address)
func (_Subnet *SubnetSession) SideChainMultiKey() (common.Address, error) {
	return _Subnet.Contract.SideChainMultiKey(&_Subnet.CallOpts)
}

// SideChainMultiKey is a free data retrieval call binding the contract method 0x97c10778.
//
// Solidity: function sideChainMultiKey() view returns(address)
func (_Subnet *SubnetCallerSession) SideChainMultiKey() (common.Address, error) {
	return _Subnet.Contract.SideChainMultiKey(&_Subnet.CallOpts)
}

// TokenContract is a free data retrieval call binding the contract method 0x55a373d6.
//
// Solidity: function tokenContract() view returns(address)
func (_Subnet *SubnetCaller) TokenContract(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "tokenContract")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TokenContract is a free data retrieval call binding the contract method 0x55a373d6.
//
// Solidity: function tokenContract() view returns(address)
func (_Subnet *SubnetSession) TokenContract() (common.Address, error) {
	return _Subnet.Contract.TokenContract(&_Subnet.CallOpts)
}

// TokenContract is a free data retrieval call binding the contract method 0x55a373d6.
//
// Solidity: function tokenContract() view returns(address)
func (_Subnet *SubnetCallerSession) TokenContract() (common.Address, error) {
	return _Subnet.Contract.TokenContract(&_Subnet.CallOpts)
}

// UserWorker is a free data retrieval call binding the contract method 0x94fb2555.
//
// Solidity: function userWorker(address user) view returns(uint64, (uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetCaller) UserWorker(opts *bind.CallOpts, user common.Address) (uint64, K8sCluster, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "userWorker", user)

	if err != nil {
		return *new(uint64), *new(K8sCluster), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)
	out1 := *abi.ConvertType(out[1], new(K8sCluster)).(*K8sCluster)

	return out0, out1, err

}

// UserWorker is a free data retrieval call binding the contract method 0x94fb2555.
//
// Solidity: function userWorker(address user) view returns(uint64, (uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetSession) UserWorker(user common.Address) (uint64, K8sCluster, error) {
	return _Subnet.Contract.UserWorker(&_Subnet.CallOpts, user)
}

// UserWorker is a free data retrieval call binding the contract method 0x94fb2555.
//
// Solidity: function userWorker(address user) view returns(uint64, (uint64,bytes,address,uint8,uint32,uint256,uint256,uint256,(uint8,uint8,uint8,uint8),uint32,uint8,bytes32))
func (_Subnet *SubnetCallerSession) UserWorker(user common.Address) (uint64, K8sCluster, error) {
	return _Subnet.Contract.UserWorker(&_Subnet.CallOpts, user)
}

// WorkerStatus is a free data retrieval call binding the contract method 0x7b683d59.
//
// Solidity: function workerStatus(uint64 ) view returns(uint8)
func (_Subnet *SubnetCaller) WorkerStatus(opts *bind.CallOpts, arg0 uint64) (uint8, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "workerStatus", arg0)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// WorkerStatus is a free data retrieval call binding the contract method 0x7b683d59.
//
// Solidity: function workerStatus(uint64 ) view returns(uint8)
func (_Subnet *SubnetSession) WorkerStatus(arg0 uint64) (uint8, error) {
	return _Subnet.Contract.WorkerStatus(&_Subnet.CallOpts, arg0)
}

// WorkerStatus is a free data retrieval call binding the contract method 0x7b683d59.
//
// Solidity: function workerStatus(uint64 ) view returns(uint8)
func (_Subnet *SubnetCallerSession) WorkerStatus(arg0 uint64) (uint8, error) {
	return _Subnet.Contract.WorkerStatus(&_Subnet.CallOpts, arg0)
}

// WorkerTotalMortgage is a free data retrieval call binding the contract method 0xf6956b56.
//
// Solidity: function workerTotalMortgage(uint64 workerId) view returns(uint256)
func (_Subnet *SubnetCaller) WorkerTotalMortgage(opts *bind.CallOpts, workerId uint64) (*big.Int, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "workerTotalMortgage", workerId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// WorkerTotalMortgage is a free data retrieval call binding the contract method 0xf6956b56.
//
// Solidity: function workerTotalMortgage(uint64 workerId) view returns(uint256)
func (_Subnet *SubnetSession) WorkerTotalMortgage(workerId uint64) (*big.Int, error) {
	return _Subnet.Contract.WorkerTotalMortgage(&_Subnet.CallOpts, workerId)
}

// WorkerTotalMortgage is a free data retrieval call binding the contract method 0xf6956b56.
//
// Solidity: function workerTotalMortgage(uint64 workerId) view returns(uint256)
func (_Subnet *SubnetCallerSession) WorkerTotalMortgage(workerId uint64) (*big.Int, error) {
	return _Subnet.Contract.WorkerTotalMortgage(&_Subnet.CallOpts, workerId)
}

// WorkerTotalResources is a free data retrieval call binding the contract method 0x73c29ff9.
//
// Solidity: function workerTotalResources(uint64 workerId) view returns(uint32, uint32, uint32, uint32, uint32, uint32)
func (_Subnet *SubnetCaller) WorkerTotalResources(opts *bind.CallOpts, workerId uint64) (uint32, uint32, uint32, uint32, uint32, uint32, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "workerTotalResources", workerId)

	if err != nil {
		return *new(uint32), *new(uint32), *new(uint32), *new(uint32), *new(uint32), *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)
	out1 := *abi.ConvertType(out[1], new(uint32)).(*uint32)
	out2 := *abi.ConvertType(out[2], new(uint32)).(*uint32)
	out3 := *abi.ConvertType(out[3], new(uint32)).(*uint32)
	out4 := *abi.ConvertType(out[4], new(uint32)).(*uint32)
	out5 := *abi.ConvertType(out[5], new(uint32)).(*uint32)

	return out0, out1, out2, out3, out4, out5, err

}

// WorkerTotalResources is a free data retrieval call binding the contract method 0x73c29ff9.
//
// Solidity: function workerTotalResources(uint64 workerId) view returns(uint32, uint32, uint32, uint32, uint32, uint32)
func (_Subnet *SubnetSession) WorkerTotalResources(workerId uint64) (uint32, uint32, uint32, uint32, uint32, uint32, error) {
	return _Subnet.Contract.WorkerTotalResources(&_Subnet.CallOpts, workerId)
}

// WorkerTotalResources is a free data retrieval call binding the contract method 0x73c29ff9.
//
// Solidity: function workerTotalResources(uint64 workerId) view returns(uint32, uint32, uint32, uint32, uint32, uint32)
func (_Subnet *SubnetCallerSession) WorkerTotalResources(workerId uint64) (uint32, uint32, uint32, uint32, uint32, uint32, error) {
	return _Subnet.Contract.WorkerTotalResources(&_Subnet.CallOpts, workerId)
}

// Workers is a free data retrieval call binding the contract method 0x6cd7fa52.
//
// Solidity: function workers(uint64 ) view returns(uint64 id, bytes name, address owner, uint8 level, uint32 regionId, uint256 startBlock, uint256 stopBlock, uint256 terminalBlock, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 status, bytes32 p2pId)
func (_Subnet *SubnetCaller) Workers(opts *bind.CallOpts, arg0 uint64) (struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
}, error) {
	var out []interface{}
	err := _Subnet.contract.Call(opts, &out, "workers", arg0)

	outstruct := new(struct {
		Id            uint64
		Name          []byte
		Owner         common.Address
		Level         uint8
		RegionId      uint32
		StartBlock    *big.Int
		StopBlock     *big.Int
		TerminalBlock *big.Int
		Ip            Ip
		Port          uint32
		Status        uint8
		P2pId         [32]byte
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Id = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.Name = *abi.ConvertType(out[1], new([]byte)).(*[]byte)
	outstruct.Owner = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)
	outstruct.Level = *abi.ConvertType(out[3], new(uint8)).(*uint8)
	outstruct.RegionId = *abi.ConvertType(out[4], new(uint32)).(*uint32)
	outstruct.StartBlock = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.StopBlock = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.TerminalBlock = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.Ip = *abi.ConvertType(out[8], new(Ip)).(*Ip)
	outstruct.Port = *abi.ConvertType(out[9], new(uint32)).(*uint32)
	outstruct.Status = *abi.ConvertType(out[10], new(uint8)).(*uint8)
	outstruct.P2pId = *abi.ConvertType(out[11], new([32]byte)).(*[32]byte)

	return *outstruct, err

}

// Workers is a free data retrieval call binding the contract method 0x6cd7fa52.
//
// Solidity: function workers(uint64 ) view returns(uint64 id, bytes name, address owner, uint8 level, uint32 regionId, uint256 startBlock, uint256 stopBlock, uint256 terminalBlock, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 status, bytes32 p2pId)
func (_Subnet *SubnetSession) Workers(arg0 uint64) (struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
}, error) {
	return _Subnet.Contract.Workers(&_Subnet.CallOpts, arg0)
}

// Workers is a free data retrieval call binding the contract method 0x6cd7fa52.
//
// Solidity: function workers(uint64 ) view returns(uint64 id, bytes name, address owner, uint8 level, uint32 regionId, uint256 startBlock, uint256 stopBlock, uint256 terminalBlock, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 status, bytes32 p2pId)
func (_Subnet *SubnetCallerSession) Workers(arg0 uint64) (struct {
	Id            uint64
	Name          []byte
	Owner         common.Address
	Level         uint8
	RegionId      uint32
	StartBlock    *big.Int
	StopBlock     *big.Int
	TerminalBlock *big.Int
	Ip            Ip
	Port          uint32
	Status        uint8
	P2pId         [32]byte
}, error) {
	return _Subnet.Contract.Workers(&_Subnet.CallOpts, arg0)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _govContract, address _sideChainMultiKey) returns()
func (_Subnet *SubnetTransactor) Initialize(opts *bind.TransactOpts, _govContract common.Address, _sideChainMultiKey common.Address) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "initialize", _govContract, _sideChainMultiKey)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _govContract, address _sideChainMultiKey) returns()
func (_Subnet *SubnetSession) Initialize(_govContract common.Address, _sideChainMultiKey common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.Initialize(&_Subnet.TransactOpts, _govContract, _sideChainMultiKey)
}

// Initialize is a paid mutator transaction binding the contract method 0x485cc955.
//
// Solidity: function initialize(address _govContract, address _sideChainMultiKey) returns()
func (_Subnet *SubnetTransactorSession) Initialize(_govContract common.Address, _sideChainMultiKey common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.Initialize(&_Subnet.TransactOpts, _govContract, _sideChainMultiKey)
}

// SecretDelete is a paid mutator transaction binding the contract method 0x8250f3c0.
//
// Solidity: function secretDelete(uint64 id) returns()
func (_Subnet *SubnetTransactor) SecretDelete(opts *bind.TransactOpts, id uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "secretDelete", id)
}

// SecretDelete is a paid mutator transaction binding the contract method 0x8250f3c0.
//
// Solidity: function secretDelete(uint64 id) returns()
func (_Subnet *SubnetSession) SecretDelete(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.SecretDelete(&_Subnet.TransactOpts, id)
}

// SecretDelete is a paid mutator transaction binding the contract method 0x8250f3c0.
//
// Solidity: function secretDelete(uint64 id) returns()
func (_Subnet *SubnetTransactorSession) SecretDelete(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.SecretDelete(&_Subnet.TransactOpts, id)
}

// SecretDeposit is a paid mutator transaction binding the contract method 0x59abe634.
//
// Solidity: function secretDeposit(uint64 id, uint256 deposit) payable returns()
func (_Subnet *SubnetTransactor) SecretDeposit(opts *bind.TransactOpts, id uint64, deposit *big.Int) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "secretDeposit", id, deposit)
}

// SecretDeposit is a paid mutator transaction binding the contract method 0x59abe634.
//
// Solidity: function secretDeposit(uint64 id, uint256 deposit) payable returns()
func (_Subnet *SubnetSession) SecretDeposit(id uint64, deposit *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SecretDeposit(&_Subnet.TransactOpts, id, deposit)
}

// SecretDeposit is a paid mutator transaction binding the contract method 0x59abe634.
//
// Solidity: function secretDeposit(uint64 id, uint256 deposit) payable returns()
func (_Subnet *SubnetTransactorSession) SecretDeposit(id uint64, deposit *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SecretDeposit(&_Subnet.TransactOpts, id, deposit)
}

// SecretRegister is a paid mutator transaction binding the contract method 0x9b7797d7.
//
// Solidity: function secretRegister(bytes name, bytes32 validatorId, bytes32 p2pId, (uint8,uint8,uint8,uint8) ip, uint32 port, bytes validatorSecp) returns(uint64)
func (_Subnet *SubnetTransactor) SecretRegister(opts *bind.TransactOpts, name []byte, validatorId [32]byte, p2pId [32]byte, ip Ip, port uint32, validatorSecp []byte) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "secretRegister", name, validatorId, p2pId, ip, port, validatorSecp)
}

// SecretRegister is a paid mutator transaction binding the contract method 0x9b7797d7.
//
// Solidity: function secretRegister(bytes name, bytes32 validatorId, bytes32 p2pId, (uint8,uint8,uint8,uint8) ip, uint32 port, bytes validatorSecp) returns(uint64)
func (_Subnet *SubnetSession) SecretRegister(name []byte, validatorId [32]byte, p2pId [32]byte, ip Ip, port uint32, validatorSecp []byte) (*types.Transaction, error) {
	return _Subnet.Contract.SecretRegister(&_Subnet.TransactOpts, name, validatorId, p2pId, ip, port, validatorSecp)
}

// SecretRegister is a paid mutator transaction binding the contract method 0x9b7797d7.
//
// Solidity: function secretRegister(bytes name, bytes32 validatorId, bytes32 p2pId, (uint8,uint8,uint8,uint8) ip, uint32 port, bytes validatorSecp) returns(uint64)
func (_Subnet *SubnetTransactorSession) SecretRegister(name []byte, validatorId [32]byte, p2pId [32]byte, ip Ip, port uint32, validatorSecp []byte) (*types.Transaction, error) {
	return _Subnet.Contract.SecretRegister(&_Subnet.TransactOpts, name, validatorId, p2pId, ip, port, validatorSecp)
}

// SecretSetSecpKey is a paid mutator transaction binding the contract method 0x5c92cef8.
//
// Solidity: function secretSetSecpKey(uint64 id, bytes validatorSecp) returns()
func (_Subnet *SubnetTransactor) SecretSetSecpKey(opts *bind.TransactOpts, id uint64, validatorSecp []byte) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "secretSetSecpKey", id, validatorSecp)
}

// SecretSetSecpKey is a paid mutator transaction binding the contract method 0x5c92cef8.
//
// Solidity: function secretSetSecpKey(uint64 id, bytes validatorSecp) returns()
func (_Subnet *SubnetSession) SecretSetSecpKey(id uint64, validatorSecp []byte) (*types.Transaction, error) {
	return _Subnet.Contract.SecretSetSecpKey(&_Subnet.TransactOpts, id, validatorSecp)
}

// SecretSetSecpKey is a paid mutator transaction binding the contract method 0x5c92cef8.
//
// Solidity: function secretSetSecpKey(uint64 id, bytes validatorSecp) returns()
func (_Subnet *SubnetTransactorSession) SecretSetSecpKey(id uint64, validatorSecp []byte) (*types.Transaction, error) {
	return _Subnet.Contract.SecretSetSecpKey(&_Subnet.TransactOpts, id, validatorSecp)
}

// SecretUpdate is a paid mutator transaction binding the contract method 0x4f579c6d.
//
// Solidity: function secretUpdate(uint64 id, bytes name, (uint8,uint8,uint8,uint8) ip, uint32 port) returns()
func (_Subnet *SubnetTransactor) SecretUpdate(opts *bind.TransactOpts, id uint64, name []byte, ip Ip, port uint32) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "secretUpdate", id, name, ip, port)
}

// SecretUpdate is a paid mutator transaction binding the contract method 0x4f579c6d.
//
// Solidity: function secretUpdate(uint64 id, bytes name, (uint8,uint8,uint8,uint8) ip, uint32 port) returns()
func (_Subnet *SubnetSession) SecretUpdate(id uint64, name []byte, ip Ip, port uint32) (*types.Transaction, error) {
	return _Subnet.Contract.SecretUpdate(&_Subnet.TransactOpts, id, name, ip, port)
}

// SecretUpdate is a paid mutator transaction binding the contract method 0x4f579c6d.
//
// Solidity: function secretUpdate(uint64 id, bytes name, (uint8,uint8,uint8,uint8) ip, uint32 port) returns()
func (_Subnet *SubnetTransactorSession) SecretUpdate(id uint64, name []byte, ip Ip, port uint32) (*types.Transaction, error) {
	return _Subnet.Contract.SecretUpdate(&_Subnet.TransactOpts, id, name, ip, port)
}

// SetAsset is a paid mutator transaction binding the contract method 0xdbef4a23.
//
// Solidity: function setAsset((string,uint8,uint256) info, uint256 price) returns(uint32)
func (_Subnet *SubnetTransactor) SetAsset(opts *bind.TransactOpts, info AssetInfo, price *big.Int) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setAsset", info, price)
}

// SetAsset is a paid mutator transaction binding the contract method 0xdbef4a23.
//
// Solidity: function setAsset((string,uint8,uint256) info, uint256 price) returns(uint32)
func (_Subnet *SubnetSession) SetAsset(info AssetInfo, price *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SetAsset(&_Subnet.TransactOpts, info, price)
}

// SetAsset is a paid mutator transaction binding the contract method 0xdbef4a23.
//
// Solidity: function setAsset((string,uint8,uint256) info, uint256 price) returns(uint32)
func (_Subnet *SubnetTransactorSession) SetAsset(info AssetInfo, price *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SetAsset(&_Subnet.TransactOpts, info, price)
}

// SetBootNodes is a paid mutator transaction binding the contract method 0x6a6afb98.
//
// Solidity: function setBootNodes(uint64[] nodes) returns()
func (_Subnet *SubnetTransactor) SetBootNodes(opts *bind.TransactOpts, nodes []uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setBootNodes", nodes)
}

// SetBootNodes is a paid mutator transaction binding the contract method 0x6a6afb98.
//
// Solidity: function setBootNodes(uint64[] nodes) returns()
func (_Subnet *SubnetSession) SetBootNodes(nodes []uint64) (*types.Transaction, error) {
	return _Subnet.Contract.SetBootNodes(&_Subnet.TransactOpts, nodes)
}

// SetBootNodes is a paid mutator transaction binding the contract method 0x6a6afb98.
//
// Solidity: function setBootNodes(uint64[] nodes) returns()
func (_Subnet *SubnetTransactorSession) SetBootNodes(nodes []uint64) (*types.Transaction, error) {
	return _Subnet.Contract.SetBootNodes(&_Subnet.TransactOpts, nodes)
}

// SetCloudContract is a paid mutator transaction binding the contract method 0x81eb6810.
//
// Solidity: function setCloudContract(address addr) returns()
func (_Subnet *SubnetTransactor) SetCloudContract(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setCloudContract", addr)
}

// SetCloudContract is a paid mutator transaction binding the contract method 0x81eb6810.
//
// Solidity: function setCloudContract(address addr) returns()
func (_Subnet *SubnetSession) SetCloudContract(addr common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.SetCloudContract(&_Subnet.TransactOpts, addr)
}

// SetCloudContract is a paid mutator transaction binding the contract method 0x81eb6810.
//
// Solidity: function setCloudContract(address addr) returns()
func (_Subnet *SubnetTransactorSession) SetCloudContract(addr common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.SetCloudContract(&_Subnet.TransactOpts, addr)
}

// SetEpochSlot is a paid mutator transaction binding the contract method 0xa172f03e.
//
// Solidity: function setEpochSlot(uint32 _epochSlot) returns()
func (_Subnet *SubnetTransactor) SetEpochSlot(opts *bind.TransactOpts, _epochSlot uint32) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setEpochSlot", _epochSlot)
}

// SetEpochSlot is a paid mutator transaction binding the contract method 0xa172f03e.
//
// Solidity: function setEpochSlot(uint32 _epochSlot) returns()
func (_Subnet *SubnetSession) SetEpochSlot(_epochSlot uint32) (*types.Transaction, error) {
	return _Subnet.Contract.SetEpochSlot(&_Subnet.TransactOpts, _epochSlot)
}

// SetEpochSlot is a paid mutator transaction binding the contract method 0xa172f03e.
//
// Solidity: function setEpochSlot(uint32 _epochSlot) returns()
func (_Subnet *SubnetTransactorSession) SetEpochSlot(_epochSlot uint32) (*types.Transaction, error) {
	return _Subnet.Contract.SetEpochSlot(&_Subnet.TransactOpts, _epochSlot)
}

// SetLevelMinMortgage is a paid mutator transaction binding the contract method 0x9e666d83.
//
// Solidity: function setLevelMinMortgage(uint8 level, uint256 amount) returns()
func (_Subnet *SubnetTransactor) SetLevelMinMortgage(opts *bind.TransactOpts, level uint8, amount *big.Int) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setLevelMinMortgage", level, amount)
}

// SetLevelMinMortgage is a paid mutator transaction binding the contract method 0x9e666d83.
//
// Solidity: function setLevelMinMortgage(uint8 level, uint256 amount) returns()
func (_Subnet *SubnetSession) SetLevelMinMortgage(level uint8, amount *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SetLevelMinMortgage(&_Subnet.TransactOpts, level, amount)
}

// SetLevelMinMortgage is a paid mutator transaction binding the contract method 0x9e666d83.
//
// Solidity: function setLevelMinMortgage(uint8 level, uint256 amount) returns()
func (_Subnet *SubnetTransactorSession) SetLevelMinMortgage(level uint8, amount *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SetLevelMinMortgage(&_Subnet.TransactOpts, level, amount)
}

// SetLevelPrice is a paid mutator transaction binding the contract method 0xc0b4df3a.
//
// Solidity: function setLevelPrice(uint8 level, (uint256,uint256,uint256,uint256) price) returns()
func (_Subnet *SubnetTransactor) SetLevelPrice(opts *bind.TransactOpts, level uint8, price RunPrice) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setLevelPrice", level, price)
}

// SetLevelPrice is a paid mutator transaction binding the contract method 0xc0b4df3a.
//
// Solidity: function setLevelPrice(uint8 level, (uint256,uint256,uint256,uint256) price) returns()
func (_Subnet *SubnetSession) SetLevelPrice(level uint8, price RunPrice) (*types.Transaction, error) {
	return _Subnet.Contract.SetLevelPrice(&_Subnet.TransactOpts, level, price)
}

// SetLevelPrice is a paid mutator transaction binding the contract method 0xc0b4df3a.
//
// Solidity: function setLevelPrice(uint8 level, (uint256,uint256,uint256,uint256) price) returns()
func (_Subnet *SubnetTransactorSession) SetLevelPrice(level uint8, price RunPrice) (*types.Transaction, error) {
	return _Subnet.Contract.SetLevelPrice(&_Subnet.TransactOpts, level, price)
}

// SetMinMortgage is a paid mutator transaction binding the contract method 0xfc5d1acb.
//
// Solidity: function setMinMortgage(uint256 amount) returns()
func (_Subnet *SubnetTransactor) SetMinMortgage(opts *bind.TransactOpts, amount *big.Int) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setMinMortgage", amount)
}

// SetMinMortgage is a paid mutator transaction binding the contract method 0xfc5d1acb.
//
// Solidity: function setMinMortgage(uint256 amount) returns()
func (_Subnet *SubnetSession) SetMinMortgage(amount *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SetMinMortgage(&_Subnet.TransactOpts, amount)
}

// SetMinMortgage is a paid mutator transaction binding the contract method 0xfc5d1acb.
//
// Solidity: function setMinMortgage(uint256 amount) returns()
func (_Subnet *SubnetTransactorSession) SetMinMortgage(amount *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.SetMinMortgage(&_Subnet.TransactOpts, amount)
}

// SetNextEpoch is a paid mutator transaction binding the contract method 0x4574b317.
//
// Solidity: function setNextEpoch(uint64 ) returns()
func (_Subnet *SubnetTransactor) SetNextEpoch(opts *bind.TransactOpts, arg0 uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setNextEpoch", arg0)
}

// SetNextEpoch is a paid mutator transaction binding the contract method 0x4574b317.
//
// Solidity: function setNextEpoch(uint64 ) returns()
func (_Subnet *SubnetSession) SetNextEpoch(arg0 uint64) (*types.Transaction, error) {
	return _Subnet.Contract.SetNextEpoch(&_Subnet.TransactOpts, arg0)
}

// SetNextEpoch is a paid mutator transaction binding the contract method 0x4574b317.
//
// Solidity: function setNextEpoch(uint64 ) returns()
func (_Subnet *SubnetTransactorSession) SetNextEpoch(arg0 uint64) (*types.Transaction, error) {
	return _Subnet.Contract.SetNextEpoch(&_Subnet.TransactOpts, arg0)
}

// SetRegion is a paid mutator transaction binding the contract method 0x10bfdfb1.
//
// Solidity: function setRegion(bytes name) returns(uint32)
func (_Subnet *SubnetTransactor) SetRegion(opts *bind.TransactOpts, name []byte) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setRegion", name)
}

// SetRegion is a paid mutator transaction binding the contract method 0x10bfdfb1.
//
// Solidity: function setRegion(bytes name) returns(uint32)
func (_Subnet *SubnetSession) SetRegion(name []byte) (*types.Transaction, error) {
	return _Subnet.Contract.SetRegion(&_Subnet.TransactOpts, name)
}

// SetRegion is a paid mutator transaction binding the contract method 0x10bfdfb1.
//
// Solidity: function setRegion(bytes name) returns(uint32)
func (_Subnet *SubnetTransactorSession) SetRegion(name []byte) (*types.Transaction, error) {
	return _Subnet.Contract.SetRegion(&_Subnet.TransactOpts, name)
}

// SetTokenContract is a paid mutator transaction binding the contract method 0xbbcd5bbe.
//
// Solidity: function setTokenContract(address _token) returns()
func (_Subnet *SubnetTransactor) SetTokenContract(opts *bind.TransactOpts, _token common.Address) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "setTokenContract", _token)
}

// SetTokenContract is a paid mutator transaction binding the contract method 0xbbcd5bbe.
//
// Solidity: function setTokenContract(address _token) returns()
func (_Subnet *SubnetSession) SetTokenContract(_token common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.SetTokenContract(&_Subnet.TransactOpts, _token)
}

// SetTokenContract is a paid mutator transaction binding the contract method 0xbbcd5bbe.
//
// Solidity: function setTokenContract(address _token) returns()
func (_Subnet *SubnetTransactorSession) SetTokenContract(_token common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.SetTokenContract(&_Subnet.TransactOpts, _token)
}

// SlashWorkerMortgage is a paid mutator transaction binding the contract method 0xd0e2c566.
//
// Solidity: function slashWorkerMortgage(uint64 workerId, uint256 amount, address to) returns()
func (_Subnet *SubnetTransactor) SlashWorkerMortgage(opts *bind.TransactOpts, workerId uint64, amount *big.Int, to common.Address) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "slashWorkerMortgage", workerId, amount, to)
}

// SlashWorkerMortgage is a paid mutator transaction binding the contract method 0xd0e2c566.
//
// Solidity: function slashWorkerMortgage(uint64 workerId, uint256 amount, address to) returns()
func (_Subnet *SubnetSession) SlashWorkerMortgage(workerId uint64, amount *big.Int, to common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.SlashWorkerMortgage(&_Subnet.TransactOpts, workerId, amount, to)
}

// SlashWorkerMortgage is a paid mutator transaction binding the contract method 0xd0e2c566.
//
// Solidity: function slashWorkerMortgage(uint64 workerId, uint256 amount, address to) returns()
func (_Subnet *SubnetTransactorSession) SlashWorkerMortgage(workerId uint64, amount *big.Int, to common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.SlashWorkerMortgage(&_Subnet.TransactOpts, workerId, amount, to)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_Subnet *SubnetTransactor) UpgradeTo(opts *bind.TransactOpts, newImplementation common.Address) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "upgradeTo", newImplementation)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_Subnet *SubnetSession) UpgradeTo(newImplementation common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.UpgradeTo(&_Subnet.TransactOpts, newImplementation)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_Subnet *SubnetTransactorSession) UpgradeTo(newImplementation common.Address) (*types.Transaction, error) {
	return _Subnet.Contract.UpgradeTo(&_Subnet.TransactOpts, newImplementation)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_Subnet *SubnetTransactor) UpgradeToAndCall(opts *bind.TransactOpts, newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "upgradeToAndCall", newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_Subnet *SubnetSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _Subnet.Contract.UpgradeToAndCall(&_Subnet.TransactOpts, newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_Subnet *SubnetTransactorSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _Subnet.Contract.UpgradeToAndCall(&_Subnet.TransactOpts, newImplementation, data)
}

// ValidatorDelete is a paid mutator transaction binding the contract method 0xa63dd16c.
//
// Solidity: function validatorDelete(uint64 id) returns()
func (_Subnet *SubnetTransactor) ValidatorDelete(opts *bind.TransactOpts, id uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "validatorDelete", id)
}

// ValidatorDelete is a paid mutator transaction binding the contract method 0xa63dd16c.
//
// Solidity: function validatorDelete(uint64 id) returns()
func (_Subnet *SubnetSession) ValidatorDelete(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.ValidatorDelete(&_Subnet.TransactOpts, id)
}

// ValidatorDelete is a paid mutator transaction binding the contract method 0xa63dd16c.
//
// Solidity: function validatorDelete(uint64 id) returns()
func (_Subnet *SubnetTransactorSession) ValidatorDelete(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.ValidatorDelete(&_Subnet.TransactOpts, id)
}

// ValidatorJoin is a paid mutator transaction binding the contract method 0xad2d2971.
//
// Solidity: function validatorJoin(uint64 id) returns()
func (_Subnet *SubnetTransactor) ValidatorJoin(opts *bind.TransactOpts, id uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "validatorJoin", id)
}

// ValidatorJoin is a paid mutator transaction binding the contract method 0xad2d2971.
//
// Solidity: function validatorJoin(uint64 id) returns()
func (_Subnet *SubnetSession) ValidatorJoin(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.ValidatorJoin(&_Subnet.TransactOpts, id)
}

// ValidatorJoin is a paid mutator transaction binding the contract method 0xad2d2971.
//
// Solidity: function validatorJoin(uint64 id) returns()
func (_Subnet *SubnetTransactorSession) ValidatorJoin(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.ValidatorJoin(&_Subnet.TransactOpts, id)
}

// WithdrawToken is a paid mutator transaction binding the contract method 0x9e281a98.
//
// Solidity: function withdrawToken(address user, uint256 ethAmount) returns()
func (_Subnet *SubnetTransactor) WithdrawToken(opts *bind.TransactOpts, user common.Address, ethAmount *big.Int) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "withdrawToken", user, ethAmount)
}

// WithdrawToken is a paid mutator transaction binding the contract method 0x9e281a98.
//
// Solidity: function withdrawToken(address user, uint256 ethAmount) returns()
func (_Subnet *SubnetSession) WithdrawToken(user common.Address, ethAmount *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.WithdrawToken(&_Subnet.TransactOpts, user, ethAmount)
}

// WithdrawToken is a paid mutator transaction binding the contract method 0x9e281a98.
//
// Solidity: function withdrawToken(address user, uint256 ethAmount) returns()
func (_Subnet *SubnetTransactorSession) WithdrawToken(user common.Address, ethAmount *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.WithdrawToken(&_Subnet.TransactOpts, user, ethAmount)
}

// WorkerMortgage is a paid mutator transaction binding the contract method 0x59c9ff0c.
//
// Solidity: function workerMortgage(uint64 id, uint32 cpu, uint32 mem, uint32 cvmCpu, uint32 cvmMem, uint32 disk, uint32 gpu, uint256 deposit) payable returns(uint64)
func (_Subnet *SubnetTransactor) WorkerMortgage(opts *bind.TransactOpts, id uint64, cpu uint32, mem uint32, cvmCpu uint32, cvmMem uint32, disk uint32, gpu uint32, deposit *big.Int) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "workerMortgage", id, cpu, mem, cvmCpu, cvmMem, disk, gpu, deposit)
}

// WorkerMortgage is a paid mutator transaction binding the contract method 0x59c9ff0c.
//
// Solidity: function workerMortgage(uint64 id, uint32 cpu, uint32 mem, uint32 cvmCpu, uint32 cvmMem, uint32 disk, uint32 gpu, uint256 deposit) payable returns(uint64)
func (_Subnet *SubnetSession) WorkerMortgage(id uint64, cpu uint32, mem uint32, cvmCpu uint32, cvmMem uint32, disk uint32, gpu uint32, deposit *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerMortgage(&_Subnet.TransactOpts, id, cpu, mem, cvmCpu, cvmMem, disk, gpu, deposit)
}

// WorkerMortgage is a paid mutator transaction binding the contract method 0x59c9ff0c.
//
// Solidity: function workerMortgage(uint64 id, uint32 cpu, uint32 mem, uint32 cvmCpu, uint32 cvmMem, uint32 disk, uint32 gpu, uint256 deposit) payable returns(uint64)
func (_Subnet *SubnetTransactorSession) WorkerMortgage(id uint64, cpu uint32, mem uint32, cvmCpu uint32, cvmMem uint32, disk uint32, gpu uint32, deposit *big.Int) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerMortgage(&_Subnet.TransactOpts, id, cpu, mem, cvmCpu, cvmMem, disk, gpu, deposit)
}

// WorkerRegister is a paid mutator transaction binding the contract method 0x811f0b63.
//
// Solidity: function workerRegister(bytes name, bytes32 p2pId, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 level, uint32 regionId) returns(uint64)
func (_Subnet *SubnetTransactor) WorkerRegister(opts *bind.TransactOpts, name []byte, p2pId [32]byte, ip Ip, port uint32, level uint8, regionId uint32) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "workerRegister", name, p2pId, ip, port, level, regionId)
}

// WorkerRegister is a paid mutator transaction binding the contract method 0x811f0b63.
//
// Solidity: function workerRegister(bytes name, bytes32 p2pId, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 level, uint32 regionId) returns(uint64)
func (_Subnet *SubnetSession) WorkerRegister(name []byte, p2pId [32]byte, ip Ip, port uint32, level uint8, regionId uint32) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerRegister(&_Subnet.TransactOpts, name, p2pId, ip, port, level, regionId)
}

// WorkerRegister is a paid mutator transaction binding the contract method 0x811f0b63.
//
// Solidity: function workerRegister(bytes name, bytes32 p2pId, (uint8,uint8,uint8,uint8) ip, uint32 port, uint8 level, uint32 regionId) returns(uint64)
func (_Subnet *SubnetTransactorSession) WorkerRegister(name []byte, p2pId [32]byte, ip Ip, port uint32, level uint8, regionId uint32) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerRegister(&_Subnet.TransactOpts, name, p2pId, ip, port, level, regionId)
}

// WorkerStart is a paid mutator transaction binding the contract method 0x3bd5a02f.
//
// Solidity: function workerStart(uint64 id) returns()
func (_Subnet *SubnetTransactor) WorkerStart(opts *bind.TransactOpts, id uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "workerStart", id)
}

// WorkerStart is a paid mutator transaction binding the contract method 0x3bd5a02f.
//
// Solidity: function workerStart(uint64 id) returns()
func (_Subnet *SubnetSession) WorkerStart(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerStart(&_Subnet.TransactOpts, id)
}

// WorkerStart is a paid mutator transaction binding the contract method 0x3bd5a02f.
//
// Solidity: function workerStart(uint64 id) returns()
func (_Subnet *SubnetTransactorSession) WorkerStart(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerStart(&_Subnet.TransactOpts, id)
}

// WorkerStop is a paid mutator transaction binding the contract method 0x7d78d79a.
//
// Solidity: function workerStop(uint64 id) returns(uint64)
func (_Subnet *SubnetTransactor) WorkerStop(opts *bind.TransactOpts, id uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "workerStop", id)
}

// WorkerStop is a paid mutator transaction binding the contract method 0x7d78d79a.
//
// Solidity: function workerStop(uint64 id) returns(uint64)
func (_Subnet *SubnetSession) WorkerStop(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerStop(&_Subnet.TransactOpts, id)
}

// WorkerStop is a paid mutator transaction binding the contract method 0x7d78d79a.
//
// Solidity: function workerStop(uint64 id) returns(uint64)
func (_Subnet *SubnetTransactorSession) WorkerStop(id uint64) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerStop(&_Subnet.TransactOpts, id)
}

// WorkerUnmortgage is a paid mutator transaction binding the contract method 0x18c6a7d9.
//
// Solidity: function workerUnmortgage(uint64 workerId, uint64 mortgageId) returns(uint64)
func (_Subnet *SubnetTransactor) WorkerUnmortgage(opts *bind.TransactOpts, workerId uint64, mortgageId uint64) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "workerUnmortgage", workerId, mortgageId)
}

// WorkerUnmortgage is a paid mutator transaction binding the contract method 0x18c6a7d9.
//
// Solidity: function workerUnmortgage(uint64 workerId, uint64 mortgageId) returns(uint64)
func (_Subnet *SubnetSession) WorkerUnmortgage(workerId uint64, mortgageId uint64) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerUnmortgage(&_Subnet.TransactOpts, workerId, mortgageId)
}

// WorkerUnmortgage is a paid mutator transaction binding the contract method 0x18c6a7d9.
//
// Solidity: function workerUnmortgage(uint64 workerId, uint64 mortgageId) returns(uint64)
func (_Subnet *SubnetTransactorSession) WorkerUnmortgage(workerId uint64, mortgageId uint64) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerUnmortgage(&_Subnet.TransactOpts, workerId, mortgageId)
}

// WorkerUpdate is a paid mutator transaction binding the contract method 0x1d767538.
//
// Solidity: function workerUpdate(uint64 id, bytes name, (uint8,uint8,uint8,uint8) ip, uint32 port) returns()
func (_Subnet *SubnetTransactor) WorkerUpdate(opts *bind.TransactOpts, id uint64, name []byte, ip Ip, port uint32) (*types.Transaction, error) {
	return _Subnet.contract.Transact(opts, "workerUpdate", id, name, ip, port)
}

// WorkerUpdate is a paid mutator transaction binding the contract method 0x1d767538.
//
// Solidity: function workerUpdate(uint64 id, bytes name, (uint8,uint8,uint8,uint8) ip, uint32 port) returns()
func (_Subnet *SubnetSession) WorkerUpdate(id uint64, name []byte, ip Ip, port uint32) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerUpdate(&_Subnet.TransactOpts, id, name, ip, port)
}

// WorkerUpdate is a paid mutator transaction binding the contract method 0x1d767538.
//
// Solidity: function workerUpdate(uint64 id, bytes name, (uint8,uint8,uint8,uint8) ip, uint32 port) returns()
func (_Subnet *SubnetTransactorSession) WorkerUpdate(id uint64, name []byte, ip Ip, port uint32) (*types.Transaction, error) {
	return _Subnet.Contract.WorkerUpdate(&_Subnet.TransactOpts, id, name, ip, port)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Subnet *SubnetTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Subnet.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Subnet *SubnetSession) Receive() (*types.Transaction, error) {
	return _Subnet.Contract.Receive(&_Subnet.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Subnet *SubnetTransactorSession) Receive() (*types.Transaction, error) {
	return _Subnet.Contract.Receive(&_Subnet.TransactOpts)
}

// SubnetAdminChangedIterator is returned from FilterAdminChanged and is used to iterate over the raw logs and unpacked data for AdminChanged events raised by the Subnet contract.
type SubnetAdminChangedIterator struct {
	Event *SubnetAdminChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SubnetAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SubnetAdminChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SubnetAdminChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SubnetAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SubnetAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SubnetAdminChanged represents a AdminChanged event raised by the Subnet contract.
type SubnetAdminChanged struct {
	PreviousAdmin common.Address
	NewAdmin      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterAdminChanged is a free log retrieval operation binding the contract event 0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f.
//
// Solidity: event AdminChanged(address previousAdmin, address newAdmin)
func (_Subnet *SubnetFilterer) FilterAdminChanged(opts *bind.FilterOpts) (*SubnetAdminChangedIterator, error) {

	logs, sub, err := _Subnet.contract.FilterLogs(opts, "AdminChanged")
	if err != nil {
		return nil, err
	}
	return &SubnetAdminChangedIterator{contract: _Subnet.contract, event: "AdminChanged", logs: logs, sub: sub}, nil
}

// WatchAdminChanged is a free log subscription operation binding the contract event 0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f.
//
// Solidity: event AdminChanged(address previousAdmin, address newAdmin)
func (_Subnet *SubnetFilterer) WatchAdminChanged(opts *bind.WatchOpts, sink chan<- *SubnetAdminChanged) (event.Subscription, error) {

	logs, sub, err := _Subnet.contract.WatchLogs(opts, "AdminChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SubnetAdminChanged)
				if err := _Subnet.contract.UnpackLog(event, "AdminChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAdminChanged is a log parse operation binding the contract event 0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f.
//
// Solidity: event AdminChanged(address previousAdmin, address newAdmin)
func (_Subnet *SubnetFilterer) ParseAdminChanged(log types.Log) (*SubnetAdminChanged, error) {
	event := new(SubnetAdminChanged)
	if err := _Subnet.contract.UnpackLog(event, "AdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SubnetBeaconUpgradedIterator is returned from FilterBeaconUpgraded and is used to iterate over the raw logs and unpacked data for BeaconUpgraded events raised by the Subnet contract.
type SubnetBeaconUpgradedIterator struct {
	Event *SubnetBeaconUpgraded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SubnetBeaconUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SubnetBeaconUpgraded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SubnetBeaconUpgraded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SubnetBeaconUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SubnetBeaconUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SubnetBeaconUpgraded represents a BeaconUpgraded event raised by the Subnet contract.
type SubnetBeaconUpgraded struct {
	Beacon common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterBeaconUpgraded is a free log retrieval operation binding the contract event 0x1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e.
//
// Solidity: event BeaconUpgraded(address indexed beacon)
func (_Subnet *SubnetFilterer) FilterBeaconUpgraded(opts *bind.FilterOpts, beacon []common.Address) (*SubnetBeaconUpgradedIterator, error) {

	var beaconRule []interface{}
	for _, beaconItem := range beacon {
		beaconRule = append(beaconRule, beaconItem)
	}

	logs, sub, err := _Subnet.contract.FilterLogs(opts, "BeaconUpgraded", beaconRule)
	if err != nil {
		return nil, err
	}
	return &SubnetBeaconUpgradedIterator{contract: _Subnet.contract, event: "BeaconUpgraded", logs: logs, sub: sub}, nil
}

// WatchBeaconUpgraded is a free log subscription operation binding the contract event 0x1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e.
//
// Solidity: event BeaconUpgraded(address indexed beacon)
func (_Subnet *SubnetFilterer) WatchBeaconUpgraded(opts *bind.WatchOpts, sink chan<- *SubnetBeaconUpgraded, beacon []common.Address) (event.Subscription, error) {

	var beaconRule []interface{}
	for _, beaconItem := range beacon {
		beaconRule = append(beaconRule, beaconItem)
	}

	logs, sub, err := _Subnet.contract.WatchLogs(opts, "BeaconUpgraded", beaconRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SubnetBeaconUpgraded)
				if err := _Subnet.contract.UnpackLog(event, "BeaconUpgraded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseBeaconUpgraded is a log parse operation binding the contract event 0x1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e.
//
// Solidity: event BeaconUpgraded(address indexed beacon)
func (_Subnet *SubnetFilterer) ParseBeaconUpgraded(log types.Log) (*SubnetBeaconUpgraded, error) {
	event := new(SubnetBeaconUpgraded)
	if err := _Subnet.contract.UnpackLog(event, "BeaconUpgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SubnetInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the Subnet contract.
type SubnetInitializedIterator struct {
	Event *SubnetInitialized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SubnetInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SubnetInitialized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SubnetInitialized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SubnetInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SubnetInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SubnetInitialized represents a Initialized event raised by the Subnet contract.
type SubnetInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_Subnet *SubnetFilterer) FilterInitialized(opts *bind.FilterOpts) (*SubnetInitializedIterator, error) {

	logs, sub, err := _Subnet.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &SubnetInitializedIterator{contract: _Subnet.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_Subnet *SubnetFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *SubnetInitialized) (event.Subscription, error) {

	logs, sub, err := _Subnet.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SubnetInitialized)
				if err := _Subnet.contract.UnpackLog(event, "Initialized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInitialized is a log parse operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_Subnet *SubnetFilterer) ParseInitialized(log types.Log) (*SubnetInitialized, error) {
	event := new(SubnetInitialized)
	if err := _Subnet.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SubnetUpgradedIterator is returned from FilterUpgraded and is used to iterate over the raw logs and unpacked data for Upgraded events raised by the Subnet contract.
type SubnetUpgradedIterator struct {
	Event *SubnetUpgraded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SubnetUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SubnetUpgraded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SubnetUpgraded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SubnetUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SubnetUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SubnetUpgraded represents a Upgraded event raised by the Subnet contract.
type SubnetUpgraded struct {
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterUpgraded is a free log retrieval operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_Subnet *SubnetFilterer) FilterUpgraded(opts *bind.FilterOpts, implementation []common.Address) (*SubnetUpgradedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Subnet.contract.FilterLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &SubnetUpgradedIterator{contract: _Subnet.contract, event: "Upgraded", logs: logs, sub: sub}, nil
}

// WatchUpgraded is a free log subscription operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_Subnet *SubnetFilterer) WatchUpgraded(opts *bind.WatchOpts, sink chan<- *SubnetUpgraded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Subnet.contract.WatchLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SubnetUpgraded)
				if err := _Subnet.contract.UnpackLog(event, "Upgraded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUpgraded is a log parse operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_Subnet *SubnetFilterer) ParseUpgraded(log types.Log) (*SubnetUpgraded, error) {
	event := new(SubnetUpgraded)
	if err := _Subnet.contract.UnpackLog(event, "Upgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
