package dkg

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/sign/schnorr"
)

// RandomDkgPayload wraps a random DKG message so that the randomIndex can be
// multiplexed over the existing DkgMessage channel.
type RandomDkgPayload struct {
	RandomIndex uint32 `json:"random_index"`
	Data        []byte `json:"data"`
}

// randomDkgState holds the in-flight state for generating a single random share.
type randomDkgState struct {
	randomIndex uint32
	generator   *pedersen.DistKeyGenerator
	deals       map[string]*model.DealBundle
	responses   map[string]*pedersen.ResponseBundle
	result      *model.DistKeyShare
	done        bool
	doneCh      chan struct{} // closed when DKG completes (success or failure)
	respReady   bool          // response generated but not yet sent
	respData    []byte        // serialized response
	respSent    bool          // response has been broadcast
	mu          sync.Mutex
}

// randomNonce creates a deterministic nonce for a given randomIndex.
func randomNonce(randomIndex uint32) []byte {
	var nonce [pedersen.NonceLength]byte
	h := sha256.New()
	_, _ = h.Write([]byte("random-dkg-nonce"))
	_, _ = h.Write(fmt.Append(nil, randomIndex))
	copy(nonce[:], h.Sum(nil))
	return nonce[:]
}

// GenerateRandomShares initiates a single batched DKG run that produces
// `count` independent random shares.  All deals are packed into one broadcast
// message and all responses are packed into one broadcast message, so the
// protocol completes in 2 P2P rounds regardless of count.
func (dkg *DKG) GenerateRandomShares(count int) error {
	if count <= 0 {
		return nil
	}

	dkg.mu.RLock()
	nodeCount := len(dkg.Nodes)
	nodes := make([]pedersen.Node, 0, nodeCount)
	for i, p := range dkg.Nodes {
		nodes = append(nodes, pedersen.Node{
			Index:  uint32(i),
			Public: p.ValidatorId.Point(),
		})
	}
	threshold := dkg.Threshold
	dkg.mu.RUnlock()

	if nodeCount == 0 {
		return errors.New("no validators available for random DKG")
	}

	batch := make([]RandomDkgPayload, 0, count)

	dkg.randomDkgMu.Lock()
	if dkg.randomDkgStates == nil {
		dkg.randomDkgStates = make(map[uint32]*randomDkgState)
	}

	for i := 0; i < count; i++ {
		randomIndex := uint32(i)
		if _, exists := dkg.randomDkgStates[randomIndex]; exists {
			continue
		}
		if dkg.RandomPool != nil {
			if _, err := dkg.RandomPool.Acquire(int64(randomIndex)); err == nil {
				continue // already in pool
			}
		}

		conf := pedersen.Config{
			Suite:     dkg.Suite,
			NewNodes:  nodes,
			Threshold: threshold,
			Auth:      schnorr.NewScheme(dkg.Suite),
			FastSync:  true,
			Longterm:  dkg.Signer.Scalar(),
			Nonce:     randomNonce(randomIndex),
			Log:       dkg.log,
		}

		generator, err := pedersen.NewDistKeyHandler(&conf)
		if err != nil {
			util.LogError("RandomDKG", "NewDistKeyHandler error for index", randomIndex, ":", err)
			continue
		}

		state := &randomDkgState{
			randomIndex: randomIndex,
			generator:   generator,
			deals:       make(map[string]*model.DealBundle),
			responses:   make(map[string]*pedersen.ResponseBundle),
			doneCh:      make(chan struct{}),
		}
		dkg.randomDkgStates[randomIndex] = state

		deal, err := generator.Deals()
		if err != nil {
			delete(dkg.randomDkgStates, randomIndex)
			util.LogError("RandomDKG", "Deals error for index", randomIndex, ":", err)
			continue
		}

		dealData, _ := json.Marshal(&model.DealBundle{DealBundle: deal})
		batch = append(batch, RandomDkgPayload{
			RandomIndex: randomIndex,
			Data:        dealData,
		})
	}
	dkg.randomDkgMu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	batchData, _ := json.Marshal(batch)
	return dkg.sendToNode(model.SendToNodes(dkg.NetIds()), &model.DkgMessage{
		Type:    "random_deal_batch",
		Payload: batchData,
	})
}

