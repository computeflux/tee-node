package dkg

import "github.com/computeflux/tee-provider/pkg/dkg/types"

// skipPartialSign is a test-only flag that skips chain-level partial signing.
var skipPartialSign = false

// SaveSponsor persists the consensus sponsor and epoch time from a ConsensusMsg.
func (dkg *DKG) SaveSponsor(newMsg *dkgtypes.ConsensusMsg) {
	if newMsg.Sponsor != nil {
		dkg.mu.Lock()
		dkg.NewEpochSponsor = newMsg.Sponsor
		dkg.NewEpochTime = newMsg.EpochTime
		dkg.mu.Unlock()
	}
}
