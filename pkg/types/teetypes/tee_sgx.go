package teetypes

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/edgelesssys/ego/attestation"
	"github.com/edgelesssys/ego/attestation/tcbstatus"
	"github.com/edgelesssys/ego/enclave"
	"github.com/vedhavyas/go-subkey/v2/ed25519"
	chain "github.com/wetee-dao/ink.go"
)

// sgx issue report
func SgxIssue(pk chain.SignerType, call *model.TeeCall) error {
	timestamp := time.Now().Unix()
	var buf bytes.Buffer
	buf.Write(Int64ToBytes(timestamp))
	buf.Write(pk.Public())
	if call.Tx != nil {
		txbuf, err := MarshalVT(call.Tx)
		if err != nil {
			return err
		}
		buf.Write(txbuf)
	}

	sig, err := pk.Sign(buf.Bytes())
	if err != nil {
		return err
	}

	report, err := enclave.GetRemoteReport(sig)
	if err != nil {
		return err
	}

	// add report to call
	call.Time = timestamp
	call.TeeType = 0
	call.Report = report
	call.Caller = pk.Public()

	return nil
}

// sgx verify
func SgxVerify(reportData *model.TeeCall) (*model.TeeVerifyResult, error) {
	var msgBytes []byte
	if reportData.Tx != nil {
		var err error
		msgBytes, err = MarshalVT(reportData.Tx)
		if err != nil {
			return nil, err
		}
	}
	reportBytes, timestamp := reportData.Report, reportData.Time

	report, err := enclave.VerifyRemoteReport(reportBytes)
	if err == attestation.ErrTCBLevelInvalid {
		fmt.Printf("Warning: TCB level is invalid: %v\n%v\n", report.TCBStatus, tcbstatus.Explain(report.TCBStatus))
	} else if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(Int64ToBytes(timestamp))
	buf.Write(reportData.Caller)
	if len(msgBytes) > 0 {
		buf.Write(msgBytes)
	}

	sig := report.Data
	if !model.Ed25519SignVerify(reportData.Caller, buf.Bytes(), sig) {
		return nil, errors.New("invalid report sign")
	}

	// TODO TODO TODO TODO TODO must open
	if report.Debug {
		util.LogError("TEE", "debug model")
		return nil, errors.New("debug mode is not allowed")
	}

	return &model.TeeVerifyResult{
		TeeType:       reportData.TeeType,
		CodeSigner:    report.SignerID,
		CodeSignature: report.UniqueID,
		CodeProductId: report.ProductID,
	}, nil
}

// client sgx verify
func ClientSgxVerify(reportData *model.TeeCall) (*model.TeeVerifyResult, error) {
	var msgBytes []byte
	if reportData.Tx != nil {
		var err error
		msgBytes, err = MarshalVT(reportData.Tx)
		if err != nil {
			return nil, err
		}
	}
	var reportBytes, timestamp = reportData.Report, reportData.Time

	// decode address
	signer := reportData.Caller

	// call sgx-verify
	reportBt := base64.StdEncoding.EncodeToString(reportBytes)
	cmd := exec.Command("/usr/local/bin/sgx-verify", reportBt)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("call sgx-verify errors" + err.Error())
	}
	outs := strings.Split(string(output), "⊂⊂⊂⊂")
	if len(outs) < 2 {
		return nil, errors.New("sgx-verify output format invalid: missing first delimiter")
	}
	datas := strings.Split(outs[1], "∐∐∐∐")
	if len(datas) < 2 {
		return nil, errors.New("sgx-verify output format invalid: missing second delimiter")
	}
	report := &attestation.Report{}
	err = json.Unmarshal([]byte(datas[1]), report)
	if err != nil {
		return nil, errors.New("call sgx-verify unmarshal errors" + err.Error())
	}
	// end call sgx-verify

	pubkey, err := ed25519.Scheme{}.FromPublicKey(signer)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(Int64ToBytes(timestamp))
	buf.Write(signer)
	if len(msgBytes) > 0 {
		buf.Write(msgBytes)
	}

	sig := report.Data
	if !pubkey.Verify(buf.Bytes(), sig) {
		return nil, errors.New("invalid sgx report")
	}

	if report.Debug {
		return nil, errors.New("debug mode is not allowed")
	}

	return &model.TeeVerifyResult{
		TeeType:       reportData.TeeType,
		CodeSigner:    report.SignerID,
		CodeSignature: report.UniqueID,
		CodeProductId: report.ProductID,
	}, nil
}

type VTMarshaler interface {
	SizeVT() int
	MarshalToVT([]byte) (int, error)
}

func MarshalVT(tx any) ([]byte, error) {
	if tx == nil {
		return nil, nil
	}
	m, ok := tx.(VTMarshaler)
	if !ok {
		return nil, fmt.Errorf("unsupported tee tx type %T", tx)
	}
	buf := make([]byte, m.SizeVT())
	if _, err := m.MarshalToVT(buf); err != nil {
		return nil, err
	}
	return buf, nil
}