// StartRandomShareGeneration initiates a batched random DKG that generates
// `count` independent random shares.  The entire batch completes in 2 P2P
// rounds regardless of count because all deals and all responses are packed
// into single broadcast messages.
func (dkg *DKG) StartRandomShareGeneration(count int) {
	if count <= 0 {
		return
	}
	go func() {
		if err := dkg.GenerateRandomShares(count); err != nil {
			util.LogError("RandomDKG", "batch start error:", err)
			return
		}

		// Collect the states we just created so we can wait for them.
		dkg.randomDkgMu.Lock()
		states := make([]*randomDkgState, 0, count)
		for i := 0; i < count; i++ {
			if s, ok := dkg.randomDkgStates[uint32(i)]; ok {
				states = append(states, s)
			}
		}
		dkg.randomDkgMu.Unlock()

		var wg sync.WaitGroup
		for _, state := range states {
			wg.Add(1)
			go func(st *randomDkgState) {
				defer wg.Done()
				if st == nil || st.doneCh == nil {
					return
				}
				select {
				case <-st.doneCh:
				case <-time.After(time.Second * 30):
					util.LogError("RandomDKG", "timeout waiting for random dkg", st.randomIndex)
				}
			}(state)
		}
		wg.Wait()
		util.LogOk("RandomDKG", count, "random shares generation completed")
	}()
}

func (dkg *DKG) cleanupRandomState(randomIndex uint32) {
	dkg.randomDkgMu.Lock()
	state, ok := dkg.randomDkgStates[randomIndex]
	delete(dkg.randomDkgStates, randomIndex)
	dkg.randomDkgMu.Unlock()
	if ok && state != nil && state.doneCh != nil {
		close(state.doneCh)
	}
}

// handleRandomDealBatch processes a batch of random_deal messages.
// It creates missing states, generates our own deals, stores received deals,
// and broadcasts a batch response when enough deals are collected.
func (dkg *DKG) handleRandomDealBatch(from string, payload []byte) error {
	var items []RandomDkgPayload
	if err := json.Unmarshal(payload, &items); err != nil {
		return fmt.Errorf("unmarshal random deal batch: %w", err)
	}

	dkg.mu.RLock()
	nodeCount := len(dkg.Nodes)
	nodes := make([]pedersen.Node, 0, nodeCount)
	for i, p := range dkg.Nodes {
		nodes = append(nodes, pedersen.Node{
			Index:  uint32(i),
			Public: p.ValidatorId.Point(),
		})
	}
	threshold := dkg.Threshold
	dkg.mu.RUnlock()

	if nodeCount == 0 {
		return errors.New("no validators available for random DKG")
	}

	// First pass: ensure states exist for all indices and generate our own deals.
	ourBatch := make([]RandomDkgPayload, 0)
	dkg.randomDkgMu.Lock()
	if dkg.randomDkgStates == nil {
		dkg.randomDkgStates = make(map[uint32]*randomDkgState)
	}
	for _, item := range items {
		if _, exists := dkg.randomDkgStates[item.RandomIndex]; exists {
			continue
		}
		if dkg.RandomPool != nil {
			if _, err := dkg.RandomPool.Acquire(int64(item.RandomIndex)); err == nil {
				continue // already in pool
			}
		}

		conf := pedersen.Config{
			Suite:     dkg.Suite,
			NewNodes:  nodes,
			Threshold: threshold,
			Auth:      schnorr.NewScheme(dkg.Suite),
			FastSync:  true,
			Longterm:  dkg.Signer.Scalar(),
			Nonce:     randomNonce(item.RandomIndex),
			Log:       dkg.log,
		}
		generator, err := pedersen.NewDistKeyHandler(&conf)
		if err != nil {
			continue
		}
		state := &randomDkgState{
			randomIndex: item.RandomIndex,
			generator:   generator,
			deals:       make(map[string]*model.DealBundle),
			responses:   make(map[string]*pedersen.ResponseBundle),
			doneCh:      make(chan struct{}),
		}
		dkg.randomDkgStates[item.RandomIndex] = state

		deal, err := generator.Deals()
		if err == nil {
			dealData, _ := json.Marshal(&model.DealBundle{DealBundle: deal})
			ourBatch = append(ourBatch, RandomDkgPayload{
				RandomIndex: item.RandomIndex,
				Data:        dealData,
			})
		}
	}
	dkg.randomDkgMu.Unlock()

	// Broadcast our deals for newly created states.
	if len(ourBatch) > 0 {
		batchData, _ := json.Marshal(ourBatch)
		_ = dkg.sendToNode(model.SendToNodes(dkg.NetIds()), &model.DkgMessage{
			Type:    "random_deal_batch",
			Payload: batchData,
		})
	}

	// Second pass: store received deals.
	for _, item := range items {
		var dealBundle model.DealBundle
		if err := json.Unmarshal(item.Data, &dealBundle); err != nil {
			continue
		}
		dkg.randomDkgMu.Lock()
		state, exists := dkg.randomDkgStates[item.RandomIndex]
		dkg.randomDkgMu.Unlock()
		if !exists || state == nil {
			continue
		}
		state.mu.Lock()
		if state.done {
			state.mu.Unlock()
			continue
		}
		state.deals[from] = &dealBundle
		state.mu.Unlock()
	}

	// Third pass: check which states have enough deals and generate responses.
	respBatch := make([]RandomDkgPayload, 0)
	dkg.randomDkgMu.Lock()
	states := make([]*randomDkgState, 0, len(dkg.randomDkgStates))
	for _, s := range dkg.randomDkgStates {
		states = append(states, s)
	}
	dkg.randomDkgMu.Unlock()

	for _, state := range states {
		state.mu.Lock()
		if state.done || state.respReady || state.respSent {
			state.mu.Unlock()
			continue
		}
		if len(state.deals) < nodeCount {
			state.mu.Unlock()
			continue
		}

		deals := make([]*pedersen.DealBundle, 0, len(state.deals))
		for _, d := range state.deals {
			deals = append(deals, d.DealBundle)
		}
		resp, err := state.generator.ProcessDeals(deals)
		if err != nil {
			state.mu.Unlock()
			dkg.cleanupRandomState(state.randomIndex)
			continue
		}
		if resp == nil {
			state.mu.Unlock()
			dkg.cleanupRandomState(state.randomIndex)
			continue
		}
		errNum := 0
		for _, r := range resp.Responses {
			if r.Status != pedersen.Success {
				errNum++
			}
		}
		if errNum > 0 {
			state.mu.Unlock()
			dkg.cleanupRandomState(state.randomIndex)
			continue
		}

		respData, _ := json.Marshal(resp)
		state.respReady = true
		state.respData = respData
		respBatch = append(respBatch, RandomDkgPayload{
			RandomIndex: state.randomIndex,
			Data:        respData,
		})
		state.mu.Unlock()
	}

	// Broadcast response batch.
	if len(respBatch) > 0 {
		batchData, _ := json.Marshal(respBatch)
		_ = dkg.sendToNode(model.SendToNodes(dkg.NetIds()), &model.DkgMessage{
			Type:    "random_deal_resp_batch",
			Payload: batchData,
		})
		for _, state := range states {
			state.mu.Lock()
			if state.respReady {
				state.respSent = true
			}
			state.mu.Unlock()
		}
	}

	return nil
}

