package dkg

import (
	"errors"
	"fmt"
	"sync"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

const (
	// DefaultRandomSharePoolCapacity is the default number of random shares to pre-generate.
	DefaultRandomSharePoolCapacity = 500
	// RandomShareNamespace is the DB namespace for persisting the random share pool.
	RandomShareNamespace = "random_shares"
)

// RandomSharePoolStore is the persistent storage format for the pool.
type RandomSharePoolStore struct {
	Shares     map[uint32]*model.DistKeyShare `json:"shares"`
	TxMappings map[int64]uint32               `json:"tx_mappings"`
}

// RandomSharePool manages a pool of pre-generated random shares for DSS signing.
// Each random share is an independent DistKeyShare generated via DKG.
// The pool ensures that all nodes use the same random share for a given txIndex
// by mapping txIndex -> shareIndex deterministically.
type RandomSharePool struct {
	mu sync.RWMutex

	// shares holds the pre-generated random DistKeyShares keyed by randomIndex.
	shares map[uint32]*model.DistKeyShare

	// txMappings maps a txIndex to the randomIndex used for that tx.
	// This ensures retries for the same tx use the same random share.
	txMappings map[int64]uint32

	// capacity is the target number of shares to maintain.
	capacity int

	// persistKey is the DB key used for persistence (node SS58 address).
	persistKey string
}

// NewRandomSharePool creates a new RandomSharePool and attempts to restore state from DB.
func NewRandomSharePool(signer *model.PrivKey, capacity int) *RandomSharePool {
	if capacity <= 0 {
		capacity = DefaultRandomSharePoolCapacity
	}
	pool := &RandomSharePool{
		capacity:   capacity,
		shares:     make(map[uint32]*model.DistKeyShare),
		txMappings: make(map[int64]uint32),
		persistKey: signer.GetPublic().SS58(),
	}
	_ = pool.load()
	return pool
}

// Acquire returns the random share for the given txIndex.
// The txIndex is used directly as the randomIndex key, ensuring all nodes
// use the same share for the same transaction (provided all nodes have
// generated a share for that index).
func (pool *RandomSharePool) Acquire(txIndex int64) (*model.DistKeyShare, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	// Use txIndex directly as randomIndex for deterministic cross-node alignment.
	idx := uint32(txIndex)

	if share, ok := pool.shares[idx]; ok {
		return share, nil
	}

	return nil, fmt.Errorf("random share for txIndex %d (randomIndex %d) not available", txIndex, idx)
}

// PeekAt returns the share at a specific randomIndex.
func (pool *RandomSharePool) PeekAt(index uint32) (*model.DistKeyShare, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	if share, ok := pool.shares[index]; ok {
		return share, nil
	}
	return nil, errors.New("index not found")
}

// Remaining returns the number of generated shares.
func (pool *RandomSharePool) Remaining() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return len(pool.shares)
}

// IsExhausted returns true if no shares are available.
func (pool *RandomSharePool) IsExhausted() bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return len(pool.shares) == 0
}

// Len returns the total number of shares in the pool.
func (pool *RandomSharePool) Len() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return len(pool.shares)
}

// Capacity returns the target capacity.
func (pool *RandomSharePool) Capacity() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return pool.capacity
}

// AddShare adds a single random share at the given randomIndex.
func (pool *RandomSharePool) AddShare(randomIndex uint32, share *model.DistKeyShare) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.shares == nil {
		pool.shares = make(map[uint32]*model.DistKeyShare)
	}
	pool.shares[randomIndex] = share
	_ = pool.save()
}

// SetShares replaces the pool's shares.
// Used during loading/initialization or after a fresh batch generation.
func (pool *RandomSharePool) SetShares(shares map[uint32]*model.DistKeyShare) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.shares = shares
	pool.txMappings = make(map[int64]uint32)
	_ = pool.save()
}

// TxMapping returns the share index assigned to a txIndex, or ok=false if not mapped.
func (pool *RandomSharePool) TxMapping(txIndex int64) (uint32, bool) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	idx, ok := pool.txMappings[txIndex]
	return idx, ok
}

func (pool *RandomSharePool) save() error {
	store := RandomSharePoolStore{
		Shares:     pool.shares,
		TxMappings: pool.txMappings,
	}
	return model.SetJson(RandomShareNamespace, pool.persistKey, &store)
}

func (pool *RandomSharePool) load() error {
	store, err := model.GetJson[RandomSharePoolStore](RandomShareNamespace, pool.persistKey)
	if err != nil || store == nil {
		// Ignore errors (e.g. old format) and start with an empty pool.
		pool.shares = make(map[uint32]*model.DistKeyShare)
		return nil
	}
	if store.Shares != nil {
		pool.shares = store.Shares
	} else {
		pool.shares = make(map[uint32]*model.DistKeyShare)
	}
	if store.TxMappings != nil {
		pool.txMappings = store.TxMappings
	}
	return nil
}
