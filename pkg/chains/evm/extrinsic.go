package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	chain "github.com/wetee-dao/ink.go"
)

// ── Extrinsic helpers ──────────────────────────────────────

func (e *EvmChain) TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*model.ChainCall, error) {
	return &model.ChainCall{Method: "tee_attest"}, nil
}

func (e *EvmChain) TxCallOfSetNextEpoch(nodeId uint64, signer model.ChainAccountID) (*model.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("setNextEpoch", nodeId)
	if err != nil {
		return nil, fmt.Errorf("pack setNextEpoch: %w", err)
	}
	return &model.ChainCall{Data: data, Method: "setNextEpoch"}, nil
}

func (e *EvmChain) BatchCall(method string, calls []model.ChainCall) (*model.ChainCall, error) {
	datas := make([][]byte, len(calls))
	for i, c := range calls {
		datas[i] = c.Data
	}
	bytesArrType, err := abi.NewType("bytes[]", "", nil)
	if err != nil {
		return nil, fmt.Errorf("new bytes[] type: %w", err)
	}
	args := abi.Arguments{{Type: bytesArrType}}
	data, err := args.Pack(datas)
	if err != nil {
		return nil, fmt.Errorf("pack batch: %w", err)
	}
	return &model.ChainCall{Data: data, Method: method}, nil
}

func (e *EvmChain) CallOfMapAccount(signer model.ChainAccountID) (*model.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("mapAccount", signer)
	if err != nil {
		return nil, fmt.Errorf("pack mapAccount: %w", err)
	}
	return &model.ChainCall{Data: data, Method: "mapAccount"}, nil
}

// ── Signing ────────────────────────────────────────────────

// revertReason tries to extract a human-readable revert reason from an
// EstimateGas / CallContract error by unpacking the revert data against
// the Subnet ABI error definitions.
func revertReason(estimateErr error, msg ethereum.CallMsg, client *ethclient.Client) string {
	abiInst, abiErr := bindings.SubnetMetaData.GetAbi()
	if abiErr != nil {
		return ""
	}

	// Try to extract the raw revert data (4-byte selector + payload).
	// go-ethereum sometimes embeds it in the error string as a hex suffix.
	raw := extractRevertData(estimateErr)
	if len(raw) < 4 {
		// Fallback: re-simulate with CallContract to obtain revert data.
		_, callErr := client.CallContract(context.Background(), msg, nil)
		if callErr != nil {
			raw = extractRevertData(callErr)
		}
	}

	if len(raw) < 4 {
		return ""
	}

	// Match the 4-byte selector against ABI errors.
	for _, e := range abiInst.Errors {
		if len(e.ID) == 4 && bytes.Equal(e.ID[:4], raw[:4]) {
			// Try to unpack error arguments (best-effort).
			if args, uerr := e.Unpack(raw[4:]); uerr == nil && args != nil {
				return fmt.Sprintf("reverted: %s(%v)", e.Name, args)
			}
			return fmt.Sprintf("reverted: %s", e.Name)
		}
	}

	// Legacy Error(string) selector 0x08c379a0
	if len(raw) >= 68 && bytes.Equal(raw[:4], []byte{0x08, 0xc3, 0x79, 0xa0}) {
		if str, uerr := abi.UnpackRevert(raw); uerr == nil {
			return fmt.Sprintf("reverted: %s", str)
		}
	}

	return fmt.Sprintf("reverted: 0x%x", raw)
}

