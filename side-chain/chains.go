package sidechain

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/cockroachdb/pebble"
	"github.com/wetee-dao/tee-dsecret/pkg/chains"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

// type ChainState struct {
// 	Chains map[uint32]*model.ChainConfig
// 	chains map[uint32]*chains.ChainApi
// }

const chainKeyPrefix = "chain:"

// chainKey 生成链配置的key
func chainKey(chainId uint32) string {
	return chainKeyPrefix + strconv.FormatUint(uint64(chainId), 10)
}

func (s *SideChain) LoadChains() error {
	// 如果 chains map 未初始化，先初始化
	if s.chains == nil {
		s.chains = make(map[uint32]*chains.ChainApi)
	}

	// 检查 DKG 是否已设置（需要私钥来连接链）
	if s.dkg == nil || s.dkg.Signer == nil {
		return errors.New("DKG not initialized, cannot connect to chains")
	}

	// 使用 GetJsonList 获取所有链配置和对应的 keys
	chainConfigs, keys, err := model.GetJsonList[model.ChainConfigType]("", chainKeyPrefix)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("failed to load chains from database: %w", err)
	}

	// 如果没有配置，直接返回
	if len(chainConfigs) == 0 {
		return nil
	}

	// 遍历所有配置，从 key 中解析 chainId 并连接到每个区块链
	for i, config := range chainConfigs {
		if config == nil {
			continue
		}

		// 从 key 中解析 chainId
		// key格式: "_chain:123" (comboKey 会在前面加下划线)
		key := string(keys[i])
		prefix := "_" + chainKeyPrefix
		if len(key) <= len(prefix) {
			continue
		}
		chainIdStr := key[len(prefix):]
		chainId, err := strconv.ParseUint(chainIdStr, 10, 32)
		if err != nil {
			continue // 跳过无效的key
		}

		// 连接到区块链
		chainApi, err := chains.ConnectChain(config, s.dkg.Signer)
		if err != nil {
			return fmt.Errorf("failed to connect to chain %d: %w", chainId, err)
		}

		// 存储连接结果
		s.chains[uint32(chainId)] = &chainApi
	}

	return nil
}

func (s *SideChain) addChain(chainId uint32, chain *model.ChainConfigType) error {
	// 如果 chains map 未初始化，先初始化
	if s.chains == nil {
		s.chains = make(map[uint32]*chains.ChainApi)
	}

	// 检查配置是否有效
	if chain == nil {
		return errors.New("chain config is nil")
	}

	// 检查 DKG 是否已设置（需要私钥来连接链）
	if s.dkg == nil || s.dkg.Signer == nil {
		return errors.New("DKG not initialized, cannot connect to chains")
	}

	// 连接到区块链
	chainApi, err := chains.ConnectChain(chain, s.dkg.Signer)
	if err != nil {
		return fmt.Errorf("failed to connect to chain %d: %w", chainId, err)
	}

	// 存储连接结果
	s.chains[chainId] = &chainApi

	// 保存配置到数据库（每个链单独存储）
	err = model.SetJson("", chainKey(chainId), chain)
	if err != nil {
		return fmt.Errorf("failed to save chain config to database: %w", err)
	}

	return nil
}

func (s *SideChain) removeChain(chainId uint32) error {
	// 从内存中删除连接
	if s.chains != nil {
		delete(s.chains, chainId)
	}

	// 从数据库删除配置（每个链单独存储）
	err := model.DeleteKey("", chainKey(chainId))
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("failed to delete chain config from database: %w", err)
	}

	return nil
}

func (s *SideChain) getChain(chainId uint32) (chains.ChainApi, error) {
	if chainId == 0 {
		return chains.MainChain, nil
	}

	chain, ok := s.chains[chainId]
	if !ok {
		return nil, errors.New("chain not found")
	}
	return *chain, nil
}
