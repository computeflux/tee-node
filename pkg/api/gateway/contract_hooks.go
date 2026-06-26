package gateway

import (
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
)

// ── Testable function pointers ───────────────────────────

var (
	findProviderNameFn       func(string) *cgateway.ProviderInfoWithID                   = findProviderName
	findGatewayModelByNameFn func(uint64, string) *cgateway.ModelInfoWithID              = findGatewayModelByName
	getCachedGatewayModelsFn func() []cgateway.ModelInfo                                 = getCachedGatewayModels
	getBridgeFn              func() *teechain.TeeChain                                   = getBridge
	gatewayGetCreditFn       func(*teechain.TeeChain, chaintypes.UniAddr) (int64, error) = GatewayGetCredit
)
