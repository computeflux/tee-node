package dkgtypes

import (
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/suites"
)

func edSuite() suites.Suite { return crypto.NewSuite(crypto.SuiteEd25519) }

func testPoint(t *testing.T, i int) kyber.Point {
	t.Helper()
	s := edSuite()
	return s.Point().Mul(s.Scalar().SetInt64(int64(i+1)), nil)
}

func testPubKey(t *testing.T, i int) crypto.PubKey {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)
	return *pub
}

func testValidator(t *testing.T, i int) *chaintypes.Validator {
	return &chaintypes.Validator{
		NodeID:      uint64(i + 1),
		ValidatorId: testPubKey(t, i),
		P2pId:       testPubKey(t, i+1),
	}
}

func TestDeepCopyIf(t *testing.T) {
	type sample struct {
		A string
		B []int
		C map[string]string
	}
	src := sample{A: "x", B: []int{1, 2, 3}, C: map[string]string{"k": "v"}}
	dst, err := DeepCopyIf(src)
	require.NoError(t, err)
	assert.Equal(t, src, *dst)

	// 修改副本不影响原值
	dst.B[0] = 99
	dst.C["k"] = "changed"
	assert.Equal(t, []int{1, 2, 3}, src.B)
	assert.Equal(t, "v", src.C["k"])
}

func TestConsensusMsgJSONRoundTrip(t *testing.T) {
	msg := &ConsensusMsg{
		Sponsor:          testValidator(t, 0),
		Epoch:            7,
		EpochTime:        123456789,
		OldValidators:    []*chaintypes.Validator{testValidator(t, 1)},
		Validators:       []*chaintypes.Validator{testValidator(t, 2), testValidator(t, 3)},
		ConsensusNodeNum: 4,
		DealBundles: map[crypto.SuiteType]*DealBundle{
			crypto.SuiteEd25519: {
				DealBundle: &pedersen.DealBundle{
					DealerIndex: 2,
					Deals: []pedersen.Deal{
						{ShareIndex: 0, EncryptedShare: []byte{1, 2, 3}},
					},
					Public:    []kyber.Point{testPoint(t, 0), testPoint(t, 1)},
					SessionID: []byte("sid"),
					Signature: []byte("sig"),
				},
				Suite: "ed25519",
			},
		},
		ShareCommits: map[crypto.SuiteType]KyberPoints{
			crypto.SuiteEd25519: {
				Public: []kyber.Point{testPoint(t, 2)},
				Suite:  "ed25519",
			},
		},
	}

	bt, err := json.Marshal(msg)
	require.NoError(t, err)

	var got ConsensusMsg
	require.NoError(t, json.Unmarshal(bt, &got))

	assert.Equal(t, msg.Sponsor, got.Sponsor)
	assert.Equal(t, msg.Epoch, got.Epoch)
	assert.Equal(t, msg.EpochTime, got.EpochTime)
	assert.Equal(t, msg.ConsensusNodeNum, got.ConsensusNodeNum)
	require.Len(t, got.Validators, 2)
	assert.Equal(t, msg.Validators[0].NodeID, got.Validators[0].NodeID)
	require.Len(t, got.OldValidators, 1)
	require.Len(t, got.DealBundles, 1)

	bundle, ok := got.DealBundles[crypto.SuiteEd25519]
	require.True(t, ok, "ed25519 deal bundle must survive round trip")
	assert.Equal(t, uint32(2), bundle.DealerIndex)
	assert.Equal(t, []byte("sid"), bundle.SessionID)
	require.Len(t, bundle.Public, 2)

	// 点往返后二进制一致
	b0, err := msg.DealBundles[crypto.SuiteEd25519].Public[0].MarshalBinary()
	require.NoError(t, err)
	g0, err := bundle.Public[0].MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, b0, g0)

	commits, ok := got.ShareCommits[crypto.SuiteEd25519]
	require.True(t, ok)
	require.Len(t, commits.Public, 1)
}

func TestConsensusMsg_UnknownSuiteSkipped(t *testing.T) {
	// 未知 suite 键应被跳过而不是报错
	raw := []byte(`{
		"Sponsor": null,
		"DealBundles": {"unknown-suite": {"DealerIndex": 1}},
		"Epoch": 3,
		"ShareCommits": {"ed25519": {"Public": null, "Suite": "ed25519"}},
		"ConsensusNodeNum": 1
	}`)
	var msg ConsensusMsg
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Empty(t, msg.DealBundles)
	assert.Len(t, msg.ShareCommits, 1)
}

func TestConsensusRespMsgJSONRoundTrip(t *testing.T) {
	resp := &ConsensusRespMsg{
		Epoch: 9,
		Responses: map[crypto.SuiteType][]byte{
			crypto.SuiteEd25519:  {1, 2, 3},
			crypto.SuiteBLS12381: {4, 5},
		},
	}
	bt, err := json.Marshal(resp)
	require.NoError(t, err)

	var got ConsensusRespMsg
	require.NoError(t, json.Unmarshal(bt, &got))
	assert.Equal(t, resp.Epoch, got.Epoch)
	require.Len(t, got.Responses, 2)
	assert.Equal(t, []byte{1, 2, 3}, got.Responses[crypto.SuiteEd25519])
	assert.Equal(t, []byte{4, 5}, got.Responses[crypto.SuiteBLS12381])
}

