package graph

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"strings"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

// DecodeCaller 解析调用方地址（32 字节 hex 或 SS58）
func DecodeCaller(caller string) ([]byte, error) {
	if strings.HasPrefix(caller, "0x") {
		return hex.DecodeString(caller[2:])
	}
	_, address, err := model.SS58Decode(caller)
	return address, err
}

// decodeArgs 将 hex 字符串数组解码为字节数组
func decodeArgs(args []string) ([][]byte, error) {
	argsBytes := make([][]byte, len(args))
	for i, arg := range args {
		arg = strings.TrimPrefix(arg, "0x")
		argBytes, err := hex.DecodeString(arg)
		if err != nil {
			return nil, fmt.Errorf("decodeArgs: decode arg: %w", err)
		}
		argsBytes[i] = argBytes
	}

	return argsBytes, nil
}

// SubmitContractCall 提交合约调用（用户自行管理 nonce）。
// nonce 必须严格递增，且已包含在签名中。
// 成功时返回交易 hash，失败返回错误。
func SubmitContractCall(caller []byte, callerType uint32, nonce uint64, contract string, method [4]byte, argsBytes [][]byte, signature []byte) (string, error) {
	c := &model.ContractCall{
		Name:   []byte(contract),
		Method: method[:],
		Args:   argsBytes,
		Nonce:  nonce,
	}
	call := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: c,
		},
	}

	err := model.SideSysCallVerify(caller, callerType, call, signature)
	if err != nil {
		return "", fmt.Errorf("SubmitContractCall: contract verify: %w", err)
	}

	buf := new(bytes.Buffer)
	protoio.WriteMessage(call, buf)
	tx := &model.Tx{
		Caller:     caller,
		CallerType: callerType,
		Call:       buf.Bytes(),
		Signature:  signature,
	}

	txHash := teechain.TxHash(tx)
	if err := teechain.SubmitTx(tx); err != nil {
		return "", err
	}
	return txHash, nil
}

// ContractQuery 只读查询
func ContractDryRun(caller []byte, callerType int, contract string, mut bool, method string, args [][]byte) (string, error) {
	var err error
	var out []byte
	if !mut {
		out, err = teeChain.ContractQuery(caller, uint32(callerType), contract, method, args)
		if err != nil {
			return "", err
		}
	} else {
		err = teeChain.ContractDryRun(caller, uint32(callerType), &model.ContractCall{
			Name:   []byte(contract),
			Method: model.MethodToSelectorBytes(method),
			Args:   args,
		})
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(out), nil
}
