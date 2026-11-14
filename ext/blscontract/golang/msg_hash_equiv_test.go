package blscontract

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// M1 assertion (docs/base-integration-plan.md): the production Go signing path's
// msg-hash rules must match, byte for byte, the canonical Solidity encodings
// recorded in vectors.json:
//   - submit:      keccak256(data)
//   - batchSubmit: keccak256(abi.encode(bytes[] datas))
// These are exactly the rules SignAndSubmit (pkg/chains/evm/extrinsic.go) and
// BLSSignEIP2537/hashToG1BLS12381 (pkg/dkg/dss_bls12381.go) implement.

type vectorEntry struct {
	Name            string   `json:"name"`
	Signers         []int    `json:"signers"`
	TargetCalldatas []string `json:"targetCalldatas"`
	MsgHash         string   `json:"msgHash"`
	HmG1            string   `json:"hmG1"`
	AggPubkey       string   `json:"aggPubkey"`
	Signature       string   `json:"signature"`
	NegSig          string   `json:"negSig"`
}

type vectorsFile struct {
	Vectors []vectorEntry `json:"vectors"`
}

func loadCommittedVectors(t *testing.T) []vectorEntry {
	t.Helper()
	path := os.Getenv("BLS_VECTORS_FILE")
	if path == "" {
		path = filepath.Join("..", "vectors", "vectors.json")
	}
	b, err := os.ReadFile(path)
	require.NoError(t, err, "read committed vectors.json")
	var f vectorsFile
	require.NoError(t, json.Unmarshal(b, &f))
	require.NotEmpty(t, f.Vectors)
	return f.Vectors
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	require.NoError(t, err)
	return b
}

func TestMsgHashRulesMatchVectors(t *testing.T) {
	vectors := loadCommittedVectors(t)
	bytesArrType, err := abi.NewType("bytes[]", "", nil)
	require.NoError(t, err)

	for _, v := range vectors {
		datas := make([][]byte, len(v.TargetCalldatas))
		for i, d := range v.TargetCalldatas {
			datas[i] = mustHex(t, d)
		}
		want := mustHex(t, v.MsgHash)

		if len(datas) == 1 {
			// submit rule (SignAndSubmit signs call.EVM.Data directly;
			// hashToG1BLS12381 maps keccak256(msg)).
			require.Equal(t, want, crypto.Keccak256(datas[0]),
				"%s: submit msgHash rule", v.Name)
			continue
		}

		// batchSubmit rule: abi.Arguments{{Type: bytes[]}}.Pack(datas)
		// — exactly what SignAndSubmit uses to build the batch data, and
		// what BatchCall packs (extrinsic.go BatchCall).
		packed, err := abi.Arguments{{Type: bytesArrType}}.Pack(datas)
		require.NoError(t, err, v.Name)
		require.Equal(t, want, crypto.Keccak256(packed),
			"%s: batchSubmit msgHash rule must equal vectors msgHash", v.Name)

		// generators self-consistency: packDynamicBytesArray == go-ethereum abi Pack
		require.Equal(t, packDynamicBytesArray(datas), packed,
			"%s: generator encoding must equal abi.Arguments(bytes[]).Pack", v.Name)
	}
}
