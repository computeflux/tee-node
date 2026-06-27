package teechain

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/computeflux/tee-provider/pkg/domain"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
	cstore "github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

const (
	RuntimeDefaultVersion = uint32(0)
)

// CheckRuntimeV0 启动时校验 genesis.json 包含 app_state.runtime.wasm。
func CheckRuntimeV0() error {
	data, err := os.ReadFile("./token_chain_data/config/genesis.json")
	if err != nil {
		return fmt.Errorf("read genesis.json: %w", err)
	}

	var genesis struct {
		AppState struct {
			Runtime struct {
				Wasm string `json:"wasm"`
			} `json:"runtime"`
		} `json:"app_state"`
	}
	if err := json.Unmarshal(data, &genesis); err != nil {
		return fmt.Errorf("parse genesis.json: %w", err)
	}

	if genesis.AppState.Runtime.Wasm == "" {
		return fmt.Errorf("genesis.json missing app_state.runtime.wasm")
	}

	return nil
}

// InitGenesis 创世初始化：设置 gov 合约 sudo 账户，并将 genesis WASM 通过合约写入存储。
func InitGenesis(genesisWasm []byte, sudoAddr cstore.UniAddr) error {
	if len(genesisWasm) == 0 {
		return fmt.Errorf("genesis app_state.wasm is empty")
	}
	return createGenesis(genesisWasm, sudoAddr)
}

func createGenesis(wasmBytes []byte, sudoAddr cstore.UniAddr) error {
	txn := dbstore.SECRET_DB.NewTransaction()

	// 创世时以 sudo 自身作为 caller，确保 CallbySystem() 为 true
	runtime := contracts.NewRuntime(0, time.Time{}, txn, sudoAddr, sudoAddr)
	gm := gov.GovMutation{Gov: *gov.NewGov(runtime)}

	// 1. 设置 gov 合约的 sudo 账户
	if err := gm.SetSudo(sudoAddr); err != nil {
		txn.Rollback()
		return fmt.Errorf("set sudo: %w", err)
	}

	// 2. 写入创世 WASM
	if err := gm.RuntimeUpgrade(gov.PendingRuntimeUpgrade{
		Version:          RuntimeDefaultVersion,
		ActivationHeight: 0,
		CodeHash:         nil,
		WasmBytes:        wasmBytes,
		Description:      "genesis runtime",
	}); err != nil {
		txn.Rollback()
		return err
	}

	return txn.Commit()
}

// getGovQuery 创建一个 GovQuery 用于读取合约存储（不受 caller 影响）。
func getGovQuery(txn *dbstore.Txn) gov.GovQuery {
	runtime := contracts.NewRuntime(0, time.Time{}, txn, cstore.UniAddr{}, cstore.UniAddr{})
	return gov.GovQuery{Gov: *gov.NewGov(runtime)}
}

// GetCurrentRuntimeVersion 获取当前 runtime 版本（通过合约查询接口）。
func GetCurrentRuntimeVersion(txn *dbstore.Txn) (uint32, error) {
	gq := getGovQuery(txn)
	v, err := gq.RuntimeVersion()
	if err != nil {
		return RuntimeDefaultVersion, err
	}
	if v == nil {
		return RuntimeDefaultVersion, nil
	}
	return *v, nil
}

// GetRuntimeVersionAtHeight 获取指定区块高度使用的运行时版本。
func GetRuntimeVersionAtHeight(txn *dbstore.Txn, height int64) (uint32, error) {
	gq := getGovQuery(txn)

	// 1. 精确匹配
	v, err := gq.RuntimeAtHeight(height)
	if err == nil && v != nil {
		return *v, nil
	}

	// 2. 查找 <= 当前高度 的最新版本
	current, err := GetCurrentRuntimeVersion(txn)
	if err != nil {
		return RuntimeDefaultVersion, err
	}

	for ver := current; ver > 0; ver-- {
		meta, err := gq.RuntimeMeta(ver)
		if err != nil || meta == nil {
			continue
		}
		if height >= int64(meta.ActivationHeight) {
			return ver, nil
		}
	}

	return RuntimeDefaultVersion, nil
}

// GetRuntimeVersionInfo 获取指定版本的元数据（通过合约查询接口）。
func GetRuntimeVersionInfo(txn *dbstore.Txn, version uint32) (*domain.RuntimeVersionInfo, error) {
	gq := getGovQuery(txn)
	meta, err := gq.RuntimeMeta(version)
	if err != nil || meta == nil {
		return nil, err
	}
	return &domain.RuntimeVersionInfo{
		Version:          meta.Version,
		ActivationHeight: int64(meta.ActivationHeight),
		CodeHash:         meta.CodeHash,
		WasmBytes:        meta.WasmBytes,
		Description:      meta.Description,
	}, nil
}
