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

// BLSMetaData contains all meta data concerning the BLS contract.
var BLSMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_target\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"_g2Generator\",\"type\":\"bytes\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"BLSSignatureInvalid\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"CallFailed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidGeneratorLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidHmG1Length\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPubkeyLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidSignatureLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"MapFpToG1Failed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"MustCallByTarget\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"PairingCallFailed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"PubkeyNotSet\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"aggPubkey\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes[]\",\"name\":\"datas\",\"type\":\"bytes[]\"},{\"internalType\":\"bytes\",\"name\":\"negSig\",\"type\":\"bytes\"}],\"name\":\"batchSubmit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"g2Generator\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"_aggPubkey\",\"type\":\"bytes\"}],\"name\":\"setAggPubkey\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_target\",\"type\":\"address\"}],\"name\":\"setTarget\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"negSig\",\"type\":\"bytes\"}],\"name\":\"submit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"target\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// BLSABI is the input ABI used to generate the binding from.
// Deprecated: Use BLSMetaData.ABI instead.
var BLSABI = BLSMetaData.ABI

// BLS is an auto generated Go binding around an Ethereum contract.
type BLS struct {
	BLSCaller     // Read-only binding to the contract
	BLSTransactor // Write-only binding to the contract
	BLSFilterer   // Log filterer for contract events
}

// BLSCaller is an auto generated read-only Go binding around an Ethereum contract.
type BLSCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BLSTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BLSTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BLSFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BLSFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BLSSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BLSSession struct {
	Contract     *BLS              // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BLSCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BLSCallerSession struct {
	Contract *BLSCaller    // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// BLSTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BLSTransactorSession struct {
	Contract     *BLSTransactor    // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BLSRaw is an auto generated low-level Go binding around an Ethereum contract.
type BLSRaw struct {
	Contract *BLS // Generic contract binding to access the raw methods on
}

// BLSCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BLSCallerRaw struct {
	Contract *BLSCaller // Generic read-only contract binding to access the raw methods on
}

// BLSTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BLSTransactorRaw struct {
	Contract *BLSTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBLS creates a new instance of BLS, bound to a specific deployed contract.