func TestDealBundleJSONRoundTrip(t *testing.T) {
	db := &DealBundle{
		DealBundle: &pedersen.DealBundle{
			DealerIndex: 1,
			Deals: []pedersen.Deal{
				{ShareIndex: 0, EncryptedShare: []byte("enc")},
			},
			Public:    []kyber.Point{testPoint(t, 5), testPoint(t, 6)},
			SessionID: []byte("sess"),
			Signature: []byte("sig"),
		},
		Suite: "ed25519",
	}
	bt, err := json.Marshal(db)
	require.NoError(t, err)

	var got DealBundle
	require.NoError(t, json.Unmarshal(bt, &got))
	require.NotNil(t, got.DealBundle)
	assert.Equal(t, uint32(1), got.DealerIndex)
	assert.Equal(t, []byte("sess"), got.SessionID)
	assert.Equal(t, []byte("sig"), got.Signature)
	assert.Equal(t, "ed25519", got.Suite)
	require.Len(t, got.Deals, 1)
	assert.Equal(t, []byte("enc"), got.Deals[0].EncryptedShare)
	require.Len(t, got.Public, 2)
}

func TestKyberPointsJSONRoundTrip(t *testing.T) {
	kp := KyberPoints{
		Public: []kyber.Point{testPoint(t, 1), testPoint(t, 2)},
		Suite:  "ed25519",
	}
	bt, err := json.Marshal(kp)
	require.NoError(t, err)

	var got KyberPoints
	require.NoError(t, json.Unmarshal(bt, &got))
	require.Len(t, got.Public, 2)
	for i := range kp.Public {
		want, _ := kp.Public[i].MarshalBinary()
		gotB, _ := got.Public[i].MarshalBinary()
		assert.Equal(t, want, gotB)
	}
}

func TestKyberPoints_LegacyArrayFormat(t *testing.T) {
	// 旧格式：直接是点数组，没有 Suite 字段
	p1, _ := testPoint(t, 1).MarshalBinary()
	p2, _ := testPoint(t, 2).MarshalBinary()
	raw := [][]byte{p1, p2}
	bt, err := json.Marshal(raw)
	require.NoError(t, err)

	var got KyberPoints
	require.NoError(t, json.Unmarshal(bt, &got))
	require.Len(t, got.Public, 2)
	assert.Equal(t, "ed25519", got.Suite) // 默认回退
}

func TestPriShareWithSuiteJSONRoundTrip(t *testing.T) {
	s := edSuite()
	scalar := s.Scalar().SetInt64(42)
	ps := &PriShareWithSuite{
		PriShare: &share.PriShare{I: 3, V: scalar},
		Suite:    "ed25519",
	}
	bt, err := json.Marshal(ps)
	require.NoError(t, err)

	var got PriShareWithSuite
	require.NoError(t, json.Unmarshal(bt, &got))
	require.NotNil(t, got.PriShare)
	assert.Equal(t, uint32(3), got.I)
	assert.Equal(t, "ed25519", got.Suite)

	want, _ := scalar.MarshalBinary()
	gotB, _ := got.V.MarshalBinary()
	assert.Equal(t, want, gotB)
}

func TestDistKeyShareRoundTrip(t *testing.T) {
	s := edSuite()
	dks := &DistKeyShare{
		CommitsWrap: KyberPoints{
			Public: []kyber.Point{testPoint(t, 3), testPoint(t, 4)},
			Suite:  "ed25519",
		},
		PriShareWrap: PriShareWithSuite{
			PriShare: &share.PriShare{I: 0, V: s.Scalar().SetInt64(7)},
			Suite:    "ed25519",
		},
	}
	bt, err := json.Marshal(dks)
	require.NoError(t, err)

	var got DistKeyShare
	require.NoError(t, json.Unmarshal(bt, &got))
	require.Len(t, got.Commitments(), 2)
	require.NotNil(t, got.PriShare())
	assert.Equal(t, uint32(0), got.PriShare().I)
}

// ── nil 安全（防止共识热路径 panic） ───────────────────────

func TestNilSafeMarshals(t *testing.T) {
	t.Run("nil_pri_share", func(t *testing.T) {
		var ps PriShareWithSuite
		_, err := json.Marshal(ps)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil private share")
	})

	t.Run("nil_deal_bundle", func(t *testing.T) {
		var db DealBundle
		_, err := json.Marshal(db)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil deal bundle")
	})

	t.Run("nil_point_in_kyber_points", func(t *testing.T) {
		kp := KyberPoints{Public: []kyber.Point{testPoint(t, 1), nil}}
		_, err := json.Marshal(kp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil point")
	})

	t.Run("nil_point_in_deal_bundle", func(t *testing.T) {
		db := DealBundle{
			DealBundle: &pedersen.DealBundle{Public: []kyber.Point{nil}},
		}
		_, err := json.Marshal(db)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil point")
	})
}
