package model

import (
	"bytes"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model/protoio"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/vedhavyas/go-subkey/v2/ed25519"
	"github.com/vedhavyas/go-subkey/v2/sr25519"
	"github.com/wetee-dao/ink.go"
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
	case 2: // EVM Wallet signature
		sigBytes, err := call.BytesForSig()
		if err != nil {
			return errors.Wrap(err, "marshal syscall for sig")
		}

		if !EVMWalletSignVerify(caller, sigBytes, signature) {
			return errors.New("invalid signature")
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

// EVMWalletSignVerify 验证 EVM 钱包签名（EIP-191 personal_sign 格式）
// caller 可以是 20 字节的以太坊地址，或 33/65 字节的 secp256k1 公钥
// signature 为 65 字节 [R || S || V]
func EVMWalletSignVerify(caller []byte, msg []byte, signature []byte) bool {
	if len(signature) != 65 {
		return false
	}

	// EIP-191: \x19Ethereum Signed Message:\n<len><msg>
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(msg))
	prefixedMsg := append([]byte(prefix), msg...)
	hash := ethcrypto.Keccak256Hash(prefixedMsg)

	// 复制签名并调整 v 值：以太坊使用 27/28，Ecrecover 期望 0/1
	sig := make([]byte, 65)
	copy(sig, signature)
	if sig[64] >= 27 {
		sig[64] -= 27
	}

	// 从签名恢复公钥
	recoveredPub, err := ethcrypto.SigToPub(hash.Bytes(), sig)
	if err != nil {
		return false
	}
	recoveredAddr := ethcrypto.PubkeyToAddress(*recoveredPub)

	// 根据 caller 长度判断比对方式
	switch len(caller) {
	case 20:
		// caller 是以太坊地址，直接比对
		return bytes.Equal(recoveredAddr.Bytes(), caller)
	case 33, 65:
		// caller 是公钥（压缩或未压缩），从公钥派生地址后比对
		callerAddr := pubKeyToEVMAddress(caller)
		return bytes.Equal(recoveredAddr.Bytes(), callerAddr)
	default:
		return false
	}
}

// pubKeyToEVMAddress 从 secp256k1 公钥派生以太坊地址，返回 20 字节地址
// 支持 65 字节未压缩公钥（0x04 前缀）、33 字节压缩公钥（0x02/0x03 前缀）
func pubKeyToEVMAddress(pubKey []byte) []byte {
	if len(pubKey) == 33 && (pubKey[0] == 0x02 || pubKey[0] == 0x03) {
		// 压缩公钥：解压为未压缩格式
		pk, err := ethcrypto.DecompressPubkey(pubKey)
		if err != nil {
			return nil
		}
		addr := ethcrypto.PubkeyToAddress(*pk)
		return addr.Bytes()
	}
	// 未压缩公钥：移除 0x04 前缀后计算地址
	var key []byte
	if len(pubKey) == 65 && pubKey[0] == 0x04 {
		key = pubKey[1:]
	} else {
		key = pubKey
	}
	hash := ethcrypto.Keccak256(key)
	addr := make([]byte, 20)
	copy(addr, hash[12:])
	return addr
}
