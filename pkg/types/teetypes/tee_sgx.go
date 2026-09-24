package teetypes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
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
		txbuf, err := protoio.MarshalVT(call.Tx)
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
		msgBytes, err = protoio.MarshalVT(reportData.Tx)
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
		msgBytes, err = protoio.MarshalVT(reportData.Tx)
		if err != nil {
			return nil, err
		}
	}
	var reportBytes, timestamp = reportData.Report, reportData.Time

	// decode address
	signer := reportData.Caller

	// call sgx verify service (see tools/sgx-verify-service)
	report, err := callSgxVerifyService(reportBytes)
	if err != nil {
		return nil, err
	}

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

// callSgxVerifyService 调用独立的 sgx 验证服务（tools/sgx-verify-service），
// 返回解析出的 report。服务地址由 SGX_VERIFY_SERVICE_URL 指定，
// 默认 http://xiaobai.asyou.me:8004。服务不可达或验证失败时返回错误（fail-closed）。
func callSgxVerifyService(reportBytes []byte) (*attestation.Report, error) {
	url := os.Getenv("SGX_VERIFY_SERVICE_URL")
	if url == "" {
		url = "http://127.0.0.1:8004"
	}
	payload, err := json.Marshal(map[string]string{"report": base64.StdEncoding.EncodeToString(reportBytes)})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(url, "/")+"/verify", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("call sgx-verify service errors: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("call sgx-verify service errors: " + err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, errors.New("call sgx-verify service errors: " + err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("sgx-verify service " + resp.Status + ": " + string(body))
	}

	report := &attestation.Report{}
	if err := json.Unmarshal(body, report); err != nil {
		return nil, errors.New("call sgx-verify service unmarshal errors: " + err.Error())
	}
	return report, nil
}
