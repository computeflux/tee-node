package teetypes

import (
	"bytes"
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/google/go-sev-guest/client"
	"github.com/google/go-sev-guest/proto/sevsnp"
	"github.com/google/go-sev-guest/verify"
	"github.com/pkg/errors"
	chain "github.com/wetee-dao/ink.go"
	"google.golang.org/protobuf/proto"
)

// snp issue
func SnpIssue(pk chain.SignerType, call *model.TeeCall) error {
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

	device, err := client.OpenDevice()
	if err != nil {
		return err
	}
	defer device.Close()

	if len(sig) < 64 {
		return errors.New("signature length must be at least 64 bytes")
	}
	var sig64 [64]byte
	copy(sig64[:], sig[:64])
	attestationReport, err := client.GetExtendedReport(device, sig64)
	if err != nil {
		return errors.New("client.GetExtendedReport:" + err.Error())
	}

	data, err := proto.Marshal(attestationReport)
	if err != nil {
		return err
	}

	// add report to call
	call.Time = timestamp
	call.TeeType = 1
	call.Report = data
	call.Caller = pk.Public()

	return nil
}

// snp verify
func SnpVerify(callData *model.TeeCall) (result *model.TeeVerifyResult, err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			result = nil
			err = errors.New("SnpVerify recover error: " + fmt.Sprint(rerr))
		}
	}()

	reportBytes, timestamp, signer := callData.Report, callData.Time, callData.Caller

	// 解析报告
	attestation := new(sevsnp.Attestation)
	err = proto.Unmarshal(reportBytes, attestation)
	if err != nil {
		return nil, err
	}

	// 验证报告
	options := verify.DefaultOptions()
	if err = verify.SnpAttestation(attestation, options); err != nil {
		return nil, errors.Wrap(err, "verify.SnpAttestation")
	}

	// 构建签名数据
	var buf bytes.Buffer
	buf.Write(Int64ToBytes(timestamp))
	buf.Write(signer)
	var msgBytes []byte
	if callData.Tx != nil {
		var err error
		msgBytes, err = MarshalVT(callData.Tx)
		if err != nil {
			return nil, err
		}
	}
	if len(msgBytes) > 0 {
		buf.Write(msgBytes)
	}

	// 验证签名
	sig := attestation.Report.ReportData
	if !model.Ed25519SignVerify(callData.Caller, buf.Bytes(), sig) {
		return nil, errors.New("invalid report sign")
	}

	return &model.TeeVerifyResult{
		TeeType:       callData.TeeType,
		CodeSigner:    []byte{},
		CodeSignature: []byte{},
		CodeProductId: []byte{},
	}, nil
}
