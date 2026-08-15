package chains

import (
	"errors"
	"testing"

	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ink "github.com/wetee-dao/ink.go"
)

// fakeChain is a minimal ChainApi stub used to exercise factory/registry logic
// without dialing a real substrate node.
type fakeChain struct {
	chainType string
	chainID   uint32
}

func (f *fakeChain) ChainType() string { return f.chainType }
func (f *fakeChain) ChainId() uint32   { return f.chainID }
func (f *fakeChain) GetCipher() crypto.SuiteType {
	return crypto.SuiteEd25519
}
func (f *fakeChain) TEECallToCall(_ *batch.HubCallIndex, _ []byte) (*chaintypes.ChainCall, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeChain) GetLatestNonce() (uint64, error) { return 0, nil }
func (f *fakeChain) GetRelayEvents(_, _ uint64) ([]chaintypes.RelayEvent, error) {
	return nil, nil
}
func (f *fakeChain) GetErc20List() ([]*chaintypes.Erc20Token, error) {
	return nil, nil
}
func (f *fakeChain) BatchCall(_ string, _ []chaintypes.ChainCall) (*chaintypes.ChainCall, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeChain) PartialSign(_ ink.PartialSignerType, _ chaintypes.ChainCall) ([]byte, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeChain) SignAndSubmit(_ ink.SignerType, _ chaintypes.ChainCall, _ bool, _ uint64) error {
	return errors.New("not implemented")
}

func TestConnectMainChain_InvalidEnv(t *testing.T) {
	_, err := ConnectMainChain("does-not-exist", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid environment")
}

func TestConnectMainChain_UnsupportedChainType(t *testing.T) {
	// "evm" 配置当前不支持：connectMainChain 必须返回错误而不是 os.Exit(1)。
	cfg := &chaintypes.ChainConfigType{ChainType: "evm"}
	_, err := connectMainChain(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported chain type")
}

func TestConnectChain_UnsupportedChainType(t *testing.T) {
	// 同样的 os.Exit(1) 问题在 connectChain 中也要避免。
	cfg := &chaintypes.ChainConfigType{ChainType: "evm"}
	_, err := connectChain(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported chain type")
}

func TestGetChainByID(t *testing.T) {
	old := Chains
	t.Cleanup(func() { Chains = old })

	Chains = []ChainApi{
		&fakeChain{chainType: "substrate", chainID: 1},
		&fakeChain{chainType: "substrate", chainID: 7},
	}

	t.Run("found", func(t *testing.T) {
		c, err := GetChainByID(7)
		require.NoError(t, err)
		assert.Equal(t, uint32(7), c.ChainId())
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := GetChainByID(99)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chain not found")
	})
}
