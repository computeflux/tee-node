package util

import (
	"encoding/hex"
	"fmt"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/suites"
)

func HexToScalar(suite suites.Suite, hexStr string) (kyber.Scalar, error) {
	okbt, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Println("HexToScalar hex.DecodeString Error:", err)
		return nil, err
	}

	pk := suite.Scalar()
	err = pk.UnmarshalBinary(okbt)
	if err != nil {
		fmt.Println("HexToScalar UnmarshalBinary Error:", err)
		return nil, err
	}
	return pk, nil
}

func HexToPoint(suite suites.Suite, hexStr string) (kyber.Point, error) {
	okbt, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Println("HexToPoint hex.DecodeString Error:", err)
		return nil, err
	}

	pk := suite.Point()
	err = pk.UnmarshalBinary(okbt)
	if err != nil {
		fmt.Println("HexToScalar UnmarshalBinary Error:", err)
		return nil, err
	}
	return pk, nil
}
