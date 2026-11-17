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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/pkg/util"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	chain "github.com/wetee-dao/ink.go"
)

// ── Extrinsic helpers ──────────────────────────────────────

// TEECallToCall builds the EVM calldata for a side-chain hub call. Only
// CallType 0 (WithdrawApply) exists today: it becomes a Token.withdraw /
// withdrawErc20 call executed by the BLS gateway's target (the Token proxy).
func (e *EvmChain) TEECallToCall(tcall *batch.HubCallIndex, dkgKey []byte) (*chaintypes.ChainCall, error) {
	if tcall == nil {
		return nil, errors.New("evm: nil hub call")
	}

	switch tcall.CallType {
	case 0: // WithdrawApply
		wa, err := contracts.QueryWithdrawApply(tcall.CallID)
		if err != nil {
			return nil, fmt.Errorf("evm: query withdraw apply %d: %w", tcall.CallID, err)
		}
		if wa == nil {
			return nil, fmt.Errorf("evm: withdraw apply %d not found", tcall.CallID)
		}
		return buildWithdrawCall(wa)
	default:
		return nil, fmt.Errorf("evm: unsupported hub call type %d", tcall.CallType)
	}
}

// buildWithdrawCall packs the Token withdrawal described by a WithdrawApply:
// withdraw(user, points, nonce) for the native token, or
// withdrawErc20(token, user, points, nonce) when Contract is set.
func buildWithdrawCall(wa *gateway.WithdrawApply) (*chaintypes.ChainCall, error) {
	if wa == nil {
		return nil, errors.New("evm: nil withdraw apply")
	}
	if wa.Credit.Int == nil {
		return nil, errors.New("evm: withdraw apply has nil credit")
	}

	user, err := evmAddress(wa.User.V)
	if err != nil {
		return nil, fmt.Errorf("evm: withdraw apply user: %w", err)
	}

	abiInst, err := bindings.TokenMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("evm: get Token ABI: %w", err)
	}

	method := "withdraw"
	var data []byte
	if !isNativeToken(wa.Contract) {
		token, err := evmAddress(wa.Contract)
		if err != nil {
			return nil, fmt.Errorf("evm: withdraw apply token: %w", err)
		}
		method = "withdrawErc20"
		data, err = abiInst.Pack(method, token, user, wa.Credit.Int, wa.Nonce)
	} else {
		data, err = abiInst.Pack(method, user, wa.Credit.Int, wa.Nonce)
	}
	if err != nil {
		return nil, fmt.Errorf("evm: pack Token.%s: %w", method, err)
	}

	return &chaintypes.ChainCall{EVM: chaintypes.EvmCall{Data: data, Method: method}}, nil
}

// isNativeToken reports whether a WithdrawApply token denotes the native coin.
// Native withdrawals are normally recorded with an empty token, but the
// gateway API also accepts symbol markers such as "ETH"; those must not be
// treated as an ERC20 contract address.
func isNativeToken(token []byte) bool {
	switch strings.ToLower(strings.TrimSpace(string(token))) {
	case "", "eth", "native", "coin", "0x0", "0x00":
		return true
	}
	return false
}

func evmAddress(b []byte) (common.Address, error) {
	if len(b) != 20 {
		return common.Address{}, fmt.Errorf("invalid EVM address length %d (want 20)", len(b))
	}
	return common.BytesToAddress(b), nil
}

// GetCodeVersions is not available on EVM chains (governance lives on the
// Substrate main chain).
func (e *EvmChain) GetCodeVersions() ([]*chaintypes.CodeVersion, error) {
	return nil, fmt.Errorf("evm: GetCodeVersions not supported on chain %d", e.chainId)
}

func (e *EvmChain) TxCallOfSetNextEpoch(nodeId uint64, signer crypto.PubKey) (*chaintypes.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("setNextEpoch", nodeId)
	if err != nil {
		return nil, fmt.Errorf("pack setNextEpoch: %w", err)
	}
	return &chaintypes.ChainCall{EVM: chaintypes.EvmCall{Data: data, Method: "setNextEpoch"}}, nil
}

