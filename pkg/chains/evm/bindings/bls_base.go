// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.
//
// Source: revive-contract/evms contracts/BaseBLS.sol (Base / OP-Stack BLS
// gateway, submit(data, hmG1, negSig); hmG1 supplied by the signer).
// Regenerate with hack/evm/contract.sh build.

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

// BaseBLSMetaData contains all meta data concerning the BaseBLS contract.
var BaseBLSMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_target\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"_g2Generator\",\"type\":\"bytes\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"BLSSignatureInvalid\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"CallFailed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidGeneratorLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidHmG1Length\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPubkeyLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidSignatureLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"MustCallByTarget\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"PairingCallFailed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"PubkeyNotSet\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"aggPubkey\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes[]\",\"name\":\"datas\",\"type\":\"bytes[]\"},{\"internalType\":\"bytes\",\"name\":\"hmG1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"negSig\",\"type\":\"bytes\"}],\"name\":\"batchSubmit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"g2Generator\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"_aggPubkey\",\"type\":\"bytes\"}],\"name\":\"setAggPubkey\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"hmG1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"negSig\",\"type\":\"bytes\"}],\"name\":\"submit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"target\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// BaseBLSABI is the input ABI used to generate the binding from.
// Deprecated: Use BaseBLSMetaData.ABI instead.
var BaseBLSABI = BaseBLSMetaData.ABI

// BaseBLS is an auto generated Go binding around an Ethereum contract.
type BaseBLS struct {
	BaseBLSCaller     // Read-only binding to the contract
	BaseBLSTransactor // Write-only binding to the contract
	BaseBLSFilterer   // Log filterer for contract events
}

