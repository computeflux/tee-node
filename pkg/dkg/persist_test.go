package dkg

import (
	"encoding/json"
	"testing"

	crypto "github.com/computeflux/tee-provider/pkg/crypto"
	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
)

// makeTestDistKeyShare creates a minimal DistKeyShare for ed25519.
func makeTestDistKeyShare(t *testing.T, index uint32) *dkgtypes.DistKeyShare {
	suite := crypto.NewSuite(crypto.SuiteEd25519)
	scalar := suite.Scalar().Pick(suite.RandomStream())
	point := suite.Point().Mul(scalar, nil)

	return &dkgtypes.DistKeyShare{
		CommitsWrap: dkgtypes.KyberPoints{
			Public: []kyber.Point{point},
			Suite:  "ed25519",
		},
		PriShareWrap: dkgtypes.PriShareWithSuite{
			PriShare: &share.PriShare{
				I: index,
				V: scalar,
			},
			Suite: "ed25519",
		},
	}
}

func TestToStore_FromStore_RoundTrip(t *testing.T) {
	share0 := makeTestDistKeyShare(t, 0)
	share1 := makeTestDistKeyShare(t, 1)

	original := map[crypto.SuiteType]*dkgtypes.DistKeyShare{
		crypto.SuiteEd25519:  share0,
		crypto.SuiteBLS12381: share1,
	}

	stored := toStore(original)
	require.Len(t, stored, 2)

	restored := fromStore(stored)
	require.Len(t, restored, 2)

	// Verify ed25519 share restored correctly
	require.NotNil(t, restored[crypto.SuiteEd25519])
	assert.Equal(t, share0.PriShareWrap.I, restored[crypto.SuiteEd25519].PriShareWrap.I)

	// Verify bls12381 share restored correctly
	require.NotNil(t, restored[crypto.SuiteBLS12381])
	assert.Equal(t, share1.PriShareWrap.I, restored[crypto.SuiteBLS12381].PriShareWrap.I)
}

func TestToStore_NilMap(t *testing.T) {
	stored := toStore(nil)
	assert.Nil(t, stored)
}

func TestFromStore_NilSlice(t *testing.T) {
	restored := fromStore(nil)
	assert.Nil(t, restored)
}

func TestFromStore_UnknownSuite(t *testing.T) {
	stored := []distKeyShareStore{
		{Suite: "unknown_suite", Commits: dkgtypes.KyberPoints{Suite: "unknown_suite"}},
	}
	restored := fromStore(stored)
	assert.Empty(t, restored)
}

func TestFromStore_EmptySlice(t *testing.T) {
	restored := fromStore([]distKeyShareStore{})
	assert.Empty(t, restored)
}

func TestToStore_SkipsNilValues(t *testing.T) {
	share0 := makeTestDistKeyShare(t, 0)
	original := map[crypto.SuiteType]*dkgtypes.DistKeyShare{
		crypto.SuiteEd25519:  share0,
		crypto.SuiteBLS12381: nil,
	}

	stored := toStore(original)
	require.Len(t, stored, 1)
	assert.Equal(t, "ed25519", stored[0].Suite)
}

func TestPubKeysToStringMap_RoundTrip(t *testing.T) {
	pub0 := &crypto.PubKey{}
	pub1 := &crypto.PubKey{}

	original := map[crypto.SuiteType]*crypto.PubKey{
		crypto.SuiteEd25519:  pub0,
		crypto.SuiteBLS12381: pub1,
	}

	strMap := pubKeysToStringMap(original)
	require.Len(t, strMap, 2)
	assert.Equal(t, pub0, strMap["ed25519"])
	assert.Equal(t, pub1, strMap["bls12381"])

	restored := pubKeysFromStringMap(strMap)
	require.Len(t, restored, 2)
	assert.Equal(t, pub0, restored[crypto.SuiteEd25519])
	assert.Equal(t, pub1, restored[crypto.SuiteBLS12381])
}

func TestPubKeysToStringMap_Nil(t *testing.T) {
	assert.Nil(t, pubKeysToStringMap(nil))
}

func TestPubKeysFromStringMap_Nil(t *testing.T) {
	assert.Nil(t, pubKeysFromStringMap(nil))
}

func TestPubKeysFromStringMap_UnknownKey(t *testing.T) {
	m := map[string]*crypto.PubKey{
		"ed25519": {},
		"unknown": {},
	}
	restored := pubKeysFromStringMap(m)
	assert.Len(t, restored, 1)
	assert.Contains(t, restored, crypto.SuiteEd25519)
}

func TestDistKeyShareStore_JSONRoundTrip(t *testing.T) {
	share := makeTestDistKeyShare(t, 5)
	store := distKeyShareStore{
		Suite:    "ed25519",
		Commits:  share.CommitsWrap,
		PriShare: share.PriShareWrap,
	}

	data, err := json.Marshal(store)
	require.NoError(t, err)

	var decoded distKeyShareStore
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, store.Suite, decoded.Suite)
	assert.Equal(t, store.PriShare.I, decoded.PriShare.I)
}
