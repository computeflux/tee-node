// Package blscontract generates and verifies BLS12-381 threshold-signature
// vectors for the on-chain BLS gateway contract (EIP-2537, Base mainnet).
//
// The pipeline mirrors exactly what happens in production:
//  1. kyber v4 Pedersen DKG on BLS12-381 (circl) — same suite as tee-node.
//  2. t-of-n threshold signature over hmG1 = MAP_FP_TO_G1(keccak256(msg)) —
//     the same hash-to-curve and aggregation as pkg/dkg partialSignBLS12381 +
//     signBLS12381.
//  3. EIP-2537 conversions from pkg/model/bls12381 (the production format).
//  4. EIP-2537 precompile verification via go-ethereum (the same formulas as
//     BLS.sol._mapFpToG1 + _verifyBLS on Base post-Fusaka).
package blscontract

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing/bls12381/circl"
	"go.dedis.ch/kyber/v4/share"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/sign/schnorr"
)

// Vector is one complete on-chain verification test case for BLS.sol.
type Vector struct {
	Name    string `json:"name"`
	Signers []int  `json:"signers"`

	// Calldata(s) executed on the target by BLS.sol (for batchSubmit: all
	// elements; for submit: the single element). Raw hex in JSON output.
	TargetCalldatas [][]byte `json:"targetCalldatas"`

	// The exact bytes BLS.sol hashes for this vector:
	//   submit      -> keccak256(data)
	//   batchSubmit -> keccak256(abi.encode(datas))
	MsgHash []byte `json:"msgHash"`

	// BLS12_MAP_FP_TO_G1(msgHash) in EIP-2537 uncompressed G1 (128 B)
	HmG1 []byte `json:"hmG1"`

	// Aggregated DKG group public key, G2, EIP-2537 (256 B)
	AggPubkey []byte `json:"aggPubkey"`

	// Recovered threshold signature, compressed G1 (48 B): sig = s * hmG1.
	Signature []byte `json:"signature"`

	// negSig: negated aggregated signature, G1, EIP-2537 (128 B)
	NegSig []byte `json:"negSig"`

	// Full pairing input for the 0x0f precompile:
	// hmG1(128) || aggPubkey(256) || negSig(128) || g2Generator(256) = 768 B
	PairingInput []byte `json:"pairingInput"`
}

