// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

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

// TokenMetaData contains all meta data concerning the Token contract.
var TokenMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"previousAdmin\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newAdmin\",\"type\":\"address\"}],\"name\":\"AdminChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"beacon\",\"type\":\"address\"}],\"name\":\"BeaconUpgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"active\",\"type\":\"bool\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"rate\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unit\",\"type\":\"uint256\"}],\"name\":\"ERC20TokenSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"active\",\"type\":\"bool\"}],\"name\":\"NativeActiveSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldRate\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newRate\",\"type\":\"uint256\"}],\"name\":\"RateUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"rate\",\"type\":\"uint256\"}],\"name\":\"RechargeFeeRateSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"points\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"}],\"name\":\"Recharged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldSubnet\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newSubnet\",\"type\":\"address\"}],\"name\":\"SubnetSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldUnit\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newUnit\",\"type\":\"uint256\"}],\"name\":\"TokenUnitUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"Upgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"rate\",\"type\":\"uint256\"}],\"name\":\"WithdrawFeeRateSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"}],\"name\":\"WithdrawalCancelled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"}],\"name\":\"WithdrawalDone\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"}],\"name\":\"Withdrawn\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"ERC20_MIN_POINTS\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WITHDRAWAL_STATUS_CANCELLED\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WITHDRAWAL_STATUS_DONE\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WITHDRAWAL_STATUS_PENDING\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"}],\"name\":\"cancelWithdrawal\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"}],\"name\":\"claimWithdrawal\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"emergencyWithdraw\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"}],\"name\":\"emergencyWithdrawErc20\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"erc20TokenList\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"erc20Tokens\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"active\",\"type\":\"bool\"},{\"internalType\":\"uint256\",\"name\":\"rate\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"unit\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"}],\"name\":\"getERC20Config\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getERC20Count\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getERC20List\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"tokens\",\"type\":\"address[]\"},{\"internalType\":\"bool[]\",\"name\":\"actives\",\"type\":\"bool[]\"},{\"internalType\":\"uint256[]\",\"name\":\"rates\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"units\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"}],\"name\":\"getPendingWithdrawal\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nativeActive\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"pendingWithdrawals\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"status\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"proxiableUUID\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rate\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"recharge\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"rechargeErc20\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rechargeFeeRate\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"active\",\"type\":\"bool\"},{\"internalType\":\"uint256\",\"name\":\"tokenRate\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"unit\",\"type\":\"uint256\"}],\"name\":\"setERC20Token\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bool\",\"name\":\"active\",\"type\":\"bool\"}],\"name\":\"setNativeActive\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newRate\",\"type\":\"uint256\"}],\"name\":\"setRate\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newRate\",\"type\":\"uint256\"}],\"name\":\"setRechargeFeeRate\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_subnet\",\"type\":\"address\"}],\"name\":\"setSubnet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"unit\",\"type\":\"uint256\"}],\"name\":\"setTokenUnit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newRate\",\"type\":\"uint256\"}],\"name\":\"setWithdrawFeeRate\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"subnet\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"toPoints\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"points\",\"type\":\"uint256\"}],\"name\":\"toValue\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tokenUnit\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"}],\"name\":\"upgradeTo\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"upgradeToAndCall\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"points\",\"type\":\"uint256\"},{\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"}],\"name\":\"withdraw\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"points\",\"type\":\"uint256\"},{\"internalType\":\"uint64\",\"name\":\"nonce\",\"type\":\"uint64\"}],\"name\":\"withdrawErc20\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"withdrawFeeRate\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
}

// TokenABI is the input ABI used to generate the binding from.
// Deprecated: Use TokenMetaData.ABI instead.
var TokenABI = TokenMetaData.ABI

// Token is an auto generated Go binding around an Ethereum contract.
type Token struct {
	TokenCaller     // Read-only binding to the contract
	TokenTransactor // Write-only binding to the contract
	TokenFilterer   // Log filterer for contract events
}

// TokenCaller is an auto generated read-only Go binding around an Ethereum contract.
type TokenCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TokenTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TokenFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TokenSession struct {
	Contract     *Token            // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TokenCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TokenCallerSession struct {
	Contract *TokenCaller  // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// TokenTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TokenTransactorSession struct {
	Contract     *TokenTransactor  // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TokenRaw is an auto generated low-level Go binding around an Ethereum contract.
type TokenRaw struct {
	Contract *Token // Generic contract binding to access the raw methods on
}

// TokenCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TokenCallerRaw struct {
	Contract *TokenCaller // Generic read-only contract binding to access the raw methods on
}

// TokenTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TokenTransactorRaw struct {
	Contract *TokenTransactor // Generic write-only contract binding to access the raw methods on
}

