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

// DecodeCaller 支持以下格式的 caller：
//   - 20 字节 hex（40 个 hex 字符，可带 0x 前缀）：以太坊地址
//   - 32 字节 hex（64 个 hex 字符，可带 0x 前缀）：ed25519/sr25519 公钥
//   - SS58 格式：Polkadot 地址
func DecodeCaller(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if b, err := hex.DecodeString(s); err == nil {
		switch len(b) {
		case 20: // 以太坊地址（20 字节）
			return b, nil
		case 32: // ed25519/sr25519 公钥
			return b, nil
		}
	}
	pub, err := model.PubKeyFromSS58(s)
	if err != nil {
		return nil, fmt.Errorf("caller 需为 20/32 字节 hex 或 SS58: %w", err)
	}
	return pub.Byte(), nil
}

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
// nonce 必须严格递增，且已包含在 signature 的签名数据中。
func SubmitContractCall(caller []byte, callerType uint32, nonce uint64, contract string, method [4]byte, argsBytes [][]byte, signature []byte) error {
	c := &model.ContractCall{
		Name:   []byte(contract),
		Method: method[:],
		Args:   argsBytes,
	}
	call := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: c,
		},
	}

	err := model.SideSysCallVerify(caller, callerType, nonce, call, signature)
	if err != nil {
		return fmt.Errorf("SubmitContractCall: contract verify: %w", err)
	}

	buf := new(bytes.Buffer)
	protoio.WriteMessage(call, buf)
	tx := &model.Tx{
		Caller:     caller,
		CallerType: callerType,
		Nonce:      nonce,
		Call:       buf.Bytes(),
		Signature:  signature,
	}

	return teechain.SubmitTx(tx)
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