// BaseBLSCaller is an auto generated read-only Go binding around an Ethereum contract.
type BaseBLSCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BaseBLSTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BaseBLSTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BaseBLSFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BaseBLSFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BaseBLSSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BaseBLSSession struct {
	Contract     *BaseBLS          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BaseBLSCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BaseBLSCallerSession struct {
	Contract *BaseBLSCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// BaseBLSTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BaseBLSTransactorSession struct {
	Contract     *BaseBLSTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// BaseBLSRaw is an auto generated low-level Go binding around an Ethereum contract.
type BaseBLSRaw struct {
	Contract *BaseBLS // Generic contract binding to access the raw methods on
}

// BaseBLSCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BaseBLSCallerRaw struct {
	Contract *BaseBLSCaller // Generic read-only contract binding to access the raw methods on
}

// BaseBLSTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BaseBLSTransactorRaw struct {
	Contract *BaseBLSTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBaseBLS creates a new instance of BaseBLS, bound to a specific deployed contract.
func NewBaseBLS(address common.Address, backend bind.ContractBackend) (*BaseBLS, error) {
	contract, err := bindBaseBLS(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BaseBLS{BaseBLSCaller: BaseBLSCaller{contract: contract}, BaseBLSTransactor: BaseBLSTransactor{contract: contract}, BaseBLSFilterer: BaseBLSFilterer{contract: contract}}, nil
}

// NewBaseBLSCaller creates a new read-only instance of BaseBLS, bound to a specific deployed contract.
func NewBaseBLSCaller(address common.Address, caller bind.ContractCaller) (*BaseBLSCaller, error) {
	contract, err := bindBaseBLS(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BaseBLSCaller{contract: contract}, nil
}

// NewBaseBLSTransactor creates a new write-only instance of BaseBLS, bound to a specific deployed contract.
func NewBaseBLSTransactor(address common.Address, transactor bind.ContractTransactor) (*BaseBLSTransactor, error) {
	contract, err := bindBaseBLS(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BaseBLSTransactor{contract: contract}, nil
}

// NewBaseBLSFilterer creates a new log filterer instance of BaseBLS, bound to a specific deployed contract.
func NewBaseBLSFilterer(address common.Address, filterer bind.ContractFilterer) (*BaseBLSFilterer, error) {
	contract, err := bindBaseBLS(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BaseBLSFilterer{contract: contract}, nil
}

// bindBaseBLS binds a generic wrapper to an already deployed contract.
func bindBaseBLS(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BaseBLSMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BaseBLS *BaseBLSRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BaseBLS.Contract.BaseBLSCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BaseBLS *BaseBLSRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BaseBLS.Contract.BaseBLSTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BaseBLS *BaseBLSRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BaseBLS.Contract.BaseBLSTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BaseBLS *BaseBLSCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BaseBLS.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BaseBLS *BaseBLSTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BaseBLS.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BaseBLS *BaseBLSTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BaseBLS.Contract.contract.Transact(opts, method, params...)
}

// AggPubkey is a free data retrieval call binding the contract method 0xb683f56d.
//
// Solidity: function aggPubkey() view returns(bytes)
func (_BaseBLS *BaseBLSCaller) AggPubkey(opts *bind.CallOpts) ([]byte, error) {
	var out []interface{}
	err := _BaseBLS.contract.Call(opts, &out, "aggPubkey")

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// AggPubkey is a free data retrieval call binding the contract method 0xb683f56d.
//
// Solidity: function aggPubkey() view returns(bytes)
func (_BaseBLS *BaseBLSSession) AggPubkey() ([]byte, error) {
	return _BaseBLS.Contract.AggPubkey(&_BaseBLS.CallOpts)
}

// AggPubkey is a free data retrieval call binding the contract method 0xb683f56d.
//
// Solidity: function aggPubkey() view returns(bytes)
func (_BaseBLS *BaseBLSCallerSession) AggPubkey() ([]byte, error) {
	return _BaseBLS.Contract.AggPubkey(&_BaseBLS.CallOpts)
}

// G2Generator is a free data retrieval call binding the contract method 0xa97b3db3.
//
// Solidity: function g2Generator() view returns(bytes)
func (_BaseBLS *BaseBLSCaller) G2Generator(opts *bind.CallOpts) ([]byte, error) {
	var out []interface{}
	err := _BaseBLS.contract.Call(opts, &out, "g2Generator")

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// G2Generator is a free data retrieval call binding the contract method 0xa97b3db3.
//
// Solidity: function g2Generator() view returns(bytes)
func (_BaseBLS *BaseBLSSession) G2Generator() ([]byte, error) {
	return _BaseBLS.Contract.G2Generator(&_BaseBLS.CallOpts)
}

// G2Generator is a free data retrieval call binding the contract method 0xa97b3db3.
//
// Solidity: function g2Generator() view returns(bytes)
func (_BaseBLS *BaseBLSCallerSession) G2Generator() ([]byte, error) {
	return _BaseBLS.Contract.G2Generator(&_BaseBLS.CallOpts)
}

// Target is a free data retrieval call binding the contract method 0xd4b83992.
//
// Solidity: function target() view returns(address)
func (_BaseBLS *BaseBLSCaller) Target(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _BaseBLS.contract.Call(opts, &out, "target")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Target is a free data retrieval call binding the contract method 0xd4b83992.
//
// Solidity: function target() view returns(address)
func (_BaseBLS *BaseBLSSession) Target() (common.Address, error) {
	return _BaseBLS.Contract.Target(&_BaseBLS.CallOpts)
}

// Target is a free data retrieval call binding the contract method 0xd4b83992.
//
// Solidity: function target() view returns(address)
func (_BaseBLS *BaseBLSCallerSession) Target() (common.Address, error) {
	return _BaseBLS.Contract.Target(&_BaseBLS.CallOpts)
}

// BatchSubmit is a paid mutator transaction binding the contract method 0x97e7b5d3.
//
// Solidity: function batchSubmit(bytes[] datas, bytes hmG1, bytes negSig) returns()
func (_BaseBLS *BaseBLSTransactor) BatchSubmit(opts *bind.TransactOpts, datas [][]byte, hmG1 []byte, negSig []byte) (*types.Transaction, error) {
	return _BaseBLS.contract.Transact(opts, "batchSubmit", datas, hmG1, negSig)
}

// BatchSubmit is a paid mutator transaction binding the contract method 0x97e7b5d3.
//
// Solidity: function batchSubmit(bytes[] datas, bytes hmG1, bytes negSig) returns()
func (_BaseBLS *BaseBLSSession) BatchSubmit(datas [][]byte, hmG1 []byte, negSig []byte) (*types.Transaction, error) {
	return _BaseBLS.Contract.BatchSubmit(&_BaseBLS.TransactOpts, datas, hmG1, negSig)
}

// BatchSubmit is a paid mutator transaction binding the contract method 0x97e7b5d3.
//
// Solidity: function batchSubmit(bytes[] datas, bytes hmG1, bytes negSig) returns()
func (_BaseBLS *BaseBLSTransactorSession) BatchSubmit(datas [][]byte, hmG1 []byte, negSig []byte) (*types.Transaction, error) {
	return _BaseBLS.Contract.BatchSubmit(&_BaseBLS.TransactOpts, datas, hmG1, negSig)
}

// SetAggPubkey is a paid mutator transaction binding the contract method 0x6b78491b.
//
// Solidity: function setAggPubkey(bytes _aggPubkey) returns()
func (_BaseBLS *BaseBLSTransactor) SetAggPubkey(opts *bind.TransactOpts, _aggPubkey []byte) (*types.Transaction, error) {
	return _BaseBLS.contract.Transact(opts, "setAggPubkey", _aggPubkey)
}

// SetAggPubkey is a paid mutator transaction binding the contract method 0x6b78491b.
//
// Solidity: function setAggPubkey(bytes _aggPubkey) returns()
func (_BaseBLS *BaseBLSSession) SetAggPubkey(_aggPubkey []byte) (*types.Transaction, error) {
	return _BaseBLS.Contract.SetAggPubkey(&_BaseBLS.TransactOpts, _aggPubkey)
}

// SetAggPubkey is a paid mutator transaction binding the contract method 0x6b78491b.
//
// Solidity: function setAggPubkey(bytes _aggPubkey) returns()
func (_BaseBLS *BaseBLSTransactorSession) SetAggPubkey(_aggPubkey []byte) (*types.Transaction, error) {
	return _BaseBLS.Contract.SetAggPubkey(&_BaseBLS.TransactOpts, _aggPubkey)
}

// Submit is a paid mutator transaction binding the contract method 0xd8585a72.
//
// Solidity: function submit(bytes data, bytes hmG1, bytes negSig) returns()
func (_BaseBLS *BaseBLSTransactor) Submit(opts *bind.TransactOpts, data []byte, hmG1 []byte, negSig []byte) (*types.Transaction, error) {
	return _BaseBLS.contract.Transact(opts, "submit", data, hmG1, negSig)
}

// Submit is a paid mutator transaction binding the contract method 0xd8585a72.
//
// Solidity: function submit(bytes data, bytes hmG1, bytes negSig) returns()
func (_BaseBLS *BaseBLSSession) Submit(data []byte, hmG1 []byte, negSig []byte) (*types.Transaction, error) {
	return _BaseBLS.Contract.Submit(&_BaseBLS.TransactOpts, data, hmG1, negSig)
}

// Submit is a paid mutator transaction binding the contract method 0xd8585a72.
//
// Solidity: function submit(bytes data, bytes hmG1, bytes negSig) returns()
func (_BaseBLS *BaseBLSTransactorSession) Submit(data []byte, hmG1 []byte, negSig []byte) (*types.Transaction, error) {
	return _BaseBLS.Contract.Submit(&_BaseBLS.TransactOpts, data, hmG1, negSig)
}
