package teetypes

import (
	"errors"

	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
)

// TdxIssue generates a TDX attestation report for the given call.
// TODO: implement TDX attestation using Intel TDX SDK.
func TdxIssue(pk chain.SignerType, call *model.TeeCall) error {
	return errors.New("TDX report issue is not yet implemented")
}

// TdxVerify verifies a TDX attestation report.
// TODO: implement TDX verification.
func TdxVerify(reportData *model.TeeCall) (*model.TeeVerifyResult, error) {
	return nil, errors.New("TDX report verify is not yet implemented")
}

// ClientTdxVerify verifies a TDX attestation report from a client perspective.
// TODO: implement client-side TDX verification.
func ClientTdxVerify(reportData *model.TeeCall) (*model.TeeVerifyResult, error) {
	return nil, errors.New("TDX client verification is not yet implemented")
}
