package store

import (
	"encoding/hex"
	"fmt"
	"math/big"
)

// -----------------------------------------------------------------------------
// 对齐的合约基础类型（SCALE 固定宽字节 / U256）。
// -----------------------------------------------------------------------------

// Address
// scale-codec-gen:generate
type UniAddr struct {
	T uint32
	V []byte
}

func (a UniAddr) String() string {
	return fmt.Sprint(a.T) + ":" + hex.EncodeToString(a.V)
}

// Amount 链上金额类型。
type Amount = U256

// U256 256 位无符号整数，内部使用 big.Int。
type U256 struct {
	Int *big.Int
}

var ZeroAmount = NewU256(big.NewInt(0))

// NewU256 creates a new U256 type
func NewU256(i *big.Int) U256 {
	return U256{i}
}

// Decode implements ScaleDecoder: decodes 32 bytes (little-endian) into a U256.
func (i *U256) Decode(data []byte) (int, error) {
	if len(data) < 32 {
		return 0, fmt.Errorf("decode U256: need 32 bytes, got %d", len(data))
	}
	bs := make([]byte, 32)
	copy(bs, data[:32])

	// reverse bytes, scale uses little-endian encoding, big.int's bytes are expected in big-endian
	Reverse(bs)

	b, err := UintBytesToBigInt(bs)
	if err != nil {
		return 0, err
	}

	// deal with zero differently to get a nil representation (this is how big.Int deals with 0)
	if b.Sign() == 0 {
		*i = U256{big.NewInt(0)}
		return 32, nil
	}

	*i = U256{b}
	return 32, nil
}

// Encode implements ScaleEncoder: encodes U256 as 32 bytes (little-endian).
func (i *U256) Encode() ([]byte, error) {
	if i.Int == nil {
		return make([]byte, 32), nil
	}

	b, err := BigIntToUintBytes(i.Int, 32)
	if err != nil {
		return nil, err
	}

	// reverse bytes, scale uses little-endian encoding, big.int's bytes are expected in big-endian
	Reverse(b)

	return b, nil
}

func (a U256) Cmp(b U256) int { return a.Int.Cmp(b.Int) }
func (a U256) String() string { return a.Int.String() }

func AmountAdd(a, b Amount) Amount {
	return NewU256(new(big.Int).Add(a.Int, b.Int))
}

func AmountSub(a, b Amount) Amount {
	return NewU256(new(big.Int).Sub(a.Int, b.Int))
}

func AmountMul(a, b Amount) Amount {
	return NewU256(new(big.Int).Mul(a.Int, b.Int))
}

func AmountDiv(a, b Amount) Amount {
	return NewU256(new(big.Int).Div(a.Int, b.Int))
}

// BlockNumber 区块高度（与 Rust BlockNumber = u32 一致）。
type BlockNumber uint32

// Bytes 为存储/消息中的变长字节，SCALE 为 compact 长度前缀 + 负载；语义同 types.rs 的 Vec<u8>。
type Bytes []byte

// Option 类似 Rust Option，用于可空字段的 SCALE 编解码。
type Option[T any] struct {
	isSome bool
	V      T
}

func NewNone[T any]() Option[T] {
	return Option[T]{isSome: false}
}

func NewSome[T any](v T) Option[T] {
	return Option[T]{isSome: true, V: v}
}

func (o Option[T]) IsSome() bool { return o.isSome }
func (o Option[T]) IsNone() bool { return !o.isSome }

func (o Option[T]) UnWrap() (T, error) {
	if !o.isSome {
		var zero T
		return zero, fmt.Errorf("unwrap error: Option is None")
	}
	return o.V, nil
}

// Encode 实现 ScaleEncoder（SCALE: None=0x00, Some(v)=0x01+v）。
func (o *Option[T]) Encode() ([]byte, error) {
	if !o.isSome {
		return []byte{0x00}, nil
	}
	data, err := ScaleEncode(o.V)
	if err != nil {
		return nil, err
	}
	return append([]byte{0x01}, data...), nil
}

// Decode 实现 ScaleDecoder。
func (o *Option[T]) Decode(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("decode Option: empty")
	}
	if data[0] == 0x00 {
		o.isSome = false
		return 1, nil
	}
	o.isSome = true
	if err := ScaleDecode(&o.V, data[1:]); err != nil {
		return 0, err
	}
	// 无法精确计算消费字节数，ScaleDecode 不返回字节数
	// 对于 SCALE，数据是紧凑的，消费全部剩余字节
	return len(data), nil
}

// TeeVerifyInfo TEE 报告验证结果（验证节点完成 quote 验签后返回的 TEE 证明代码三要素）。
//
//	TeeType       TEE 类型（0=SGX, 1=SEV-SNP, 2=TDX, 9999=非 TEE）
//	CodeSigner    TEE 证明代码的 ed25519 公钥（32 字节），供合约验证用户 CodeSignature
//	CodeSignature TEE 证明代码的签名/指纹（白名单信任锚，须与节点注册值一致）
//
// scale-codec-gen:generate
type TeeVerifyInfo struct {
	TeeType       uint32
	CodeSigner    []byte
	CodeSignature []byte
}

// Result 类似 Rust Result<T, E>，用于可恢复错误的 SCALE 编解码。
type Result[T, Err any] struct {
	isErr bool
	V     T
	E     Err
}

func NewOk[T, Err any](v T) Result[T, Err] {
	return Result[T, Err]{isErr: false, V: v}
}

func NewErr[T, Err any](e Err) Result[T, Err] {
	return Result[T, Err]{isErr: true, E: e}
}

func (r Result[T, Err]) IsOk() bool  { return !r.isErr }
func (r Result[T, Err]) IsErr() bool { return r.isErr }

func (r Result[T, Err]) UnWrap() (T, error) {
	if r.isErr {
		var zero T
		return zero, fmt.Errorf("unwrap error: Result is Err: %v", r.E)
	}
	return r.V, nil
}

// Encode 实现 ScaleEncoder（SCALE: Ok(v)=0x00+v, Err(e)=0x01+e）。
func (r *Result[T, Err]) Encode() ([]byte, error) {
	if r.isErr {
		data, err := ScaleEncode(r.E)
		if err != nil {
			return nil, err
		}
		return append([]byte{0x01}, data...), nil
	}
	data, err := ScaleEncode(r.V)
	if err != nil {
		return nil, err
	}
	return append([]byte{0x00}, data...), nil
}

// Decode 实现 ScaleDecoder。
func (r *Result[T, Err]) Decode(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("decode Result: empty")
	}
	switch data[0] {
	case 0x00:
		r.isErr = false
		if err := ScaleDecode(&r.V, data[1:]); err != nil {
			return 0, err
		}
		// 无法精确计算消费字节数，ScaleDecode 不返回字节数
		// 对于 SCALE，数据是紧凑的，消费全部剩余字节
		return len(data), nil
	case 0x01:
		r.isErr = true
		if err := ScaleDecode(&r.E, data[1:]); err != nil {
			return 0, err
		}
		return len(data), nil
	default:
		return 0, fmt.Errorf("decode Result: unrecognized variant byte 0x%02x", data[0])
	}
}