// Setup runs a real Pedersen DKG on BLS12-381 (circl) with n participants and
// the given threshold, and returns the resulting key shares plus the group
// public key (a G2 point).
func Setup(n, threshold int) ([]*pedersen.DistKeyShare, kyber.Point, error) {
	// Same suite the production DKG uses for BLS12-381 (the circl adapter:
	// Point() -> G2, Scalar() -> G1 scalar).
	suite := circl.NewSuiteBLS12381()

	nodes := make([]pedersen.Node, n)
	longterms := make([]kyber.Scalar, n)
	for i := 0; i < n; i++ {
		s := suite.Scalar().Pick(suite.RandomStream())
		longterms[i] = s
		nodes[i] = pedersen.Node{Index: uint32(i), Public: suite.Point().Mul(s, nil)}
	}

	nonce := pedersen.GetNonce()
	handlers := make([]*pedersen.DistKeyGenerator, n)
	for i := 0; i < n; i++ {
		h, err := pedersen.NewDistKeyHandler(&pedersen.Config{
			Suite:     suite,
			Longterm:  longterms[i],
			NewNodes:  nodes,
			Threshold: uint32(threshold),
			FastSync:  true,
			Nonce:     nonce,
			Auth:      schnorr.NewScheme(suite),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("dkg handler %d: %w", i, err)
		}
		handlers[i] = h
	}

	deals := make([]*pedersen.DealBundle, 0, n)
	for i, h := range handlers {
		d, err := h.Deals()
		if err != nil {
			return nil, nil, fmt.Errorf("dkg deals %d: %w", i, err)
		}
		deals = append(deals, d)
	}

	responses := make([]*pedersen.ResponseBundle, 0, n)
	for i, h := range handlers {
		resp, err := h.ProcessDeals(deals)
		if err != nil {
			return nil, nil, fmt.Errorf("dkg process deals %d: %w", i, err)
		}
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	shares := make([]*pedersen.DistKeyShare, n)
	for i, h := range handlers {
		res, _, err := h.ProcessResponses(responses)
		if err != nil {
			return nil, nil, fmt.Errorf("dkg process responses %d: %w", i, err)
		}
		if res == nil {
			return nil, nil, fmt.Errorf("dkg %d: no result (protocol incomplete)", i)
		}
		shares[i] = res.Key
	}

	// All shares must be consistent with the same distributed public key.
	groupPub := shares[0].Public()
	for i := 1; i < n; i++ {
		if !shares[i].Public().Equal(groupPub) {
			return nil, nil, fmt.Errorf("dkg share %d public key mismatch", i)
		}
	}
	return shares, groupPub, nil
}

// SignAggregate produces a t-of-n threshold BLS signature (G1, compressed)
// with the given DKG shares over msgHash (a 32-byte value already prepared).
// Each share signs hmG1 = MAP_FP_TO_G1(msgHash) directly (sig_i = s_i * hmG1),
// then the partials are Lagrange-combined into sig = s * hmG1 — the same
// scheme as production partialSignBLS12381 + signBLS12381.
func SignAggregate(shares []*pedersen.DistKeyShare, threshold int, msgHash []byte) ([]byte, error) {
	ps := circl.NewSuite()

	// hmG1 = MAP_FP_TO_G1(msgHash) — the same map the 0x10 precompile
	// performs on-chain.
	hmCompressed, err := bls12381.MapFpToG1Compressed(msgHash)
	if err != nil {
		return nil, fmt.Errorf("map fp to g1: %w", err)
	}
	hmG1 := ps.G1().Point()
	if err := hmG1.UnmarshalBinary(hmCompressed); err != nil {
		return nil, fmt.Errorf("unmarshal hmG1: %w", err)
	}

	pubPoly := share.NewPubPoly(ps.G2(), ps.G2().Point().Base(), shares[0].Commitments())

	pubShares := make([]*share.PubShare, 0, threshold)
	for i := 0; i < threshold && i < len(shares); i++ {
		shareIdx := shares[i].PriShare().I
		sigPoint := ps.G1().Point().Mul(shares[i].PriShare().V, hmG1)
		// Verify the partial against the public polynomial (same check as
		// production signBLS12381): e(hmG1, pubPoly(i)) == e(sig_i, g2Base).
		if !ps.ValidatePairing(hmG1, pubPoly.Eval(shareIdx).V, sigPoint, ps.G2().Point().Base()) {
			return nil, fmt.Errorf("partial signature %d failed pairing check", shareIdx)
		}
		pubShares = append(pubShares, &share.PubShare{I: shareIdx, V: sigPoint})
	}

	fullSig, err := share.RecoverCommit(ps.G1(), pubShares, uint32(threshold), uint32(len(shares)))
	if err != nil {
		return nil, fmt.Errorf("recover commit: %w", err)
	}
	return fullSig.MarshalBinary()
}

// BuildVector converts a recovered threshold signature into all the EIP-2537
// artifacts BLS.sol needs, and verifies them with go-ethereum's EIP-2537
// precompiles (the exact same formulas as the on-chain contract).
func BuildVector(name string, targetCalldatas [][]byte, msgHash, sig []byte, groupPub kyber.Point) (*Vector, error) {
	// hmG1 = BLS12_MAP_FP_TO_G1(msgHash) — EIP-2537 uncompressed G1.
	hmG1, err := bls12381.MapFpToG1(msgHash)
	if err != nil {
		return nil, fmt.Errorf("map fp to g1: %w", err)
	}

	sigEIP, err := bls12381.G1ToEIP2537(sig)
	if err != nil {
		return nil, fmt.Errorf("g1 to eip2537: %w", err)
	}
	negSig, err := bls12381.NegateG1EIP2537(sigEIP)
	if err != nil {
		return nil, fmt.Errorf("negate g1: %w", err)
	}
	aggPubkey, err := bls12381.G2ToEIP2537FromKyber(groupPub)
	if err != nil {
		return nil, fmt.Errorf("g2 to eip2537: %w", err)
	}

	// BLS.sol._verifyBLS pairing input:
	// hmG1(128) || aggPubkey(256) || negSig(128) || g2Generator(256) = 768
	input := make([]byte, 0, 768)
	input = append(input, hmG1...)
	input = append(input, aggPubkey...)
	input = append(input, negSig...)
	input = append(input, bls12381.G2GeneratorEIP2537()...)

	return &Vector{
		Name:            name,
		TargetCalldatas: targetCalldatas,
		MsgHash:         msgHash,
		HmG1:            hmG1,
		AggPubkey:       aggPubkey,
		Signature:       sig,
		NegSig:          negSig,
		PairingInput:    input,
	}, nil
}

// VerifyPairing runs the EIP-2537 pairing precompile over a 768-byte input
// (two 384-byte G1||G2 pairs) and returns an error unless the result is
// exactly uint256(1).
func VerifyPairing(input []byte) error {
	if len(input) != 768 {
		return fmt.Errorf("pairing input must be 768 bytes, got %d", len(input))
	}
	// In geth's PrecompiledContractsBLS map, the EIP-2537 precompiles are keyed
	// by their decimal EIP numbers: 0x10 (16) = pairing, 0x11 (17) = G1 map.
	precompile := vm.PrecompiledContractsBLS[common.BytesToAddress([]byte{0x10})]
	if precompile == nil {
		return fmt.Errorf("EIP-2537 pairing precompile not found")
	}
	out, err := precompile.Run(input)
	if err != nil {
		return fmt.Errorf("pairing precompile failed: %w", err)
	}
	if len(out) != 32 {
		return fmt.Errorf("pairing output must be 32 bytes, got %d", len(out))
	}
	var result common.Hash
	copy(result[:], out)
	if result != common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001") {
		return fmt.Errorf("pairing verification failed: result %s != 1", result.Hex())
	}
	return nil
}

// WriteJSON writes vectors.json to outDir. Chain metadata comes from
// BLS_VECTOR_CHAIN / BLS_VECTOR_CHAIN_ID (default: Base mainnet).
func WriteJSON(outDir string, aggPubkey []byte, vectors []Vector) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	type jsonVector struct {
		Name            string   `json:"name"`
		Signers         []int    `json:"signers"`
		TargetCalldatas []string `json:"targetCalldatas"`
		MsgHash         string   `json:"msgHash"`
		HmG1            string   `json:"hmG1"`
		AggPubkey       string   `json:"aggPubkey"`
		Signature       string   `json:"signature"`
		NegSig          string   `json:"negSig"`
		PairingInput    string   `json:"pairingInput"`
	}
	type output struct {
		Manifest struct {
			Chain       string `json:"chain"`
			ChainID     int64  `json:"chainId"`
			Precompiles struct {
				Pairing   string `json:"pairing"`
				MapFpToG1 string `json:"mapFpToG1"`
			} `json:"precompiles"`
			AggPubkey string `json:"aggPubkey"`
			Vectors   int    `json:"vectors"`
		} `json:"manifest"`
		Vectors []jsonVector `json:"vectors"`
	}

	var out output
	out.Manifest.Chain = os.Getenv("BLS_VECTOR_CHAIN")
	if out.Manifest.Chain == "" {
		out.Manifest.Chain = "base"
	}
	chainID := int64(8453)
	if v := os.Getenv("BLS_VECTOR_CHAIN_ID"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("BLS_VECTOR_CHAIN_ID: %w", err)
		}
		chainID = parsed
	}
	out.Manifest.ChainID = chainID
	out.Manifest.Precompiles.Pairing = "0x000000000000000000000000000000000000000f"
	out.Manifest.Precompiles.MapFpToG1 = "0x0000000000000000000000000000000000000010"
	out.Manifest.AggPubkey = "0x" + hex.EncodeToString(aggPubkey)
	out.Manifest.Vectors = len(vectors)

	for _, v := range vectors {
		jv := jsonVector{
			Name:            v.Name,
			Signers:         v.Signers,
			TargetCalldatas: hexList(v.TargetCalldatas),
			MsgHash:         "0x" + hex.EncodeToString(v.MsgHash),
			HmG1:            "0x" + hex.EncodeToString(v.HmG1),
			AggPubkey:       "0x" + hex.EncodeToString(v.AggPubkey),
			Signature:       "0x" + hex.EncodeToString(v.Signature),
			NegSig:          "0x" + hex.EncodeToString(v.NegSig),
			PairingInput:    "0x" + hex.EncodeToString(v.PairingInput),
		}
		out.Vectors = append(out.Vectors, jv)
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "vectors.json"), b, 0o644)
}

func hexList(items [][]byte) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, "0x"+hex.EncodeToString(item))
	}
	return out
}