// handleRandomDealRespBatch processes a batch of random_deal_resp messages.
func (dkg *DKG) handleRandomDealRespBatch(from string, payload []byte) error {
	var items []RandomDkgPayload
	if err := json.Unmarshal(payload, &items); err != nil {
		return fmt.Errorf("unmarshal random deal resp batch: %w", err)
	}

	// Store all responses.
	for _, item := range items {
		dkg.randomDkgMu.Lock()
		state, exists := dkg.randomDkgStates[item.RandomIndex]
		dkg.randomDkgMu.Unlock()
		if !exists || state == nil {
			continue
		}
		var resp pedersen.ResponseBundle
		if err := json.Unmarshal(item.Data, &resp); err != nil {
			continue
		}
		state.mu.Lock()
		if state.done {
			state.mu.Unlock()
			continue
		}
		state.responses[from] = &resp
		state.mu.Unlock()
	}

	// Check which states have enough responses and extract results.
	dkg.randomDkgMu.Lock()
	states := make([]*randomDkgState, 0, len(dkg.randomDkgStates))
	for _, s := range dkg.randomDkgStates {
		states = append(states, s)
	}
	dkg.randomDkgMu.Unlock()

	for _, state := range states {
		state.mu.Lock()
		if state.done {
			state.mu.Unlock()
			continue
		}
		if len(state.responses) < len(dkg.Nodes) {
			state.mu.Unlock()
			continue
		}

		responses := make([]*pedersen.ResponseBundle, 0, len(state.responses))
		for _, r := range state.responses {
			responses = append(responses, r)
		}
		res, justification, err := state.generator.ProcessResponses(responses)
		if err != nil {
			state.mu.Unlock()
			dkg.cleanupRandomState(state.randomIndex)
			continue
		}

		extractResult := func(r *pedersen.Result) {
			if r == nil || r.Key == nil {
				return
			}
			state.result = &model.DistKeyShare{
				CommitsWrap:  model.KyberPoints{Public: r.Key.Commits},
				PriShareWrap: model.PriShare{PriShare: r.Key.Share},
			}
			state.done = true
			if dkg.RandomPool != nil {
				dkg.RandomPool.AddShare(state.randomIndex, state.result)
			}
			if state.doneCh != nil {
				close(state.doneCh)
			}
			dkg.randomDkgMu.Lock()
			delete(dkg.randomDkgStates, state.randomIndex)
			dkg.randomDkgMu.Unlock()
		}

		if res != nil {
			extractResult(res)
			state.mu.Unlock()
			continue
		}
		if justification == nil {
			res2, err := state.generator.ProcessJustifications(nil)
			if err == nil && res2 != nil {
				extractResult(res2)
				state.mu.Unlock()
				continue
			}
		}

		state.mu.Unlock()
		dkg.cleanupRandomState(state.randomIndex)
	}

	return nil
}


