package auth

import (
	"errors"

	"github.com/computeflux/tee-provider/pkg/model"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidThreshold  = errors.New("invalid threshold")
)

// Auth 认证合约 — 使用 []byte 作为 key
type Auth struct {
	api model.ContractApi

	users        *model.StoreMapping[[]byte, UserInfo]
	shareRoots   *model.StoreMapping[[]byte, []byte]
	recoveries   *model.StoreMapping[[]byte, RecoveryConfig]
	recoveryReqs *model.StoreMapping[[]byte, RecoveryRequest]

	dkgPubKey  *model.StoreValue[[]byte]
	threshold  *model.StoreValue[uint32]
	totalNodes *model.StoreValue[uint32]
}

type AuthQuery struct{ Auth }
type AuthMutation struct{ Auth }

// ── Init ─────────────────────────────────────────────────

func (a AuthMutation) Init(dkgPubKey []byte, threshold uint32, totalNodes uint32) error {
	a.dkgPubKey.Set(a.api.GetTxn(), dkgPubKey)
	a.threshold.Set(a.api.GetTxn(), threshold)
	return a.totalNodes.Set(a.api.GetTxn(), totalNodes)
}

// ── RegisterUser ─────────────────────────────────────────

func (a AuthMutation) RegisterUser(emailHash []byte, shareRoot []byte, address []byte) error {
	existing, _ := a.users.Get(a.api.GetTxn(), emailHash)
	if existing != nil {
		return ErrUserAlreadyExists
	}
	return a.users.Set(a.api.GetTxn(), emailHash, UserInfo{
		EmailHash:      emailHash,
		ShareRoot:      shareRoot,
		DerivedAddress: address,
		RegisteredAt:   uint64(a.api.GetHeight()),
	})
}

func (a AuthMutation) UpdateShareRoot(emailHash []byte, newRoot []byte) error {
	user, err := a.users.Get(a.api.GetTxn(), emailHash)
	if err != nil || user == nil {
		return ErrUserNotFound
	}
	user.ShareRoot = newRoot
	return a.users.Set(a.api.GetTxn(), emailHash, *user)
}

// ── Recovery ─────────────────────────────────────────────

func (a AuthMutation) SetRecovery(emailHash []byte, cfg RecoveryConfig) error {
	return a.recoveries.Set(a.api.GetTxn(), emailHash, cfg)
}

func (a AuthMutation) RequestRecovery(emailHash []byte, newRoot []byte) error {
	a.recoveries.Get(a.api.GetTxn(), emailHash) // validate exists
	return a.recoveryReqs.Set(a.api.GetTxn(), emailHash, RecoveryRequest{
		EmailHash:    emailHash,
		NewShareRoot: newRoot,
		Approvals:    [][]byte{},
		ExpiresAt:    uint64(a.api.GetHeight()) + 100,
	})
}

func (a AuthMutation) ApproveRecovery(emailHash []byte, guardian []byte) error {
	req, err := a.recoveryReqs.Get(a.api.GetTxn(), emailHash)
	if err != nil || req == nil {
		return ErrUserNotFound
	}
	for _, g := range req.Approvals {
		if bytesEq(g, guardian) {
			return nil
		}
	}
	req.Approvals = append(req.Approvals, guardian)
	return a.recoveryReqs.Set(a.api.GetTxn(), emailHash, *req)
}

func (a AuthMutation) CompleteRecovery(emailHash []byte, newShareRoot []byte) error {
	req, err := a.recoveryReqs.Get(a.api.GetTxn(), emailHash)
	if err != nil || req == nil {
		return ErrUserNotFound
	}
	cfg, err := a.recoveries.Get(a.api.GetTxn(), emailHash)
	if err != nil || cfg == nil {
		return ErrUserNotFound
	}
	if uint32(len(req.Approvals)) < cfg.Threshold {
		return errors.New("insufficient approvals")
	}
	return a.UpdateShareRoot(emailHash, newShareRoot)
}

// ── Query ────────────────────────────────────────────────

func (a AuthQuery) GetUser(emailHash []byte) (*UserInfo, error) {
	return a.users.Get(a.api.GetTxn(), emailHash)
}

func (a AuthQuery) HasKey(emailHash []byte) (bool, error) {
	u, err := a.users.Get(a.api.GetTxn(), emailHash)
	return u != nil && err == nil, err
}

func (a AuthQuery) GetRecovery(emailHash []byte) (*RecoveryConfig, error) {
	return a.recoveries.Get(a.api.GetTxn(), emailHash)
}

func (a AuthQuery) GetDKGPublicKey() ([]byte, error) {
	v, err := a.dkgPubKey.Get(a.api.GetTxn())
	if err != nil {
		return nil, err
	}
	return *v, nil
}

func bytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
