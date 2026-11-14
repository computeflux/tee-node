package blscontract

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// Default output directory: ext/blscontract/vectors (relative to this package).
// Vectors live with their producer (this package); the evms Solidity side
// reads them from here via BLS_VECTORS_DIR.
var vectorsDir = filepath.Join("..", "vectors")

var (
	bytesType, _   = abi.NewType("bytes", "", nil)
	uint256Type, _ = abi.NewType("uint256", "", nil)
)

// mockTargetExecute builds MockTarget.execute(bytes,uint256) calldata.
func mockTargetExecute(data []byte, value int64) []byte {
	selector := crypto.Keccak256([]byte("execute(bytes,uint256)"))[:4]
	packed, err := abi.Arguments{
		{Type: bytesType},
		{Type: uint256Type},
	}.Pack(data, big.NewInt(value))
	if err != nil {
		panic(err)
	}
	return append(append([]byte{}, selector...), packed...)
}

// mockTargetAlwaysFails builds MockTarget.alwaysFails() calldata.
func mockTargetAlwaysFails() []byte {
	return crypto.Keccak256([]byte("alwaysFails()"))[:4]
}

func TestGenerateVectors(t *testing.T) {
	n, threshold := 5, 3

	// 1. Real Pedersen DKG on BLS12-381 (kyber v4 / circl).
	signers, groupPub, err := Setup(n, threshold)
	require.NoError(t, err)
	t.Logf("DKG done: n=%d t=%d", n, threshold)

	aggPubEIP, err := bls12381.G2ToEIP2537FromKyber(groupPub)
	require.NoError(t, err)
	require.Len(t, aggPubEIP, 256)

	type tc struct {
		name            string
		targetCalldatas [][]byte
	}
	cases := []tc{
		// submit(data, negSig): contract hashes keccak256(data)
		{"submit-single", [][]byte{mockTargetExecute([]byte{0xde, 0xad, 0xbe, 0xef}, 42)}},
		// batchSubmit(datas, negSig): contract hashes
		// keccak256(abi.encode(datas)) — Solidity canonical encoding
		{"batchSubmit-batch", [][]byte{
			mockTargetExecute([]byte{0x01}, 1),
			mockTargetExecute([]byte{0x02, 0x03}, 2),
			mockTargetExecute([]byte{0x04, 0x05, 0x06}, 3),
		}},
		// submit with a target call that always reverts -> CallFailed
		{"submit-callfail", [][]byte{mockTargetAlwaysFails()}},
	}

	vectors := []Vector{}
	for _, c := range cases {
		var msgHash []byte
		switch len(c.targetCalldatas) {
		case 1:
			msgHash = crypto.Keccak256(c.targetCalldatas[0])
		default:
			msgHash = crypto.Keccak256(packDynamicBytesArray(c.targetCalldatas))
		}

		sig, err := SignAggregate(signers, threshold, msgHash)
		require.NoError(t, err, c.name)

		v, err := BuildVector(c.name, c.targetCalldatas, msgHash, sig, groupPub)
		require.NoError(t, err, c.name)

		// off-chain pairing check (same formula as BLS.sol._verifyBLS)
		require.NoError(t, VerifyPairing(v.PairingInput), c.name)

		for i := 0; i < threshold; i++ {
			v.Signers = append(v.Signers, i)
		}
		vectors = append(vectors, *v)
		t.Logf("%s: msgHash=%x... hmG1[16:24]=%x pairing_len=%d OK",
			c.name, v.MsgHash[:4], v.HmG1[16:24], len(v.PairingInput))
	}

	// ── negative control: tampered message must fail pairing ─────────────
	{
		badHash := crypto.Keccak256([]byte("tampered"))
		v, err := BuildVector("negative", nil, badHash, vectors[0].Signature, groupPub)
		require.NoError(t, err)
		err = VerifyPairing(v.PairingInput)
		require.Error(t, err, "tampered message must NOT verify")
		t.Logf("negative control correctly rejected: %v", err)
	}

	// ── write vectors.json to the Solidity project ───────────────────────
	out := os.Getenv("BLS_VECTORS_DIR")
	if out == "" {
		out = vectorsDir
	}
	require.NoError(t, os.MkdirAll(out, 0o755))
	require.NoError(t, WriteJSON(out, aggPubEIP, vectors))
	t.Logf("vectors written to %s", out)
}

// packDynamicBytesArray encodes `abi.encode(bytes[] memory)` exactly as
// Solidity: head = offset(0x20) to the array; array = length word, then a
// head of per-element offsets (relative to the length word), then each
// element encoded as length + padded data.
func packDynamicBytesArray(items [][]byte) []byte {
	out := leftPad32(big.NewInt(0x20))

	// array encoding (offsets inside are relative to the length word)
	arr := leftPad32(big.NewInt(int64(len(items))))
	offset := 32 * len(items)
	elems := [][]byte{}
	for _, item := range items {
		elems = append(elems, packBytes(item))
	}
	for i := range items {
		arr = append(arr, leftPad32(big.NewInt(int64(offset)))...)
		offset += len(elems[i])
	}
	for _, elem := range elems {
		arr = append(arr, elem...)
	}
	return append(out, arr...)
}

// packBytes encodes a single Solidity `bytes` value: length + padded data.
func packBytes(b []byte) []byte {
	padded := make([]byte, (len(b)+31)/32*32)
	copy(padded, b)
	return append(leftPad32(big.NewInt(int64(len(b)))), padded...)
}

func leftPad32(v *big.Int) []byte {
	out := make([]byte, 32)
	v.FillBytes(out)
	return out
}
