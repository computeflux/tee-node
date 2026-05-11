package model

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"unicode"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"golang.org/x/crypto/blake2b"
)

// -----------------------------------------------------------------------------
// 对齐的合约基础类型（SCALE 固定宽字节 / U256）。
// -----------------------------------------------------------------------------

// Address
type UniAddr struct {
	T uint32
	V []byte
}

type Amount = types.U256

var ZeroAmount = types.NewU256(*big.NewInt(0))

func AmountAdd(a, b Amount) Amount {
	bigInt := new(big.Int).Add(a.Int, b.Int)
	return types.NewU256(*bigInt)
}

func AmountSub(a, b Amount) Amount {
	bigInt := new(big.Int).Sub(a.Int, b.Int)
	return types.NewU256(*bigInt)
}

func AmountMul(a, b Amount) Amount {
	bigInt := new(big.Int).Mul(a.Int, b.Int)
	return types.NewU256(*bigInt)
}

func AmountDiv(a, b Amount) Amount {
	bigInt := new(big.Int).Div(a.Int, b.Int)
	return types.NewU256(*bigInt)
}

// BlockNumber 区块高度（与 Rust BlockNumber = u32 一致）。
type BlockNumber uint32

// Bytes 为存储/消息中的变长字节，SCALE 为 compact 长度前缀 + 负载；语义同 types.rs 的 Vec<u8>。
type Bytes []byte

// type ContractCall struct {
// 	Contract string
// 	Method   [4]byte
// 	Args     [][]byte
// }

// ContractApi 侧链合约运行时：区块高度、读写事务、已解析的调用方（如由 TxBox 回填）。
type ContractApi interface {
	GetHeight() int64
	GetTxn() *Txn
	GetCaller() UniAddr
	GetSudoAccount() UniAddr
	Query(UniAddr, ContractCall) ([]byte, error)
	Call(UniAddr, ContractCall) ([]byte, error)
}

func RequireArgLen[T any](args []T, n int, method string) error {
	if len(args) < n {
		return fmt.Errorf("gov: %s expects at least %d arg(s), got %d", method, n, len(args))
	}
	return nil
}

// hexToBytes 解析十六进制（可带 0x），不允许 SS58；空串返回 nil 字节切片。
func hexToBytes(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	s = strings.TrimPrefix(s, "0x")
	return hex.DecodeString(s)
}

// DecodeScaleArg 将单个参数视为 hex( SCALE(T) )，解码为 T（泛型入口，供合约 dispatch / 查询共用）。
func DecodeScaleArg[T any](s string) (T, error) {
	var z T
	raw, err := hexToBytes(s)
	if err != nil {
		return z, fmt.Errorf("hex: %w", err)
	}
	if len(raw) == 0 {
		return z, fmt.Errorf("gov: empty arg after hex decode")
	}
	return DecodeScaleArgBytes[T](raw)
}

func DecodeScaleArgBytes[T any](s []byte) (T, error) {
	var z T
	if err := codec.Decode(s, &z); err != nil {
		return z, fmt.Errorf("scale: %w", err)
	}
	return z, nil
}

// MethodToSelector 将 method 字符串转换为 4 字节 selector（默认 mutates=true）。
// 支持两种格式：
// 1. 十六进制字符串（如 "0x1d623327" 或 "1d623327"）→ 直接解析
// 2. 普通字符串（如 "add_track" 或 "AddTrack"）→ 自动转为 snake_case 后计算 blake2b 哈希
// 注意：普通字符串默认使用 mutates=true，如果需要区分，请使用 MethodToSelectorWithMutates。
func MethodToSelector(method string) [4]byte {
	return MethodToSelectorWithMutates(method, true)
}

func MethodToSelectorBytes(method string) []byte {
	bt := MethodToSelector(method)
	return bt[:]
}

// MethodToSelectorWithMutates 将 method 字符串转换为 4 字节 selector，可指定 mutates。
// 计算方式与 contractgen 一致：blake2b("wetee/contractgen/v1:{snake_case(method)}:mutates={mutates}")[:4]
func MethodToSelectorWithMutates(method string, mutates bool) [4]byte {
	// 尝试解析十六进制格式
	s := strings.TrimPrefix(method, "0x")
	if len(s) == 8 {
		if b, err := hex.DecodeString(s); err == nil && len(b) == 4 {
			return [4]byte(b)
		}
	}

	// 将方法名转为 snake_case（与 contractgen 一致）
	snake := toSnakeCase(method)

	// 计算 blake2b 哈希（与 contractgen pickSelectorInk 一致）
	preimage := fmt.Sprintf("wetee/contractgen/v1:%s:mutates=%v", snake, mutates)
	h := blake2b.Sum256([]byte(preimage))
	var sel [4]byte
	copy(sel[:], h[:4])
	return sel
}

// toSnakeCase 将驼峰命名转为蛇形命名（与 contractgen snakeCase 一致）
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
