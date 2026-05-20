package teechain

import (
	"errors"

	"github.com/computeflux/tee-provider/tee-chain/relay"
)

// type ChainState struct {
// 	Chains map[uint32]*model.ChainConfig
// 	chains map[uint32]*chains.ChainApi
// }

func (s *TeeChain) LoadChains() error {
	// 检查 DKG 是否已设置（需要私钥来连接链）
	if s.dkg == nil || s.dkg.Signer == nil {
		return errors.New("DKG not initialized, cannot connect to chains")
	}

	// 初始化 relay engine
	s.relay = relay.NewEngine(nil)

	// 启动 relay 后台拉取 worker
	s.startRelayWorker()

	return nil
}
