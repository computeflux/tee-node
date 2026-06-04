package crypto

import (
	"fmt"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing/bls12381/circl"
	"go.dedis.ch/kyber/v4/suites"
)

// SuiteType 标识签名算法类型（uint8 便于 map 查找和比较）
type SuiteType uint8

const (
	SuiteEd25519  SuiteType = 0
	SuiteBLS12381 SuiteType = 1
)

// String 返回 SuiteType 的字符串表示（日志/JSON 序列化用）
func (s SuiteType) String() string {
	switch s {
	case SuiteEd25519:
		return "ed25519"
	case SuiteBLS12381:
		return "bls12381"
	default:
		return "unknown"
	}
}

// SuiteTypeFromString 从字符串解析 SuiteType
func SuiteTypeFromString(s string) (SuiteType, error) {
	switch s {
	case "ed25519":
		return SuiteEd25519, nil
	case "bls12381":
		return SuiteBLS12381, nil
	default:
		return SuiteEd25519, fmt.Errorf("unknown suite type: %s", s)
	}
}

// AllSuites returns all supported suite types.
func AllSuites() []SuiteType {
	return []SuiteType{SuiteEd25519, SuiteBLS12381}
}

// NewSuite 根据类型创建 kyber Suite
func NewSuite(st SuiteType) suites.Suite {
	switch st {
	case SuiteEd25519:
		return suites.MustFind("Ed25519")
	case SuiteBLS12381:
		return circl.NewSuiteBLS12381()
	default:
		return suites.MustFind("Ed25519")
	}
}

// SuiteToPoint 将字节反序列化为对应 suite 的 Point
func SuiteToPoint(suite suites.Suite, data []byte) (kyber.Point, error) {
	point := suite.Point()
	if err := point.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("unmarshal point: %w", err)
	}
	return point, nil
}

// SuiteToScalar 将字节反序列化为对应 suite 的 Scalar
func SuiteToScalar(suite suites.Suite, data []byte) (kyber.Scalar, error) {
	scalar := suite.Scalar()
	if err := scalar.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("unmarshal scalar: %w", err)
	}
	return scalar, nil
}

// SuiteFromPubKeyBytes 根据公钥字节长度推断 suite 类型
func SuiteFromPubKeyBytes(pub []byte) (SuiteType, error) {
	switch len(pub) {
	case 32:
		return SuiteEd25519, nil
	case 96:
		return SuiteBLS12381, nil
	default:
		return 255, fmt.Errorf("unknown public key length: %d", len(pub))
	}
}
