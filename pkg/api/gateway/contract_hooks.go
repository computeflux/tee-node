package gateway

import (
	"github.com/computeflux/tee-provider/pkg/contractquery"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

// ── Testable function pointers ───────────────────────────

var (
	getBridgeFn        func() *teechain.TeeChain                                   = getBridge
	gatewayGetCreditFn func(*teechain.TeeChain, chaintypes.UniAddr) (int64, error) = contractquery.GatewayGetCredit
)
