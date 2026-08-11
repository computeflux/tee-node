package model

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model/protoio"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/vedhavyas/go-subkey/v2/ed25519"
	"github.com/vedhavyas/go-subkey/v2/sr25519"
	"golang.org/x/crypto/blake2b"
)

func TestUint64ToBytes(t *testing.T) {
	require.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0}, Uint64ToBytes(0))
	require.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 1}, Uint64ToBytes(1))
	require.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8}, Uint64ToBytes(0x0102030405060708))
	require.Equal(t, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, Uint64ToBytes(^uint64(0)))
}

func TestContractForSig_DeterministicLayout(t *testing.T) {
	call := &ContractCall{
		Name:   []byte("token"),
		Method: []byte("transfer"),
		Args:   [][]byte{[]byte("to"), []byte("100")},
		Nonce:  42,
	}

	got := ContractForSig(call)
	want := bytes.Join([][]byte{
		[]byte("<Bytes>"),
		[]byte("token"),
		[]byte("transfer"),
		[]byte("to"),
		[]byte("100"),
		Uint64ToBytes(42),
		[]byte("</Bytes>"),
	}, nil)
	require.Equal(t, want, got)

	// 确定性：两次结果一致
	require.Equal(t, got, ContractForSig(call))

	// nil Method / 空 Args 不 panic；格式为 <Bytes> + name + nonce + </Bytes>
	empty := ContractForSig(&ContractCall{Name: []byte("n")})
	require.Equal(t, bytes.Join([][]byte{[]byte("<Bytes>n"), Uint64ToBytes(0), []byte("</Bytes>")}, nil), empty)
}

func TestSysCallBytesForSig(t *testing.T) {
	// 合约调用 → ContractForSig
	cc := &ContractCall{Name: []byte("c"), Method: []byte("m"), Nonce: 3}
	call := &SysCall{Payload: &SysCall_Contract{Contract: cc}}
	got, err := call.BytesForSig()
	require.NoError(t, err)
	require.Equal(t, ContractForSig(cc), got)

	// 非合约调用 → MarshalVT
	empty := &SysCall{Payload: &SysCall_Empty{}}
	got, err = empty.BytesForSig()
	require.NoError(t, err)
	want, err := empty.MarshalVT()
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestEd25519SignVerify(t *testing.T) {
	kp, err := ed25519.Scheme{}.Generate()
	require.NoError(t, err)
	pub := kp.Public()

	msg := []byte("hello tee-node")
	sig, err := kp.Sign(msg)
	require.NoError(t, err)

	require.True(t, Ed25519SignVerify(pub, msg, sig))
	require.False(t, Ed25519SignVerify(pub, []byte("tampered"), sig))
	require.False(t, Ed25519SignVerify([]byte("bad-key"), msg, sig))

	// 长消息走 blake2b 哈希路径（>256 字节）
	longMsg := bytes.Repeat([]byte("a"), 300)
	h := blake2b.Sum256(longMsg)
	sig, err = kp.Sign(h[:])
	require.NoError(t, err)
	require.True(t, Ed25519SignVerify(pub, longMsg, sig))
	require.False(t, Ed25519SignVerify(pub, longMsg[:250], sig))
}

func TestSideSysCallVerify_NilCall(t *testing.T) {
	// nil call 必须报错而不是 panic（case 0/1/2 都会解引用 call）
	kp, err := ed25519.Scheme{}.Generate()
	require.NoError(t, err)
	require.Error(t, SideSysCallVerify(kp.Public(), 0, nil, []byte("sig")))
	require.Error(t, SideSysCallVerify([]byte("caller"), 2, nil, make([]byte, 65)))
}

func TestSideSysCallVerify_Ed25519(t *testing.T) {
	kp, err := ed25519.Scheme{}.Generate()
	require.NoError(t, err)
	pub := kp.Public()

	call := &SysCall{Payload: &SysCall_Contract{Contract: &ContractCall{
		Name: []byte("n"), Method: []byte("m"), Nonce: 1,
	}}}
	sigBytes, err := call.BytesForSig()
	require.NoError(t, err)
	sig, err := kp.Sign(sigBytes)
	require.NoError(t, err)

	require.NoError(t, SideSysCallVerify(pub, 0, call, sig))

	// 篡改消息 → 失败
	badSig, err := kp.Sign([]byte("different"))
	require.NoError(t, err)
	require.Error(t, SideSysCallVerify(pub, 0, call, badSig))

	// 未知 callerType → 失败
	require.Error(t, SideSysCallVerify(pub, 99, call, sig))
}

func TestSideSysCallVerify_Sr25519(t *testing.T) {
	kp, err := sr25519.Scheme{}.Generate()
	require.NoError(t, err)
	pub := kp.Public()

	call := &SysCall{Payload: &SysCall_Contract{Contract: &ContractCall{
		Name: []byte("n"), Method: []byte("m"), Nonce: 1,
	}}}
	sigBytes, err := call.BytesForSig()
	require.NoError(t, err)
	sig, err := kp.Sign(sigBytes)
	require.NoError(t, err)

	require.NoError(t, SideSysCallVerify(pub, 1, call, sig))

	badSig, err := kp.Sign([]byte("different"))
	require.NoError(t, err)
	require.Error(t, SideSysCallVerify(pub, 1, call, badSig))
}

// personal_sign(EIP-191) 格式的 EVM 钱包签名验证
func TestEVMWalletSignVerify(t *testing.T) {
	priv, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	addr := ethcrypto.PubkeyToAddress(priv.PublicKey)

	msg := []byte("evm message")
	sigData := "call_tee_contract:" + hex.EncodeToString(msg)
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(sigData))
	prefixedMsg := append([]byte(prefix), []byte(sigData)...)
	hash := ethcrypto.Keccak256Hash(prefixedMsg)
	sig, err := ethcrypto.Sign(hash.Bytes(), priv)
	require.NoError(t, err)
	require.Len(t, sig, 65)

	// 20 字节地址 caller
	require.True(t, EVMWalletSignVerify(addr.Bytes(), msg, sig))
	// 65 字节未压缩公钥 caller
	require.True(t, EVMWalletSignVerify(ethcrypto.FromECDSAPub(&priv.PublicKey), msg, sig))
	// 33 字节压缩公钥 caller
	require.True(t, EVMWalletSignVerify(ethcrypto.CompressPubkey(&priv.PublicKey), msg, sig))

	// 错误消息 → false
	require.False(t, EVMWalletSignVerify(addr.Bytes(), []byte("other"), sig))
	// 错误地址 → false
	otherPriv, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	otherAddr := ethcrypto.PubkeyToAddress(otherPriv.PublicKey)
	require.False(t, EVMWalletSignVerify(otherAddr.Bytes(), msg, sig))
}

