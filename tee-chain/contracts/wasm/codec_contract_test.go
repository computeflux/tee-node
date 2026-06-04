package wasm

import (
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// gov — 治理模块
// ===========================================================================

func TestCodec_Gov_Member(t *testing.T) {
	v := gov.Member{Account: store.UniAddr{T: 1, V: []byte("alice")}, Balance: store.Amount{Int: big.NewInt(100)}}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Member_Zero(t *testing.T) {
	v := gov.Member{}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_TrackData(t *testing.T) {
	v := gov.TrackData{
		Name: "root", PreparePeriod: 100, MaxDeciding: 50,
		ConfirmPeriod: 20, DecisionPeriod: 30, MinEnactmentPeriod: 10,
		DecisionDeposit: store.Amount{Int: big.NewInt(1000)},
		MaxBalance:      store.Amount{Int: big.NewInt(1000000)},
		MinApproval:     gov.NewLinearDecreasingCurve(10000, 5000, 1000),
		MinSupport:      gov.NewReciprocalCurve(1000, 10, 0, 0),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_TrackData_SteppedCurve(t *testing.T) {
	v := gov.TrackData{
		Name: "stepped", PreparePeriod: 50,
		DecisionDeposit: store.Amount{Int: big.NewInt(500)},
		MinApproval:     gov.NewSteppedDecreasingCurve(8000, 2000, 500, 100),
		MinSupport:      gov.NewLinearDecreasingCurve(6000, 3000, 500),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_TrackData_AllCurves(t *testing.T) {
	v := gov.TrackData{
		Name: "all_curves",
		MinApproval: gov.Curve{
			Type:              gov.CurveTypeReciprocal,
			ReciprocalFactor:  10000,
			ReciprocalXScale:  100,
			ReciprocalXOffset: 10,
			ReciprocalYOffset: 5,
		},
		MinSupport: gov.Curve{
			Type:         gov.CurveTypeLinearDecreasing,
			LinearBegin:  9000,
			LinearEnd:    1000,
			LinearLength: 2000,
		},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_TrackWithID(t *testing.T) {
	v := gov.TrackWithID{
		ID: 7,
		Track: gov.TrackData{
			Name: "treasury", PreparePeriod: 200,
			DecisionDeposit: store.Amount{Int: big.NewInt(1000)},
			MinApproval:     gov.NewLinearDecreasingCurve(8000, 4000, 500),
			MinSupport:      gov.NewSteppedDecreasingCurve(5000, 1000, 200, 50),
		},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_CallContent(t *testing.T) {
	v := gov.CallContent{
		Contract: []byte{1, 2, 3, 4},
		Selector: [4]byte{0xde, 0xad, 0xbe, 0xef},
		Args:     [][]byte{{10, 20}, {30, 40, 50}},
		Amount:   store.Amount{Int: big.NewInt(500)},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_CallContent_Zero(t *testing.T) {
	v := gov.CallContent{}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalStatus(t *testing.T) {
	for _, st := range []uint8{
		gov.ProposalStatusPending, gov.ProposalStatusOngoing,
		gov.ProposalStatusConfirming, gov.ProposalStatusConfirmed,
		gov.ProposalStatusApproved, gov.ProposalStatusRejected,
		gov.ProposalStatusCanceled,
	} {
		v := gov.ProposalStatus{State: st, Block: 1000}
		testRoundTrip(t, &v)
	}
}

func TestCodec_Gov_ProposalStatusQuery(t *testing.T) {
	v := gov.ProposalStatusQuery{
		State: gov.ProposalStatusOngoing, BlockHeight: 100,
		ConfirmedNumber: 5, LastConfirmedBlock: 95,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalDeposit(t *testing.T) {
	v := gov.ProposalDeposit{
		Depositor: store.UniAddr{T: 1, V: []byte("depositor")},
		Amount:    store.Amount{Int: big.NewInt(10000)},
		Block:     500,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalDeposit_Zero(t *testing.T) {
	v := gov.ProposalDeposit{}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Proposal(t *testing.T) {
	v := gov.Proposal{
		Call: store.NewSome(gov.CallContent{
			Contract: []byte{1, 2, 3, 4},
			Selector: [4]byte{0xaa, 0xbb, 0xcc, 0xdd},
			Args:     [][]byte{{1}},
			Amount:   store.Amount{Int: big.NewInt(100)},
		}),
		TrackID:     1,
		Caller:      store.UniAddr{T: 1, V: []byte("proposer")},
		Status:      gov.ProposalStatus{State: gov.ProposalStatusOngoing, Block: 200},
		SubmitBlock: 200,
		Deposit: gov.ProposalDeposit{
			Depositor: store.UniAddr{T: 1, V: []byte("proposer")},
			Amount:    store.Amount{Int: big.NewInt(500)},
			Block:     200,
		},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Proposal_NoneCall(t *testing.T) {
	v := gov.Proposal{
		Call:        store.NewNone[gov.CallContent](),
		TrackID:     2,
		Caller:      store.UniAddr{T: 1, V: []byte("proposer")},
		Status:      gov.ProposalStatus{State: gov.ProposalStatusPending, Block: 100},
		SubmitBlock: 100,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalWithID(t *testing.T) {
	v := gov.ProposalWithID{
		ID: 42,
		Proposal: gov.Proposal{
			Call:        store.NewNone[gov.CallContent](),
			TrackID:     3,
			Caller:      store.UniAddr{T: 2, V: []byte("caller")},
			Status:      gov.ProposalStatus{State: gov.ProposalStatusRejected, Block: 999},
			SubmitBlock: 100,
		},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalResult(t *testing.T) {
	v := gov.ProposalResult{
		Result:    store.NewNone[[]byte](),
		ExecError: store.NewSome([]byte("execution failed")),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalResult_BothSome(t *testing.T) {
	v := gov.ProposalResult{
		Result:    store.NewSome([]byte{0x01, 0x02, 0x03}),
		ExecError: store.NewNone[[]byte](),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalResult_BothNone(t *testing.T) {
	v := gov.ProposalResult{
		Result:    store.NewNone[[]byte](),
		ExecError: store.NewNone[[]byte](),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Vote(t *testing.T) {
	v := gov.Vote{
		Index:      1,
		ProposalID: 42,
		Caller:     store.UniAddr{T: 1, V: []byte("alice")},
		Pledge:     store.Amount{Int: big.NewInt(1000)},
		VoteWeight: store.Amount{Int: big.NewInt(500)},
		OpinionYes: true,
		VoteBlock:  500,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Vote_Deleted(t *testing.T) {
	v := gov.Vote{
		Index: 2, ProposalID: 43,
		Caller:     store.UniAddr{T: 1, V: []byte("bob")},
		OpinionYes: false, Deleted: true, VoteBlock: 600,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_VoteRef(t *testing.T) {
	v := gov.VoteRef{ProposalID: 42, VoteIndex: 3}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Spend(t *testing.T) {
	v := gov.Spend{
		ID:      1,
		Caller:  store.UniAddr{T: 1, V: []byte("alice")},
		To:      store.UniAddr{T: 2, V: []byte("bob")},
		Amount:  store.Amount{Int: big.NewInt(500)},
		TrackID: 1,
		Payout:  true,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Spend_NotPayout(t *testing.T) {
	v := gov.Spend{
		ID: 2, Caller: store.UniAddr{T: 1, V: []byte("carol")},
		Amount: store.Amount{Int: big.NewInt(0)}, Payout: false,
	}
	testRoundTrip(t, &v)
}

// ===========================================================================
// auth — 认证模块
// ===========================================================================

func TestCodec_Auth_UserInfo(t *testing.T) {
	v := auth.UserInfo{
		EmailHash:      []byte("hash"),
		ShareRoot:      make([]byte, 32),
		DerivedAddress: []byte{1, 2, 3},
		RegisteredAt:   100,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_UserInfo_Full(t *testing.T) {
	v := auth.UserInfo{
		EmailHash:      make([]byte, 32),
		ShareRoot:      make([]byte, 32),
		DerivedAddress: make([]byte, 20),
		RegisteredAt:   math.MaxUint64,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_AuthUserRecord(t *testing.T) {
	v := auth.AuthUserRecord{
		Ref:        "alice@example.com",
		Ciphertext: []byte{0xde, 0xad, 0xbe, 0xef},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_AuthUserRecord_Empty(t *testing.T) {
	v := auth.AuthUserRecord{}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_PrivateKeyRecord(t *testing.T) {
	v := auth.PrivateKeyRecord{
		Ref:        "bob@example.com",
		Ciphertext: make([]byte, 128),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_PrivateKeyRecord_Empty(t *testing.T) {
	v := auth.PrivateKeyRecord{}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_RecoveryConfig(t *testing.T) {
	v := auth.RecoveryConfig{
		Guardians:     [][]byte{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}},
		Threshold:     3,
		RecoveryEmail: []byte("recovery@example.com"),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_RecoveryConfig_Zero(t *testing.T) {
	v := auth.RecoveryConfig{}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_RecoveryRequest(t *testing.T) {
	v := auth.RecoveryRequest{
		EmailHash:    []byte("hash123"),
		NewShareRoot: make([]byte, 32),
		Approvals:    [][]byte{{1, 2}, {3, 4}},
		ExpiresAt:    99999,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_RecoveryRequest_Zero(t *testing.T) {
	v := auth.RecoveryRequest{}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_AuthConfig(t *testing.T) {
	v := auth.AuthConfig{
		SMTPHost: "smtp.test.com", SMTPPort: 465, MockEmail: true,
		JWTSecret: "secret123", SeedSecret: "seed456",
		SMTPUser: "user", SMTPPassword: "pass", SMTPFrom: "noreply@test.com",
		SMTPFromName: "Test", OTPLength: 6, OTPTTLMins: 10,
		GoogleClientID: "gid", GoogleClientSecret: "gsecret",
		GoogleRedirectURL: "https://test.com/oauth/google",
	}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_AuthConfig_Full(t *testing.T) {
	v := auth.AuthConfig{
		JWTSecret:           "jwt",
		SeedSecret:          "seed",
		SMTPHost:            "smtp.example.com",
		SMTPPort:            587,
		SMTPUser:            "admin",
		SMTPPassword:        "pwd",
		SMTPFrom:            "no-reply@example.com",
		SMTPFromName:        "MyApp",
		MockEmail:           false,
		OTPLength:           6,
		OTPTTLMins:          5,
		GoogleClientID:      "google-id",
		GoogleClientSecret:  "google-secret",
		GoogleRedirectURL:   "https://app.com/auth/google/callback",
		GitHubClientID:      "github-id",
		GitHubClientSecret:  "github-secret",
		GitHubRedirectURL:   "https://app.com/auth/github/callback",
		TwitterClientID:     "twitter-id",
		TwitterClientSecret: "twitter-secret",
		TwitterRedirectURL:  "https://app.com/auth/twitter/callback",
	}
	testRoundTrip(t, &v)
}

// ===========================================================================
// gateway — API 网关模块
// ===========================================================================

func TestCodec_Gateway_APIKeyInfo(t *testing.T) {
	v := gateway.APIKeyInfo{
		OwnerKey: 12345, Owner: store.UniAddr{T: 1, V: []byte("alice")},
		KeyHash: make([]byte, 32), RateLimit: 100, IsActive: true, CreatedAt: 1234567890,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_APIKeyInfo_Inactive(t *testing.T) {
	v := gateway.APIKeyInfo{
		OwnerKey: 0, Owner: store.UniAddr{T: 0, V: nil},
		KeyHash: nil, RateLimit: 0, IsActive: false, CreatedAt: 0,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ModelInfo(t *testing.T) {
	v := gateway.ModelInfo{
		Name: "gpt-4", Owner: store.UniAddr{T: 1, V: []byte("openai")},
		ProviderID: 1, InputModalities: gateway.ModalityText,
		ContextLength: 128000, InPrice: 100, CachedPrice: 50, OutPrice: 200,
		ProviderType: gateway.ProviderOpenAI, ModelID: "gpt-4-turbo",
		URI: "https://api.openai.com", APIKeys: []string{"sk-xxx"},
		IsActive: true, CreatedAt: 1234567890,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ModelInfo_MultiModality(t *testing.T) {
	v := gateway.ModelInfo{
		Name: "gemini-pro", Owner: store.UniAddr{T: 1, V: []byte("google")},
		ProviderID:      2,
		InputModalities: gateway.ModalityText | gateway.ModalityImage | gateway.ModalityAudio,
		ProviderType:    gateway.ProviderGemini,
		APIKeys:         []string{"key1", "key2", "key3"},
		IsActive:        true,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ModelInfoWithID(t *testing.T) {
	v := gateway.ModelInfoWithID{
		ID: 99,
		Data: gateway.ModelInfo{
			Name: "llama", ProviderID: 3,
			ProviderType: gateway.ProviderLlama,
			ModelID:      "llama-3-70b",
			IsActive:     true,
		},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ModelInfoWithProvider(t *testing.T) {
	v := gateway.ModelInfoWithProvider{
		ID: 10,
		Model: gateway.ModelInfo{
			Name: "claude", ProviderID: 2,
			ProviderType: gateway.ProviderClaude,
			IsActive:     true,
		},
		Provider: gateway.ProviderInfo{
			Name: "anthropic", Owner: store.UniAddr{T: 1, V: []byte("anthropic")},
			Region: gateway.RegionNorthAmerica, Deposit: 1000000,
			FeeRate: 100, Priority: 10, IsActive: true, CreatedAt: 1234567890,
		},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ProviderInfo(t *testing.T) {
	v := gateway.ProviderInfo{
		Name: "openai", Owner: store.UniAddr{T: 1, V: []byte("openai")},
		IsActive: true,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ProviderInfo_Full(t *testing.T) {
	v := gateway.ProviderInfo{
		Owner:         store.UniAddr{T: 1, V: []byte("owner")},
		Name:          "MyProvider",
		LogoURL:       "https://cdn.example.com/logo.png",
		Description:   "Best AI provider ever",
		ModelIds:      []uint64{1, 2, 3, 4, 5},
		Region:        gateway.RegionAsiaEast | gateway.RegionEurope,
		Deposit:       math.MaxUint64,
		FeeRate:       500,
		Priority:      100,
		TotalRequests: 9999999,
		SuccessRate:   9990,
		AvgLatency:    150,
		UptimeSLA:     9999,
		IsActive:      true,
		CreatedAt:     math.MaxUint64,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ProviderInfoWithID(t *testing.T) {
	v := gateway.ProviderInfoWithID{
		ID: 5,
		Data: gateway.ProviderInfo{
			Name:     "azure",
			Owner:    store.UniAddr{T: 1, V: []byte("microsoft")},
			ModelIds: []uint64{10, 20},
			Region:   gateway.RegionNorthAmerica | gateway.RegionEurope,
			IsActive: true,
		},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_Settlement(t *testing.T) {
	v := gateway.Settlement{
		User:        store.UniAddr{T: 1, V: []byte("user")},
		ModelID:     42,
		ProviderID:  7,
		TokenIn:     1500,
		TokenOut:    800,
		CachedToken: 200,
		Count:       3,
		Nonce:       make([]byte, 16),
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_DailyUsage(t *testing.T) {
	v := gateway.DailyUsage{
		User:        store.UniAddr{T: 1, V: []byte("user")},
		ProviderID:  1,
		ModelID:     2,
		Count:       500,
		TokenIn:     10000,
		TokenOut:    5000,
		CachedToken: 1000,
		PointsUsed:  250,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_RechargeRecord(t *testing.T) {
	v := gateway.RechargeRecord{
		User: store.UniAddr{T: 1, V: []byte("alice")}, Amount: 1000, Time: 1234567890,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_RechargeRecord_NegativeTime(t *testing.T) {
	v := gateway.RechargeRecord{
		User: store.UniAddr{T: 1, V: []byte("alice")}, Amount: 0, Time: -1,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_UserSettlementRecord(t *testing.T) {
	v := gateway.UserSettlementRecord{
		ProviderID:  10,
		ModelID:     20,
		TokenIn:     3000,
		TokenOut:    1500,
		CachedToken: 500,
		Cost:        75,
		Count:       5,
		Time:        1234567890,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ProviderSettlementRecord(t *testing.T) {
	v := gateway.ProviderSettlementRecord{
		User:        store.UniAddr{T: 1, V: []byte("user")},
		ModelID:     30,
		TokenIn:     5000,
		TokenOut:    2500,
		CachedToken: 800,
		Cost:        120,
		Count:       10,
		Time:        1234567890,
	}
	testRoundTrip(t, &v)
}

// ===========================================================================
// batch — 批处理模块
// ===========================================================================

func TestCodec_Batch_BatchCall(t *testing.T) {
	v := batch.BatchCall{Name: []byte("gov"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{10, 20}}}
	testRoundTrip(t, &v)
}

func TestCodec_Batch_BatchCall_MultiArgs(t *testing.T) {
	v := batch.BatchCall{
		Name:   []byte("treasury"),
		Method: []byte{0xaa, 0xbb, 0xcc, 0xdd},
		Args:   [][]byte{make([]byte, 32), {0x01}, {0xff, 0xfe}},
	}
	testRoundTrip(t, &v)
}

func TestCodec_Batch_BatchCall_Empty(t *testing.T) {
	v := batch.BatchCall{}
	testRoundTrip(t, &v)
}

// ===========================================================================
// store 基础类型
// ===========================================================================

func TestCodec_BlockNumber(t *testing.T) {
	// BlockNumber 底层是 uint32，验证基础 uint32 编解码
	for _, v := range []uint32{0, 1, 42, math.MaxUint32} {
		data, _ := store.ScaleEncode(v)
		var out uint32
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Equal(t, v, out)
	}
}

func TestCodec_Bytes(t *testing.T) {
	for _, v := range []store.Bytes{nil, {}, {0x01}, {0xde, 0xad, 0xbe, 0xef}} {
		data, _ := store.ScaleEncode(v)
		var out store.Bytes
		require.NoError(t, store.ScaleDecode(&out, data))
		if len(v) == 0 {
			require.Empty(t, out)
		} else {
			require.Equal(t, v, out)
		}
	}
}

func TestCodec_UniAddr_Zero(t *testing.T) {
	v := store.UniAddr{}
	testRoundTrip(t, &v)
}

func TestCodec_UniAddr_Max(t *testing.T) {
	v := store.UniAddr{T: math.MaxUint32, V: make([]byte, 256)}
	testRoundTrip(t, &v)
}

func TestCodec_U256_Boundary(t *testing.T) {
	for _, val := range []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(1000000),
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)), // 2^128 - 1
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1)), // 2^255 - 1
	} {
		v := store.U256{Int: val}
		testRoundTrip(t, &v)
	}
}

func TestCodec_Amount(t *testing.T) {
	for _, val := range []*big.Int{
		big.NewInt(0), big.NewInt(1), big.NewInt(1000),
	} {
		v := store.Amount{Int: val}
		testRoundTrip(t, &v)
	}
}

func TestCodec_Int(t *testing.T) {
	for _, v := range []int{0, -1, 1, 65536, math.MinInt64, math.MaxInt64} {
		data, _ := store.ScaleEncode(v)
		var out int
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Equal(t, v, out)
	}
}

// ===========================================================================
// store.Option — 各种类型
// ===========================================================================

func TestCodec_OptionNone(t *testing.T) {
	v := store.NewNone[uint32]()
	data, _ := store.ScaleEncode(v)
	require.Equal(t, []byte{0x00}, data)
	var out store.Option[uint32]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.False(t, out.IsSome())
}

func TestCodec_OptionSome_Uint32(t *testing.T) {
	v := store.NewSome(uint32(42))
	data, _ := store.ScaleEncode(v)
	var out store.Option[uint32]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.True(t, out.IsSome())
	val, _ := out.UnWrap()
	require.Equal(t, uint32(42), val)
}

func TestCodec_Option_String(t *testing.T) {
	v := store.NewSome("hello world")
	data, _ := store.ScaleEncode(v)
	var out store.Option[string]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.True(t, out.IsSome())
	val, _ := out.UnWrap()
	require.Equal(t, "hello world", val)
}

func TestCodec_Option_Bytes(t *testing.T) {
	v := store.NewSome([]byte{0xde, 0xad, 0xbe, 0xef})
	data, _ := store.ScaleEncode(v)
	var out store.Option[[]byte]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.True(t, out.IsSome())
	val, _ := out.UnWrap()
	require.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, val)
}

func TestCodec_Option_NoneBytes(t *testing.T) {
	v := store.NewNone[[]byte]()
	data, _ := store.ScaleEncode(v)
	require.Equal(t, []byte{0x00}, data)
}

func TestCodec_OptionOfTrackData(t *testing.T) {
	v := store.NewSome(gov.TrackData{
		Name: "custom", PreparePeriod: 50,
		DecisionDeposit: store.Amount{Int: big.NewInt(100)},
		MaxBalance:      store.Amount{Int: big.NewInt(10000)},
	})
	data, _ := store.ScaleEncode(v)
	var out store.Option[gov.TrackData]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.True(t, out.IsSome())
}

func TestCodec_OptionOfUniAddr(t *testing.T) {
	v := store.NewSome(store.UniAddr{T: 42, V: []byte("addr")})
	data, _ := store.ScaleEncode(v)
	var out store.Option[store.UniAddr]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.True(t, out.IsSome())
	val, _ := out.UnWrap()
	require.Equal(t, uint32(42), val.T)
	require.Equal(t, []byte("addr"), val.V)
}

func TestCodec_OptionOfU256(t *testing.T) {
	v := store.NewSome(store.U256{Int: big.NewInt(99999)})
	data, _ := store.ScaleEncode(v)
	var out store.Option[store.U256]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.True(t, out.IsSome())
	val, _ := out.UnWrap()
	require.Equal(t, int64(99999), val.Int.Int64())
}

// ===========================================================================
// Slice — 切片类型
// ===========================================================================

func TestCodec_SliceOfAPIKeyInfo(t *testing.T) {
	v := []gateway.APIKeyInfo{
		{OwnerKey: 1, IsActive: true, KeyHash: make([]byte, 32)},
		{OwnerKey: 2, IsActive: false, KeyHash: make([]byte, 32)},
	}
	data, _ := store.ScaleEncode(v)
	var out []gateway.APIKeyInfo
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Len(t, out, 2)
	require.Equal(t, uint64(1), out[0].OwnerKey)
	require.Equal(t, uint64(2), out[1].OwnerKey)
}

func TestCodec_SliceOfBatchCall(t *testing.T) {
	v := []batch.BatchCall{
		{Name: []byte("gov"), Method: []byte{1, 2, 3, 4}},
		{Name: []byte("auth"), Method: []byte{5, 6, 7, 8}},
	}
	data, _ := store.ScaleEncode(v)
	var out []batch.BatchCall
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Len(t, out, 2)
}

func TestCodec_SliceOfModelInfo(t *testing.T) {
	v := []gateway.ModelInfo{
		{Name: "gpt-4", ProviderID: 1, IsActive: true},
		{Name: "claude", ProviderID: 2, IsActive: true},
		{Name: "gemini", ProviderID: 3, IsActive: false},
	}
	data, _ := store.ScaleEncode(v)
	var out []gateway.ModelInfo
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Len(t, out, 3)
	require.Equal(t, "gpt-4", out[0].Name)
	require.Equal(t, "claude", out[1].Name)
	require.Equal(t, "gemini", out[2].Name)
}

func TestCodec_SliceOfString(t *testing.T) {
	v := []string{"a", "bb", "ccc", ""}
	data, _ := store.ScaleEncode(v)
	var out []string
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Equal(t, v, out)
}

func TestCodec_SliceOfUint64(t *testing.T) {
	v := []uint64{0, 1, math.MaxUint32, math.MaxUint64}
	data, _ := store.ScaleEncode(v)
	var out []uint64
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Equal(t, v, out)
}

func TestCodec_SliceEmpty(t *testing.T) {
	for _, v := range [][]byte{nil, {}} {
		data, _ := store.ScaleEncode(v)
		var out []byte
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Empty(t, out)
	}
}

// ── Query 返回值对应的切片类型 ──────────────────────

func TestCodec_SliceOfMember(t *testing.T) {
	v := []gov.Member{
		{Account: store.UniAddr{T: 1, V: []byte("alice")}, Balance: store.Amount{Int: big.NewInt(100)}},
		{Account: store.UniAddr{T: 2, V: []byte("bob")}, Balance: store.Amount{Int: big.NewInt(200)}},
	}
	data, _ := store.ScaleEncode(v)
	var out []gov.Member
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Len(t, out, 2)
	require.Equal(t, int64(100), out[0].Balance.Int.Int64())
}

func TestCodec_SliceOfVote(t *testing.T) {
	v := []gov.Vote{
		{Index: 0, ProposalID: 1, Caller: store.UniAddr{T: 1, V: []byte("alice")},
			Pledge: store.Amount{Int: big.NewInt(100)}, VoteWeight: store.Amount{Int: big.NewInt(50)}, OpinionYes: true},
		{Index: 1, ProposalID: 1, Caller: store.UniAddr{T: 2, V: []byte("bob")},
			Pledge: store.Amount{Int: big.NewInt(200)}, OpinionYes: false},
	}
	testRoundTrip(t, &v)
}

func TestCodec_SliceOfVoteRef(t *testing.T) {
	v := []gov.VoteRef{{ProposalID: 1, VoteIndex: 0}, {ProposalID: 1, VoteIndex: 1}, {ProposalID: 2, VoteIndex: 0}}
	testRoundTrip(t, &v)
}

func TestCodec_SliceOfProposalWithID(t *testing.T) {
	v := []gov.ProposalWithID{
		{ID: 1, Proposal: gov.Proposal{TrackID: 1, SubmitBlock: 100}},
		{ID: 2, Proposal: gov.Proposal{TrackID: 2, SubmitBlock: 200}},
	}
	testRoundTrip(t, &v)
}

func TestCodec_SliceOfTrackWithID(t *testing.T) {
	v := []gov.TrackWithID{
		{ID: 1, Track: gov.TrackData{Name: "root", PreparePeriod: 100}},
		{ID: 2, Track: gov.TrackData{Name: "treasury", PreparePeriod: 200}},
	}
	testRoundTrip(t, &v)
}

func TestCodec_SliceOfSpend(t *testing.T) {
	v := []gov.Spend{
		{ID: 1, Amount: store.Amount{Int: big.NewInt(100)}, Payout: true},
		{ID: 2, Amount: store.Amount{Int: big.NewInt(200)}, Payout: false},
	}
	testRoundTrip(t, &v)
}

func TestCodec_SliceOfModelInfoWithID(t *testing.T) {
	v := []gateway.ModelInfoWithID{
		{ID: 1, Data: gateway.ModelInfo{Name: "gpt-4"}},
		{ID: 2, Data: gateway.ModelInfo{Name: "claude"}},
	}
	testRoundTrip(t, &v)
}

func TestCodec_SliceOfUserInfo(t *testing.T) {
	v := []auth.UserInfo{
		{EmailHash: []byte("a@b.com"), RegisteredAt: 100},
		{EmailHash: []byte("c@d.com"), RegisteredAt: 200},
	}
	testRoundTrip(t, &v)
}

// ===========================================================================
// 数值边界
// ===========================================================================

func TestCodec_Uint32_Boundary(t *testing.T) {
	for _, v := range []uint32{0, 1, math.MaxUint16, math.MaxUint32} {
		data, _ := store.ScaleEncode(v)
		var out uint32
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Equal(t, v, out)
	}
}

func TestCodec_Uint64_Boundary(t *testing.T) {
	for _, v := range []uint64{0, 1, math.MaxUint32, math.MaxUint64} {
		data, _ := store.ScaleEncode(v)
		var out uint64
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Equal(t, v, out)
	}
}

func TestCodec_Int64_Boundary(t *testing.T) {
	for _, v := range []int64{0, -1, 1, math.MinInt64, math.MaxInt64} {
		data, _ := store.ScaleEncode(v)
		var out int64
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Equal(t, v, out)
	}
}

func TestCodec_Bool_Boundary(t *testing.T) {
	for _, v := range []bool{false, true} {
		data, _ := store.ScaleEncode(v)
		var out bool
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Equal(t, v, out)
	}
}

// ===========================================================================
// helper
// ===========================================================================

// testRoundTrip 编码→解码→再编码，对比两次编码结果。
func testRoundTrip[T any](t *testing.T, v *T) {
	t.Helper()
	data1, err := store.ScaleEncode(v)
	require.NoError(t, err)

	var out T
	err = store.ScaleDecode(&out, data1)
	require.NoError(t, err, "decode %T", out)

	data2, err := store.ScaleEncode(&out)
	require.NoError(t, err, "re-encode %T", out)

	if !reflect.DeepEqual(data1, data2) {
		t.Errorf("round-trip mismatch for %T:\n  original: %x\n  decoded:  %x", *v, data1, data2)
		t.Logf("len(data1)=%d len(data2)=%d", len(data1), len(data2))
		if len(data1) >= 14 && len(data2) >= 14 {
			t.Logf("data1[10:14]=%x data2[10:14]=%x", data1[10:14], data2[10:14])
			t.Logf("data1[38:42]=%x data2[38:42]=%x", data1[38:42], data2[38:42])
		}
	}
}

// ===========================================================================
// 随机 fuzz 测试 —— 每个类型 500 次随机往返
// ===========================================================================

func TestFuzz_AllTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuzz in short mode")
	}
	rng := rand.New(rand.NewSource(0xdeadbeef))
	const N = 500

	// gov types
	for i := 0; i < N; i++ {
		testRoundTrip(t, randMember(rng))
		testRoundTrip(t, randTrackData(rng))
		testRoundTrip(t, randTrackWithID(rng))
		testRoundTrip(t, randCallContent(rng))
		testRoundTrip(t, randProposalStatus(rng))
		testRoundTrip(t, randProposalStatusQuery(rng))
		testRoundTrip(t, randProposalDeposit(rng))
		testRoundTrip(t, randProposal(rng))
		testRoundTrip(t, randProposalWithID(rng))
		testRoundTrip(t, randProposalResult(rng))
		testRoundTrip(t, randVote(rng))
		testRoundTrip(t, randSpend(rng))
		testRoundTrip(t, randVoteRef(rng))
		testRoundTrip(t, randCurve(rng))
	}

	// auth types
	for i := 0; i < N; i++ {
		testRoundTrip(t, randUserInfo(rng))
		testRoundTrip(t, randAuthUserRecord(rng))
		testRoundTrip(t, randPrivateKeyRecord(rng))
		testRoundTrip(t, randRecoveryConfig(rng))
		testRoundTrip(t, randRecoveryRequest(rng))
		testRoundTrip(t, randAuthConfig(rng))
	}

	// gateway types
	for i := 0; i < N; i++ {
		testRoundTrip(t, randAPIKeyInfo(rng))
		testRoundTrip(t, randModelInfo(rng))
		testRoundTrip(t, randModelInfoWithID(rng))
		testRoundTrip(t, randModelInfoWithProvider(rng))
		testRoundTrip(t, randProviderInfo(rng))
		testRoundTrip(t, randProviderInfoWithID(rng))
		testRoundTrip(t, randSettlement(rng))
		testRoundTrip(t, randDailyUsage(rng))
		testRoundTrip(t, randRechargeRecord(rng))
		testRoundTrip(t, randUserSettlementRecord(rng))
		testRoundTrip(t, randProviderSettlementRecord(rng))
	}

	// batch types
	for i := 0; i < N; i++ {
		testRoundTrip(t, randBatchCall(rng))
	}

	// store base types
	for i := 0; i < N; i++ {
		testRoundTrip(t, randUniAddr(rng))
		testRoundTrip(t, randU256(rng))
		// randAmount is same as randU256
	}

	// slice fuzz
	for i := 0; i < 200; i++ {
		n := rng.Intn(20)
		members := make([]gov.Member, n)
		for j := 0; j < n; j++ {
			members[j] = *randMember(rng)
		}
		testRoundTrip(t, &members)

		n = rng.Intn(10)
		proposals := make([]gov.ProposalWithID, n)
		for j := 0; j < n; j++ {
			proposals[j] = *randProposalWithID(rng)
		}
		testRoundTrip(t, &proposals)
	}
}

// ── gov random generators ────────────────────────────

func randMember(rng *rand.Rand) *gov.Member {
	return &gov.Member{
		Account: *randUniAddr(rng),
		Balance: store.Amount{Int: randBigInt(rng)},
	}
}

func randTrackData(rng *rand.Rand) *gov.TrackData {
	return &gov.TrackData{
		Name:               randString(rng, 32),
		PreparePeriod:      rng.Uint32(),
		MaxDeciding:        rng.Uint32(),
		ConfirmPeriod:      rng.Uint32(),
		DecisionPeriod:     rng.Uint32(),
		MinEnactmentPeriod: rng.Uint32(),
		DecisionDeposit:    store.Amount{Int: randBigInt(rng)},
		MaxBalance:         store.Amount{Int: randBigInt(rng)},
		MinApproval:        *randCurve(rng),
		MinSupport:         *randCurve(rng),
	}
}

func randTrackWithID(rng *rand.Rand) *gov.TrackWithID {
	return &gov.TrackWithID{ID: rng.Uint32(), Track: *randTrackData(rng)}
}

func randCallContent(rng *rand.Rand) *gov.CallContent {
	n := rng.Intn(5)
	args := make([][]byte, n)
	for j := 0; j < n; j++ {
		args[j] = randBytes(rng, rng.Intn(128))
	}
	var sel [4]byte
	rng.Read(sel[:])
	return &gov.CallContent{
		Contract: randBytes(rng, 32),
		Selector: sel,
		Args:     args,
		Amount:   store.Amount{Int: randBigInt(rng)},
	}
}

func randProposalStatus(rng *rand.Rand) *gov.ProposalStatus {
	states := []uint8{0, 1, 2, 3, 4, 5, 6}
	return &gov.ProposalStatus{State: states[rng.Intn(len(states))], Block: randInt63(rng)}
}

func randProposalStatusQuery(rng *rand.Rand) *gov.ProposalStatusQuery {
	return &gov.ProposalStatusQuery{
		State:              uint8(rng.Intn(7)),
		BlockHeight:        randInt63(rng),
		ConfirmedNumber:    rng.Uint32(),
		LastConfirmedBlock: randInt63(rng),
	}
}

func randProposalDeposit(rng *rand.Rand) *gov.ProposalDeposit {
	return &gov.ProposalDeposit{
		Depositor: *randUniAddr(rng),
		Amount:    store.Amount{Int: randBigInt(rng)},
		Block:     randInt63(rng),
	}
}

func randProposal(rng *rand.Rand) *gov.Proposal {
	p := &gov.Proposal{
		Call:        store.NewNone[gov.CallContent](),
		TrackID:     rng.Uint32(),
		Caller:      *randUniAddr(rng),
		Status:      *randProposalStatus(rng),
		SubmitBlock: randInt63(rng),
		Deposit:     *randProposalDeposit(rng),
	}
	if rng.Intn(2) == 1 {
		p.Call = store.NewSome(*randCallContent(rng))
	}
	return p
}

func randProposalWithID(rng *rand.Rand) *gov.ProposalWithID {
	return &gov.ProposalWithID{ID: rng.Uint32(), Proposal: *randProposal(rng)}
}

func randProposalResult(rng *rand.Rand) *gov.ProposalResult {
	r := &gov.ProposalResult{}
	switch rng.Intn(4) {
	case 0:
		r.Result = store.NewNone[[]byte]()
		r.ExecError = store.NewNone[[]byte]()
	case 1:
		r.Result = store.NewSome(randBytes(rng, 64))
		r.ExecError = store.NewNone[[]byte]()
	case 2:
		r.Result = store.NewNone[[]byte]()
		r.ExecError = store.NewSome(randBytes(rng, 64))
	case 3:
		r.Result = store.NewSome(randBytes(rng, 64))
		r.ExecError = store.NewSome(randBytes(rng, 64))
	}
	return r
}

func randVote(rng *rand.Rand) *gov.Vote {
	return &gov.Vote{
		Index:      rng.Uint32(),
		ProposalID: rng.Uint32(),
		Caller:     *randUniAddr(rng),
		Pledge:     store.Amount{Int: randBigInt(rng)},
		OpinionYes: rng.Intn(2) == 1,
		VoteWeight: store.Amount{Int: randBigInt(rng)},
		VoteBlock:  randInt63(rng),
		Deleted:    rng.Intn(2) == 1,
	}
}

func randSpend(rng *rand.Rand) *gov.Spend {
	return &gov.Spend{
		ID:      rng.Uint64(),
		Caller:  *randUniAddr(rng),
		To:      *randUniAddr(rng),
		Amount:  store.Amount{Int: randBigInt(rng)},
		TrackID: rng.Uint32(),
		Payout:  rng.Intn(2) == 1,
	}
}

func randVoteRef(rng *rand.Rand) *gov.VoteRef {
	return &gov.VoteRef{ProposalID: rng.Uint32(), VoteIndex: rng.Uint32()}
}

func randCurve(rng *rand.Rand) *gov.Curve {
	types := []string{gov.CurveTypeLinearDecreasing, gov.CurveTypeSteppedDecreasing, gov.CurveTypeReciprocal}
	t := types[rng.Intn(len(types))]
	return &gov.Curve{
		Type:              t,
		LinearBegin:       rng.Uint32(),
		LinearEnd:         rng.Uint32(),
		LinearLength:      randInt63(rng),
		StepBegin:         rng.Uint32(),
		StepEnd:           rng.Uint32(),
		StepSize:          rng.Uint32(),
		StepPeriod:        randInt63(rng),
		ReciprocalFactor:  rng.Uint32(),
		ReciprocalXScale:  rng.Uint32(),
		ReciprocalXOffset: randInt63(rng),
		ReciprocalYOffset: randInt63(rng),
	}
}

// ── auth random generators ──────────────────────────

func randUserInfo(rng *rand.Rand) *auth.UserInfo {
	return &auth.UserInfo{
		EmailHash:      randBytes(rng, 32),
		ShareRoot:      randBytes(rng, 32),
		DerivedAddress: randBytes(rng, 20),
		RegisteredAt:   rng.Uint64(),
	}
}

func randAuthUserRecord(rng *rand.Rand) *auth.AuthUserRecord {
	return &auth.AuthUserRecord{
		Ref:        randString(rng, 64),
		Ciphertext: randBytes(rng, rng.Intn(512)),
	}
}

func randPrivateKeyRecord(rng *rand.Rand) *auth.PrivateKeyRecord {
	return &auth.PrivateKeyRecord{
		Ref:        randString(rng, 64),
		Ciphertext: randBytes(rng, rng.Intn(512)),
	}
}

func randRecoveryConfig(rng *rand.Rand) *auth.RecoveryConfig {
	n := rng.Intn(10)
	g := make([][]byte, n)
	for j := 0; j < n; j++ {
		g[j] = randBytes(rng, 32)
	}
	return &auth.RecoveryConfig{
		Guardians:     g,
		Threshold:     rng.Uint32(),
		RecoveryEmail: randBytes(rng, rng.Intn(128)),
	}
}

func randRecoveryRequest(rng *rand.Rand) *auth.RecoveryRequest {
	n := rng.Intn(5)
	a := make([][]byte, n)
	for j := 0; j < n; j++ {
		a[j] = randBytes(rng, 32)
	}
	return &auth.RecoveryRequest{
		EmailHash:    randBytes(rng, 32),
		NewShareRoot: randBytes(rng, 32),
		Approvals:    a,
		ExpiresAt:    rng.Uint64(),
	}
}

func randAuthConfig(rng *rand.Rand) *auth.AuthConfig {
	return &auth.AuthConfig{
		JWTSecret:           randString(rng, 32),
		SeedSecret:          randString(rng, 32),
		SMTPHost:            randString(rng, 64),
		SMTPPort:            rng.Uint32(),
		SMTPUser:            randString(rng, 32),
		SMTPPassword:        randString(rng, 32),
		SMTPFrom:            randString(rng, 64),
		SMTPFromName:        randString(rng, 32),
		MockEmail:           rng.Intn(2) == 1,
		OTPLength:           rng.Uint32(),
		OTPTTLMins:          rng.Uint32(),
		GoogleClientID:      randString(rng, 64),
		GoogleClientSecret:  randString(rng, 64),
		GoogleRedirectURL:   randString(rng, 128),
		GitHubClientID:      randString(rng, 64),
		GitHubClientSecret:  randString(rng, 64),
		GitHubRedirectURL:   randString(rng, 128),
		TwitterClientID:     randString(rng, 64),
		TwitterClientSecret: randString(rng, 64),
		TwitterRedirectURL:  randString(rng, 128),
	}
}

// ── gateway random generators ───────────────────────

func randAPIKeyInfo(rng *rand.Rand) *gateway.APIKeyInfo {
	return &gateway.APIKeyInfo{
		OwnerKey:  rng.Uint64(),
		Owner:     *randUniAddr(rng),
		KeyHash:   randBytes(rng, 32),
		RateLimit: rng.Uint32(),
		IsActive:  rng.Intn(2) == 1,
		CreatedAt: rng.Uint64(),
	}
}

func randModelInfo(rng *rand.Rand) *gateway.ModelInfo {
	n := rng.Intn(5)
	keys := make([]string, n)
	for j := 0; j < n; j++ {
		keys[j] = randString(rng, 48)
	}
	return &gateway.ModelInfo{
		Name:            randString(rng, 32),
		Owner:           *randUniAddr(rng),
		ProviderID:      rng.Uint64(),
		InputModalities: uint8(rng.Intn(256)),
		ContextLength:   rng.Uint64(),
		InPrice:         rng.Uint64(),
		CachedPrice:     rng.Uint64(),
		OutPrice:        rng.Uint64(),
		ProviderType:    uint16(rng.Intn(65536)),
		ModelID:         randString(rng, 32),
		URI:             randString(rng, 128),
		APIKeys:         keys,
		IsActive:        rng.Intn(2) == 1,
		CreatedAt:       rng.Uint64(),
	}
}

func randModelInfoWithID(rng *rand.Rand) *gateway.ModelInfoWithID {
	return &gateway.ModelInfoWithID{ID: rng.Uint64(), Data: *randModelInfo(rng)}
}

func randModelInfoWithProvider(rng *rand.Rand) *gateway.ModelInfoWithProvider {
	return &gateway.ModelInfoWithProvider{
		ID:       rng.Uint64(),
		Model:    *randModelInfo(rng),
		Provider: *randProviderInfo(rng),
	}
}

func randProviderInfo(rng *rand.Rand) *gateway.ProviderInfo {
	n := rng.Intn(10)
	ids := make([]uint64, n)
	for j := 0; j < n; j++ {
		ids[j] = rng.Uint64()
	}
	return &gateway.ProviderInfo{
		Owner:         *randUniAddr(rng),
		Name:          randString(rng, 32),
		LogoURL:       randString(rng, 128),
		Description:   randString(rng, 256),
		ModelIds:      ids,
		Region:        uint8(rng.Intn(256)),
		Deposit:       rng.Uint64(),
		FeeRate:       rng.Uint32(),
		Priority:      rng.Uint32(),
		TotalRequests: rng.Uint64(),
		SuccessRate:   rng.Uint32(),
		AvgLatency:    rng.Uint32(),
		UptimeSLA:     rng.Uint32(),
		IsActive:      rng.Intn(2) == 1,
		CreatedAt:     rng.Uint64(),
	}
}

func randProviderInfoWithID(rng *rand.Rand) *gateway.ProviderInfoWithID {
	return &gateway.ProviderInfoWithID{ID: rng.Uint64(), Data: *randProviderInfo(rng)}
}

func randSettlement(rng *rand.Rand) *gateway.Settlement {
	return &gateway.Settlement{
		User:        *randUniAddr(rng),
		ModelID:     rng.Uint64(),
		ProviderID:  rng.Uint64(),
		TokenIn:     rng.Uint64(),
		TokenOut:    rng.Uint64(),
		CachedToken: rng.Uint64(),
		Count:       uint16(rng.Intn(65536)),
		Nonce:       randBytes(rng, 16),
	}
}

func randDailyUsage(rng *rand.Rand) *gateway.DailyUsage {
	return &gateway.DailyUsage{
		User:        *randUniAddr(rng),
		ProviderID:  rng.Uint64(),
		ModelID:     rng.Uint64(),
		Count:       rng.Uint64(),
		TokenIn:     rng.Uint64(),
		TokenOut:    rng.Uint64(),
		CachedToken: rng.Uint64(),
		PointsUsed:  rng.Uint64(),
	}
}

func randRechargeRecord(rng *rand.Rand) *gateway.RechargeRecord {
	return &gateway.RechargeRecord{
		User:   *randUniAddr(rng),
		Amount: rng.Uint64(),
		Time:   randInt63(rng),
	}
}

func randUserSettlementRecord(rng *rand.Rand) *gateway.UserSettlementRecord {
	return &gateway.UserSettlementRecord{
		ProviderID:  rng.Uint64(),
		ModelID:     rng.Uint64(),
		TokenIn:     rng.Uint64(),
		TokenOut:    rng.Uint64(),
		CachedToken: rng.Uint64(),
		Cost:        rng.Uint64(),
		Count:       uint16(rng.Intn(65536)),
		Time:        randInt63(rng),
	}
}

func randProviderSettlementRecord(rng *rand.Rand) *gateway.ProviderSettlementRecord {
	return &gateway.ProviderSettlementRecord{
		User:        *randUniAddr(rng),
		ModelID:     rng.Uint64(),
		TokenIn:     rng.Uint64(),
		TokenOut:    rng.Uint64(),
		CachedToken: rng.Uint64(),
		Cost:        rng.Uint64(),
		Count:       uint16(rng.Intn(65536)),
		Time:        randInt63(rng),
	}
}

// ── batch random generators ─────────────────────────

func randBatchCall(rng *rand.Rand) *batch.BatchCall {
	n := rng.Intn(8)
	args := make([][]byte, n)
	for j := 0; j < n; j++ {
		args[j] = randBytes(rng, rng.Intn(256))
	}
	return &batch.BatchCall{
		Name:   randBytes(rng, rng.Intn(32)),
		Method: randBytes(rng, 4),
		Args:   args,
	}
}

// ── store random generators ─────────────────────────

func randUniAddr(rng *rand.Rand) *store.UniAddr {
	return &store.UniAddr{
		T: rng.Uint32(),
		V: randBytes(rng, rng.Intn(128)),
	}
}

func randU256(rng *rand.Rand) *store.U256 {
	return &store.U256{Int: randBigInt(rng)}
}

// ── primitive helpers ───────────────────────────────

func randBytes(rng *rand.Rand, n int) []byte {
	if n == 0 {
		return nil
	}
	b := make([]byte, n)
	rng.Read(b)
	return b
}

func randString(rng *rand.Rand, maxLen int) string {
	n := rng.Intn(maxLen + 1)
	if n == 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(32 + rng.Intn(95)) // printable ASCII
	}
	return string(b)
}

func randBigInt(rng *rand.Rand) *big.Int {
	b := make([]byte, 1+rng.Intn(32))
	rng.Read(b)
	b[0] &= 0x7f // ensure non-negative
	return new(big.Int).SetBytes(b)
}

func randInt63(rng *rand.Rand) int64 {
	return int64(rng.Uint64() >> 1)
}
