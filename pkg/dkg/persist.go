package dkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
)

// Restore dkg state
func (dkg *DKG) reState() error {
	v, err := model.GetKey("DKG", dkg.Signer.GetPublicForSuite(model.SuiteEd25519).SS58())
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get dkg: %w", err)
	}
	if len(v) == 0 {
		return nil
	}

	var raw dkgStoreJSON
	if err := json.Unmarshal(v, &raw); err != nil {
		util.LogWithYellow("DKG", "skip restoring old state (format mismatch), reinitializing DKG")
		os.Exit(1)
	}

	dkg.Threshold = raw.Threshold
	dkg.Epoch = raw.Epoch
	dkg.status = raw.Status
	dkg.Nodes = raw.Nodes
	dkg.DkgPubKeys = pubKeysFromStringMap(raw.DkgPubKeys)
	dkg.DkgKeyShares = fromStore(raw.DkgKeyShares)

	if dkg.DkgPubKeys != nil {
		for _, pubKey := range dkg.DkgPubKeys {
			if pubKey != nil {
				util.LogWithCyan("DKG PUBKEY", pubKey.AccountString())
			}
		}
	}

	dkg.NewNodes = raw.NewNodes
	dkg.NewEpoch = raw.NewEpoch
	dkg.NewDkgPubKeys = pubKeysFromStringMap(raw.NewDkgPubKeys)
	dkg.NewDkgKeyShares = fromStore(raw.NewDkgKeyShares)

	return nil
}

func (dkg *DKG) saveState() error {
	dkg.mu.RLock()
	defer dkg.mu.RUnlock()
	raw := dkgStoreJSON{
		Nodes:           dkg.Nodes,
		Threshold:       dkg.Threshold,
		Epoch:           dkg.Epoch,
		DkgPubKeys:      pubKeysToStringMap(dkg.DkgPubKeys),
		DkgKeyShares:    toStore(dkg.DkgKeyShares),
		NewNodes:        dkg.NewNodes,
		NewEpoch:        dkg.NewEpoch,
		NewDkgPubKeys:   pubKeysToStringMap(dkg.NewDkgPubKeys),
		NewDkgKeyShares: toStore(dkg.NewDkgKeyShares),
		Status:          dkg.status,
	}

	bt, err := json.Marshal(&raw)
	if err != nil {
		return fmt.Errorf("marshal dkg state: %w", err)
	}
	return model.SetKey("DKG", dkg.Signer.GetPublicForSuite(model.SuiteEd25519).SS58(), bt)
}

// distKeyShareStore is a JSON-serializable form of DistKeyShare that stores
// the suite type explicitly so that kyber Points/Scalars can be restored with
// the correct suite after a restart.
type distKeyShareStore struct {
	Suite    string                  `json:"suite"`
	Commits  model.KyberPoints       `json:"commits"`
	PriShare model.PriShareWithSuite `json:"priShare"`
}

// toStore converts a suite-typed DistKeyShare map into a storable slice.
func toStore(shares map[model.SuiteType]*model.DistKeyShare) []distKeyShareStore {
	if shares == nil {
		return nil
	}
	out := make([]distKeyShareStore, 0, len(shares))
	for st, dks := range shares {
		if dks == nil {
			continue
		}
		suiteStr := st.String()
		commits := dks.CommitsWrap
		// Ensure the KyberPoints carry the suite string.
		commits.Suite = suiteStr
		out = append(out, distKeyShareStore{
			Suite:    suiteStr,
			Commits:  commits,
			PriShare: dks.PriShareWrap,
		})
	}
	return out
}

// fromStore restores a suite-typed DistKeyShare map from a stored slice.
func fromStore(stored []distKeyShareStore) map[model.SuiteType]*model.DistKeyShare {
	if stored == nil {
		return nil
	}
	out := make(map[model.SuiteType]*model.DistKeyShare, len(stored))
	for _, s := range stored {
		st, err := model.SuiteTypeFromString(s.Suite)
		if err != nil {
			continue
		}
		// Ensure the commits know their suite for point unmarshalling.
		s.Commits.Suite = s.Suite
		out[st] = &model.DistKeyShare{
			CommitsWrap:  s.Commits,
			PriShareWrap: s.PriShare,
		}
	}
	return out
}

// dkgStoreJSON is the JSON-serializable form of DKGStore.
type dkgStoreJSON struct {
	Nodes           []*model.Validator       `json:"nodes"`
	Threshold       int                      `json:"threshold"`
	Epoch           uint32                   `json:"epoch"`
	DkgPubKeys      map[string]*model.PubKey `json:"dkgPubKeys"`
	DkgKeyShares    []distKeyShareStore      `json:"dkgKeyShares"`
	SideKeyPub      types.AccountID          `json:"sideKeyPub"`
	SideKeyShare    []byte                   `json:"sideKeyShare"`
	NewNodes        []*model.Validator       `json:"newNodes"`
	NewEpoch        uint32                   `json:"newEpoch"`
	NewDkgPubKeys   map[string]*model.PubKey `json:"newDkgPubKeys"`
	NewDkgKeyShares []distKeyShareStore      `json:"newDkgKeyShares"`
	NewSideKeyPub   types.AccountID          `json:"newSideKeyPub"`
	NewSideKeyShare []byte                   `json:"newSideKeyShare"`
	Status          uint8                    `json:"status"`
}

func pubKeysToStringMap(m map[model.SuiteType]*model.PubKey) map[string]*model.PubKey {
	if m == nil {
		return nil
	}
	out := make(map[string]*model.PubKey, len(m))
	for st, pk := range m {
		out[st.String()] = pk
	}
	return out
}

func pubKeysFromStringMap(m map[string]*model.PubKey) map[model.SuiteType]*model.PubKey {
	if m == nil {
		return nil
	}
	out := make(map[model.SuiteType]*model.PubKey, len(m))
	for s, pk := range m {
		st, err := model.SuiteTypeFromString(s)
		if err != nil {
			continue
		}
		out[st] = pk
	}
	return out
}