func TestEVMWalletSignVerify_EdgeCases(t *testing.T) {
	require.False(t, EVMWalletSignVerify([]byte("caller"), []byte("m"), []byte("short")))
	require.False(t, EVMWalletSignVerify(nil, []byte("m"), make([]byte, 65)))
	// caller 长度非法（既不是地址也不是公钥）
	require.False(t, EVMWalletSignVerify(make([]byte, 10), []byte("m"), make([]byte, 65)))
	// v=27/28 也支持（以太坊经典格式）
	priv, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	addr := ethcrypto.PubkeyToAddress(priv.PublicKey)
	msg := []byte("v-test")
	sigData := "call_tee_contract:" + hex.EncodeToString(msg)
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(sigData))
	hash := ethcrypto.Keccak256Hash(append([]byte(prefix), []byte(sigData)...))
	sig, err := ethcrypto.Sign(hash.Bytes(), priv)
	require.NoError(t, err)
	sig[64] += 27 // 转换为 27/28
	require.True(t, EVMWalletSignVerify(addr.Bytes(), msg, sig))
}

func TestVerifyTxSigner(t *testing.T) {
	kp, err := ed25519.Scheme{}.Generate()
	require.NoError(t, err)

	call := &SysCall{Payload: &SysCall_Contract{Contract: &ContractCall{
		Name: []byte("token"), Method: []byte("transfer"), Nonce: 5,
	}}}
	callBytes, err := protoio.Marshal(call)
	require.NoError(t, err)

	sigBytes, err := call.BytesForSig()
	require.NoError(t, err)
	sig, err := kp.Sign(sigBytes)
	require.NoError(t, err)

	tx := &Tx{
		Caller:     kp.Public(),
		CallerType: 0,
		Call:       callBytes,
		Signature:  sig,
	}

	got, err := VerifyTxSigner(tx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "transfer", string(got.GetContract().GetMethod()))

	// 篡改 nonce → 签名不匹配
	badTx := &Tx{
		Caller:     kp.Public(),
		CallerType: 0,
		Call:       callBytes,
		Signature:  sig,
	}
	badCall := &SysCall{Payload: &SysCall_Contract{Contract: &ContractCall{
		Name: []byte("token"), Method: []byte("transfer"), Nonce: 6,
	}}}
	badCallBytes, err := protoio.Marshal(badCall)
	require.NoError(t, err)
	badTx.Call = badCallBytes
	_, err = VerifyTxSigner(badTx)
	require.Error(t, err)
}

func TestVerifyTxSigner_Errors(t *testing.T) {
	_, err := VerifyTxSigner(nil)
	require.Error(t, err)

	_, err = VerifyTxSigner(&Tx{})
	require.Error(t, err) // missing caller

	_, err = VerifyTxSigner(&Tx{Caller: []byte("x")})
	require.Error(t, err) // missing signature

	_, err = VerifyTxSigner(&Tx{Caller: []byte("x"), Signature: []byte("s"), Call: []byte("garbage")})
	require.Error(t, err) // bad call bytes

	// 未知 callerType
	_, err = VerifyTxSigner(&Tx{Caller: []byte("x"), CallerType: 99, Signature: []byte("s"), Call: []byte{0x0a, 0x00}})
	require.Error(t, err)
}