// NewToken creates a new instance of Token, bound to a specific deployed contract.
func NewToken(address common.Address, backend bind.ContractBackend) (*Token, error) {
	contract, err := bindToken(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Token{TokenCaller: TokenCaller{contract: contract}, TokenTransactor: TokenTransactor{contract: contract}, TokenFilterer: TokenFilterer{contract: contract}}, nil
}

// NewTokenCaller creates a new read-only instance of Token, bound to a specific deployed contract.
func NewTokenCaller(address common.Address, caller bind.ContractCaller) (*TokenCaller, error) {
	contract, err := bindToken(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TokenCaller{contract: contract}, nil
}

// NewTokenTransactor creates a new write-only instance of Token, bound to a specific deployed contract.
func NewTokenTransactor(address common.Address, transactor bind.ContractTransactor) (*TokenTransactor, error) {
	contract, err := bindToken(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TokenTransactor{contract: contract}, nil
}

// NewTokenFilterer creates a new log filterer instance of Token, bound to a specific deployed contract.
func NewTokenFilterer(address common.Address, filterer bind.ContractFilterer) (*TokenFilterer, error) {
	contract, err := bindToken(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TokenFilterer{contract: contract}, nil
}

// bindToken binds a generic wrapper to an already deployed contract.
func bindToken(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := TokenMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Token *TokenRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Token.Contract.TokenCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Token *TokenRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Token.Contract.TokenTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Token *TokenRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Token.Contract.TokenTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Token *TokenCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Token.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Token *TokenTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Token.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Token *TokenTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Token.Contract.contract.Transact(opts, method, params...)
}

// ERC20MINPOINTS is a free data retrieval call binding the contract method 0x42f4c59d.
//
// Solidity: function ERC20_MIN_POINTS() view returns(uint256)
func (_Token *TokenCaller) ERC20MINPOINTS(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "ERC20_MIN_POINTS")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ERC20MINPOINTS is a free data retrieval call binding the contract method 0x42f4c59d.
//
// Solidity: function ERC20_MIN_POINTS() view returns(uint256)
func (_Token *TokenSession) ERC20MINPOINTS() (*big.Int, error) {
	return _Token.Contract.ERC20MINPOINTS(&_Token.CallOpts)
}

// ERC20MINPOINTS is a free data retrieval call binding the contract method 0x42f4c59d.
//
// Solidity: function ERC20_MIN_POINTS() view returns(uint256)
func (_Token *TokenCallerSession) ERC20MINPOINTS() (*big.Int, error) {
	return _Token.Contract.ERC20MINPOINTS(&_Token.CallOpts)
}

// WITHDRAWALSTATUSCANCELLED is a free data retrieval call binding the contract method 0x0edf202d.
//
// Solidity: function WITHDRAWAL_STATUS_CANCELLED() view returns(uint8)
func (_Token *TokenCaller) WITHDRAWALSTATUSCANCELLED(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "WITHDRAWAL_STATUS_CANCELLED")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// WITHDRAWALSTATUSCANCELLED is a free data retrieval call binding the contract method 0x0edf202d.
//
// Solidity: function WITHDRAWAL_STATUS_CANCELLED() view returns(uint8)
func (_Token *TokenSession) WITHDRAWALSTATUSCANCELLED() (uint8, error) {
	return _Token.Contract.WITHDRAWALSTATUSCANCELLED(&_Token.CallOpts)
}

// WITHDRAWALSTATUSCANCELLED is a free data retrieval call binding the contract method 0x0edf202d.
//
// Solidity: function WITHDRAWAL_STATUS_CANCELLED() view returns(uint8)
func (_Token *TokenCallerSession) WITHDRAWALSTATUSCANCELLED() (uint8, error) {
	return _Token.Contract.WITHDRAWALSTATUSCANCELLED(&_Token.CallOpts)
}

// WITHDRAWALSTATUSDONE is a free data retrieval call binding the contract method 0x35efdd9e.
//
// Solidity: function WITHDRAWAL_STATUS_DONE() view returns(uint8)
func (_Token *TokenCaller) WITHDRAWALSTATUSDONE(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "WITHDRAWAL_STATUS_DONE")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// WITHDRAWALSTATUSDONE is a free data retrieval call binding the contract method 0x35efdd9e.
//
// Solidity: function WITHDRAWAL_STATUS_DONE() view returns(uint8)
func (_Token *TokenSession) WITHDRAWALSTATUSDONE() (uint8, error) {
	return _Token.Contract.WITHDRAWALSTATUSDONE(&_Token.CallOpts)
}

// WITHDRAWALSTATUSDONE is a free data retrieval call binding the contract method 0x35efdd9e.
//
// Solidity: function WITHDRAWAL_STATUS_DONE() view returns(uint8)
func (_Token *TokenCallerSession) WITHDRAWALSTATUSDONE() (uint8, error) {
	return _Token.Contract.WITHDRAWALSTATUSDONE(&_Token.CallOpts)
}

// WITHDRAWALSTATUSPENDING is a free data retrieval call binding the contract method 0x6f6c37e8.
//
// Solidity: function WITHDRAWAL_STATUS_PENDING() view returns(uint8)
func (_Token *TokenCaller) WITHDRAWALSTATUSPENDING(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "WITHDRAWAL_STATUS_PENDING")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// WITHDRAWALSTATUSPENDING is a free data retrieval call binding the contract method 0x6f6c37e8.
//
// Solidity: function WITHDRAWAL_STATUS_PENDING() view returns(uint8)
func (_Token *TokenSession) WITHDRAWALSTATUSPENDING() (uint8, error) {
	return _Token.Contract.WITHDRAWALSTATUSPENDING(&_Token.CallOpts)
}

// WITHDRAWALSTATUSPENDING is a free data retrieval call binding the contract method 0x6f6c37e8.
//
// Solidity: function WITHDRAWAL_STATUS_PENDING() view returns(uint8)
func (_Token *TokenCallerSession) WITHDRAWALSTATUSPENDING() (uint8, error) {
	return _Token.Contract.WITHDRAWALSTATUSPENDING(&_Token.CallOpts)
}

// Erc20TokenList is a free data retrieval call binding the contract method 0xac18b534.
//
// Solidity: function erc20TokenList(uint256 ) view returns(address)
func (_Token *TokenCaller) Erc20TokenList(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "erc20TokenList", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Erc20TokenList is a free data retrieval call binding the contract method 0xac18b534.
//
// Solidity: function erc20TokenList(uint256 ) view returns(address)
func (_Token *TokenSession) Erc20TokenList(arg0 *big.Int) (common.Address, error) {
	return _Token.Contract.Erc20TokenList(&_Token.CallOpts, arg0)
}

// Erc20TokenList is a free data retrieval call binding the contract method 0xac18b534.
//
// Solidity: function erc20TokenList(uint256 ) view returns(address)
func (_Token *TokenCallerSession) Erc20TokenList(arg0 *big.Int) (common.Address, error) {
	return _Token.Contract.Erc20TokenList(&_Token.CallOpts, arg0)
}

// Erc20Tokens is a free data retrieval call binding the contract method 0x59c29632.
//
// Solidity: function erc20Tokens(address ) view returns(bool active, uint256 rate, uint256 unit)
func (_Token *TokenCaller) Erc20Tokens(opts *bind.CallOpts, arg0 common.Address) (struct {
	Active bool
	Rate   *big.Int
	Unit   *big.Int
}, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "erc20Tokens", arg0)

	outstruct := new(struct {
		Active bool
		Rate   *big.Int
		Unit   *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Active = *abi.ConvertType(out[0], new(bool)).(*bool)
	outstruct.Rate = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.Unit = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Erc20Tokens is a free data retrieval call binding the contract method 0x59c29632.
//
// Solidity: function erc20Tokens(address ) view returns(bool active, uint256 rate, uint256 unit)
func (_Token *TokenSession) Erc20Tokens(arg0 common.Address) (struct {
	Active bool
	Rate   *big.Int
	Unit   *big.Int
}, error) {
	return _Token.Contract.Erc20Tokens(&_Token.CallOpts, arg0)
}

// Erc20Tokens is a free data retrieval call binding the contract method 0x59c29632.
//
// Solidity: function erc20Tokens(address ) view returns(bool active, uint256 rate, uint256 unit)
func (_Token *TokenCallerSession) Erc20Tokens(arg0 common.Address) (struct {
	Active bool
	Rate   *big.Int
	Unit   *big.Int
}, error) {
	return _Token.Contract.Erc20Tokens(&_Token.CallOpts, arg0)
}

// GetERC20Config is a free data retrieval call binding the contract method 0xab2ea17c.
//
// Solidity: function getERC20Config(address token) view returns(bool, uint256, uint256)
func (_Token *TokenCaller) GetERC20Config(opts *bind.CallOpts, token common.Address) (bool, *big.Int, *big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "getERC20Config", token)

	if err != nil {
		return *new(bool), *new(*big.Int), *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	out2 := *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return out0, out1, out2, err

}

// GetERC20Config is a free data retrieval call binding the contract method 0xab2ea17c.
//
// Solidity: function getERC20Config(address token) view returns(bool, uint256, uint256)
func (_Token *TokenSession) GetERC20Config(token common.Address) (bool, *big.Int, *big.Int, error) {
	return _Token.Contract.GetERC20Config(&_Token.CallOpts, token)
}

// GetERC20Config is a free data retrieval call binding the contract method 0xab2ea17c.
//
// Solidity: function getERC20Config(address token) view returns(bool, uint256, uint256)
func (_Token *TokenCallerSession) GetERC20Config(token common.Address) (bool, *big.Int, *big.Int, error) {
	return _Token.Contract.GetERC20Config(&_Token.CallOpts, token)
}

// GetERC20Count is a free data retrieval call binding the contract method 0xd904d11e.
//
// Solidity: function getERC20Count() view returns(uint256)
func (_Token *TokenCaller) GetERC20Count(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "getERC20Count")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetERC20Count is a free data retrieval call binding the contract method 0xd904d11e.
//
// Solidity: function getERC20Count() view returns(uint256)
func (_Token *TokenSession) GetERC20Count() (*big.Int, error) {
	return _Token.Contract.GetERC20Count(&_Token.CallOpts)
}

// GetERC20Count is a free data retrieval call binding the contract method 0xd904d11e.
//
// Solidity: function getERC20Count() view returns(uint256)
func (_Token *TokenCallerSession) GetERC20Count() (*big.Int, error) {
	return _Token.Contract.GetERC20Count(&_Token.CallOpts)
}

// GetERC20List is a free data retrieval call binding the contract method 0x6657df03.
//
// Solidity: function getERC20List() view returns(address[] tokens, bool[] actives, uint256[] rates, uint256[] units)
func (_Token *TokenCaller) GetERC20List(opts *bind.CallOpts) (struct {
	Tokens  []common.Address
	Actives []bool
	Rates   []*big.Int
	Units   []*big.Int
}, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "getERC20List")

	outstruct := new(struct {
		Tokens  []common.Address
		Actives []bool
		Rates   []*big.Int
		Units   []*big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Tokens = *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	outstruct.Actives = *abi.ConvertType(out[1], new([]bool)).(*[]bool)
	outstruct.Rates = *abi.ConvertType(out[2], new([]*big.Int)).(*[]*big.Int)
	outstruct.Units = *abi.ConvertType(out[3], new([]*big.Int)).(*[]*big.Int)

	return *outstruct, err

}

// GetERC20List is a free data retrieval call binding the contract method 0x6657df03.
//
// Solidity: function getERC20List() view returns(address[] tokens, bool[] actives, uint256[] rates, uint256[] units)
func (_Token *TokenSession) GetERC20List() (struct {
	Tokens  []common.Address
	Actives []bool
	Rates   []*big.Int
	Units   []*big.Int
}, error) {
	return _Token.Contract.GetERC20List(&_Token.CallOpts)
}

// GetERC20List is a free data retrieval call binding the contract method 0x6657df03.
//
// Solidity: function getERC20List() view returns(address[] tokens, bool[] actives, uint256[] rates, uint256[] units)
func (_Token *TokenCallerSession) GetERC20List() (struct {
	Tokens  []common.Address
	Actives []bool
	Rates   []*big.Int
	Units   []*big.Int
}, error) {
	return _Token.Contract.GetERC20List(&_Token.CallOpts)
}

// GetPendingWithdrawal is a free data retrieval call binding the contract method 0x946f508a.
//
// Solidity: function getPendingWithdrawal(uint64 nonce) view returns(address user, address token, uint256 amount, uint8 status)
func (_Token *TokenCaller) GetPendingWithdrawal(opts *bind.CallOpts, nonce uint64) (struct {
	User   common.Address
	Token  common.Address
	Amount *big.Int
	Status uint8
}, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "getPendingWithdrawal", nonce)

	outstruct := new(struct {
		User   common.Address
		Token  common.Address
		Amount *big.Int
		Status uint8
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.User = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Token = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.Amount = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.Status = *abi.ConvertType(out[3], new(uint8)).(*uint8)

	return *outstruct, err

}

// GetPendingWithdrawal is a free data retrieval call binding the contract method 0x946f508a.
//
// Solidity: function getPendingWithdrawal(uint64 nonce) view returns(address user, address token, uint256 amount, uint8 status)
func (_Token *TokenSession) GetPendingWithdrawal(nonce uint64) (struct {
	User   common.Address
	Token  common.Address
	Amount *big.Int
	Status uint8
}, error) {
	return _Token.Contract.GetPendingWithdrawal(&_Token.CallOpts, nonce)
}

// GetPendingWithdrawal is a free data retrieval call binding the contract method 0x946f508a.
//
// Solidity: function getPendingWithdrawal(uint64 nonce) view returns(address user, address token, uint256 amount, uint8 status)
func (_Token *TokenCallerSession) GetPendingWithdrawal(nonce uint64) (struct {
	User   common.Address
	Token  common.Address
	Amount *big.Int
	Status uint8
}, error) {
	return _Token.Contract.GetPendingWithdrawal(&_Token.CallOpts, nonce)
}

// NativeActive is a free data retrieval call binding the contract method 0x758bd72f.
//
// Solidity: function nativeActive() view returns(bool)
func (_Token *TokenCaller) NativeActive(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "nativeActive")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// NativeActive is a free data retrieval call binding the contract method 0x758bd72f.
//
// Solidity: function nativeActive() view returns(bool)
func (_Token *TokenSession) NativeActive() (bool, error) {
	return _Token.Contract.NativeActive(&_Token.CallOpts)
}

// NativeActive is a free data retrieval call binding the contract method 0x758bd72f.
//
// Solidity: function nativeActive() view returns(bool)
func (_Token *TokenCallerSession) NativeActive() (bool, error) {
	return _Token.Contract.NativeActive(&_Token.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Token *TokenCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Token *TokenSession) Owner() (common.Address, error) {
	return _Token.Contract.Owner(&_Token.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Token *TokenCallerSession) Owner() (common.Address, error) {
	return _Token.Contract.Owner(&_Token.CallOpts)
}

// PendingWithdrawals is a free data retrieval call binding the contract method 0xccbff031.
//
// Solidity: function pendingWithdrawals(uint64 ) view returns(address user, address token, uint256 amount, uint8 status)
func (_Token *TokenCaller) PendingWithdrawals(opts *bind.CallOpts, arg0 uint64) (struct {
	User   common.Address
	Token  common.Address
	Amount *big.Int
	Status uint8
}, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "pendingWithdrawals", arg0)

	outstruct := new(struct {
		User   common.Address
		Token  common.Address
		Amount *big.Int
		Status uint8
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.User = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Token = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.Amount = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.Status = *abi.ConvertType(out[3], new(uint8)).(*uint8)

	return *outstruct, err

}

// PendingWithdrawals is a free data retrieval call binding the contract method 0xccbff031.
//
// Solidity: function pendingWithdrawals(uint64 ) view returns(address user, address token, uint256 amount, uint8 status)
func (_Token *TokenSession) PendingWithdrawals(arg0 uint64) (struct {
	User   common.Address
	Token  common.Address
	Amount *big.Int
	Status uint8
}, error) {
	return _Token.Contract.PendingWithdrawals(&_Token.CallOpts, arg0)
}

// PendingWithdrawals is a free data retrieval call binding the contract method 0xccbff031.
//
// Solidity: function pendingWithdrawals(uint64 ) view returns(address user, address token, uint256 amount, uint8 status)
func (_Token *TokenCallerSession) PendingWithdrawals(arg0 uint64) (struct {
	User   common.Address
	Token  common.Address
	Amount *big.Int
	Status uint8
}, error) {
	return _Token.Contract.PendingWithdrawals(&_Token.CallOpts, arg0)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_Token *TokenCaller) ProxiableUUID(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "proxiableUUID")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_Token *TokenSession) ProxiableUUID() ([32]byte, error) {
	return _Token.Contract.ProxiableUUID(&_Token.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_Token *TokenCallerSession) ProxiableUUID() ([32]byte, error) {
	return _Token.Contract.ProxiableUUID(&_Token.CallOpts)
}

// Rate is a free data retrieval call binding the contract method 0x2c4e722e.
//
// Solidity: function rate() view returns(uint256)
func (_Token *TokenCaller) Rate(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "rate")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Rate is a free data retrieval call binding the contract method 0x2c4e722e.
//
// Solidity: function rate() view returns(uint256)
func (_Token *TokenSession) Rate() (*big.Int, error) {
	return _Token.Contract.Rate(&_Token.CallOpts)
}

// Rate is a free data retrieval call binding the contract method 0x2c4e722e.
//
// Solidity: function rate() view returns(uint256)
func (_Token *TokenCallerSession) Rate() (*big.Int, error) {
	return _Token.Contract.Rate(&_Token.CallOpts)
}

// RechargeFeeRate is a free data retrieval call binding the contract method 0x575f09e0.
//
// Solidity: function rechargeFeeRate() view returns(uint256)
func (_Token *TokenCaller) RechargeFeeRate(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "rechargeFeeRate")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RechargeFeeRate is a free data retrieval call binding the contract method 0x575f09e0.
//
// Solidity: function rechargeFeeRate() view returns(uint256)
func (_Token *TokenSession) RechargeFeeRate() (*big.Int, error) {
	return _Token.Contract.RechargeFeeRate(&_Token.CallOpts)
}

// RechargeFeeRate is a free data retrieval call binding the contract method 0x575f09e0.
//
// Solidity: function rechargeFeeRate() view returns(uint256)
func (_Token *TokenCallerSession) RechargeFeeRate() (*big.Int, error) {
	return _Token.Contract.RechargeFeeRate(&_Token.CallOpts)
}

// Subnet is a free data retrieval call binding the contract method 0xcaf2ebf2.
//
// Solidity: function subnet() view returns(address)
func (_Token *TokenCaller) Subnet(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "subnet")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Subnet is a free data retrieval call binding the contract method 0xcaf2ebf2.
//
// Solidity: function subnet() view returns(address)
func (_Token *TokenSession) Subnet() (common.Address, error) {
	return _Token.Contract.Subnet(&_Token.CallOpts)
}

// Subnet is a free data retrieval call binding the contract method 0xcaf2ebf2.
//
// Solidity: function subnet() view returns(address)
func (_Token *TokenCallerSession) Subnet() (common.Address, error) {
	return _Token.Contract.Subnet(&_Token.CallOpts)
}

// ToPoints is a free data retrieval call binding the contract method 0xc718a401.
//
// Solidity: function toPoints(uint256 amount) view returns(uint256)
func (_Token *TokenCaller) ToPoints(opts *bind.CallOpts, amount *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "toPoints", amount)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ToPoints is a free data retrieval call binding the contract method 0xc718a401.
//
// Solidity: function toPoints(uint256 amount) view returns(uint256)
func (_Token *TokenSession) ToPoints(amount *big.Int) (*big.Int, error) {
	return _Token.Contract.ToPoints(&_Token.CallOpts, amount)
}

// ToPoints is a free data retrieval call binding the contract method 0xc718a401.
//
// Solidity: function toPoints(uint256 amount) view returns(uint256)
func (_Token *TokenCallerSession) ToPoints(amount *big.Int) (*big.Int, error) {
	return _Token.Contract.ToPoints(&_Token.CallOpts, amount)
}

// ToValue is a free data retrieval call binding the contract method 0xf648fc0d.
//
// Solidity: function toValue(uint256 points) view returns(uint256)
func (_Token *TokenCaller) ToValue(opts *bind.CallOpts, points *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "toValue", points)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ToValue is a free data retrieval call binding the contract method 0xf648fc0d.
//
// Solidity: function toValue(uint256 points) view returns(uint256)
func (_Token *TokenSession) ToValue(points *big.Int) (*big.Int, error) {
	return _Token.Contract.ToValue(&_Token.CallOpts, points)
}

// ToValue is a free data retrieval call binding the contract method 0xf648fc0d.
//
// Solidity: function toValue(uint256 points) view returns(uint256)
func (_Token *TokenCallerSession) ToValue(points *big.Int) (*big.Int, error) {
	return _Token.Contract.ToValue(&_Token.CallOpts, points)
}

// TokenUnit is a free data retrieval call binding the contract method 0xe93c980d.
//
// Solidity: function tokenUnit() view returns(uint256)
func (_Token *TokenCaller) TokenUnit(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "tokenUnit")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TokenUnit is a free data retrieval call binding the contract method 0xe93c980d.
//
// Solidity: function tokenUnit() view returns(uint256)
func (_Token *TokenSession) TokenUnit() (*big.Int, error) {
	return _Token.Contract.TokenUnit(&_Token.CallOpts)
}

// TokenUnit is a free data retrieval call binding the contract method 0xe93c980d.
//
// Solidity: function tokenUnit() view returns(uint256)
func (_Token *TokenCallerSession) TokenUnit() (*big.Int, error) {
	return _Token.Contract.TokenUnit(&_Token.CallOpts)
}

// WithdrawFeeRate is a free data retrieval call binding the contract method 0xea99e689.
//
// Solidity: function withdrawFeeRate() view returns(uint256)
func (_Token *TokenCaller) WithdrawFeeRate(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "withdrawFeeRate")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// WithdrawFeeRate is a free data retrieval call binding the contract method 0xea99e689.
//
// Solidity: function withdrawFeeRate() view returns(uint256)
func (_Token *TokenSession) WithdrawFeeRate() (*big.Int, error) {
	return _Token.Contract.WithdrawFeeRate(&_Token.CallOpts)
}

// WithdrawFeeRate is a free data retrieval call binding the contract method 0xea99e689.
//
// Solidity: function withdrawFeeRate() view returns(uint256)
func (_Token *TokenCallerSession) WithdrawFeeRate() (*big.Int, error) {
	return _Token.Contract.WithdrawFeeRate(&_Token.CallOpts)
}

// CancelWithdrawal is a paid mutator transaction binding the contract method 0x2bd34cc9.
//
// Solidity: function cancelWithdrawal(uint64 nonce) returns()
func (_Token *TokenTransactor) CancelWithdrawal(opts *bind.TransactOpts, nonce uint64) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "cancelWithdrawal", nonce)
}

// CancelWithdrawal is a paid mutator transaction binding the contract method 0x2bd34cc9.
//
// Solidity: function cancelWithdrawal(uint64 nonce) returns()
func (_Token *TokenSession) CancelWithdrawal(nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.CancelWithdrawal(&_Token.TransactOpts, nonce)
}

// CancelWithdrawal is a paid mutator transaction binding the contract method 0x2bd34cc9.
//
// Solidity: function cancelWithdrawal(uint64 nonce) returns()
func (_Token *TokenTransactorSession) CancelWithdrawal(nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.CancelWithdrawal(&_Token.TransactOpts, nonce)
}

// ClaimWithdrawal is a paid mutator transaction binding the contract method 0x71af55ee.
//
// Solidity: function claimWithdrawal(uint64 nonce) returns()
func (_Token *TokenTransactor) ClaimWithdrawal(opts *bind.TransactOpts, nonce uint64) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "claimWithdrawal", nonce)
}

// ClaimWithdrawal is a paid mutator transaction binding the contract method 0x71af55ee.
//
// Solidity: function claimWithdrawal(uint64 nonce) returns()
func (_Token *TokenSession) ClaimWithdrawal(nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.ClaimWithdrawal(&_Token.TransactOpts, nonce)
}

// ClaimWithdrawal is a paid mutator transaction binding the contract method 0x71af55ee.
//
// Solidity: function claimWithdrawal(uint64 nonce) returns()
func (_Token *TokenTransactorSession) ClaimWithdrawal(nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.ClaimWithdrawal(&_Token.TransactOpts, nonce)
}

// EmergencyWithdraw is a paid mutator transaction binding the contract method 0xdb2e21bc.
//
// Solidity: function emergencyWithdraw() returns()
func (_Token *TokenTransactor) EmergencyWithdraw(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "emergencyWithdraw")
}

// EmergencyWithdraw is a paid mutator transaction binding the contract method 0xdb2e21bc.
//
// Solidity: function emergencyWithdraw() returns()
func (_Token *TokenSession) EmergencyWithdraw() (*types.Transaction, error) {
	return _Token.Contract.EmergencyWithdraw(&_Token.TransactOpts)
}

// EmergencyWithdraw is a paid mutator transaction binding the contract method 0xdb2e21bc.
//
// Solidity: function emergencyWithdraw() returns()
func (_Token *TokenTransactorSession) EmergencyWithdraw() (*types.Transaction, error) {
	return _Token.Contract.EmergencyWithdraw(&_Token.TransactOpts)
}

// EmergencyWithdrawErc20 is a paid mutator transaction binding the contract method 0xb6b9d02e.
//
// Solidity: function emergencyWithdrawErc20(address token) returns()
func (_Token *TokenTransactor) EmergencyWithdrawErc20(opts *bind.TransactOpts, token common.Address) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "emergencyWithdrawErc20", token)
}

// EmergencyWithdrawErc20 is a paid mutator transaction binding the contract method 0xb6b9d02e.
//
// Solidity: function emergencyWithdrawErc20(address token) returns()
func (_Token *TokenSession) EmergencyWithdrawErc20(token common.Address) (*types.Transaction, error) {
	return _Token.Contract.EmergencyWithdrawErc20(&_Token.TransactOpts, token)
}

// EmergencyWithdrawErc20 is a paid mutator transaction binding the contract method 0xb6b9d02e.
//
// Solidity: function emergencyWithdrawErc20(address token) returns()
func (_Token *TokenTransactorSession) EmergencyWithdrawErc20(token common.Address) (*types.Transaction, error) {
	return _Token.Contract.EmergencyWithdrawErc20(&_Token.TransactOpts, token)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _owner) returns()
func (_Token *TokenTransactor) Initialize(opts *bind.TransactOpts, _owner common.Address) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "initialize", _owner)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _owner) returns()
func (_Token *TokenSession) Initialize(_owner common.Address) (*types.Transaction, error) {
	return _Token.Contract.Initialize(&_Token.TransactOpts, _owner)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _owner) returns()
func (_Token *TokenTransactorSession) Initialize(_owner common.Address) (*types.Transaction, error) {
	return _Token.Contract.Initialize(&_Token.TransactOpts, _owner)
}

// Recharge is a paid mutator transaction binding the contract method 0x4cddae28.
//
// Solidity: function recharge() payable returns(uint256)
func (_Token *TokenTransactor) Recharge(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "recharge")
}

// Recharge is a paid mutator transaction binding the contract method 0x4cddae28.
//
// Solidity: function recharge() payable returns(uint256)
func (_Token *TokenSession) Recharge() (*types.Transaction, error) {
	return _Token.Contract.Recharge(&_Token.TransactOpts)
}

// Recharge is a paid mutator transaction binding the contract method 0x4cddae28.
//
// Solidity: function recharge() payable returns(uint256)
func (_Token *TokenTransactorSession) Recharge() (*types.Transaction, error) {
	return _Token.Contract.Recharge(&_Token.TransactOpts)
}

// RechargeErc20 is a paid mutator transaction binding the contract method 0x8dd808fa.
//
// Solidity: function rechargeErc20(address token, uint256 amount) returns(uint256)
func (_Token *TokenTransactor) RechargeErc20(opts *bind.TransactOpts, token common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "rechargeErc20", token, amount)
}

// RechargeErc20 is a paid mutator transaction binding the contract method 0x8dd808fa.
//
// Solidity: function rechargeErc20(address token, uint256 amount) returns(uint256)
func (_Token *TokenSession) RechargeErc20(token common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Token.Contract.RechargeErc20(&_Token.TransactOpts, token, amount)
}

// RechargeErc20 is a paid mutator transaction binding the contract method 0x8dd808fa.
//
// Solidity: function rechargeErc20(address token, uint256 amount) returns(uint256)
func (_Token *TokenTransactorSession) RechargeErc20(token common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Token.Contract.RechargeErc20(&_Token.TransactOpts, token, amount)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Token *TokenTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Token *TokenSession) RenounceOwnership() (*types.Transaction, error) {
	return _Token.Contract.RenounceOwnership(&_Token.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Token *TokenTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Token.Contract.RenounceOwnership(&_Token.TransactOpts)
}

// SetERC20Token is a paid mutator transaction binding the contract method 0xcee98c87.
//
// Solidity: function setERC20Token(address token, bool active, uint256 tokenRate, uint256 unit) returns()
func (_Token *TokenTransactor) SetERC20Token(opts *bind.TransactOpts, token common.Address, active bool, tokenRate *big.Int, unit *big.Int) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "setERC20Token", token, active, tokenRate, unit)
}