func (e *EvmChain) BatchCall(method string, calls []chaintypes.ChainCall) (*chaintypes.ChainCall, error) {
	datas := make([][]byte, len(calls))
	for i, c := range calls {
		datas[i] = c.EVM.Data
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
	return &chaintypes.ChainCall{EVM: chaintypes.EvmCall{Data: data, Method: method}}, nil
}

func (e *EvmChain) CallOfMapAccount(signer crypto.PubKey) (*chaintypes.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("mapAccount", signer)
	if err != nil {
		return nil, fmt.Errorf("pack mapAccount: %w", err)
	}
	return &chaintypes.ChainCall{EVM: chaintypes.EvmCall{Data: data, Method: "mapAccount"}}, nil
}

// ── Signing ────────────────────────────────────────────────

// revertReason tries to extract a human-readable revert reason from an
// EstimateGas / CallContract error by unpacking the revert data against
// the given gateway ABI error definitions. abiInst may be nil, in which case
// only Error(string) / Panic(uint256) are decoded.
func revertReason(estimateErr error, msg ethereum.CallMsg, client *ethclient.Client, abiInst *abi.ABI) string {
	raw := extractRevertData(estimateErr)
	if len(raw) < 4 && client != nil {
		// Fallback: re-simulate with CallContract to obtain revert data.
		// Drop the estimation gas cap so the node reports the real revert
		// instead of an out-of-gas error.
		sim := msg
		sim.Gas = 0
		if _, callErr := client.CallContract(context.Background(), sim, nil); callErr != nil {
			raw = extractRevertData(callErr)
		}
	}

	if len(raw) < 4 {
		return ""
	}

	// Match the 4-byte selector against the ABI custom errors.
	if abiInst != nil {
		for name, errDef := range abiInst.Errors {
			if !bytes.Equal(errDef.ID[:4], raw[:4]) {
				continue
			}
			if args, uerr := errDef.Unpack(raw); uerr == nil {
				if list, ok := args.([]interface{}); ok && len(list) > 0 {
					return fmt.Sprintf("reverted: %s(%v)", name, list)
				}
			}
			return fmt.Sprintf("reverted: %s", name)
		}
	}

	switch {
	case bytes.Equal(raw[:4], []byte{0x08, 0xc3, 0x79, 0xa0}): // Error(string)
		if str, uerr := abi.UnpackRevert(raw); uerr == nil {
			return fmt.Sprintf("reverted: %s", str)
		}
	case bytes.Equal(raw[:4], []byte{0x4e, 0x48, 0x7b, 0x71}) && len(raw) >= 36: // Panic(uint256)
		code := new(big.Int).SetBytes(raw[4:36])
		return fmt.Sprintf("reverted: Panic(0x%x)", code)
	}

	return fmt.Sprintf("reverted: 0x%x", raw)
}

// extractRevertData tries to pull the raw revert bytes out of an error
// returned by go-ethereum's JSON-RPC layer.
func extractRevertData(err error) []byte {
	if err == nil {
		return nil
	}
	// Structured JSON-RPC errors carry the revert payload in error.data;
	// geth's Error() string is only "execution reverted", so parsing the
	// string alone loses the selector.
	for e := err; e != nil; e = errors.Unwrap(e) {
		de, ok := e.(rpc.DataError)
		if !ok {
			continue
		}
		if raw := decodeErrorData(de.ErrorData()); len(raw) > 0 {
			return raw
		}
	}

	// Fallback: some RPC proxies embed it in the message as "... 0x...".
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

// decodeErrorData normalises the various shapes an RPC server may use for
// the error.data field.
func decodeErrorData(data interface{}) []byte {
	switch v := data.(type) {
	case nil:
		return nil
	case string:
		return decodeHexString(v)
	case []byte:
		return v
	case hexutil.Bytes:
		return v
	case map[string]interface{}:
		// Some non-geth nodes nest the payload: {"data":"0x..."}.
		for _, k := range []string{"data", "result", "return"} {
			if s, ok := v[k].(string); ok {
				if raw := decodeHexString(s); len(raw) > 0 {
					return raw
				}
			}
		}
	}
	return nil
}

func decodeHexString(s string) []byte {
	s = strings.TrimSpace(s)
	if len(s) < 2 || !strings.EqualFold(s[:2], "0x") {
		return nil
	}
	s = s[2:]
	if len(s)%2 != 0 {
		s = s[:len(s)-1]
	}
	if s == "" {
		return nil
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return raw
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func (e *EvmChain) PartialSign(signer chain.PartialSignerType, call chaintypes.ChainCall) ([]byte, error) {
	return signer.PartialSign(call.EVM.Data)
}

// blsEIP2537Signer is a local interface to avoid importing pkg/dkg
// (which would create an import cycle).
type blsEIP2537Signer interface {
	chain.SignerType
	Suite() crypto.SuiteType
	BLSSignEIP2537(msg []byte) (signature []byte, negSigEIP []byte, hmG1EIP []byte, err error)
}

func (e *EvmChain) getBLSSigner(signer chain.SignerType) (blsEIP2537Signer, error) {
	blsSigner, ok := signer.(blsEIP2537Signer)
	if !ok {
		return nil, errors.New("signer does not support BLS EIP-2537")
	}
	if blsSigner.Suite() != crypto.SuiteBLS12381 {
		return nil, errors.New("signer suite is not BLS12381")
	}
	return blsSigner, nil
}

func (e *EvmChain) blsSign(signer blsEIP2537Signer, msg []byte) (sig, negSig, hmG1EIP []byte, err error) {
	sig, negSig, hmG1EIP, err = signer.BLSSignEIP2537(msg)
	return sig, negSig, hmG1EIP, err
}

// checkAggPubkey verifies that the gateway's configured aggPubkey equals the
// signer's current DKG group key. The on-chain pairing check only uses
// aggPubkey, so a key that was never set or set from a different source (or a
// group re-bootstrapped with a fresh DKG) makes every submit revert with
// BLSSignatureInvalid. Epoch resharing keeps the same group key, so it does not
// change aggPubkey. Fail fast with an actionable message instead of paying for
// an estimate that can only fail.
func (e *EvmChain) checkAggPubkey(signer blsEIP2537Signer) error {
	pub := signer.Public()
	if len(pub) == 0 {
		return nil // signer exposes no group key; let the chain decide
	}
	want, err := bls12381.G2ToEIP2537(pub)
	if err != nil {
		return nil // unexpected encoding; let the chain decide
	}

	e.mu.RLock()
	var onChain []byte
	if e.blsBase != nil {
		onChain, err = e.blsBase.AggPubkey(nil)
	} else if e.bls != nil {
		onChain, err = e.bls.AggPubkey(nil)
	}
	e.mu.RUnlock()
	if err != nil {
		util.LogWithGray("checkAggPubkey", "read aggPubkey error:", err)
		return nil
	}
	if len(onChain) == 0 {
		return fmt.Errorf("BLS gateway %s has no aggPubkey set (submit would revert PubkeyNotSet); set it to 0x%x", e.blsAddr.Hex(), want)
	}
	if !bytes.Equal(onChain, want) {
		return fmt.Errorf("BLS gateway %s aggPubkey mismatch: on-chain 0x%x != current DKG group key 0x%x (set aggPubkey to the current group key; only a fresh DKG bootstrap changes it, epoch resharing does not)",
			e.blsAddr.Hex(), onChain, want)
	}
	return nil
}

// blsABI returns the ABI of the configured BLS gateway, so revert selectors
// decode to that contract's custom errors.
func (e *EvmChain) blsABI() *abi.ABI {
	meta := bindings.BLSMetaData
	if e.blsBase != nil {
		meta = bindings.BaseBLSMetaData
	}
	inst, err := meta.GetAbi()
	if err != nil {
		return nil
	}
	return inst
}

func (e *EvmChain) sendBLSTx(submitData []byte, untilFinalized bool, nonce uint64) error {
	if nonce == 0 {
		// Match ink.go's SignAndSubmit semantics: nonce 0 means "use the next
		// account nonce". Callers (SyncToHub) always pass 0; without this the
		// second submit would reuse nonce 0 and be rejected as "nonce too low".
		e.mu.RLock()
		pending, err := e.client.PendingNonceAt(context.Background(), e.auth.From)
		e.mu.RUnlock()
		if err != nil {
			return fmt.Errorf("pending nonce: %w", err)
		}
		nonce = pending
	}

	e.mu.RLock()
	msg := ethereum.CallMsg{
		From: e.auth.From,
		To:   &e.blsAddr,
		Data: submitData,
		// Simulation cap for eth_estimateGas. The gateway runs two EIP-2537
		// precompiles (0x10 map_fp_to_g1 + 0x0f pairing) plus target.call into
		// Token.withdraw (native/ERC20 transfers); a 500k cap truncates the
		// estimate ("out of gas: gas required exceeds: 500000") on Base.
		Gas: 5_000_000,
	}
	gasLimit, err := e.client.EstimateGas(context.Background(), msg)
	e.mu.RUnlock()
	if err != nil {
		if reason := revertReason(err, msg, e.client, e.blsABI()); reason != "" {
			return fmt.Errorf("estimate gas: %s", reason)
		}
		return fmt.Errorf("estimate gas: %w", err)
	}
	gasLimit = gasLimit * 120 / 100
	if gasLimit < 500_000 {
		gasLimit = 500_000
	}

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
//
// Gateway variants (selected by chainID in client.go):
//   - EthBLS (ETH / Polkadot Hub): submit(data, negSig) — the contract maps
//     keccak256(data) to G1 on-chain via the 0x10 precompile.
//   - BaseBLS (Base / OP-Stack): submit(data, hmG1, negSig) — hmG1 =
//     MAP_FP_TO_G1(keccak256(msg)) is computed off-chain (BLSSignEIP2537) and
//     submitted with the signature. The contract only verifies the pairing
//     e(hmG1, aggPubkey)·e(negSig, g2) == 1, so hmG1 is trusted and the
//     signature is not bound to data on chain. aggPubkey is fixed in the
//     contract and must equal the current DKG group key. Epoch resharing keeps
//     that key unchanged; only a fresh DKG bootstrap produces a new one.
//
// The BLS message is always the exact raw bytes that PartialSign signed
// (call.EVM.Data), so partial signatures collected off-chain recover
// correctly: submit signs keccak256(data); batchSubmit signs
// keccak256(abi.encode(bytes[] datas)) — BatchCall packs exactly that.
func (e *EvmChain) SignAndSubmit(signer chain.SignerType, call chaintypes.ChainCall, untilFinalized bool, nonce uint64) error {
	if e.bls == nil && e.blsBase == nil {
		return errors.New("BLS contract not configured")
	}

	// Try to decode as bytes[] (produced by BatchCall)
	var datas [][]byte
	bytesArrType, _ := abi.NewType("bytes[]", "", nil)
	decoded, err := abi.Arguments{{Type: bytesArrType}}.Unpack(call.EVM.Data)
	if err == nil && len(decoded) > 0 {
		if d, ok := decoded[0].([][]byte); ok && len(d) > 0 {
			datas = d
		}
	}

	var blsFunc string
	if len(datas) > 0 {
		// Any call produced by BatchCall (even a single element) must use
		// batchSubmit: the collected partial signatures cover
		// abi.encode(bytes[] datas), not datas[0].
		blsFunc = "batchSubmit"
	} else {
		datas = [][]byte{call.EVM.Data}
		blsFunc = "submit"
	}
	blsSigner, err := e.getBLSSigner(signer)
	if err != nil {
		return err
	}
	if err := e.checkAggPubkey(blsSigner); err != nil {
		return err
	}
	sig, negSig, hmG1EIP, err := e.blsSign(blsSigner, call.EVM.Data)
	if err != nil {
		return fmt.Errorf("BLS sign: %w", err)
	}

	util.LogWithBlue("SignAndSubmit",
		"func:", blsFunc,
		"calls:", len(datas),
		"sig_len:", len(sig),
		"negSig_len:", len(negSig),
		"hmG1_len:", len(hmG1EIP),
	)

	var submitData []byte
	if e.blsBase != nil {
		// BaseBLS: hmG1 supplied by the signer.
		blsABI, err := bindings.BaseBLSMetaData.GetAbi()
		if err != nil {
			return fmt.Errorf("get BaseBLS ABI: %w", err)
		}
		if hmG1EIP == nil {
			return errors.New("BaseBLS gateway: signer returned no hmG1")
		}
		if blsFunc == "batchSubmit" {
			submitData, err = blsABI.Pack("batchSubmit", datas, hmG1EIP, negSig)
		} else {
			submitData, err = blsABI.Pack("submit", datas[0], hmG1EIP, negSig)
		}
		if err != nil {
			return fmt.Errorf("pack BaseBLS.%s: %w", blsFunc, err)
		}
	} else {
		blsABI, err := bindings.BLSMetaData.GetAbi()
		if err != nil {
			return fmt.Errorf("get BLS ABI: %w", err)
		}
		if blsFunc == "batchSubmit" {
			submitData, err = blsABI.Pack("batchSubmit", datas, negSig)
		} else {
			submitData, err = blsABI.Pack("submit", datas[0], negSig)
		}
		if err != nil {
			return fmt.Errorf("pack BLS.%s: %w", blsFunc, err)
		}
	}

	return e.sendBLSTx(submitData, untilFinalized, nonce)
}