func NewBLS(address common.Address, backend bind.ContractBackend) (*BLS, error) {
	contract, err := bindBLS(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BLS{BLSCaller: BLSCaller{contract: contract}, BLSTransactor: BLSTransactor{contract: contract}, BLSFilterer: BLSFilterer{contract: contract}}, nil
}

// NewBLSCaller creates a new read-only instance of BLS, bound to a specific deployed contract.
func NewBLSCaller(address common.Address, caller bind.ContractCaller) (*BLSCaller, error) {
	contract, err := bindBLS(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BLSCaller{contract: contract}, nil
}

// NewBLSTransactor creates a new write-only instance of BLS, bound to a specific deployed contract.
func NewBLSTransactor(address common.Address, transactor bind.ContractTransactor) (*BLSTransactor, error) {
	contract, err := bindBLS(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BLSTransactor{contract: contract}, nil
}

// NewBLSFilterer creates a new log filterer instance of BLS, bound to a specific deployed contract.
func NewBLSFilterer(address common.Address, filterer bind.ContractFilterer) (*BLSFilterer, error) {
	contract, err := bindBLS(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BLSFilterer{contract: contract}, nil
}

// bindBLS binds a generic wrapper to an already deployed contract.
func bindBLS(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BLSMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BLS *BLSRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BLS.Contract.BLSCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BLS *BLSRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BLS.Contract.BLSTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BLS *BLSRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BLS.Contract.BLSTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BLS *BLSCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BLS.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BLS *BLSTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BLS.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BLS *BLSTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BLS.Contract.contract.Transact(opts, method, params...)
}

// AggPubkey is a free data retrieval call binding the contract method 0xb683f56d.
//
// Solidity: function aggPubkey() view returns(bytes)
func (_BLS *BLSCaller) AggPubkey(opts *bind.CallOpts) ([]byte, error) {
	var out []interface{}
	err := _BLS.contract.Call(opts, &out, "aggPubkey")

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// AggPubkey is a free data retrieval call binding the contract method 0xb683f56d.
//
// Solidity: function aggPubkey() view returns(bytes)
func (_BLS *BLSSession) AggPubkey() ([]byte, error) {
	return _BLS.Contract.AggPubkey(&_BLS.CallOpts)
}

// AggPubkey is a free data retrieval call binding the contract method 0xb683f56d.
//
// Solidity: function aggPubkey() view returns(bytes)
func (_BLS *BLSCallerSession) AggPubkey() ([]byte, error) {
	return _BLS.Contract.AggPubkey(&_BLS.CallOpts)
}

// G2Generator is a free data retrieval call binding the contract method 0xa97b3db3.
//
// Solidity: function g2Generator() view returns(bytes)
func (_BLS *BLSCaller) G2Generator(opts *bind.CallOpts) ([]byte, error) {
	var out []interface{}
	err := _BLS.contract.Call(opts, &out, "g2Generator")

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// G2Generator is a free data retrieval call binding the contract method 0xa97b3db3.
//
// Solidity: function g2Generator() view returns(bytes)
func (_BLS *BLSSession) G2Generator() ([]byte, error) {
	return _BLS.Contract.G2Generator(&_BLS.CallOpts)
}

// G2Generator is a free data retrieval call binding the contract method 0xa97b3db3.
//
// Solidity: function g2Generator() view returns(bytes)
func (_BLS *BLSCallerSession) G2Generator() ([]byte, error) {
	return _BLS.Contract.G2Generator(&_BLS.CallOpts)
}

// Target is a free data retrieval call binding the contract method 0xd4b83992.
//
// Solidity: function target() view returns(address)
func (_BLS *BLSCaller) Target(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _BLS.contract.Call(opts, &out, "target")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Target is a free data retrieval call binding the contract method 0xd4b83992.
//
// Solidity: function target() view returns(address)
func (_BLS *BLSSession) Target() (common.Address, error) {
	return _BLS.Contract.Target(&_BLS.CallOpts)
}

// Target is a free data retrieval call binding the contract method 0xd4b83992.
//
// Solidity: function target() view returns(address)
func (_BLS *BLSCallerSession) Target() (common.Address, error) {
	return _BLS.Contract.Target(&_BLS.CallOpts)
}

// BatchSubmit is a paid mutator transaction binding the contract method 0x408a9a2c.
//
// Solidity: function batchSubmit(bytes[] datas, bytes negSig) returns()
func (_BLS *BLSTransactor) BatchSubmit(opts *bind.TransactOpts, datas [][]byte, negSig []byte) (*types.Transaction, error) {
	return _BLS.contract.Transact(opts, "batchSubmit", datas, negSig)
}

// BatchSubmit is a paid mutator transaction binding the contract method 0x408a9a2c.
//
// Solidity: function batchSubmit(bytes[] datas, bytes negSig) returns()
func (_BLS *BLSSession) BatchSubmit(datas [][]byte, negSig []byte) (*types.Transaction, error) {
	return _BLS.Contract.BatchSubmit(&_BLS.TransactOpts, datas, negSig)
}

// BatchSubmit is a paid mutator transaction binding the contract method 0x408a9a2c.
//
// Solidity: function batchSubmit(bytes[] datas, bytes negSig) returns()
func (_BLS *BLSTransactorSession) BatchSubmit(datas [][]byte, negSig []byte) (*types.Transaction, error) {
	return _BLS.Contract.BatchSubmit(&_BLS.TransactOpts, datas, negSig)
}

// SetAggPubkey is a paid mutator transaction binding the contract method 0x6b78491b.
//
// Solidity: function setAggPubkey(bytes _aggPubkey) returns()
func (_BLS *BLSTransactor) SetAggPubkey(opts *bind.TransactOpts, _aggPubkey []byte) (*types.Transaction, error) {
	return _BLS.contract.Transact(opts, "setAggPubkey", _aggPubkey)
}

// SetAggPubkey is a paid mutator transaction binding the contract method 0x6b78491b.
//
// Solidity: function setAggPubkey(bytes _aggPubkey) returns()
func (_BLS *BLSSession) SetAggPubkey(_aggPubkey []byte) (*types.Transaction, error) {
	return _BLS.Contract.SetAggPubkey(&_BLS.TransactOpts, _aggPubkey)
}

// SetAggPubkey is a paid mutator transaction binding the contract method 0x6b78491b.
//
// Solidity: function setAggPubkey(bytes _aggPubkey) returns()
func (_BLS *BLSTransactorSession) SetAggPubkey(_aggPubkey []byte) (*types.Transaction, error) {
	return _BLS.Contract.SetAggPubkey(&_BLS.TransactOpts, _aggPubkey)
}

// SetTarget is a paid mutator transaction binding the contract method 0x776d1a01.
//
// Solidity: function setTarget(address _target) returns()
func (_BLS *BLSTransactor) SetTarget(opts *bind.TransactOpts, _target common.Address) (*types.Transaction, error) {
	return _BLS.contract.Transact(opts, "setTarget", _target)
}

// SetTarget is a paid mutator transaction binding the contract method 0x776d1a01.
//
// Solidity: function setTarget(address _target) returns()
func (_BLS *BLSSession) SetTarget(_target common.Address) (*types.Transaction, error) {
	return _BLS.Contract.SetTarget(&_BLS.TransactOpts, _target)
}

// SetTarget is a paid mutator transaction binding the contract method 0x776d1a01.
//
// Solidity: function setTarget(address _target) returns()
func (_BLS *BLSTransactorSession) SetTarget(_target common.Address) (*types.Transaction, error) {
	return _BLS.Contract.SetTarget(&_BLS.TransactOpts, _target)
}

// Submit is a paid mutator transaction binding the contract method 0x39b973ad.
//
// Solidity: function submit(bytes data, bytes negSig) returns()
func (_BLS *BLSTransactor) Submit(opts *bind.TransactOpts, data []byte, negSig []byte) (*types.Transaction, error) {
	return _BLS.contract.Transact(opts, "submit", data, negSig)
}

// Submit is a paid mutator transaction binding the contract method 0x39b973ad.
//
// Solidity: function submit(bytes data, bytes negSig) returns()
func (_BLS *BLSSession) Submit(data []byte, negSig []byte) (*types.Transaction, error) {
	return _BLS.Contract.Submit(&_BLS.TransactOpts, data, negSig)
}

// Submit is a paid mutator transaction binding the contract method 0x39b973ad.
//
// Solidity: function submit(bytes data, bytes negSig) returns()
func (_BLS *BLSTransactorSession) Submit(data []byte, negSig []byte) (*types.Transaction, error) {
	return _BLS.Contract.Submit(&_BLS.TransactOpts, data, negSig)
}