// SetERC20Token is a paid mutator transaction binding the contract method 0xcee98c87.
//
// Solidity: function setERC20Token(address token, bool active, uint256 tokenRate, uint256 unit) returns()
func (_Token *TokenSession) SetERC20Token(token common.Address, active bool, tokenRate *big.Int, unit *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetERC20Token(&_Token.TransactOpts, token, active, tokenRate, unit)
}

// SetERC20Token is a paid mutator transaction binding the contract method 0xcee98c87.
//
// Solidity: function setERC20Token(address token, bool active, uint256 tokenRate, uint256 unit) returns()
func (_Token *TokenTransactorSession) SetERC20Token(token common.Address, active bool, tokenRate *big.Int, unit *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetERC20Token(&_Token.TransactOpts, token, active, tokenRate, unit)
}

// SetNativeActive is a paid mutator transaction binding the contract method 0x41d24d4f.
//
// Solidity: function setNativeActive(bool active) returns()
func (_Token *TokenTransactor) SetNativeActive(opts *bind.TransactOpts, active bool) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "setNativeActive", active)
}

// SetNativeActive is a paid mutator transaction binding the contract method 0x41d24d4f.
//
// Solidity: function setNativeActive(bool active) returns()
func (_Token *TokenSession) SetNativeActive(active bool) (*types.Transaction, error) {
	return _Token.Contract.SetNativeActive(&_Token.TransactOpts, active)
}

