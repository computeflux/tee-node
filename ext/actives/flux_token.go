// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package actives

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

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
	_ = time.Tick
	_ = context.Background
)

// FluxTestMetaData contains all meta data concerning the FluxTest contract.
var FluxTestMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"previousAdmin\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newAdmin\",\"type\":\"address\"}],\"name\":\"AdminChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"beacon\",\"type\":\"address\"}],\"name\":\"BeaconUpgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nativeAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"tokenAmount\",\"type\":\"uint256\"}],\"name\":\"Distributed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"PointsBurned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newAmount\",\"type\":\"uint256\"}],\"name\":\"RewardAmountUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldTarget\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newTarget\",\"type\":\"address\"}],\"name\":\"TransferTargetUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"Upgraded\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"}],\"name\":\"allowance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"tos\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"name\":\"batchMint\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"decimals\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"subtractedValue\",\"type\":\"uint256\"}],\"name\":\"decreaseAllowance\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"addedValue\",\"type\":\"uint256\"}],\"name\":\"increaseAllowance\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"initialOwner\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"holders\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"name\":\"migrateBalances\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"proxiableUUID\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rewardAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"setRewardAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"target\",\"type\":\"address\"}],\"name\":\"setTransferTarget\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"transferTarget\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"}],\"name\":\"upgradeTo\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"upgradeToAndCall\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
}

// FluxTestABI is the input ABI used to generate the binding from.
// Deprecated: Use FluxTestMetaData.ABI instead.
var FluxTestABI = FluxTestMetaData.ABI

// FluxTest is an auto generated Go binding around an Ethereum contract.
type FluxTest struct {
	FluxTestCaller     // Read-only binding to the contract
	FluxTestTransactor // Write-only binding to the contract
	FluxTestFilterer   // Log filterer for contract events
}

// FluxTestCaller is an auto generated read-only Go binding around an Ethereum contract.
type FluxTestCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FluxTestTransactor is an auto generated write-only Go binding around an Ethereum contract.
type FluxTestTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FluxTestFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type FluxTestFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FluxTestSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type FluxTestSession struct {
	Contract     *FluxTest         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// FluxTestCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type FluxTestCallerSession struct {
	Contract *FluxTestCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// FluxTestTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type FluxTestTransactorSession struct {
	Contract     *FluxTestTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// FluxTestRaw is an auto generated low-level Go binding around an Ethereum contract.
type FluxTestRaw struct {
	Contract *FluxTest // Generic contract binding to access the raw methods on
}

// FluxTestCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type FluxTestCallerRaw struct {
	Contract *FluxTestCaller // Generic read-only contract binding to access the raw methods on
}

// FluxTestTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type FluxTestTransactorRaw struct {
	Contract *FluxTestTransactor // Generic write-only contract binding to access the raw methods on
}

