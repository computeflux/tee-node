package sidechain

import (
	"fmt"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/side-chain/contracts"
)

func (app *SideChain) ContractMutation(caller []byte, callerType uint32, txn *model.Txn, height int64, call *model.ContractCall) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ContractMutation recovered from panic: %v", r)
		}
	}()

	address := model.UniAddr{
		T: callerType,
		V: caller,
	}

	pub, err := GetDkgPubkey()
	if err != nil {
		return fmt.Errorf("get dkg pubkey: %w", err)
	}
	if pub == nil {
		return fmt.Errorf("dkg pubkey is nil")
	}
	runtime := contracts.NewRuntime(height, txn, address, model.UniAddr{
		V: pub.ToBytes(),
	})

	err = contracts.Mutation(call, runtime)
	return err
}

// ContractQuery 只读合约查询。contract=dao 时 method 为查询方法名，args 为字符串参数列表（与 ExecuteQuery 约定一致）。
func (app *SideChain) ContractQuery(caller []byte, callerType uint32, contract string, method string, args [][]byte) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			data = nil
			err = fmt.Errorf("ContractMutation recovered from panic: %v", r)
		}
	}()

	height := app.state.Height
	txn := model.DBINS.NewTransaction()
	defer txn.Rollback()

	address := model.UniAddr{
		T: callerType,
		V: caller,
	}

	runtime := contracts.NewRuntime(height, txn, address, model.UniAddr{
		V: app.dkg.DkgPubKey.Byte(),
	})

	data, err = contracts.Query(&model.ContractCall{
		Name:   []byte(contract),
		Method: model.MethodToSelectorBytes(method),
		Args:   args,
	}, runtime)

	return data, err
}

func (app *SideChain) ContractDryRun(caller []byte, callerType uint32, call *model.ContractCall) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ContractMutation recovered from panic: %v", r)
		}
	}()

	height := app.state.Height
	txn := model.DBINS.NewTransaction()
	defer txn.Rollback()

	address := model.UniAddr{
		T: callerType,
		V: caller,
	}

	runtime := contracts.NewRuntime(height, txn, address, model.UniAddr{
		V: app.dkg.DkgPubKey.Byte(),
	})

	err = contracts.Mutation(call, runtime)
	return err
}