// SetNativeActive is a paid mutator transaction binding the contract method 0x41d24d4f.
//
// Solidity: function setNativeActive(bool active) returns()
func (_Token *TokenTransactorSession) SetNativeActive(active bool) (*types.Transaction, error) {
	return _Token.Contract.SetNativeActive(&_Token.TransactOpts, active)
}

// SetRate is a paid mutator transaction binding the contract method 0x34fcf437.
//
// Solidity: function setRate(uint256 newRate) returns()
func (_Token *TokenTransactor) SetRate(opts *bind.TransactOpts, newRate *big.Int) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "setRate", newRate)
}

// SetRate is a paid mutator transaction binding the contract method 0x34fcf437.
//
// Solidity: function setRate(uint256 newRate) returns()
func (_Token *TokenSession) SetRate(newRate *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetRate(&_Token.TransactOpts, newRate)
}

// SetRate is a paid mutator transaction binding the contract method 0x34fcf437.
//
// Solidity: function setRate(uint256 newRate) returns()
func (_Token *TokenTransactorSession) SetRate(newRate *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetRate(&_Token.TransactOpts, newRate)
}

// SetRechargeFeeRate is a paid mutator transaction binding the contract method 0xe91818a5.
//
// Solidity: function setRechargeFeeRate(uint256 newRate) returns()
func (_Token *TokenTransactor) SetRechargeFeeRate(opts *bind.TransactOpts, newRate *big.Int) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "setRechargeFeeRate", newRate)
}

// SetRechargeFeeRate is a paid mutator transaction binding the contract method 0xe91818a5.
//
// Solidity: function setRechargeFeeRate(uint256 newRate) returns()
func (_Token *TokenSession) SetRechargeFeeRate(newRate *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetRechargeFeeRate(&_Token.TransactOpts, newRate)
}

// SetRechargeFeeRate is a paid mutator transaction binding the contract method 0xe91818a5.
//
// Solidity: function setRechargeFeeRate(uint256 newRate) returns()
func (_Token *TokenTransactorSession) SetRechargeFeeRate(newRate *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetRechargeFeeRate(&_Token.TransactOpts, newRate)
}

// SetSubnet is a paid mutator transaction binding the contract method 0x00de3edb.
//
// Solidity: function setSubnet(address _subnet) returns()
func (_Token *TokenTransactor) SetSubnet(opts *bind.TransactOpts, _subnet common.Address) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "setSubnet", _subnet)
}

// SetSubnet is a paid mutator transaction binding the contract method 0x00de3edb.
//
// Solidity: function setSubnet(address _subnet) returns()
func (_Token *TokenSession) SetSubnet(_subnet common.Address) (*types.Transaction, error) {
	return _Token.Contract.SetSubnet(&_Token.TransactOpts, _subnet)
}

// SetSubnet is a paid mutator transaction binding the contract method 0x00de3edb.
//
// Solidity: function setSubnet(address _subnet) returns()
func (_Token *TokenTransactorSession) SetSubnet(_subnet common.Address) (*types.Transaction, error) {
	return _Token.Contract.SetSubnet(&_Token.TransactOpts, _subnet)
}

// SetTokenUnit is a paid mutator transaction binding the contract method 0xf474fc78.
//
// Solidity: function setTokenUnit(uint256 unit) returns()
func (_Token *TokenTransactor) SetTokenUnit(opts *bind.TransactOpts, unit *big.Int) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "setTokenUnit", unit)
}

// SetTokenUnit is a paid mutator transaction binding the contract method 0xf474fc78.
//
// Solidity: function setTokenUnit(uint256 unit) returns()
func (_Token *TokenSession) SetTokenUnit(unit *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetTokenUnit(&_Token.TransactOpts, unit)
}

// SetTokenUnit is a paid mutator transaction binding the contract method 0xf474fc78.
//
// Solidity: function setTokenUnit(uint256 unit) returns()
func (_Token *TokenTransactorSession) SetTokenUnit(unit *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetTokenUnit(&_Token.TransactOpts, unit)
}

// SetWithdrawFeeRate is a paid mutator transaction binding the contract method 0x096015b0.
//
// Solidity: function setWithdrawFeeRate(uint256 newRate) returns()
func (_Token *TokenTransactor) SetWithdrawFeeRate(opts *bind.TransactOpts, newRate *big.Int) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "setWithdrawFeeRate", newRate)
}

