package model

import (
	"encoding/json"
	"errors"
	"math/big"
)

const u128ByteSize = 16

// U128 表示 0 ~ 2^128-1 的无符号整数，用于 DAO 资产；嵌入 big.Int 便于运算，JSON 序列化为十进制字符串。
type U128 struct {
	big.Int
}

// NewU128 从 *big.Int 构造，若为 nil 或负数则视为 0；若超过 2^128-1 则截断为 max。
func NewU128(z *big.Int) U128 {
	if z == nil || z.Sign() < 0 {
		return U128{Int: *big.NewInt(0)}
	}
	var u U128
	u.Set(z)
	u.clampToU128()
	return u
}

// NewU128FromString 从十进制字符串解析，非法或超范围则返回零值。
func NewU128FromString(s string) U128 {
	if s == "" {
		return U128{Int: *big.NewInt(0)}
	}
	var u U128
	if _, ok := u.SetString(s, 10); !ok {
		return U128{Int: *big.NewInt(0)}
	}
	u.clampToU128()
	return u
}

func (u *U128) clampToU128() {
	max := u128Max()
	if u.Cmp(max) > 0 {
		u.Set(max)
	}
}

func u128Max() *big.Int {
	// 2^128 - 1
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	return max.Sub(max, big.NewInt(1))
}

// MarshalJSON 序列化为十进制字符串。
func (u U128) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON 从十进制字符串解析。
func (u *U128) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		u.SetInt64(0)
		return nil
	}
	if _, ok := u.SetString(s, 10); !ok {
		return errInvalidU128
	}
	u.clampToU128()
	return nil
}

var errInvalidU128 = errors.New("invalid u128")

// ToBigInt 返回当前值的副本，便于与 *big.Int 运算。
func (u *U128) ToBigInt() *big.Int {
	if u == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(&u.Int)
}

// U128ToBytes 将 *big.Int 编码为 16 字节大端（u128），不足补零。
func U128ToBytes(z *big.Int) []byte {
	if z == nil || z.Sign() < 0 {
		return make([]byte, u128ByteSize)
	}
	return z.FillBytes(make([]byte, u128ByteSize))
}

// BytesToU128 将 16 字节大端解码为 *big.Int，nil 或空视为 0。
func BytesToU128(b []byte) *big.Int {
	if len(b) == 0 {
		return big.NewInt(0)
	}
	// 若不足 16 字节则前面补零
	if len(b) < u128ByteSize {
		padded := make([]byte, u128ByteSize)
		copy(padded[u128ByteSize-len(b):], b)
		b = padded
	} else if len(b) > u128ByteSize {
		b = b[len(b)-u128ByteSize:]
	}
	return new(big.Int).SetBytes(b)
}
