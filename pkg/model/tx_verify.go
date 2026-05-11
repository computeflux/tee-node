package model

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vedhavyas/go-subkey/v2/ed25519"
	"github.com/vedhavyas/go-subkey/v2/sr25519"
	"github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
	"golang.org/x/crypto/blake2b"
)

// VerifyTxSigner 验证交易发起方：caller 对 Tx（不含 signature）的签名必须有效
func VerifyTxSigner(tx *Tx) (*SysCall, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}

	caller := tx.GetCaller()
	sig := tx.GetSignature()
	if len(caller) == 0 {
		return nil, errors.New("tx: missing caller")
	}
	if len(sig) == 0 {
		return nil, errors.New("tx: missing signature")
	}

	// 处理系统请求
	call := new(SysCall)
	err := protoio.ReadMessage(bytes.NewBuffer(tx.Call), call)
	if err != nil {
		return nil, err
	}

	// switch tx.CallerType {
	// case 0: // node ed25519
	// 	if !Ed25519SignVerify(caller, call.BytesForSig(), sig) {
	// 		return nil, errors.Wrap(err, "node")
	// 	}
	// case 1: // polkadot sign sr25519 side contract
	// 	if err := SideContractPolkadotVerify(caller, call.BytesForSig(), sig); err != nil {
	// 		return nil, errors.Wrap(err, "contract")
	// 	}
	// default:
	// 	return nil, errors.New("unknown caller type")
	// }
	err = SideSysCallVerify(caller, tx.CallerType, call, sig)
	if err != nil {
		return nil, err
	}

	return call, nil
}

func SideSysCallVerify(caller []byte, callerType uint32, call *SysCall, signature []byte) error {
	switch callerType {
	case 0: // node ed25519
		sigBytes, err := call.BytesForSig()
		if err != nil {
			return errors.Wrap(err, "marshal syscall for sig")
		}
		if !Ed25519SignVerify(caller, sigBytes, signature) {
			return errors.New("invalid signature")
		}
	case 1: // polkadot sign sr25519 side contract
		sigBytes, err := call.BytesForSig()
		if err != nil {
			return errors.Wrap(err, "marshal syscall for sig")
		}
		if err := SideContractPolkadotVerify(caller, sigBytes, signature); err != nil {
			return errors.Wrap(err, "contract")
		}
	default:
		return errors.New("unknown caller type")
	}
	return nil
}

// func SideContractVerify(caller []byte, callerType uint32, call *ContractCall, signature []byte) error {
// 	bt := ContractForSig(call)
// 	switch callerType {
// 	case 1:
// 		return SideContractPolkadotVerify(caller, bt, signature)
// 	default:
// 		return errors.New("unknown caller type")
// 	}
// }

func SideContractPolkadotVerify(caller []byte, call []byte, signature []byte) error {
	// 解析公钥
	pubkey, err := sr25519.Scheme{}.FromPublicKey(caller)
	if err != nil {
		return fmt.Errorf("invalid sr25519 public key: %w", err)
	}

	// 验证签名
	ok := pubkey.Verify(call, signature)
	if !ok {
		return errors.New("invalid signature")
	}

	return nil
}

func SideContractPolkadotSign(caller *ink.Signer, call *ContractCall) ([]byte, error) {
	parts := append([][]byte{[]byte("<Bytes>"), call.Name, call.Method[:]}, call.Args...)
	parts = append(parts, []byte("</Bytes>"))

	// 合并所有部分
	result := bytes.Join(parts, nil)

	return caller.Sign(result)
}

func Ed25519SignVerify(pubkeyBt []byte, msg []byte, signature []byte) bool {
	pubkey, err := ed25519.Scheme{}.FromPublicKey(pubkeyBt)
	if err != nil {
		return false
	}

	if len(msg) > 256 {
		h := blake2b.Sum256(msg)
		msg = h[:]
	}

	return pubkey.Verify(msg, signature)
}