// SetWithdrawFeeRate is a paid mutator transaction binding the contract method 0x096015b0.
//
// Solidity: function setWithdrawFeeRate(uint256 newRate) returns()
func (_Token *TokenSession) SetWithdrawFeeRate(newRate *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetWithdrawFeeRate(&_Token.TransactOpts, newRate)
}

// SetWithdrawFeeRate is a paid mutator transaction binding the contract method 0x096015b0.
//
// Solidity: function setWithdrawFeeRate(uint256 newRate) returns()
func (_Token *TokenTransactorSession) SetWithdrawFeeRate(newRate *big.Int) (*types.Transaction, error) {
	return _Token.Contract.SetWithdrawFeeRate(&_Token.TransactOpts, newRate)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Token *TokenTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Token *TokenSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Token.Contract.TransferOwnership(&_Token.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Token *TokenTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Token.Contract.TransferOwnership(&_Token.TransactOpts, newOwner)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_Token *TokenTransactor) UpgradeTo(opts *bind.TransactOpts, newImplementation common.Address) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "upgradeTo", newImplementation)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_Token *TokenSession) UpgradeTo(newImplementation common.Address) (*types.Transaction, error) {
	return _Token.Contract.UpgradeTo(&_Token.TransactOpts, newImplementation)
}

// UpgradeTo is a paid mutator transaction binding the contract method 0x3659cfe6.
//
// Solidity: function upgradeTo(address newImplementation) returns()
func (_Token *TokenTransactorSession) UpgradeTo(newImplementation common.Address) (*types.Transaction, error) {
	return _Token.Contract.UpgradeTo(&_Token.TransactOpts, newImplementation)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_Token *TokenTransactor) UpgradeToAndCall(opts *bind.TransactOpts, newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "upgradeToAndCall", newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_Token *TokenSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _Token.Contract.UpgradeToAndCall(&_Token.TransactOpts, newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_Token *TokenTransactorSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _Token.Contract.UpgradeToAndCall(&_Token.TransactOpts, newImplementation, data)
}

// Withdraw is a paid mutator transaction binding the contract method 0x8693a290.
//
// Solidity: function withdraw(address user, uint256 points, uint64 nonce) returns()
func (_Token *TokenTransactor) Withdraw(opts *bind.TransactOpts, user common.Address, points *big.Int, nonce uint64) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "withdraw", user, points, nonce)
}

// Withdraw is a paid mutator transaction binding the contract method 0x8693a290.
//
// Solidity: function withdraw(address user, uint256 points, uint64 nonce) returns()
func (_Token *TokenSession) Withdraw(user common.Address, points *big.Int, nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.Withdraw(&_Token.TransactOpts, user, points, nonce)
}

// Withdraw is a paid mutator transaction binding the contract method 0x8693a290.
//
// Solidity: function withdraw(address user, uint256 points, uint64 nonce) returns()
func (_Token *TokenTransactorSession) Withdraw(user common.Address, points *big.Int, nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.Withdraw(&_Token.TransactOpts, user, points, nonce)
}

// WithdrawErc20 is a paid mutator transaction binding the contract method 0x1256f7a6.
//
// Solidity: function withdrawErc20(address token, address user, uint256 points, uint64 nonce) returns()
func (_Token *TokenTransactor) WithdrawErc20(opts *bind.TransactOpts, token common.Address, user common.Address, points *big.Int, nonce uint64) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "withdrawErc20", token, user, points, nonce)
}

// WithdrawErc20 is a paid mutator transaction binding the contract method 0x1256f7a6.
//
// Solidity: function withdrawErc20(address token, address user, uint256 points, uint64 nonce) returns()
func (_Token *TokenSession) WithdrawErc20(token common.Address, user common.Address, points *big.Int, nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.WithdrawErc20(&_Token.TransactOpts, token, user, points, nonce)
}

