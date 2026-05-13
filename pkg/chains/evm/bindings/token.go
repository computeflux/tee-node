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

// TokenMetaData contains all meta data concerning the Token contract.
var TokenMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldRate\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newRate\",\"type\":\"uint256\"}],\"name\":\"RateUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"ethAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"pointsAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"}],\"name\":\"Recharged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldSubnet\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newSubnet\",\"type\":\"address\"}],\"name\":\"SubnetSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"ethAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"pointsAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"}],\"name\":\"Withdrawn\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"balances\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"emergencyWithdraw\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rate\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"recharge\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newRate\",\"type\":\"uint256\"}],\"name\":\"setRate\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_subnet\",\"type\":\"address\"}],\"name\":\"setSubnet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"subnet\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"ethAmount\",\"type\":\"uint256\"}],\"name\":\"toPoints\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"ethAmount\",\"type\":\"uint256\"}],\"name\":\"withdraw\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
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

// Balances is a free data retrieval call binding the contract method 0x27e235e3.
//
// Solidity: function balances(address ) view returns(uint256)
func (_Token *TokenCaller) Balances(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "balances", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Balances is a free data retrieval call binding the contract method 0x27e235e3.
//
// Solidity: function balances(address ) view returns(uint256)
func (_Token *TokenSession) Balances(arg0 common.Address) (*big.Int, error) {
	return _Token.Contract.Balances(&_Token.CallOpts, arg0)
}

// Balances is a free data retrieval call binding the contract method 0x27e235e3.
//
// Solidity: function balances(address ) view returns(uint256)
func (_Token *TokenCallerSession) Balances(arg0 common.Address) (*big.Int, error) {
	return _Token.Contract.Balances(&_Token.CallOpts, arg0)
}

// GetBalance is a free data retrieval call binding the contract method 0xf8b2cb4f.
//
// Solidity: function getBalance(address user) view returns(uint256)
func (_Token *TokenCaller) GetBalance(opts *bind.CallOpts, user common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "getBalance", user)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetBalance is a free data retrieval call binding the contract method 0xf8b2cb4f.
//
// Solidity: function getBalance(address user) view returns(uint256)
func (_Token *TokenSession) GetBalance(user common.Address) (*big.Int, error) {
	return _Token.Contract.GetBalance(&_Token.CallOpts, user)
}

// GetBalance is a free data retrieval call binding the contract method 0xf8b2cb4f.
//
// Solidity: function getBalance(address user) view returns(uint256)
func (_Token *TokenCallerSession) GetBalance(user common.Address) (*big.Int, error) {
	return _Token.Contract.GetBalance(&_Token.CallOpts, user)
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
// Solidity: function toPoints(uint256 ethAmount) view returns(uint256)
func (_Token *TokenCaller) ToPoints(opts *bind.CallOpts, ethAmount *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Token.contract.Call(opts, &out, "toPoints", ethAmount)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ToPoints is a free data retrieval call binding the contract method 0xc718a401.
//
// Solidity: function toPoints(uint256 ethAmount) view returns(uint256)
func (_Token *TokenSession) ToPoints(ethAmount *big.Int) (*big.Int, error) {
	return _Token.Contract.ToPoints(&_Token.CallOpts, ethAmount)
}

// ToPoints is a free data retrieval call binding the contract method 0xc718a401.
//
// Solidity: function toPoints(uint256 ethAmount) view returns(uint256)
func (_Token *TokenCallerSession) ToPoints(ethAmount *big.Int) (*big.Int, error) {
	return _Token.Contract.ToPoints(&_Token.CallOpts, ethAmount)
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

// Withdraw is a paid mutator transaction binding the contract method 0xf3fef3a3.
//
// Solidity: function withdraw(address user, uint256 ethAmount) returns()
func (_Token *TokenTransactor) Withdraw(opts *bind.TransactOpts, user common.Address, ethAmount *big.Int) (*types.Transaction, error) {
	return _Token.contract.Transact(opts, "withdraw", user, ethAmount)
}

// Withdraw is a paid mutator transaction binding the contract method 0xf3fef3a3.
//
// Solidity: function withdraw(address user, uint256 ethAmount) returns()
func (_Token *TokenSession) Withdraw(user common.Address, ethAmount *big.Int) (*types.Transaction, error) {
	return _Token.Contract.Withdraw(&_Token.TransactOpts, user, ethAmount)
}

// Withdraw is a paid mutator transaction binding the contract method 0xf3fef3a3.
//
// Solidity: function withdraw(address user, uint256 ethAmount) returns()
func (_Token *TokenTransactorSession) Withdraw(user common.Address, ethAmount *big.Int) (*types.Transaction, error) {
	return _Token.Contract.Withdraw(&_Token.TransactOpts, user, ethAmount)
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
	User         common.Address
	EthAmount    *big.Int
	PointsAmount *big.Int
	Timestamp    *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRecharged is a free log retrieval operation binding the contract event 0x309666ab835daa82b4efb4335ea881921a4294306428f580a2f0378bc7525aac.
//
// Solidity: event Recharged(address indexed user, uint256 ethAmount, uint256 pointsAmount, uint256 timestamp)
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
// Solidity: event Recharged(address indexed user, uint256 ethAmount, uint256 pointsAmount, uint256 timestamp)
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
// Solidity: event Recharged(address indexed user, uint256 ethAmount, uint256 pointsAmount, uint256 timestamp)
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
	User         common.Address
	EthAmount    *big.Int
	PointsAmount *big.Int
	Timestamp    *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterWithdrawn is a free log retrieval operation binding the contract event 0x75e161b3e824b114fc1a33274bd7091918dd4e639cede50b78b15a4eea956a21.
//
// Solidity: event Withdrawn(address indexed user, uint256 ethAmount, uint256 pointsAmount, uint256 timestamp)
func (_Token *TokenFilterer) FilterWithdrawn(opts *bind.FilterOpts, user []common.Address) (*TokenWithdrawnIterator, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _Token.contract.FilterLogs(opts, "Withdrawn", userRule)
	if err != nil {
		return nil, err
	}
	return &TokenWithdrawnIterator{contract: _Token.contract, event: "Withdrawn", logs: logs, sub: sub}, nil
}

// WatchWithdrawn is a free log subscription operation binding the contract event 0x75e161b3e824b114fc1a33274bd7091918dd4e639cede50b78b15a4eea956a21.
//
// Solidity: event Withdrawn(address indexed user, uint256 ethAmount, uint256 pointsAmount, uint256 timestamp)
func (_Token *TokenFilterer) WatchWithdrawn(opts *bind.WatchOpts, sink chan<- *TokenWithdrawn, user []common.Address) (event.Subscription, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}

	logs, sub, err := _Token.contract.WatchLogs(opts, "Withdrawn", userRule)
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

// ParseWithdrawn is a log parse operation binding the contract event 0x75e161b3e824b114fc1a33274bd7091918dd4e639cede50b78b15a4eea956a21.
//
// Solidity: event Withdrawn(address indexed user, uint256 ethAmount, uint256 pointsAmount, uint256 timestamp)
func (_Token *TokenFilterer) ParseWithdrawn(log types.Log) (*TokenWithdrawn, error) {
	event := new(TokenWithdrawn)
	if err := _Token.contract.UnpackLog(event, "Withdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