// extractRevertData tries to pull the raw revert bytes out of an error
// returned by go-ethereum's JSON-RPC layer.
func extractRevertData(err error) []byte {
	if err == nil {
		return nil
	}
	// Common RPC pattern: "execution reverted: 0x..."
	msg := err.Error()
	const prefix = "0x"
	// Walk through all descendants in the error chain.
	for {
		if idx := strings.Index(msg, prefix); idx >= 0 {
			rest := msg[idx+len(prefix):]
			// Take only hex characters.
			end := 0
			for end < len(rest) && isHex(rest[end]) {
				end++
			}
			if end > 0 {
				data, _ := hex.DecodeString(rest[:end])
				return data
			}
		}
		// Unwrap
		unwrap := errors.Unwrap(err)
		if unwrap == nil {
			break
		}
		err = unwrap
		msg = err.Error()
	}
	return nil
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func (e *EvmChain) PartialSign(signer chain.PartialSignerType, call model.ChainCall) ([]byte, error) {
	return signer.PartialSign(call.Data)
}

// blsEIP2537Signer is a local interface to avoid importing pkg/dkg
// (which would create an import cycle).
type blsEIP2537Signer interface {
	chain.SignerType
	Suite() model.SuiteType
	BLSSignEIP2537(msg []byte) (signature []byte, negSigEIP []byte, hmG1EIP []byte, err error)
}

func (e *EvmChain) getBLSSigner(signer chain.SignerType) (blsEIP2537Signer, error) {
	blsSigner, ok := signer.(blsEIP2537Signer)
	if !ok {
		return nil, errors.New("signer does not support BLS EIP-2537")
	}
	if blsSigner.Suite() != model.SuiteBLS12381 {
		return nil, errors.New("signer suite is not BLS12381")
	}
	return blsSigner, nil
}

func (e *EvmChain) blsSign(signer chain.SignerType, msgHash []byte) (sig, negSig, hmG1 []byte, err error) {
	blsSigner, err := e.getBLSSigner(signer)
	if err != nil {
		return nil, nil, nil, err
	}
	return blsSigner.BLSSignEIP2537(msgHash)
}

func (e *EvmChain) sendBLSTx(submitData []byte, untilFinalized bool, nonce uint64) error {
	e.mu.RLock()
	msg := ethereum.CallMsg{
		From: e.auth.From,
		To:   &e.blsAddr,
		Data: submitData,
		Gas:  500000,
	}
	gasLimit, err := e.client.EstimateGas(context.Background(), msg)
	e.mu.RUnlock()
	if err != nil {
		if reason := revertReason(err, msg, e.client); reason != "" {
			return fmt.Errorf("estimate gas: %s", reason)
		}
		return fmt.Errorf("estimate gas: %w", err)
	}
	gasLimit = gasLimit * 120 / 100

	e.mu.RLock()
	gasPrice, err := e.client.SuggestGasPrice(context.Background())
	e.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("suggest gas price: %w", err)
	}

	tx := types.NewTransaction(
		nonce,
		e.blsAddr,
		big.NewInt(0),
		gasLimit,
		gasPrice,
		submitData,
	)

	signedTx, err := e.auth.Signer(e.auth.From, tx)
	if err != nil {
		return fmt.Errorf("sign tx: %w", err)
	}
	e.mu.RLock()
	err = e.client.SendTransaction(context.Background(), signedTx)
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			err = e.client.SendTransaction(context.Background(), signedTx)
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return fmt.Errorf("send tx: %w", err)
	}
	if untilFinalized {
		_, err = bind.WaitMined(context.Background(), e.client, signedTx)
		return err
	}
	return nil
}

// SignAndSubmit submits a transaction (single or batch) to the chain via the BLS gateway contract.
// If call.Data was produced by BatchCall, it is decoded as bytes[] and routed to BLS.batchSubmit;
// otherwise it is treated as a single call and routed to BLS.submit.
func (e *EvmChain) SignAndSubmit(signer chain.SignerType, call model.ChainCall, untilFinalized bool, nonce uint64) error {
	if e.bls == nil {
		return errors.New("BLS contract not configured")
	}

	// Try to decode as bytes[] (produced by BatchCall)
	var datas [][]byte
	bytesArrType, _ := abi.NewType("bytes[]", "", nil)
	decoded, err := abi.Arguments{{Type: bytesArrType}}.Unpack(call.Data)
	if err == nil && len(decoded) > 0 {
		if d, ok := decoded[0].([][]byte); ok && len(d) > 0 {
			datas = d
		}
	}

	// The BLS message must be the exact raw bytes that PartialSign signed,
	// so that the partial signatures collected off-chain can be recovered
	// on-chain.  We use call.Data directly (no extra keccak256) to stay
	// compatible with PartialSign(signer, call).
	var blsFunc string
	var msgHash [32]byte
	if len(datas) > 1 {
		blsFunc = "batchSubmit"
		msgHash = crypto.Keccak256Hash(call.Data)
	} else {
		if len(datas) == 0 {
			datas = [][]byte{call.Data}
		}
		blsFunc = "submit"
		msgHash = crypto.Keccak256Hash(call.Data)
	}
	sig, negSig, hmG1, err := e.blsSign(signer, call.Data)
	if err != nil {
		return fmt.Errorf("BLS sign: %w", err)
	}

	util.LogWithBlue("SignAndSubmit",
		"func:", blsFunc,
		"calls:", len(datas),
		"sig_len:", len(sig),
		"negSig_len:", len(negSig),
		"hmG1_len:", len(hmG1),
	)

	blsABI, err := bindings.BLSMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("get BLS ABI: %w", err)
	}

	var submitData []byte
	if blsFunc == "batchSubmit" {
		submitData, err = blsABI.Pack("batchSubmit", datas, negSig, hmG1, msgHash)
	} else {
		submitData, err = blsABI.Pack("submit", datas[0], negSig, hmG1, msgHash)
	}
	if err != nil {
		return fmt.Errorf("pack BLS.%s: %w", blsFunc, err)
	}

	return e.sendBLSTx(submitData, untilFinalized, nonce)
}