// WithdrawErc20 is a paid mutator transaction binding the contract method 0x1256f7a6.
//
// Solidity: function withdrawErc20(address token, address user, uint256 points, uint64 nonce) returns()
func (_Token *TokenTransactorSession) WithdrawErc20(token common.Address, user common.Address, points *big.Int, nonce uint64) (*types.Transaction, error) {
	return _Token.Contract.WithdrawErc20(&_Token.TransactOpts, token, user, points, nonce)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Token *TokenTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Token.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Token *TokenSession) Receive() (*types.Transaction, error) {
	return _Token.Contract.Receive(&_Token.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Token *TokenTransactorSession) Receive() (*types.Transaction, error) {
	return _Token.Contract.Receive(&_Token.TransactOpts)
}

// TokenAdminChangedIterator is returned from FilterAdminChanged and is used to iterate over the raw logs and unpacked data for AdminChanged events raised by the Token contract.
type TokenAdminChangedIterator struct {
	Event *TokenAdminChanged // Event containing the contract specifics and raw log

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
func (it *TokenAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenAdminChanged)
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
		it.Event = new(TokenAdminChanged)
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
func (it *TokenAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenAdminChanged represents a AdminChanged event raised by the Token contract.
type TokenAdminChanged struct {
	PreviousAdmin common.Address
	NewAdmin      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterAdminChanged is a free log retrieval operation binding the contract event 0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f.
//
// Solidity: event AdminChanged(address previousAdmin, address newAdmin)
func (_Token *TokenFilterer) FilterAdminChanged(opts *bind.FilterOpts) (*TokenAdminChangedIterator, error) {

	logs, sub, err := _Token.contract.FilterLogs(opts, "AdminChanged")
	if err != nil {
		return nil, err
	}
	return &TokenAdminChangedIterator{contract: _Token.contract, event: "AdminChanged", logs: logs, sub: sub}, nil
}

// WatchAdminChanged is a free log subscription operation binding the contract event 0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f.
//
// Solidity: event AdminChanged(address previousAdmin, address newAdmin)
func (_Token *TokenFilterer) WatchAdminChanged(opts *bind.WatchOpts, sink chan<- *TokenAdminChanged) (event.Subscription, error) {

	logs, sub, err := _Token.contract.WatchLogs(opts, "AdminChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenAdminChanged)
				if err := _Token.contract.UnpackLog(event, "AdminChanged", log); err != nil {
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
func (_Token *TokenFilterer) ParseAdminChanged(log types.Log) (*TokenAdminChanged, error) {
	event := new(TokenAdminChanged)
	if err := _Token.contract.UnpackLog(event, "AdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenBeaconUpgradedIterator is returned from FilterBeaconUpgraded and is used to iterate over the raw logs and unpacked data for BeaconUpgraded events raised by the Token contract.
type TokenBeaconUpgradedIterator struct {
	Event *TokenBeaconUpgraded // Event containing the contract specifics and raw log

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
func (it *TokenBeaconUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenBeaconUpgraded)
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
		it.Event = new(TokenBeaconUpgraded)
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
func (it *TokenBeaconUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenBeaconUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenBeaconUpgraded represents a BeaconUpgraded event raised by the Token contract.
type TokenBeaconUpgraded struct {
	Beacon common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterBeaconUpgraded is a free log retrieval operation binding the contract event 0x1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e.
//
// Solidity: event BeaconUpgraded(address indexed beacon)
func (_Token *TokenFilterer) FilterBeaconUpgraded(opts *bind.FilterOpts, beacon []common.Address) (*TokenBeaconUpgradedIterator, error) {

	var beaconRule []interface{}
	for _, beaconItem := range beacon {
		beaconRule = append(beaconRule, beaconItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "BeaconUpgraded", beaconRule)
	if err != nil {
		return nil, err
	}
	return &TokenBeaconUpgradedIterator{contract: _Token.contract, event: "BeaconUpgraded", logs: logs, sub: sub}, nil
}

// WatchBeaconUpgraded is a free log subscription operation binding the contract event 0x1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e.
//
// Solidity: event BeaconUpgraded(address indexed beacon)
func (_Token *TokenFilterer) WatchBeaconUpgraded(opts *bind.WatchOpts, sink chan<- *TokenBeaconUpgraded, beacon []common.Address) (event.Subscription, error) {

	var beaconRule []interface{}
	for _, beaconItem := range beacon {
		beaconRule = append(beaconRule, beaconItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "BeaconUpgraded", beaconRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenBeaconUpgraded)
				if err := _Token.contract.UnpackLog(event, "BeaconUpgraded", log); err != nil {
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
func (_Token *TokenFilterer) ParseBeaconUpgraded(log types.Log) (*TokenBeaconUpgraded, error) {
	event := new(TokenBeaconUpgraded)
	if err := _Token.contract.UnpackLog(event, "BeaconUpgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenERC20TokenSetIterator is returned from FilterERC20TokenSet and is used to iterate over the raw logs and unpacked data for ERC20TokenSet events raised by the Token contract.
type TokenERC20TokenSetIterator struct {
	Event *TokenERC20TokenSet // Event containing the contract specifics and raw log

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
func (it *TokenERC20TokenSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenERC20TokenSet)
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
		it.Event = new(TokenERC20TokenSet)
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
func (it *TokenERC20TokenSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenERC20TokenSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenERC20TokenSet represents a ERC20TokenSet event raised by the Token contract.
type TokenERC20TokenSet struct {
	Token  common.Address
	Active bool
	Rate   *big.Int
	Unit   *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterERC20TokenSet is a free log retrieval operation binding the contract event 0xa3498854cfd3a9b3b6d13de4ddbaf25801c97667070992f7e7ba26c5c60a7d6d.
//
// Solidity: event ERC20TokenSet(address indexed token, bool active, uint256 rate, uint256 unit)
func (_Token *TokenFilterer) FilterERC20TokenSet(opts *bind.FilterOpts, token []common.Address) (*TokenERC20TokenSetIterator, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "ERC20TokenSet", tokenRule)
	if err != nil {
		return nil, err
	}
	return &TokenERC20TokenSetIterator{contract: _Token.contract, event: "ERC20TokenSet", logs: logs, sub: sub}, nil
}

// WatchERC20TokenSet is a free log subscription operation binding the contract event 0xa3498854cfd3a9b3b6d13de4ddbaf25801c97667070992f7e7ba26c5c60a7d6d.
//
// Solidity: event ERC20TokenSet(address indexed token, bool active, uint256 rate, uint256 unit)
func (_Token *TokenFilterer) WatchERC20TokenSet(opts *bind.WatchOpts, sink chan<- *TokenERC20TokenSet, token []common.Address) (event.Subscription, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "ERC20TokenSet", tokenRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenERC20TokenSet)
				if err := _Token.contract.UnpackLog(event, "ERC20TokenSet", log); err != nil {
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

// ParseERC20TokenSet is a log parse operation binding the contract event 0xa3498854cfd3a9b3b6d13de4ddbaf25801c97667070992f7e7ba26c5c60a7d6d.
//
// Solidity: event ERC20TokenSet(address indexed token, bool active, uint256 rate, uint256 unit)
func (_Token *TokenFilterer) ParseERC20TokenSet(log types.Log) (*TokenERC20TokenSet, error) {
	event := new(TokenERC20TokenSet)
	if err := _Token.contract.UnpackLog(event, "ERC20TokenSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the Token contract.
type TokenInitializedIterator struct {
	Event *TokenInitialized // Event containing the contract specifics and raw log

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
func (it *TokenInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenInitialized)
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
		it.Event = new(TokenInitialized)
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
func (it *TokenInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenInitialized represents a Initialized event raised by the Token contract.
type TokenInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_Token *TokenFilterer) FilterInitialized(opts *bind.FilterOpts) (*TokenInitializedIterator, error) {

	logs, sub, err := _Token.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &TokenInitializedIterator{contract: _Token.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_Token *TokenFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *TokenInitialized) (event.Subscription, error) {

	logs, sub, err := _Token.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenInitialized)
				if err := _Token.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_Token *TokenFilterer) ParseInitialized(log types.Log) (*TokenInitialized, error) {
	event := new(TokenInitialized)
	if err := _Token.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenNativeActiveSetIterator is returned from FilterNativeActiveSet and is used to iterate over the raw logs and unpacked data for NativeActiveSet events raised by the Token contract.
type TokenNativeActiveSetIterator struct {
	Event *TokenNativeActiveSet // Event containing the contract specifics and raw log

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
func (it *TokenNativeActiveSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenNativeActiveSet)
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
		it.Event = new(TokenNativeActiveSet)
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
func (it *TokenNativeActiveSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenNativeActiveSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenNativeActiveSet represents a NativeActiveSet event raised by the Token contract.
type TokenNativeActiveSet struct {
	Active bool
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterNativeActiveSet is a free log retrieval operation binding the contract event 0x4bd81ceea565af31b40ca7dedbec315849dd01d8b73b13574a1cef611c630733.
//
// Solidity: event NativeActiveSet(bool active)
func (_Token *TokenFilterer) FilterNativeActiveSet(opts *bind.FilterOpts) (*TokenNativeActiveSetIterator, error) {

	logs, sub, err := _Token.contract.FilterLogs(opts, "NativeActiveSet")
	if err != nil {
		return nil, err
	}
	return &TokenNativeActiveSetIterator{contract: _Token.contract, event: "NativeActiveSet", logs: logs, sub: sub}, nil
}

// WatchNativeActiveSet is a free log subscription operation binding the contract event 0x4bd81ceea565af31b40ca7dedbec315849dd01d8b73b13574a1cef611c630733.
//
// Solidity: event NativeActiveSet(bool active)
func (_Token *TokenFilterer) WatchNativeActiveSet(opts *bind.WatchOpts, sink chan<- *TokenNativeActiveSet) (event.Subscription, error) {

	logs, sub, err := _Token.contract.WatchLogs(opts, "NativeActiveSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenNativeActiveSet)
				if err := _Token.contract.UnpackLog(event, "NativeActiveSet", log); err != nil {
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

// ParseNativeActiveSet is a log parse operation binding the contract event 0x4bd81ceea565af31b40ca7dedbec315849dd01d8b73b13574a1cef611c630733.
//
// Solidity: event NativeActiveSet(bool active)
func (_Token *TokenFilterer) ParseNativeActiveSet(log types.Log) (*TokenNativeActiveSet, error) {
	event := new(TokenNativeActiveSet)
	if err := _Token.contract.UnpackLog(event, "NativeActiveSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Token contract.
type TokenOwnershipTransferredIterator struct {
	Event *TokenOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *TokenOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenOwnershipTransferred)
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
		it.Event = new(TokenOwnershipTransferred)
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
func (it *TokenOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenOwnershipTransferred represents a OwnershipTransferred event raised by the Token contract.
type TokenOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Token *TokenFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*TokenOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &TokenOwnershipTransferredIterator{contract: _Token.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Token *TokenFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *TokenOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenOwnershipTransferred)
				if err := _Token.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_Token *TokenFilterer) ParseOwnershipTransferred(log types.Log) (*TokenOwnershipTransferred, error) {
	event := new(TokenOwnershipTransferred)
	if err := _Token.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenRateUpdatedIterator is returned from FilterRateUpdated and is used to iterate over the raw logs and unpacked data for RateUpdated events raised by the Token contract.
type TokenRateUpdatedIterator struct {
	Event *TokenRateUpdated // Event containing the contract specifics and raw log

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
func (it *TokenRateUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenRateUpdated)
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
		it.Event = new(TokenRateUpdated)
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
func (it *TokenRateUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenRateUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenRateUpdated represents a RateUpdated event raised by the Token contract.
type TokenRateUpdated struct {
	OldRate *big.Int
	NewRate *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRateUpdated is a free log retrieval operation binding the contract event 0xb38780ddde1f073d91c150de2696f3f7085883648ba21cc5ef01029cb21d1916.
//
// Solidity: event RateUpdated(uint256 oldRate, uint256 newRate)
func (_Token *TokenFilterer) FilterRateUpdated(opts *bind.FilterOpts) (*TokenRateUpdatedIterator, error) {

	logs, sub, err := _Token.contract.FilterLogs(opts, "RateUpdated")
	if err != nil {
		return nil, err
	}
	return &TokenRateUpdatedIterator{contract: _Token.contract, event: "RateUpdated", logs: logs, sub: sub}, nil
}

// WatchRateUpdated is a free log subscription operation binding the contract event 0xb38780ddde1f073d91c150de2696f3f7085883648ba21cc5ef01029cb21d1916.
//
// Solidity: event RateUpdated(uint256 oldRate, uint256 newRate)
func (_Token *TokenFilterer) WatchRateUpdated(opts *bind.WatchOpts, sink chan<- *TokenRateUpdated) (event.Subscription, error) {

	logs, sub, err := _Token.contract.WatchLogs(opts, "RateUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenRateUpdated)
				if err := _Token.contract.UnpackLog(event, "RateUpdated", log); err != nil {
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

// ParseRateUpdated is a log parse operation binding the contract event 0xb38780ddde1f073d91c150de2696f3f7085883648ba21cc5ef01029cb21d1916.
//
// Solidity: event RateUpdated(uint256 oldRate, uint256 newRate)
func (_Token *TokenFilterer) ParseRateUpdated(log types.Log) (*TokenRateUpdated, error) {
	event := new(TokenRateUpdated)
	if err := _Token.contract.UnpackLog(event, "RateUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenRechargeFeeRateSetIterator is returned from FilterRechargeFeeRateSet and is used to iterate over the raw logs and unpacked data for RechargeFeeRateSet events raised by the Token contract.
type TokenRechargeFeeRateSetIterator struct {
	Event *TokenRechargeFeeRateSet // Event containing the contract specifics and raw log

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
func (it *TokenRechargeFeeRateSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenRechargeFeeRateSet)
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
		it.Event = new(TokenRechargeFeeRateSet)
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
func (it *TokenRechargeFeeRateSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenRechargeFeeRateSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenRechargeFeeRateSet represents a RechargeFeeRateSet event raised by the Token contract.
type TokenRechargeFeeRateSet struct {
	Rate *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterRechargeFeeRateSet is a free log retrieval operation binding the contract event 0x2d980ed97eb2d663625b9e4cc15c1d7cff84ba7d6661106b9007876e8a34c0aa.
//
// Solidity: event RechargeFeeRateSet(uint256 rate)
func (_Token *TokenFilterer) FilterRechargeFeeRateSet(opts *bind.FilterOpts) (*TokenRechargeFeeRateSetIterator, error) {

	logs, sub, err := _Token.contract.FilterLogs(opts, "RechargeFeeRateSet")
	if err != nil {
		return nil, err
	}
	return &TokenRechargeFeeRateSetIterator{contract: _Token.contract, event: "RechargeFeeRateSet", logs: logs, sub: sub}, nil
}

// WatchRechargeFeeRateSet is a free log subscription operation binding the contract event 0x2d980ed97eb2d663625b9e4cc15c1d7cff84ba7d6661106b9007876e8a34c0aa.
//
// Solidity: event RechargeFeeRateSet(uint256 rate)
func (_Token *TokenFilterer) WatchRechargeFeeRateSet(opts *bind.WatchOpts, sink chan<- *TokenRechargeFeeRateSet) (event.Subscription, error) {

	logs, sub, err := _Token.contract.WatchLogs(opts, "RechargeFeeRateSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenRechargeFeeRateSet)
				if err := _Token.contract.UnpackLog(event, "RechargeFeeRateSet", log); err != nil {
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

// ParseRechargeFeeRateSet is a log parse operation binding the contract event 0x2d980ed97eb2d663625b9e4cc15c1d7cff84ba7d6661106b9007876e8a34c0aa.
//
// Solidity: event RechargeFeeRateSet(uint256 rate)
func (_Token *TokenFilterer) ParseRechargeFeeRateSet(log types.Log) (*TokenRechargeFeeRateSet, error) {
	event := new(TokenRechargeFeeRateSet)
	if err := _Token.contract.UnpackLog(event, "RechargeFeeRateSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenRechargedIterator is returned from FilterRecharged and is used to iterate over the raw logs and unpacked data for Recharged events raised by the Token contract.
type TokenRechargedIterator struct {
	Event *TokenRecharged // Event containing the contract specifics and raw log

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
func (it *TokenRechargedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenRecharged)
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
		it.Event = new(TokenRecharged)
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
func (it *TokenRechargedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenRechargedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenRecharged represents a Recharged event raised by the Token contract.
type TokenRecharged struct {
	User      common.Address
	Amount    *big.Int
	Points    *big.Int
	Timestamp *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterRecharged is a free log retrieval operation binding the contract event 0x309666ab835daa82b4efb4335ea881921a4294306428f580a2f0378bc7525aac.
//
// Solidity: event Recharged(address indexed user, uint256 amount, uint256 points, uint256 timestamp)
func (_Token *TokenFilterer) FilterRecharged(opts *bind.FilterOpts, user []common.Address) (*TokenRechargedIterator, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "Recharged", userRule)
	if err != nil {
		return nil, err
	}
	return &TokenRechargedIterator{contract: _Token.contract, event: "Recharged", logs: logs, sub: sub}, nil
}

// WatchRecharged is a free log subscription operation binding the contract event 0x309666ab835daa82b4efb4335ea881921a4294306428f580a2f0378bc7525aac.
//
// Solidity: event Recharged(address indexed user, uint256 amount, uint256 points, uint256 timestamp)
func (_Token *TokenFilterer) WatchRecharged(opts *bind.WatchOpts, sink chan<- *TokenRecharged, user []common.Address) (event.Subscription, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "Recharged", userRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenRecharged)
				if err := _Token.contract.UnpackLog(event, "Recharged", log); err != nil {
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

// ParseRecharged is a log parse operation binding the contract event 0x309666ab835daa82b4efb4335ea881921a4294306428f580a2f0378bc7525aac.
//
// Solidity: event Recharged(address indexed user, uint256 amount, uint256 points, uint256 timestamp)
func (_Token *TokenFilterer) ParseRecharged(log types.Log) (*TokenRecharged, error) {
	event := new(TokenRecharged)
	if err := _Token.contract.UnpackLog(event, "Recharged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenSubnetSetIterator is returned from FilterSubnetSet and is used to iterate over the raw logs and unpacked data for SubnetSet events raised by the Token contract.
type TokenSubnetSetIterator struct {
	Event *TokenSubnetSet // Event containing the contract specifics and raw log

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
func (it *TokenSubnetSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenSubnetSet)
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
		it.Event = new(TokenSubnetSet)
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
func (it *TokenSubnetSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenSubnetSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenSubnetSet represents a SubnetSet event raised by the Token contract.
type TokenSubnetSet struct {
	OldSubnet common.Address
	NewSubnet common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterSubnetSet is a free log retrieval operation binding the contract event 0x184fe0bbf0ea567e82d43a9a81b2d152abe7dda367eb5ea51e2eecf1467bfafe.
//
// Solidity: event SubnetSet(address indexed oldSubnet, address indexed newSubnet)
func (_Token *TokenFilterer) FilterSubnetSet(opts *bind.FilterOpts, oldSubnet []common.Address, newSubnet []common.Address) (*TokenSubnetSetIterator, error) {

	var oldSubnetRule []interface{}
	for _, oldSubnetItem := range oldSubnet {
		oldSubnetRule = append(oldSubnetRule, oldSubnetItem)
	}
	var newSubnetRule []interface{}
	for _, newSubnetItem := range newSubnet {
		newSubnetRule = append(newSubnetRule, newSubnetItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "SubnetSet", oldSubnetRule, newSubnetRule)
	if err != nil {
		return nil, err
	}
	return &TokenSubnetSetIterator{contract: _Token.contract, event: "SubnetSet", logs: logs, sub: sub}, nil
}

// WatchSubnetSet is a free log subscription operation binding the contract event 0x184fe0bbf0ea567e82d43a9a81b2d152abe7dda367eb5ea51e2eecf1467bfafe.
//
// Solidity: event SubnetSet(address indexed oldSubnet, address indexed newSubnet)
func (_Token *TokenFilterer) WatchSubnetSet(opts *bind.WatchOpts, sink chan<- *TokenSubnetSet, oldSubnet []common.Address, newSubnet []common.Address) (event.Subscription, error) {

	var oldSubnetRule []interface{}
	for _, oldSubnetItem := range oldSubnet {
		oldSubnetRule = append(oldSubnetRule, oldSubnetItem)
	}
	var newSubnetRule []interface{}
	for _, newSubnetItem := range newSubnet {
		newSubnetRule = append(newSubnetRule, newSubnetItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "SubnetSet", oldSubnetRule, newSubnetRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenSubnetSet)
				if err := _Token.contract.UnpackLog(event, "SubnetSet", log); err != nil {
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

// ParseSubnetSet is a log parse operation binding the contract event 0x184fe0bbf0ea567e82d43a9a81b2d152abe7dda367eb5ea51e2eecf1467bfafe.
//
// Solidity: event SubnetSet(address indexed oldSubnet, address indexed newSubnet)
func (_Token *TokenFilterer) ParseSubnetSet(log types.Log) (*TokenSubnetSet, error) {
	event := new(TokenSubnetSet)
	if err := _Token.contract.UnpackLog(event, "SubnetSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenTokenUnitUpdatedIterator is returned from FilterTokenUnitUpdated and is used to iterate over the raw logs and unpacked data for TokenUnitUpdated events raised by the Token contract.
type TokenTokenUnitUpdatedIterator struct {
	Event *TokenTokenUnitUpdated // Event containing the contract specifics and raw log

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
func (it *TokenTokenUnitUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenTokenUnitUpdated)
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
		it.Event = new(TokenTokenUnitUpdated)
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
func (it *TokenTokenUnitUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenTokenUnitUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenTokenUnitUpdated represents a TokenUnitUpdated event raised by the Token contract.
type TokenTokenUnitUpdated struct {
	OldUnit *big.Int
	NewUnit *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterTokenUnitUpdated is a free log retrieval operation binding the contract event 0x16c0eecd5f6d771dc2f3e6f57b46ce4f5f2cfaca03ed935d8756f3bdce34bd6b.
//
// Solidity: event TokenUnitUpdated(uint256 oldUnit, uint256 newUnit)
func (_Token *TokenFilterer) FilterTokenUnitUpdated(opts *bind.FilterOpts) (*TokenTokenUnitUpdatedIterator, error) {

	logs, sub, err := _Token.contract.FilterLogs(opts, "TokenUnitUpdated")
	if err != nil {
		return nil, err
	}
	return &TokenTokenUnitUpdatedIterator{contract: _Token.contract, event: "TokenUnitUpdated", logs: logs, sub: sub}, nil
}

// WatchTokenUnitUpdated is a free log subscription operation binding the contract event 0x16c0eecd5f6d771dc2f3e6f57b46ce4f5f2cfaca03ed935d8756f3bdce34bd6b.
//
// Solidity: event TokenUnitUpdated(uint256 oldUnit, uint256 newUnit)
func (_Token *TokenFilterer) WatchTokenUnitUpdated(opts *bind.WatchOpts, sink chan<- *TokenTokenUnitUpdated) (event.Subscription, error) {

	logs, sub, err := _Token.contract.WatchLogs(opts, "TokenUnitUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenTokenUnitUpdated)
				if err := _Token.contract.UnpackLog(event, "TokenUnitUpdated", log); err != nil {
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

// ParseTokenUnitUpdated is a log parse operation binding the contract event 0x16c0eecd5f6d771dc2f3e6f57b46ce4f5f2cfaca03ed935d8756f3bdce34bd6b.
//
// Solidity: event TokenUnitUpdated(uint256 oldUnit, uint256 newUnit)
func (_Token *TokenFilterer) ParseTokenUnitUpdated(log types.Log) (*TokenTokenUnitUpdated, error) {
	event := new(TokenTokenUnitUpdated)
	if err := _Token.contract.UnpackLog(event, "TokenUnitUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenUpgradedIterator is returned from FilterUpgraded and is used to iterate over the raw logs and unpacked data for Upgraded events raised by the Token contract.
type TokenUpgradedIterator struct {
	Event *TokenUpgraded // Event containing the contract specifics and raw log

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
func (it *TokenUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenUpgraded)
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
		it.Event = new(TokenUpgraded)
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
func (it *TokenUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenUpgraded represents a Upgraded event raised by the Token contract.
type TokenUpgraded struct {
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterUpgraded is a free log retrieval operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_Token *TokenFilterer) FilterUpgraded(opts *bind.FilterOpts, implementation []common.Address) (*TokenUpgradedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &TokenUpgradedIterator{contract: _Token.contract, event: "Upgraded", logs: logs, sub: sub}, nil
}

// WatchUpgraded is a free log subscription operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_Token *TokenFilterer) WatchUpgraded(opts *bind.WatchOpts, sink chan<- *TokenUpgraded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenUpgraded)
				if err := _Token.contract.UnpackLog(event, "Upgraded", log); err != nil {
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
func (_Token *TokenFilterer) ParseUpgraded(log types.Log) (*TokenUpgraded, error) {
	event := new(TokenUpgraded)
	if err := _Token.contract.UnpackLog(event, "Upgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenWithdrawFeeRateSetIterator is returned from FilterWithdrawFeeRateSet and is used to iterate over the raw logs and unpacked data for WithdrawFeeRateSet events raised by the Token contract.
type TokenWithdrawFeeRateSetIterator struct {
	Event *TokenWithdrawFeeRateSet // Event containing the contract specifics and raw log

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
func (it *TokenWithdrawFeeRateSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenWithdrawFeeRateSet)
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
		it.Event = new(TokenWithdrawFeeRateSet)
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
func (it *TokenWithdrawFeeRateSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenWithdrawFeeRateSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenWithdrawFeeRateSet represents a WithdrawFeeRateSet event raised by the Token contract.
type TokenWithdrawFeeRateSet struct {
	Rate *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterWithdrawFeeRateSet is a free log retrieval operation binding the contract event 0x560f2197e4942a95021269d6a486230bd79f2f37d8ce202ae58cbdb19463f0a6.
//
// Solidity: event WithdrawFeeRateSet(uint256 rate)
func (_Token *TokenFilterer) FilterWithdrawFeeRateSet(opts *bind.FilterOpts) (*TokenWithdrawFeeRateSetIterator, error) {

	logs, sub, err := _Token.contract.FilterLogs(opts, "WithdrawFeeRateSet")
	if err != nil {
		return nil, err
	}
	return &TokenWithdrawFeeRateSetIterator{contract: _Token.contract, event: "WithdrawFeeRateSet", logs: logs, sub: sub}, nil
}

// WatchWithdrawFeeRateSet is a free log subscription operation binding the contract event 0x560f2197e4942a95021269d6a486230bd79f2f37d8ce202ae58cbdb19463f0a6.
//
// Solidity: event WithdrawFeeRateSet(uint256 rate)
func (_Token *TokenFilterer) WatchWithdrawFeeRateSet(opts *bind.WatchOpts, sink chan<- *TokenWithdrawFeeRateSet) (event.Subscription, error) {

	logs, sub, err := _Token.contract.WatchLogs(opts, "WithdrawFeeRateSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenWithdrawFeeRateSet)
				if err := _Token.contract.UnpackLog(event, "WithdrawFeeRateSet", log); err != nil {
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

// ParseWithdrawFeeRateSet is a log parse operation binding the contract event 0x560f2197e4942a95021269d6a486230bd79f2f37d8ce202ae58cbdb19463f0a6.
//
// Solidity: event WithdrawFeeRateSet(uint256 rate)
func (_Token *TokenFilterer) ParseWithdrawFeeRateSet(log types.Log) (*TokenWithdrawFeeRateSet, error) {
	event := new(TokenWithdrawFeeRateSet)
	if err := _Token.contract.UnpackLog(event, "WithdrawFeeRateSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenWithdrawalCancelledIterator is returned from FilterWithdrawalCancelled and is used to iterate over the raw logs and unpacked data for WithdrawalCancelled events raised by the Token contract.
type TokenWithdrawalCancelledIterator struct {
	Event *TokenWithdrawalCancelled // Event containing the contract specifics and raw log

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
func (it *TokenWithdrawalCancelledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenWithdrawalCancelled)
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
		it.Event = new(TokenWithdrawalCancelled)
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
func (it *TokenWithdrawalCancelledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenWithdrawalCancelledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenWithdrawalCancelled represents a WithdrawalCancelled event raised by the Token contract.
type TokenWithdrawalCancelled struct {
	Nonce uint64
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterWithdrawalCancelled is a free log retrieval operation binding the contract event 0x5381b5dfdd23d3afd9a020b901f9fe8a8d81faed504e034fb60f953681603302.
//
// Solidity: event WithdrawalCancelled(uint64 indexed nonce)
func (_Token *TokenFilterer) FilterWithdrawalCancelled(opts *bind.FilterOpts, nonce []uint64) (*TokenWithdrawalCancelledIterator, error) {

	var nonceRule []interface{}
	for _, nonceItem := range nonce {
		nonceRule = append(nonceRule, nonceItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "WithdrawalCancelled", nonceRule)
	if err != nil {
		return nil, err
	}
	return &TokenWithdrawalCancelledIterator{contract: _Token.contract, event: "WithdrawalCancelled", logs: logs, sub: sub}, nil
}

// WatchWithdrawalCancelled is a free log subscription operation binding the contract event 0x5381b5dfdd23d3afd9a020b901f9fe8a8d81faed504e034fb60f953681603302.
//
// Solidity: event WithdrawalCancelled(uint64 indexed nonce)
func (_Token *TokenFilterer) WatchWithdrawalCancelled(opts *bind.WatchOpts, sink chan<- *TokenWithdrawalCancelled, nonce []uint64) (event.Subscription, error) {

	var nonceRule []interface{}
	for _, nonceItem := range nonce {
		nonceRule = append(nonceRule, nonceItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "WithdrawalCancelled", nonceRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenWithdrawalCancelled)
				if err := _Token.contract.UnpackLog(event, "WithdrawalCancelled", log); err != nil {
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

// ParseWithdrawalCancelled is a log parse operation binding the contract event 0x5381b5dfdd23d3afd9a020b901f9fe8a8d81faed504e034fb60f953681603302.
//
// Solidity: event WithdrawalCancelled(uint64 indexed nonce)
func (_Token *TokenFilterer) ParseWithdrawalCancelled(log types.Log) (*TokenWithdrawalCancelled, error) {
	event := new(TokenWithdrawalCancelled)
	if err := _Token.contract.UnpackLog(event, "WithdrawalCancelled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenWithdrawalDoneIterator is returned from FilterWithdrawalDone and is used to iterate over the raw logs and unpacked data for WithdrawalDone events raised by the Token contract.
type TokenWithdrawalDoneIterator struct {
	Event *TokenWithdrawalDone // Event containing the contract specifics and raw log

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
func (it *TokenWithdrawalDoneIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenWithdrawalDone)
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
		it.Event = new(TokenWithdrawalDone)
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
func (it *TokenWithdrawalDoneIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenWithdrawalDoneIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenWithdrawalDone represents a WithdrawalDone event raised by the Token contract.
type TokenWithdrawalDone struct {
	Nonce uint64
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterWithdrawalDone is a free log retrieval operation binding the contract event 0xeb76c8e52d5e6f3be558e3b47e323e61ba19ce2ba7bee200e780fcc3576a9654.
//
// Solidity: event WithdrawalDone(uint64 indexed nonce)
func (_Token *TokenFilterer) FilterWithdrawalDone(opts *bind.FilterOpts, nonce []uint64) (*TokenWithdrawalDoneIterator, error) {

	var nonceRule []interface{}
	for _, nonceItem := range nonce {
		nonceRule = append(nonceRule, nonceItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "WithdrawalDone", nonceRule)
	if err != nil {
		return nil, err
	}
	return &TokenWithdrawalDoneIterator{contract: _Token.contract, event: "WithdrawalDone", logs: logs, sub: sub}, nil
}

// WatchWithdrawalDone is a free log subscription operation binding the contract event 0xeb76c8e52d5e6f3be558e3b47e323e61ba19ce2ba7bee200e780fcc3576a9654.
//
// Solidity: event WithdrawalDone(uint64 indexed nonce)
func (_Token *TokenFilterer) WatchWithdrawalDone(opts *bind.WatchOpts, sink chan<- *TokenWithdrawalDone, nonce []uint64) (event.Subscription, error) {

	var nonceRule []interface{}
	for _, nonceItem := range nonce {
		nonceRule = append(nonceRule, nonceItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "WithdrawalDone", nonceRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenWithdrawalDone)
				if err := _Token.contract.UnpackLog(event, "WithdrawalDone", log); err != nil {
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

// ParseWithdrawalDone is a log parse operation binding the contract event 0xeb76c8e52d5e6f3be558e3b47e323e61ba19ce2ba7bee200e780fcc3576a9654.
//
// Solidity: event WithdrawalDone(uint64 indexed nonce)
func (_Token *TokenFilterer) ParseWithdrawalDone(log types.Log) (*TokenWithdrawalDone, error) {
	event := new(TokenWithdrawalDone)
	if err := _Token.contract.UnpackLog(event, "WithdrawalDone", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenWithdrawnIterator is returned from FilterWithdrawn and is used to iterate over the raw logs and unpacked data for Withdrawn events raised by the Token contract.
type TokenWithdrawnIterator struct {
	Event *TokenWithdrawn // Event containing the contract specifics and raw log

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
func (it *TokenWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenWithdrawn)
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
		it.Event = new(TokenWithdrawn)
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
func (it *TokenWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenWithdrawn represents a Withdrawn event raised by the Token contract.
type TokenWithdrawn struct {
	Nonce  uint64
	User   common.Address
	Token  common.Address
	Amount *big.Int
	Status uint8
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterWithdrawn is a free log retrieval operation binding the contract event 0xf7253fb01fc17e98a4e5c66f36bf2c1cf179752301a29b1888123ad2ab3d6263.
//
// Solidity: event Withdrawn(uint64 indexed nonce, address indexed user, address token, uint256 amount, uint8 status)
func (_Token *TokenFilterer) FilterWithdrawn(opts *bind.FilterOpts, nonce []uint64, user []common.Address) (*TokenWithdrawnIterator, error) {

	var nonceRule []interface{}
	for _, nonceItem := range nonce {
		nonceRule = append(nonceRule, nonceItem)
	}
	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "Withdrawn", nonceRule, userRule)
	if err != nil {
		return nil, err
	}
	return &TokenWithdrawnIterator{contract: _Token.contract, event: "Withdrawn", logs: logs, sub: sub}, nil
}

// WatchWithdrawn is a free log subscription operation binding the contract event 0xf7253fb01fc17e98a4e5c66f36bf2c1cf179752301a29b1888123ad2ab3d6263.
//
// Solidity: event Withdrawn(uint64 indexed nonce, address indexed user, address token, uint256 amount, uint8 status)
func (_Token *TokenFilterer) WatchWithdrawn(opts *bind.WatchOpts, sink chan<- *TokenWithdrawn, nonce []uint64, user []common.Address) (event.Subscription, error) {

	var nonceRule []interface{}
	for _, nonceItem := range nonce {
		nonceRule = append(nonceRule, nonceItem)
	}
	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "Withdrawn", nonceRule, userRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenWithdrawn)
				if err := _Token.contract.UnpackLog(event, "Withdrawn", log); err != nil {
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

// ParseWithdrawn is a log parse operation binding the contract event 0xf7253fb01fc17e98a4e5c66f36bf2c1cf179752301a29b1888123ad2ab3d6263.
//
// Solidity: event Withdrawn(uint64 indexed nonce, address indexed user, address token, uint256 amount, uint8 status)
func (_Token *TokenFilterer) ParseWithdrawn(log types.Log) (*TokenWithdrawn, error) {
	event := new(TokenWithdrawn)
	if err := _Token.contract.UnpackLog(event, "Withdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