// NewFluxTest creates a new instance of FluxTest, bound to a specific deployed contract.
func NewFluxTest(address common.Address, backend bind.ContractBackend) (*FluxTest, error) {
	contract, err := bindFluxTest(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &FluxTest{FluxTestCaller: FluxTestCaller{contract: contract}, FluxTestTransactor: FluxTestTransactor{contract: contract}, FluxTestFilterer: FluxTestFilterer{contract: contract}}, nil
}

// NewFluxTestCaller creates a new read-only instance of FluxTest, bound to a specific deployed contract.
func NewFluxTestCaller(address common.Address, caller bind.ContractCaller) (*FluxTestCaller, error) {
	contract, err := bindFluxTest(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FluxTestCaller{contract: contract}, nil
}

// NewFluxTestTransactor creates a new write-only instance of FluxTest, bound to a specific deployed contract.
func NewFluxTestTransactor(address common.Address, transactor bind.ContractTransactor) (*FluxTestTransactor, error) {
	contract, err := bindFluxTest(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FluxTestTransactor{contract: contract}, nil
}

// NewFluxTestFilterer creates a new log filterer instance of FluxTest, bound to a specific deployed contract.
func NewFluxTestFilterer(address common.Address, filterer bind.ContractFilterer) (*FluxTestFilterer, error) {
	contract, err := bindFluxTest(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FluxTestFilterer{contract: contract}, nil
}

// bindFluxTest binds a generic wrapper to an already deployed contract.
func bindFluxTest(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := FluxTestMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FluxTest *FluxTestRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FluxTest.Contract.FluxTestCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FluxTest *FluxTestRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FluxTest.Contract.FluxTestTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FluxTest *FluxTestRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FluxTest.Contract.FluxTestTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FluxTest *FluxTestCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FluxTest.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FluxTest *FluxTestTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FluxTest.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FluxTest *FluxTestTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FluxTest.Contract.contract.Transact(opts, method, params...)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_FluxTest *FluxTestCaller) Allowance(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "allowance", owner, spender)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_FluxTest *FluxTestSession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _FluxTest.Contract.Allowance(&_FluxTest.CallOpts, owner, spender)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_FluxTest *FluxTestCallerSession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _FluxTest.Contract.Allowance(&_FluxTest.CallOpts, owner, spender)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_FluxTest *FluxTestCaller) BalanceOf(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "balanceOf", account)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_FluxTest *FluxTestSession) BalanceOf(account common.Address) (*big.Int, error) {
	return _FluxTest.Contract.BalanceOf(&_FluxTest.CallOpts, account)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_FluxTest *FluxTestCallerSession) BalanceOf(account common.Address) (*big.Int, error) {
	return _FluxTest.Contract.BalanceOf(&_FluxTest.CallOpts, account)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() pure returns(uint8)
func (_FluxTest *FluxTestCaller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "decimals")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() pure returns(uint8)
func (_FluxTest *FluxTestSession) Decimals() (uint8, error) {
	return _FluxTest.Contract.Decimals(&_FluxTest.CallOpts)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() pure returns(uint8)
func (_FluxTest *FluxTestCallerSession) Decimals() (uint8, error) {
	return _FluxTest.Contract.Decimals(&_FluxTest.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_FluxTest *FluxTestCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_FluxTest *FluxTestSession) Name() (string, error) {
	return _FluxTest.Contract.Name(&_FluxTest.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_FluxTest *FluxTestCallerSession) Name() (string, error) {
	return _FluxTest.Contract.Name(&_FluxTest.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FluxTest *FluxTestCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FluxTest *FluxTestSession) Owner() (common.Address, error) {
	return _FluxTest.Contract.Owner(&_FluxTest.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FluxTest *FluxTestCallerSession) Owner() (common.Address, error) {
	return _FluxTest.Contract.Owner(&_FluxTest.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_FluxTest *FluxTestCaller) ProxiableUUID(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "proxiableUUID")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_FluxTest *FluxTestSession) ProxiableUUID() ([32]byte, error) {
	return _FluxTest.Contract.ProxiableUUID(&_FluxTest.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_FluxTest *FluxTestCallerSession) ProxiableUUID() ([32]byte, error) {
	return _FluxTest.Contract.ProxiableUUID(&_FluxTest.CallOpts)
}

// RewardAmount is a free data retrieval call binding the contract method 0xf7b2a7be.
//
// Solidity: function rewardAmount() view returns(uint256)
func (_FluxTest *FluxTestCaller) RewardAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "rewardAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RewardAmount is a free data retrieval call binding the contract method 0xf7b2a7be.
//
// Solidity: function rewardAmount() view returns(uint256)
func (_FluxTest *FluxTestSession) RewardAmount() (*big.Int, error) {
	return _FluxTest.Contract.RewardAmount(&_FluxTest.CallOpts)
}

// RewardAmount is a free data retrieval call binding the contract method 0xf7b2a7be.
//
// Solidity: function rewardAmount() view returns(uint256)
func (_FluxTest *FluxTestCallerSession) RewardAmount() (*big.Int, error) {
	return _FluxTest.Contract.RewardAmount(&_FluxTest.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_FluxTest *FluxTestCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_FluxTest *FluxTestSession) Symbol() (string, error) {
	return _FluxTest.Contract.Symbol(&_FluxTest.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_FluxTest *FluxTestCallerSession) Symbol() (string, error) {
	return _FluxTest.Contract.Symbol(&_FluxTest.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_FluxTest *FluxTestCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_FluxTest *FluxTestSession) TotalSupply() (*big.Int, error) {
	return _FluxTest.Contract.TotalSupply(&_FluxTest.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_FluxTest *FluxTestCallerSession) TotalSupply() (*big.Int, error) {
	return _FluxTest.Contract.TotalSupply(&_FluxTest.CallOpts)
}

// TransferTarget is a free data retrieval call binding the contract method 0x41ae9b58.
//
// Solidity: function transferTarget() view returns(address)
func (_FluxTest *FluxTestCaller) TransferTarget(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FluxTest.contract.Call(opts, &out, "transferTarget")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TransferTarget is a free data retrieval call binding the contract method 0x41ae9b58.
//
// Solidity: function transferTarget() view returns(address)
func (_FluxTest *FluxTestSession) TransferTarget() (common.Address, error) {
	return _FluxTest.Contract.TransferTarget(&_FluxTest.CallOpts)
}

// TransferTarget is a free data retrieval call binding the contract method 0x41ae9b58.
//
// Solidity: function transferTarget() view returns(address)
func (_FluxTest *FluxTestCallerSession) TransferTarget() (common.Address, error) {
	return _FluxTest.Contract.TransferTarget(&_FluxTest.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 amount) returns(bool)
func (_FluxTest *FluxTestTransactor) Approve(opts *bind.TransactOpts, spender common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "approve", spender, amount)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 amount) returns(bool)
func (_FluxTest *FluxTestSession) Approve(spender common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.Approve(&_FluxTest.TransactOpts, spender, amount)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 amount) returns(bool)
func (_FluxTest *FluxTestTransactorSession) Approve(spender common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.Approve(&_FluxTest.TransactOpts, spender, amount)
}

// BatchMint is a paid mutator transaction binding the contract method 0x68573107.
//
// Solidity: function batchMint(address[] tos, uint256[] amounts) returns()
func (_FluxTest *FluxTestTransactor) BatchMint(opts *bind.TransactOpts, tos []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "batchMint", tos, amounts)
}

// BatchMint is a paid mutator transaction binding the contract method 0x68573107.
//
// Solidity: function batchMint(address[] tos, uint256[] amounts) returns()
func (_FluxTest *FluxTestSession) BatchMint(tos []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.BatchMint(&_FluxTest.TransactOpts, tos, amounts)
}

// BatchMint is a paid mutator transaction binding the contract method 0x68573107.
//
// Solidity: function batchMint(address[] tos, uint256[] amounts) returns()
func (_FluxTest *FluxTestTransactorSession) BatchMint(tos []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.BatchMint(&_FluxTest.TransactOpts, tos, amounts)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address user, uint256 amount) returns()
func (_FluxTest *FluxTestTransactor) Burn(opts *bind.TransactOpts, user common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "burn", user, amount)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address user, uint256 amount) returns()
func (_FluxTest *FluxTestSession) Burn(user common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.Burn(&_FluxTest.TransactOpts, user, amount)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address user, uint256 amount) returns()
func (_FluxTest *FluxTestTransactorSession) Burn(user common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.Burn(&_FluxTest.TransactOpts, user, amount)
}

// DecreaseAllowance is a paid mutator transaction binding the contract method 0xa457c2d7.
//
// Solidity: function decreaseAllowance(address spender, uint256 subtractedValue) returns(bool)
func (_FluxTest *FluxTestTransactor) DecreaseAllowance(opts *bind.TransactOpts, spender common.Address, subtractedValue *big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "decreaseAllowance", spender, subtractedValue)
}

// DecreaseAllowance is a paid mutator transaction binding the contract method 0xa457c2d7.
//
// Solidity: function decreaseAllowance(address spender, uint256 subtractedValue) returns(bool)
func (_FluxTest *FluxTestSession) DecreaseAllowance(spender common.Address, subtractedValue *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.DecreaseAllowance(&_FluxTest.TransactOpts, spender, subtractedValue)
}

// DecreaseAllowance is a paid mutator transaction binding the contract method 0xa457c2d7.
//
// Solidity: function decreaseAllowance(address spender, uint256 subtractedValue) returns(bool)
func (_FluxTest *FluxTestTransactorSession) DecreaseAllowance(spender common.Address, subtractedValue *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.DecreaseAllowance(&_FluxTest.TransactOpts, spender, subtractedValue)
}

// IncreaseAllowance is a paid mutator transaction binding the contract method 0x39509351.
//
// Solidity: function increaseAllowance(address spender, uint256 addedValue) returns(bool)
func (_FluxTest *FluxTestTransactor) IncreaseAllowance(opts *bind.TransactOpts, spender common.Address, addedValue *big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "increaseAllowance", spender, addedValue)
}

// IncreaseAllowance is a paid mutator transaction binding the contract method 0x39509351.
//
// Solidity: function increaseAllowance(address spender, uint256 addedValue) returns(bool)
func (_FluxTest *FluxTestSession) IncreaseAllowance(spender common.Address, addedValue *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.IncreaseAllowance(&_FluxTest.TransactOpts, spender, addedValue)
}

// IncreaseAllowance is a paid mutator transaction binding the contract method 0x39509351.
//
// Solidity: function increaseAllowance(address spender, uint256 addedValue) returns(bool)
func (_FluxTest *FluxTestTransactorSession) IncreaseAllowance(spender common.Address, addedValue *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.IncreaseAllowance(&_FluxTest.TransactOpts, spender, addedValue)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address initialOwner) returns()
func (_FluxTest *FluxTestTransactor) Initialize(opts *bind.TransactOpts, initialOwner common.Address) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "initialize", initialOwner)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address initialOwner) returns()
func (_FluxTest *FluxTestSession) Initialize(initialOwner common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.Initialize(&_FluxTest.TransactOpts, initialOwner)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address initialOwner) returns()
func (_FluxTest *FluxTestTransactorSession) Initialize(initialOwner common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.Initialize(&_FluxTest.TransactOpts, initialOwner)
}

// MigrateBalances is a paid mutator transaction binding the contract method 0x01e625fc.
//
// Solidity: function migrateBalances(address[] holders, uint256[] amounts) returns()
func (_FluxTest *FluxTestTransactor) MigrateBalances(opts *bind.TransactOpts, holders []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "migrateBalances", holders, amounts)
}

// MigrateBalances is a paid mutator transaction binding the contract method 0x01e625fc.
//
// Solidity: function migrateBalances(address[] holders, uint256[] amounts) returns()
func (_FluxTest *FluxTestSession) MigrateBalances(holders []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.MigrateBalances(&_FluxTest.TransactOpts, holders, amounts)
}

// MigrateBalances is a paid mutator transaction binding the contract method 0x01e625fc.
//
// Solidity: function migrateBalances(address[] holders, uint256[] amounts) returns()
func (_FluxTest *FluxTestTransactorSession) MigrateBalances(holders []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.MigrateBalances(&_FluxTest.TransactOpts, holders, amounts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_FluxTest *FluxTestTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_FluxTest *FluxTestSession) RenounceOwnership() (*types.Transaction, error) {
	return _FluxTest.Contract.RenounceOwnership(&_FluxTest.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_FluxTest *FluxTestTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _FluxTest.Contract.RenounceOwnership(&_FluxTest.TransactOpts)
}

// SetRewardAmount is a paid mutator transaction binding the contract method 0xa8a65a78.
//
// Solidity: function setRewardAmount(uint256 amount) returns()
func (_FluxTest *FluxTestTransactor) SetRewardAmount(opts *bind.TransactOpts, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "setRewardAmount", amount)
}

// SetRewardAmount is a paid mutator transaction binding the contract method 0xa8a65a78.
//
// Solidity: function setRewardAmount(uint256 amount) returns()
func (_FluxTest *FluxTestSession) SetRewardAmount(amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.SetRewardAmount(&_FluxTest.TransactOpts, amount)
}

// SetRewardAmount is a paid mutator transaction binding the contract method 0xa8a65a78.
//
// Solidity: function setRewardAmount(uint256 amount) returns()
func (_FluxTest *FluxTestTransactorSession) SetRewardAmount(amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.SetRewardAmount(&_FluxTest.TransactOpts, amount)
}

// SetTransferTarget is a paid mutator transaction binding the contract method 0xa2e1b510.
//
// Solidity: function setTransferTarget(address target) returns()
func (_FluxTest *FluxTestTransactor) SetTransferTarget(opts *bind.TransactOpts, target common.Address) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "setTransferTarget", target)
}

// SetTransferTarget is a paid mutator transaction binding the contract method 0xa2e1b510.
//
// Solidity: function setTransferTarget(address target) returns()
func (_FluxTest *FluxTestSession) SetTransferTarget(target common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.SetTransferTarget(&_FluxTest.TransactOpts, target)
}

// SetTransferTarget is a paid mutator transaction binding the contract method 0xa2e1b510.
//
// Solidity: function setTransferTarget(address target) returns()
func (_FluxTest *FluxTestTransactorSession) SetTransferTarget(target common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.SetTransferTarget(&_FluxTest.TransactOpts, target)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 amount) returns(bool)
func (_FluxTest *FluxTestTransactor) Transfer(opts *bind.TransactOpts, to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "transfer", to, amount)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 amount) returns(bool)
func (_FluxTest *FluxTestSession) Transfer(to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.Transfer(&_FluxTest.TransactOpts, to, amount)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 amount) returns(bool)
func (_FluxTest *FluxTestTransactorSession) Transfer(to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.Transfer(&_FluxTest.TransactOpts, to, amount)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 amount) returns(bool)
func (_FluxTest *FluxTestTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "transferFrom", from, to, amount)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 amount) returns(bool)
func (_FluxTest *FluxTestSession) TransferFrom(from common.Address, to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.TransferFrom(&_FluxTest.TransactOpts, from, to, amount)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 amount) returns(bool)
func (_FluxTest *FluxTestTransactorSession) TransferFrom(from common.Address, to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _FluxTest.Contract.TransferFrom(&_FluxTest.TransactOpts, from, to, amount)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_FluxTest *FluxTestTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_FluxTest *FluxTestSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.TransferOwnership(&_FluxTest.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_FluxTest *FluxTestTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.TransferOwnership(&_FluxTest.TransactOpts, newOwner)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_FluxTest *FluxTestTransactor) UpgradeTo(opts *bind.TransactOpts, newImplementation common.Address) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "upgradeTo", newImplementation)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_FluxTest *FluxTestSession) UpgradeTo(newImplementation common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.UpgradeTo(&_FluxTest.TransactOpts, newImplementation)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_FluxTest *FluxTestTransactorSession) UpgradeTo(newImplementation common.Address) (*types.Transaction, error) {
	return _FluxTest.Contract.UpgradeTo(&_FluxTest.TransactOpts, newImplementation)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_FluxTest *FluxTestTransactor) UpgradeToAndCall(opts *bind.TransactOpts, newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _FluxTest.contract.Transact(opts, "upgradeToAndCall", newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_FluxTest *FluxTestSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _FluxTest.Contract.UpgradeToAndCall(&_FluxTest.TransactOpts, newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_FluxTest *FluxTestTransactorSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _FluxTest.Contract.UpgradeToAndCall(&_FluxTest.TransactOpts, newImplementation, data)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_FluxTest *FluxTestTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FluxTest.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_FluxTest *FluxTestSession) Receive() (*types.Transaction, error) {
	return _FluxTest.Contract.Receive(&_FluxTest.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_FluxTest *FluxTestTransactorSession) Receive() (*types.Transaction, error) {
	return _FluxTest.Contract.Receive(&_FluxTest.TransactOpts)
}

// FluxTestAdminChangedIterator is returned from FilterAdminChanged and is used to iterate over the raw logs and unpacked data for AdminChanged events raised by the FluxTest contract.
type FluxTestAdminChangedIterator struct {
	Event *FluxTestAdminChanged // Event containing the contract specifics and raw log

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
func (it *FluxTestAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestAdminChanged)
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
		it.Event = new(FluxTestAdminChanged)
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
func (it *FluxTestAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestAdminChanged represents a AdminChanged event raised by the FluxTest contract.
type FluxTestAdminChanged struct {
	PreviousAdmin common.Address
	NewAdmin      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterAdminChanged is a free log retrieval operation binding the contract event 0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f.
//
// Solidity: event AdminChanged(address previousAdmin, address newAdmin)
func (_FluxTest *FluxTestFilterer) FilterAdminChanged(opts *bind.FilterOpts) (*FluxTestAdminChangedIterator, error) {

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "AdminChanged")
	if err != nil {
		return nil, err
	}
	return &FluxTestAdminChangedIterator{contract: _FluxTest.contract, event: "AdminChanged", logs: logs, sub: sub}, nil
}

// WatchAdminChanged is a free log subscription operation binding the contract event 0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f.
//
// Solidity: event AdminChanged(address previousAdmin, address newAdmin)
func (_FluxTest *FluxTestFilterer) WatchAdminChanged(opts *bind.WatchOpts, sink chan<- *FluxTestAdminChanged) (event.Subscription, error) {

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "AdminChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestAdminChanged)
				if err := _FluxTest.contract.UnpackLog(event, "AdminChanged", log); err != nil {
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
func (_FluxTest *FluxTestFilterer) ParseAdminChanged(log types.Log) (*FluxTestAdminChanged, error) {
	event := new(FluxTestAdminChanged)
	if err := _FluxTest.contract.UnpackLog(event, "AdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the FluxTest contract.
type FluxTestApprovalIterator struct {
	Event *FluxTestApproval // Event containing the contract specifics and raw log

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
func (it *FluxTestApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestApproval)
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
		it.Event = new(FluxTestApproval)
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
func (it *FluxTestApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestApproval represents a Approval event raised by the FluxTest contract.
type FluxTestApproval struct {
	Owner   common.Address
	Spender common.Address
	Value   *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_FluxTest *FluxTestFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, spender []common.Address) (*FluxTestApprovalIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestApprovalIterator{contract: _FluxTest.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_FluxTest *FluxTestFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *FluxTestApproval, owner []common.Address, spender []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestApproval)
				if err := _FluxTest.contract.UnpackLog(event, "Approval", log); err != nil {
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

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_FluxTest *FluxTestFilterer) ParseApproval(log types.Log) (*FluxTestApproval, error) {
	event := new(FluxTestApproval)
	if err := _FluxTest.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestBeaconUpgradedIterator is returned from FilterBeaconUpgraded and is used to iterate over the raw logs and unpacked data for BeaconUpgraded events raised by the FluxTest contract.
type FluxTestBeaconUpgradedIterator struct {
	Event *FluxTestBeaconUpgraded // Event containing the contract specifics and raw log

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
func (it *FluxTestBeaconUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestBeaconUpgraded)
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
		it.Event = new(FluxTestBeaconUpgraded)
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
func (it *FluxTestBeaconUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestBeaconUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestBeaconUpgraded represents a BeaconUpgraded event raised by the FluxTest contract.
type FluxTestBeaconUpgraded struct {
	Beacon common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterBeaconUpgraded is a free log retrieval operation binding the contract event 0x1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e.
//
// Solidity: event BeaconUpgraded(address indexed beacon)
func (_FluxTest *FluxTestFilterer) FilterBeaconUpgraded(opts *bind.FilterOpts, beacon []common.Address) (*FluxTestBeaconUpgradedIterator, error) {

	var beaconRule []interface{}
	for _, beaconItem := range beacon {
		beaconRule = append(beaconRule, beaconItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "BeaconUpgraded", beaconRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestBeaconUpgradedIterator{contract: _FluxTest.contract, event: "BeaconUpgraded", logs: logs, sub: sub}, nil
}

// WatchBeaconUpgraded is a free log subscription operation binding the contract event 0x1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e.
//
// Solidity: event BeaconUpgraded(address indexed beacon)
func (_FluxTest *FluxTestFilterer) WatchBeaconUpgraded(opts *bind.WatchOpts, sink chan<- *FluxTestBeaconUpgraded, beacon []common.Address) (event.Subscription, error) {

	var beaconRule []interface{}
	for _, beaconItem := range beacon {
		beaconRule = append(beaconRule, beaconItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "BeaconUpgraded", beaconRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestBeaconUpgraded)
				if err := _FluxTest.contract.UnpackLog(event, "BeaconUpgraded", log); err != nil {
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
func (_FluxTest *FluxTestFilterer) ParseBeaconUpgraded(log types.Log) (*FluxTestBeaconUpgraded, error) {
	event := new(FluxTestBeaconUpgraded)
	if err := _FluxTest.contract.UnpackLog(event, "BeaconUpgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestDistributedIterator is returned from FilterDistributed and is used to iterate over the raw logs and unpacked data for Distributed events raised by the FluxTest contract.
type FluxTestDistributedIterator struct {
	Event *FluxTestDistributed // Event containing the contract specifics and raw log

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
func (it *FluxTestDistributedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestDistributed)
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
		it.Event = new(FluxTestDistributed)
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
func (it *FluxTestDistributedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestDistributedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestDistributed represents a Distributed event raised by the FluxTest contract.
type FluxTestDistributed struct {
	To           common.Address
	NativeAmount *big.Int
	TokenAmount  *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterDistributed is a free log retrieval operation binding the contract event 0x60ce3cc2d133631eac66a476f14997a9fa682bd05a60dd993cf02285822d78d8.
//
// Solidity: event Distributed(address indexed to, uint256 nativeAmount, uint256 tokenAmount)
func (_FluxTest *FluxTestFilterer) FilterDistributed(opts *bind.FilterOpts, to []common.Address) (*FluxTestDistributedIterator, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "Distributed", toRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestDistributedIterator{contract: _FluxTest.contract, event: "Distributed", logs: logs, sub: sub}, nil
}

// WatchDistributed is a free log subscription operation binding the contract event 0x60ce3cc2d133631eac66a476f14997a9fa682bd05a60dd993cf02285822d78d8.
//
// Solidity: event Distributed(address indexed to, uint256 nativeAmount, uint256 tokenAmount)
func (_FluxTest *FluxTestFilterer) WatchDistributed(opts *bind.WatchOpts, sink chan<- *FluxTestDistributed, to []common.Address) (event.Subscription, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "Distributed", toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestDistributed)
				if err := _FluxTest.contract.UnpackLog(event, "Distributed", log); err != nil {
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

// ParseDistributed is a log parse operation binding the contract event 0x60ce3cc2d133631eac66a476f14997a9fa682bd05a60dd993cf02285822d78d8.
//
// Solidity: event Distributed(address indexed to, uint256 nativeAmount, uint256 tokenAmount)
func (_FluxTest *FluxTestFilterer) ParseDistributed(log types.Log) (*FluxTestDistributed, error) {
	event := new(FluxTestDistributed)
	if err := _FluxTest.contract.UnpackLog(event, "Distributed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the FluxTest contract.
type FluxTestInitializedIterator struct {
	Event *FluxTestInitialized // Event containing the contract specifics and raw log

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
func (it *FluxTestInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestInitialized)
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
		it.Event = new(FluxTestInitialized)
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
func (it *FluxTestInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestInitialized represents a Initialized event raised by the FluxTest contract.
type FluxTestInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FluxTest *FluxTestFilterer) FilterInitialized(opts *bind.FilterOpts) (*FluxTestInitializedIterator, error) {

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &FluxTestInitializedIterator{contract: _FluxTest.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FluxTest *FluxTestFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *FluxTestInitialized) (event.Subscription, error) {

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestInitialized)
				if err := _FluxTest.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_FluxTest *FluxTestFilterer) ParseInitialized(log types.Log) (*FluxTestInitialized, error) {
	event := new(FluxTestInitialized)
	if err := _FluxTest.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the FluxTest contract.
type FluxTestOwnershipTransferredIterator struct {
	Event *FluxTestOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *FluxTestOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestOwnershipTransferred)
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
		it.Event = new(FluxTestOwnershipTransferred)
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
func (it *FluxTestOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestOwnershipTransferred represents a OwnershipTransferred event raised by the FluxTest contract.
type FluxTestOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_FluxTest *FluxTestFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*FluxTestOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestOwnershipTransferredIterator{contract: _FluxTest.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_FluxTest *FluxTestFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *FluxTestOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestOwnershipTransferred)
				if err := _FluxTest.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_FluxTest *FluxTestFilterer) ParseOwnershipTransferred(log types.Log) (*FluxTestOwnershipTransferred, error) {
	event := new(FluxTestOwnershipTransferred)
	if err := _FluxTest.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestPointsBurnedIterator is returned from FilterPointsBurned and is used to iterate over the raw logs and unpacked data for PointsBurned events raised by the FluxTest contract.
type FluxTestPointsBurnedIterator struct {
	Event *FluxTestPointsBurned // Event containing the contract specifics and raw log

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
func (it *FluxTestPointsBurnedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestPointsBurned)
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
		it.Event = new(FluxTestPointsBurned)
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
func (it *FluxTestPointsBurnedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestPointsBurnedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestPointsBurned represents a PointsBurned event raised by the FluxTest contract.
type FluxTestPointsBurned struct {
	User   common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterPointsBurned is a free log retrieval operation binding the contract event 0xb0f2e2c9b12547a7425feac9e52f454451705c31972086a5a0e563fc5273f709.
//
// Solidity: event PointsBurned(address indexed user, uint256 amount)
func (_FluxTest *FluxTestFilterer) FilterPointsBurned(opts *bind.FilterOpts, user []common.Address) (*FluxTestPointsBurnedIterator, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "PointsBurned", userRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestPointsBurnedIterator{contract: _FluxTest.contract, event: "PointsBurned", logs: logs, sub: sub}, nil
}

// WatchPointsBurned is a free log subscription operation binding the contract event 0xb0f2e2c9b12547a7425feac9e52f454451705c31972086a5a0e563fc5273f709.
//
// Solidity: event PointsBurned(address indexed user, uint256 amount)
func (_FluxTest *FluxTestFilterer) WatchPointsBurned(opts *bind.WatchOpts, sink chan<- *FluxTestPointsBurned, user []common.Address) (event.Subscription, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "PointsBurned", userRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestPointsBurned)
				if err := _FluxTest.contract.UnpackLog(event, "PointsBurned", log); err != nil {
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

// ParsePointsBurned is a log parse operation binding the contract event 0xb0f2e2c9b12547a7425feac9e52f454451705c31972086a5a0e563fc5273f709.
//
// Solidity: event PointsBurned(address indexed user, uint256 amount)
func (_FluxTest *FluxTestFilterer) ParsePointsBurned(log types.Log) (*FluxTestPointsBurned, error) {
	event := new(FluxTestPointsBurned)
	if err := _FluxTest.contract.UnpackLog(event, "PointsBurned", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestRewardAmountUpdatedIterator is returned from FilterRewardAmountUpdated and is used to iterate over the raw logs and unpacked data for RewardAmountUpdated events raised by the FluxTest contract.
type FluxTestRewardAmountUpdatedIterator struct {
	Event *FluxTestRewardAmountUpdated // Event containing the contract specifics and raw log

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
func (it *FluxTestRewardAmountUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestRewardAmountUpdated)
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
		it.Event = new(FluxTestRewardAmountUpdated)
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
func (it *FluxTestRewardAmountUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestRewardAmountUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestRewardAmountUpdated represents a RewardAmountUpdated event raised by the FluxTest contract.
type FluxTestRewardAmountUpdated struct {
	OldAmount *big.Int
	NewAmount *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterRewardAmountUpdated is a free log retrieval operation binding the contract event 0xf0d37c3ae852021ac329281f604b658691cbfa6b9e9c22909f06b64a8ce87c94.
//
// Solidity: event RewardAmountUpdated(uint256 oldAmount, uint256 newAmount)
func (_FluxTest *FluxTestFilterer) FilterRewardAmountUpdated(opts *bind.FilterOpts) (*FluxTestRewardAmountUpdatedIterator, error) {

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "RewardAmountUpdated")
	if err != nil {
		return nil, err
	}
	return &FluxTestRewardAmountUpdatedIterator{contract: _FluxTest.contract, event: "RewardAmountUpdated", logs: logs, sub: sub}, nil
}

// WatchRewardAmountUpdated is a free log subscription operation binding the contract event 0xf0d37c3ae852021ac329281f604b658691cbfa6b9e9c22909f06b64a8ce87c94.
//
// Solidity: event RewardAmountUpdated(uint256 oldAmount, uint256 newAmount)
func (_FluxTest *FluxTestFilterer) WatchRewardAmountUpdated(opts *bind.WatchOpts, sink chan<- *FluxTestRewardAmountUpdated) (event.Subscription, error) {

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "RewardAmountUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestRewardAmountUpdated)
				if err := _FluxTest.contract.UnpackLog(event, "RewardAmountUpdated", log); err != nil {
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

// ParseRewardAmountUpdated is a log parse operation binding the contract event 0xf0d37c3ae852021ac329281f604b658691cbfa6b9e9c22909f06b64a8ce87c94.
//
// Solidity: event RewardAmountUpdated(uint256 oldAmount, uint256 newAmount)
func (_FluxTest *FluxTestFilterer) ParseRewardAmountUpdated(log types.Log) (*FluxTestRewardAmountUpdated, error) {
	event := new(FluxTestRewardAmountUpdated)
	if err := _FluxTest.contract.UnpackLog(event, "RewardAmountUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the FluxTest contract.
type FluxTestTransferIterator struct {
	Event *FluxTestTransfer // Event containing the contract specifics and raw log

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
func (it *FluxTestTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestTransfer)
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
		it.Event = new(FluxTestTransfer)
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
func (it *FluxTestTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestTransfer represents a Transfer event raised by the FluxTest contract.
type FluxTestTransfer struct {
	From  common.Address
	To    common.Address
	Value *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_FluxTest *FluxTestFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address) (*FluxTestTransferIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestTransferIterator{contract: _FluxTest.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_FluxTest *FluxTestFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *FluxTestTransfer, from []common.Address, to []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestTransfer)
				if err := _FluxTest.contract.UnpackLog(event, "Transfer", log); err != nil {
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

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_FluxTest *FluxTestFilterer) ParseTransfer(log types.Log) (*FluxTestTransfer, error) {
	event := new(FluxTestTransfer)
	if err := _FluxTest.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestTransferTargetUpdatedIterator is returned from FilterTransferTargetUpdated and is used to iterate over the raw logs and unpacked data for TransferTargetUpdated events raised by the FluxTest contract.
type FluxTestTransferTargetUpdatedIterator struct {
	Event *FluxTestTransferTargetUpdated // Event containing the contract specifics and raw log

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
func (it *FluxTestTransferTargetUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestTransferTargetUpdated)
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
		it.Event = new(FluxTestTransferTargetUpdated)
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
func (it *FluxTestTransferTargetUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestTransferTargetUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestTransferTargetUpdated represents a TransferTargetUpdated event raised by the FluxTest contract.
type FluxTestTransferTargetUpdated struct {
	OldTarget common.Address
	NewTarget common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterTransferTargetUpdated is a free log retrieval operation binding the contract event 0xf8d6a9615a46aca215c2eab76ca7e2395b105eae9c5cebde227c82ce6097d849.
//
// Solidity: event TransferTargetUpdated(address indexed oldTarget, address indexed newTarget)
func (_FluxTest *FluxTestFilterer) FilterTransferTargetUpdated(opts *bind.FilterOpts, oldTarget []common.Address, newTarget []common.Address) (*FluxTestTransferTargetUpdatedIterator, error) {

	var oldTargetRule []interface{}
	for _, oldTargetItem := range oldTarget {
		oldTargetRule = append(oldTargetRule, oldTargetItem)
	}
	var newTargetRule []interface{}
	for _, newTargetItem := range newTarget {
		newTargetRule = append(newTargetRule, newTargetItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "TransferTargetUpdated", oldTargetRule, newTargetRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestTransferTargetUpdatedIterator{contract: _FluxTest.contract, event: "TransferTargetUpdated", logs: logs, sub: sub}, nil
}

// WatchTransferTargetUpdated is a free log subscription operation binding the contract event 0xf8d6a9615a46aca215c2eab76ca7e2395b105eae9c5cebde227c82ce6097d849.
//
// Solidity: event TransferTargetUpdated(address indexed oldTarget, address indexed newTarget)
func (_FluxTest *FluxTestFilterer) WatchTransferTargetUpdated(opts *bind.WatchOpts, sink chan<- *FluxTestTransferTargetUpdated, oldTarget []common.Address, newTarget []common.Address) (event.Subscription, error) {

	var oldTargetRule []interface{}
	for _, oldTargetItem := range oldTarget {
		oldTargetRule = append(oldTargetRule, oldTargetItem)
	}
	var newTargetRule []interface{}
	for _, newTargetItem := range newTarget {
		newTargetRule = append(newTargetRule, newTargetItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "TransferTargetUpdated", oldTargetRule, newTargetRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestTransferTargetUpdated)
				if err := _FluxTest.contract.UnpackLog(event, "TransferTargetUpdated", log); err != nil {
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

// ParseTransferTargetUpdated is a log parse operation binding the contract event 0xf8d6a9615a46aca215c2eab76ca7e2395b105eae9c5cebde227c82ce6097d849.
//
// Solidity: event TransferTargetUpdated(address indexed oldTarget, address indexed newTarget)
func (_FluxTest *FluxTestFilterer) ParseTransferTargetUpdated(log types.Log) (*FluxTestTransferTargetUpdated, error) {
	event := new(FluxTestTransferTargetUpdated)
	if err := _FluxTest.contract.UnpackLog(event, "TransferTargetUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FluxTestUpgradedIterator is returned from FilterUpgraded and is used to iterate over the raw logs and unpacked data for Upgraded events raised by the FluxTest contract.
type FluxTestUpgradedIterator struct {
	Event *FluxTestUpgraded // Event containing the contract specifics and raw log

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
func (it *FluxTestUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FluxTestUpgraded)
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
		it.Event = new(FluxTestUpgraded)
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
func (it *FluxTestUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FluxTestUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FluxTestUpgraded represents a Upgraded event raised by the FluxTest contract.
type FluxTestUpgraded struct {
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterUpgraded is a free log retrieval operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_FluxTest *FluxTestFilterer) FilterUpgraded(opts *bind.FilterOpts, implementation []common.Address) (*FluxTestUpgradedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _FluxTest.contract.FilterLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &FluxTestUpgradedIterator{contract: _FluxTest.contract, event: "Upgraded", logs: logs, sub: sub}, nil
}

// WatchUpgraded is a free log subscription operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_FluxTest *FluxTestFilterer) WatchUpgraded(opts *bind.WatchOpts, sink chan<- *FluxTestUpgraded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _FluxTest.contract.WatchLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FluxTestUpgraded)
				if err := _FluxTest.contract.UnpackLog(event, "Upgraded", log); err != nil {
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
func (_FluxTest *FluxTestFilterer) ParseUpgraded(log types.Log) (*FluxTestUpgraded, error) {
	event := new(FluxTestUpgraded)
	if err := _FluxTest.contract.UnpackLog(event, "Upgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
